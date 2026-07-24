/*
 * Copyright 2026 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

const (
	releasePlanCollabSessionKeyPrefix = "release-plan:collab:session:"
	releasePlanCollabPlanSetPrefix    = "release-plan:collab:plan:"
	releasePlanCollabBroadcastChannel = "release-plan-collaboration"
	releasePlanCollabSessionTTL       = 90 * time.Second
	releasePlanCollabWSWriteWait      = 10 * time.Second
	releasePlanCollabWSPongWait       = 60 * time.Second
	releasePlanCollabWSPingPeriod     = releasePlanCollabWSPongWait * 9 / 10
	releasePlanCollabWSReadLimit      = 16 * 1024
	releasePlanCollabRedisRetryWait   = 3 * time.Second
)

const (
	releasePlanCollabSectionMetadata                = "metadata"
	releasePlanCollabSectionMetadataName            = "metadata:name"
	releasePlanCollabSectionMetadataManager         = "metadata:manager"
	releasePlanCollabSectionMetadataTimeRange       = "metadata:time_range"
	releasePlanCollabSectionMetadataScheduleExecute = "metadata:schedule_execute_time"
	releasePlanCollabSectionMetadataDescription     = "metadata:description"
	releasePlanCollabSectionMetadataJiraSprint      = "metadata:jira_sprint_association"
	releasePlanCollabSectionApproval                = "approval"
)

var releasePlanCollabMetadataSectionNames = map[string]string{
	releasePlanCollabSectionMetadata:                "基础信息",
	releasePlanCollabSectionMetadataName:            "名称",
	releasePlanCollabSectionMetadataManager:         "发布负责人",
	releasePlanCollabSectionMetadataTimeRange:       "发布窗口日期",
	releasePlanCollabSectionMetadataScheduleExecute: "定时执行",
	releasePlanCollabSectionMetadataDescription:     "需求关联",
	releasePlanCollabSectionMetadataJiraSprint:      "关联冲刺",
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkReleasePlanCollaborationOrigin,
}

func storeReleasePlanEditingSession(session *ReleasePlanEditingSession, payload string) error {
	redisCache := cache.NewRedisCache(configbase.RedisCommonCacheTokenDB())
	if err := redisCache.Write(releasePlanCollabSessionKey(session.SessionID), payload, releasePlanCollabSessionTTL); err != nil {
		return err
	}
	return redisCache.AddElementsToSet(releasePlanCollabPlanSetKey(session.PlanID), []string{session.SessionID}, releasePlanCollabSessionTTL)
}

type ReleasePlanEditingSession struct {
	PlanID           string `json:"plan_id"`
	SessionID        string `json:"session_id"`
	ConnectionID     string `json:"connection_id,omitempty"`
	UserID           string `json:"user_id"`
	UserName         string `json:"user_name"`
	Account          string `json:"account"`
	IdentityType     string `json:"identity_type,omitempty"`
	SectionKey       string `json:"section_key"`
	SectionType      string `json:"section_type"`
	SectionName      string `json:"section_name"`
	BaseVersion      int64  `json:"base_version"`
	EditingStartedAt int64  `json:"editing_started_at"`
	LastHeartbeatAt  int64  `json:"last_heartbeat_at"`
}

type ReleasePlanCollaborationGroup struct {
	SectionKey  string                       `json:"section_key"`
	SectionType string                       `json:"section_type"`
	SectionName string                       `json:"section_name"`
	Editors     []*ReleasePlanEditingSession `json:"editors"`
}

type ReleasePlanCollaborationSnapshot struct {
	PlanID      string                           `json:"plan_id"`
	PlanVersion int64                            `json:"plan_version"`
	Groups      []*ReleasePlanCollaborationGroup `json:"groups"`
}

type releasePlanCollabWSMessage struct {
	Type        string `json:"type"`
	SessionID   string `json:"session_id,omitempty"`
	SectionKey  string `json:"section_key,omitempty"`
	SectionType string `json:"section_type,omitempty"`
	SectionName string `json:"section_name,omitempty"`
	BaseVersion int64  `json:"base_version,omitempty"`
}

type releasePlanCollabWSOutbound struct {
	Type     string                            `json:"type"`
	Snapshot *ReleasePlanCollaborationSnapshot `json:"snapshot,omitempty"`
	Error    string                            `json:"error,omitempty"`
}

type collaborationClient struct {
	planID string
	id     string
	conn   *websocket.Conn
	send   chan []byte

	sessionMu  sync.Mutex
	sessionIDs map[string]struct{}
}

var collaborationHub = struct {
	sync.RWMutex
	clients map[string]map[*collaborationClient]struct{}
}{
	clients: map[string]map[*collaborationClient]struct{}{},
}

var collaborationLoopOnce sync.Once

func ensureReleasePlanCollaborationLoop() {
	collaborationLoopOnce.Do(func() {
		util.Go(watchReleasePlanCollaborationBroadcasts)
	})
}

func watchReleasePlanCollaborationBroadcasts() {
	for {
		ch, closeFn := cache.NewRedisCache(configbase.RedisCommonCacheTokenDB()).Subscribe(releasePlanCollabBroadcastChannel)
		for msg := range ch {
			if msg == nil {
				continue
			}
			planID := strings.TrimSpace(msg.Payload)
			if planID == "" {
				continue
			}
			broadcastReleasePlanCollaborationSnapshot(planID)
		}
		if err := closeFn(); err != nil {
			log.Warnf("close release plan collaboration redis subscription error: %v", err)
		}
		log.Warnf("release plan collaboration redis subscription closed, retrying in %s", releasePlanCollabRedisRetryWait)
		time.Sleep(releasePlanCollabRedisRetryWait)
	}
}

func releasePlanCollabSessionKey(sessionID string) string {
	return releasePlanCollabSessionKeyPrefix + sessionID
}

func releasePlanCollabPlanSetKey(planID string) string {
	return fmt.Sprintf("%s%s:sessions", releasePlanCollabPlanSetPrefix, planID)
}

func checkReleasePlanCollaborationOrigin(r *http.Request) bool {
	if r == nil {
		return false
	}

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	expectedHost := releasePlanRequestHost(r)
	if expectedHost == "" {
		return false
	}

	originScheme := strings.ToLower(originURL.Scheme)
	if originScheme != "http" && originScheme != "https" {
		return false
	}
	originHost, originPort := splitReleasePlanHostPort(originURL.Host)
	requestHost, requestPort := splitReleasePlanHostPort(expectedHost)
	if originHost == "" || requestHost == "" {
		return false
	}
	if !strings.EqualFold(originHost, requestHost) {
		return false
	}
	if releasePlanEffectivePort(originScheme, originPort) != releasePlanEffectivePort(releasePlanRequestScheme(r), requestPort) {
		return false
	}

	return true
}

func releasePlanRequestScheme(r *http.Request) string {
	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		if idx := strings.Index(forwardedProto, ","); idx >= 0 {
			forwardedProto = forwardedProto[:idx]
		}
		return strings.ToLower(strings.TrimSpace(forwardedProto))
	}
	if r.URL != nil && r.URL.Scheme != "" {
		return strings.ToLower(r.URL.Scheme)
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func releasePlanEffectivePort(scheme, port string) string {
	if port != "" {
		return port
	}
	if scheme == "https" {
		return "443"
	}
	return "80"
}

func normalizeReleasePlanCollaborationSection(sectionKey, sectionType, sectionName string) (string, string, string) {
	sectionKey = strings.TrimSpace(sectionKey)
	sectionType = strings.TrimSpace(sectionType)
	sectionName = strings.TrimSpace(sectionName)

	switch {
	case sectionType == "metadata" || sectionKey == releasePlanCollabSectionMetadata || strings.HasPrefix(sectionKey, releasePlanCollabSectionMetadata+":"):
		normalizedKey, normalizedName := normalizeReleasePlanMetadataCollaborationSection(sectionKey, sectionName)
		return normalizedKey, "metadata", normalizedName
	case sectionType == "approval" || sectionKey == releasePlanCollabSectionApproval:
		if sectionKey == "" {
			sectionKey = releasePlanCollabSectionApproval
		}
		if sectionName == "" {
			sectionName = "审批配置"
		}
		return sectionKey, "approval", sectionName
	case sectionType == "job":
		if sectionName == "" {
			sectionName = "发布内容"
		}
		return sectionKey, "job", sectionName
	default:
		return sectionKey, sectionType, sectionName
	}
}

func normalizeReleasePlanMetadataCollaborationSection(sectionKey, sectionName string) (string, string) {
	if sectionKey != releasePlanCollabSectionMetadata {
		if normalizedName, exists := releasePlanCollabMetadataSectionNames[sectionKey]; exists {
			return sectionKey, normalizedName
		}
	}

	switch strings.TrimSpace(sectionName) {
	case "", "基础信息":
		return releasePlanCollabSectionMetadata, releasePlanCollabMetadataSectionNames[releasePlanCollabSectionMetadata]
	case "名称", "发布计划名称":
		return releasePlanCollabSectionMetadataName, releasePlanCollabMetadataSectionNames[releasePlanCollabSectionMetadataName]
	case "负责人", "发布负责人":
		return releasePlanCollabSectionMetadataManager, releasePlanCollabMetadataSectionNames[releasePlanCollabSectionMetadataManager]
	case "发布窗口日期", "发布窗口", "发布时间窗口":
		return releasePlanCollabSectionMetadataTimeRange, releasePlanCollabMetadataSectionNames[releasePlanCollabSectionMetadataTimeRange]
	case "定时执行":
		return releasePlanCollabSectionMetadataScheduleExecute, releasePlanCollabMetadataSectionNames[releasePlanCollabSectionMetadataScheduleExecute]
	case "需求关联":
		return releasePlanCollabSectionMetadataDescription, releasePlanCollabMetadataSectionNames[releasePlanCollabSectionMetadataDescription]
	case "关联冲刺", "Jira Sprint", "Jira Sprint 关联":
		return releasePlanCollabSectionMetadataJiraSprint, releasePlanCollabMetadataSectionNames[releasePlanCollabSectionMetadataJiraSprint]
	}

	if normalizedName, exists := releasePlanCollabMetadataSectionNames[sectionKey]; exists {
		return sectionKey, normalizedName
	}

	if sectionKey == "" {
		sectionKey = releasePlanCollabSectionMetadata
	}
	return sectionKey, sectionName
}

func releasePlanRequestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		if idx := strings.Index(forwardedHost, ","); idx >= 0 {
			forwardedHost = forwardedHost[:idx]
		}
		return strings.TrimSpace(forwardedHost)
	}
	return strings.TrimSpace(r.Host)
}

func splitReleasePlanHostPort(rawHost string) (string, string) {
	rawHost = strings.TrimSpace(rawHost)
	if rawHost == "" {
		return "", ""
	}

	if host, port, err := net.SplitHostPort(rawHost); err == nil {
		return strings.ToLower(host), port
	}

	parsed := &url.URL{Host: rawHost}
	return strings.ToLower(parsed.Hostname()), parsed.Port()
}

func broadcastReleasePlanCollaboration(planID string) error {
	if planID == "" {
		return nil
	}
	return cache.NewRedisCache(configbase.RedisCommonCacheTokenDB()).Publish(releasePlanCollabBroadcastChannel, planID)
}

func registerCollaborationClient(planID string, client *collaborationClient) {
	collaborationHub.Lock()
	defer collaborationHub.Unlock()

	if _, exists := collaborationHub.clients[planID]; !exists {
		collaborationHub.clients[planID] = make(map[*collaborationClient]struct{})
	}
	collaborationHub.clients[planID][client] = struct{}{}
}

func unregisterCollaborationClient(planID string, client *collaborationClient) {
	collaborationHub.Lock()
	defer collaborationHub.Unlock()

	if _, exists := collaborationHub.clients[planID]; !exists {
		return
	}
	delete(collaborationHub.clients[planID], client)
	if len(collaborationHub.clients[planID]) == 0 {
		delete(collaborationHub.clients, planID)
	}
}

func rememberCollaborationClientSession(client *collaborationClient, sessionID string) {
	if client == nil || sessionID == "" {
		return
	}

	client.sessionMu.Lock()
	defer client.sessionMu.Unlock()

	if client.sessionIDs == nil {
		client.sessionIDs = make(map[string]struct{})
	}
	client.sessionIDs[sessionID] = struct{}{}
}

func forgetCollaborationClientSession(client *collaborationClient, sessionID string) {
	if client == nil || sessionID == "" {
		return
	}

	client.sessionMu.Lock()
	defer client.sessionMu.Unlock()

	delete(client.sessionIDs, sessionID)
}

func listCollaborationClientSessionIDs(client *collaborationClient) []string {
	if client == nil {
		return nil
	}

	client.sessionMu.Lock()
	defer client.sessionMu.Unlock()

	resp := make([]string, 0, len(client.sessionIDs))
	for sessionID := range client.sessionIDs {
		resp = append(resp, sessionID)
	}
	sort.Strings(resp)
	return resp
}

func cleanupReleasePlanEditingSessionsForClient(client *collaborationClient) {
	if client == nil || client.planID == "" {
		return
	}

	for _, sessionID := range listCollaborationClientSessionIDs(client) {
		session, err := getReleasePlanEditingSession(client.planID, sessionID)
		if err != nil {
			continue
		}
		if session == nil || client.id == "" || session.ConnectionID != client.id {
			continue
		}
		if err := removeReleasePlanEditingSession(client.planID, sessionID); err != nil {
			log.Errorf("remove release plan editing session on disconnect error: %v", err)
			continue
		}
		forgetCollaborationClientSession(client, sessionID)
	}
}

func sendSnapshotToLocalClients(planID string, snapshot *ReleasePlanCollaborationSnapshot) {
	if snapshot == nil {
		return
	}
	payload, err := json.Marshal(&releasePlanCollabWSOutbound{
		Type:     "snapshot",
		Snapshot: snapshot,
	})
	if err != nil {
		return
	}

	collaborationHub.RLock()
	clients := make([]*collaborationClient, 0, len(collaborationHub.clients[planID]))
	for client := range collaborationHub.clients[planID] {
		clients = append(clients, client)
	}
	collaborationHub.RUnlock()

	for _, client := range clients {
		select {
		case client.send <- payload:
		default:
			_ = client.conn.Close()
		}
	}
}

func queueCollaborationClientMessage(client *collaborationClient, outbound *releasePlanCollabWSOutbound) {
	if client == nil || outbound == nil {
		return
	}
	payload, err := json.Marshal(outbound)
	if err != nil {
		return
	}
	select {
	case client.send <- payload:
	default:
	}
}

func setupReleasePlanCollaborationWSDeadline(ws *websocket.Conn) {
	ws.SetReadLimit(releasePlanCollabWSReadLimit)
	_ = ws.SetReadDeadline(time.Now().Add(releasePlanCollabWSPongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(releasePlanCollabWSPongWait))
	})
}

func writeReleasePlanCollaborationWSMessage(ws *websocket.Conn, messageType int, payload []byte) error {
	if err := ws.SetWriteDeadline(time.Now().Add(releasePlanCollabWSWriteWait)); err != nil {
		return err
	}
	return ws.WriteMessage(messageType, payload)
}

func broadcastReleasePlanCollaborationSnapshot(planID string) {
	snapshot, err := GetReleasePlanCollaborationSnapshot(planID)
	if err != nil {
		log.Errorf("get release plan collaboration snapshot error: %v", err)
		return
	}
	sendSnapshotToLocalClients(planID, snapshot)
}

func GetReleasePlanCollaborationSnapshot(planID string) (*ReleasePlanCollaborationSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		return nil, errors.Wrap(err, "get plan")
	}

	editors, err := listActiveReleasePlanEditingSessions(planID)
	if err != nil {
		return nil, err
	}

	groupMap := map[string]*ReleasePlanCollaborationGroup{}
	groupOrder := make([]string, 0)
	for _, session := range editors {
		key := session.SectionKey
		group, exists := groupMap[key]
		if !exists {
			group = &ReleasePlanCollaborationGroup{
				SectionKey:  session.SectionKey,
				SectionType: session.SectionType,
				SectionName: session.SectionName,
				Editors:     make([]*ReleasePlanEditingSession, 0),
			}
			groupMap[key] = group
			groupOrder = append(groupOrder, key)
		}
		displaySession := *session
		displaySession.ConnectionID = ""
		group.Editors = append(group.Editors, &displaySession)
	}

	sort.Strings(groupOrder)
	resp := make([]*ReleasePlanCollaborationGroup, 0, len(groupOrder))
	for _, key := range groupOrder {
		resp = append(resp, groupMap[key])
	}

	return &ReleasePlanCollaborationSnapshot{
		PlanID:      planID,
		PlanVersion: plan.Version,
		Groups:      resp,
	}, nil
}

func listActiveReleasePlanEditingSessions(planID string) ([]*ReleasePlanEditingSession, error) {
	redisCache := cache.NewRedisCache(configbase.RedisCommonCacheTokenDB())
	sessionIDs, err := redisCache.ListSetMembers(releasePlanCollabPlanSetKey(planID))
	if err != nil {
		return nil, err
	}
	if len(sessionIDs) == 0 {
		return []*ReleasePlanEditingSession{}, nil
	}

	keys := make([]string, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		keys = append(keys, releasePlanCollabSessionKey(sessionID))
	}

	values, err := redisCache.MGet(keys)
	if err != nil {
		return nil, err
	}

	resp := decodeReleasePlanEditingSessions(planID, values)

	sort.Slice(resp, func(i, j int) bool {
		if resp[i].SectionKey == resp[j].SectionKey {
			return resp[i].EditingStartedAt < resp[j].EditingStartedAt
		}
		return resp[i].SectionKey < resp[j].SectionKey
	})

	return resp, nil
}

func decodeReleasePlanEditingSessions(planID string, values []interface{}) []*ReleasePlanEditingSession {
	resp := make([]*ReleasePlanEditingSession, 0, len(values))
	for _, value := range values {
		raw, ok := value.(string)
		if !ok || raw == "" {
			continue
		}
		session := new(ReleasePlanEditingSession)
		if err := json.Unmarshal([]byte(raw), session); err != nil {
			continue
		}
		if session.PlanID != planID {
			continue
		}
		session.SectionKey, session.SectionType, session.SectionName = normalizeReleasePlanCollaborationSection(session.SectionKey, session.SectionType, session.SectionName)
		resp = append(resp, session)
	}
	return resp
}

func persistReleasePlanEditingSession(session *ReleasePlanEditingSession) error {
	return saveReleasePlanEditingSession(session, true)
}

func refreshReleasePlanEditingSession(session *ReleasePlanEditingSession) error {
	return saveReleasePlanEditingSession(session, false)
}

func saveReleasePlanEditingSession(session *ReleasePlanEditingSession, broadcast bool) error {
	if session == nil {
		return errors.New("nil editing session")
	}
	if session.PlanID == "" || session.SessionID == "" {
		return errors.New("missing session id or plan id")
	}
	if session.EditingStartedAt == 0 {
		session.EditingStartedAt = time.Now().Unix()
	}
	session.LastHeartbeatAt = time.Now().Unix()

	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}

	if err := storeReleasePlanEditingSession(session, string(payload)); err != nil {
		return err
	}
	if !broadcast {
		return nil
	}
	return broadcastReleasePlanCollaboration(session.PlanID)
}

func removeReleasePlanEditingSession(planID, sessionID string) error {
	redisCache := cache.NewRedisCache(configbase.RedisCommonCacheTokenDB())
	if err := redisCache.Delete(releasePlanCollabSessionKey(sessionID)); err != nil {
		return err
	}
	if err := redisCache.RemoveElementsFromSet(releasePlanCollabPlanSetKey(planID), []string{sessionID}); err != nil {
		return err
	}
	return broadcastReleasePlanCollaboration(planID)
}

func authorizeReleasePlanEditing(ctx *handler.Context, sectionType string) bool {
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	switch sectionType {
	case "metadata":
		return ctx.Resources.SystemActions.ReleasePlan.EditMetadata
	case "approval":
		return ctx.Resources.SystemActions.ReleasePlan.EditApproval
	case "job":
		return ctx.Resources.SystemActions.ReleasePlan.EditSubtasks
	default:
		return false
	}
}

func validateReleasePlanEditingPlan(plan *models.ReleasePlan) error {
	if plan == nil {
		return errors.New("nil plan")
	}
	if plan.Status != config.ReleasePlanStatusPlanning {
		return errors.Errorf("plan status is %s, can not edit", plan.Status)
	}
	return nil
}

func getReleasePlanEditingSession(planID, sessionID string) (*ReleasePlanEditingSession, error) {
	if sessionID == "" {
		return nil, errors.New("empty session id")
	}
	value, err := cache.NewRedisCache(configbase.RedisCommonCacheTokenDB()).GetString(releasePlanCollabSessionKey(sessionID))
	if err != nil {
		return nil, err
	}
	session := new(ReleasePlanEditingSession)
	if err := json.Unmarshal([]byte(value), session); err != nil {
		return nil, err
	}
	if session.PlanID != planID {
		return nil, errors.New("session does not belong to current plan")
	}
	session.SectionKey, session.SectionType, session.SectionName = normalizeReleasePlanCollaborationSection(session.SectionKey, session.SectionType, session.SectionName)
	return session, nil
}

func canManageReleasePlanEditingSession(session *ReleasePlanEditingSession, userID string, isSystemAdmin bool) bool {
	if isSystemAdmin {
		return true
	}
	if session == nil || userID == "" {
		return false
	}
	return session.UserID == userID
}

func OpenReleasePlanCollaborationWS(gCtx *gin.Context, ctx *handler.Context, planID string) error {
	ws, err := upgrader.Upgrade(gCtx.Writer, gCtx.Request, nil)
	if err != nil {
		return e.ErrInvalidParam.AddErr(err)
	}
	var closeWSOnce sync.Once
	closeWS := func() {
		_ = ws.Close()
	}
	defer closeWSOnce.Do(closeWS)
	setupReleasePlanCollaborationWSDeadline(ws)

	ensureReleasePlanCollaborationLoop()

	client := &collaborationClient{
		planID:     planID,
		id:         uuid.NewString(),
		conn:       ws,
		send:       make(chan []byte, 16),
		sessionIDs: map[string]struct{}{},
	}
	registerCollaborationClient(planID, client)
	defer cleanupReleasePlanEditingSessionsForClient(client)
	defer unregisterCollaborationClient(planID, client)

	done := make(chan struct{})
	util.Go(func() {
		defer close(done)
		for {
			_, payload, err := ws.ReadMessage()
			if err != nil {
				return
			}

			msg := new(releasePlanCollabWSMessage)
			if err := json.Unmarshal(payload, msg); err != nil {
				continue
			}

			switch msg.Type {
			case "join", "focus_section":
				sectionKey, sectionType, sectionName := normalizeReleasePlanCollaborationSection(msg.SectionKey, msg.SectionType, msg.SectionName)
				if !authorizeReleasePlanEditing(ctx, sectionType) {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "error", Error: "permission denied"})
					continue
				}
				plan, err := mongodb.NewReleasePlanColl().GetByID(context.Background(), planID)
				if err != nil {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "error", Error: err.Error()})
					continue
				}
				if err := validateReleasePlanEditingPlan(plan); err != nil {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "error", Error: err.Error()})
					continue
				}
				existingSession, _ := getReleasePlanEditingSession(planID, msg.SessionID)
				if existingSession != nil && !canManageReleasePlanEditingSession(existingSession, ctx.UserID, ctx.Resources != nil && ctx.Resources.IsSystemAdmin) {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "error", Error: "permission denied"})
					continue
				}
				session := &ReleasePlanEditingSession{
					PlanID:           planID,
					SessionID:        msg.SessionID,
					ConnectionID:     client.id,
					UserID:           ctx.UserID,
					UserName:         ctx.UserName,
					Account:          ctx.Account,
					IdentityType:     ctx.IdentityType,
					SectionKey:       sectionKey,
					SectionType:      sectionType,
					SectionName:      sectionName,
					BaseVersion:      msg.BaseVersion,
					EditingStartedAt: time.Now().Unix(),
				}
				if existingSession != nil {
					session.EditingStartedAt = existingSession.EditingStartedAt
					if session.BaseVersion == 0 {
						session.BaseVersion = existingSession.BaseVersion
					}
					if existingSession.SectionKey != "" && existingSession.SectionKey != sectionKey {
						session.EditingStartedAt = time.Now().Unix()
						session.BaseVersion = 0
					}
				}
				if session.BaseVersion == 0 {
					session.BaseVersion = plan.Version
				}
				if err := persistReleasePlanEditingSession(session); err != nil {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "error", Error: err.Error()})
					continue
				}
				rememberCollaborationClientSession(client, msg.SessionID)
				snapshot, err := GetReleasePlanCollaborationSnapshot(planID)
				if err == nil {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "snapshot", Snapshot: snapshot})
				}
			case "heartbeat":
				session, err := getReleasePlanEditingSession(planID, msg.SessionID)
				if err != nil {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "error", Error: err.Error()})
					continue
				}
				if !authorizeReleasePlanEditing(ctx, session.SectionType) {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "error", Error: "permission denied"})
					continue
				}
				if !canManageReleasePlanEditingSession(session, ctx.UserID, ctx.Resources != nil && ctx.Resources.IsSystemAdmin) {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "error", Error: "permission denied"})
					continue
				}
				if err := refreshReleasePlanEditingSession(session); err != nil {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "error", Error: err.Error()})
					continue
				}
			case "leave":
				session, err := getReleasePlanEditingSession(planID, msg.SessionID)
				if err != nil {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "error", Error: err.Error()})
					continue
				}
				if !canManageReleasePlanEditingSession(session, ctx.UserID, ctx.Resources != nil && ctx.Resources.IsSystemAdmin) {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "error", Error: "permission denied"})
					continue
				}
				if err := removeReleasePlanEditingSession(planID, msg.SessionID); err != nil {
					queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "error", Error: err.Error()})
					continue
				}
				forgetCollaborationClientSession(client, msg.SessionID)
			}
		}
	})

	util.Go(func() {
		ticker := time.NewTicker(releasePlanCollabWSPingPeriod)
		defer ticker.Stop()
		defer closeWSOnce.Do(closeWS)
		for {
			select {
			case payload := <-client.send:
				if err := writeReleasePlanCollaborationWSMessage(ws, websocket.TextMessage, payload); err != nil {
					return
				}
			case <-ticker.C:
				if err := writeReleasePlanCollaborationWSMessage(ws, websocket.PingMessage, nil); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	})

	snapshot, err := GetReleasePlanCollaborationSnapshot(planID)
	if err == nil {
		queueCollaborationClientMessage(client, &releasePlanCollabWSOutbound{Type: "snapshot", Snapshot: snapshot})
	}

	<-done
	return nil
}
