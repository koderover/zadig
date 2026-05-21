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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

const (
	releasePlanHashPruneMinMapKeys    = 4
	releasePlanHashPruneMinArrayItems = 4
	releasePlanDiffChangeTypeOrder    = "order_changed"
)

type ReleasePlanVersionDiffResponse struct {
	PlanID          string                         `json:"plan_id"`
	Version         int64                          `json:"version"`
	PreviousVersion int64                          `json:"previous_version"`
	Groups          []*ReleasePlanVersionDiffGroup `json:"groups"`
}

type ReleasePlanVersionDiffGroup struct {
	GroupKey  string                          `json:"group_key"`
	GroupName string                          `json:"group_name"`
	GroupType string                          `json:"group_type"`
	Changes   []*ReleasePlanVersionDiffChange `json:"changes"`
}

type ReleasePlanVersionDiffOrderItem struct {
	Key  string `json:"key,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type ReleasePlanVersionDiffChange struct {
	TaskName    string                             `json:"task_name,omitempty"`
	TaskType    string                             `json:"task_type,omitempty"`
	ChangeType  string                             `json:"change_type,omitempty"`
	Path        string                             `json:"path"`
	Label       string                             `json:"label"`
	Before      interface{}                        `json:"before,omitempty"`
	After       interface{}                        `json:"after,omitempty"`
	BeforeOrder []*ReleasePlanVersionDiffOrderItem `json:"before_order,omitempty"`
	AfterOrder  []*ReleasePlanVersionDiffOrderItem `json:"after_order,omitempty"`
	LargeText   bool                               `json:"large_text,omitempty"`
	Masked      bool                               `json:"masked,omitempty"`
}

type releasePlanRawDiffEntry struct {
	Path        string
	ChangeType  string
	Before      interface{}
	After       interface{}
	BeforeOrder []*ReleasePlanVersionDiffOrderItem
	AfterOrder  []*ReleasePlanVersionDiffOrderItem
}

type releasePlanDiffContext struct {
	GroupType string
}

type releasePlanArrayDiffStrategy int

const (
	releasePlanArrayDiffStrategyIndex releasePlanArrayDiffStrategy = iota
	releasePlanArrayDiffStrategyKeyedUnordered
	releasePlanArrayDiffStrategyKeyedOrdered
)

var releasePlanFieldLabels = map[string]string{
	"name":                       "名称",
	"manager":                    "负责人",
	"manager_id":                 "负责人 ID",
	"start_time":                 "开始时间",
	"end_time":                   "结束时间",
	"schedule_execute_time":      "定时执行时间",
	"description":                "需求关联",
	"approval":                   "审批配置",
	"type":                       "类型",
	"enabled":                    "是否启用",
	"content":                    "内容",
	"remark":                     "备注",
	"branch":                     "代码分支",
	"tag":                        "Tag",
	"pr":                         "PR",
	"repo_name":                  "仓库名称",
	"repo_namespace":             "仓库命名空间",
	"remote_name":                "远端名称",
	"job_name":                   "任务名称",
	"build_name":                 "构建名称",
	"service_name":               "服务名称",
	"service_module":             "服务组件",
	"image":                      "镜像",
	"image_name":                 "镜像名称",
	"namespace":                  "命名空间",
	"env":                        "环境",
	"cluster_id":                 "集群",
	"cluster_source":             "集群来源",
	"target":                     "目标",
	"targets":                    "目标列表",
	"key_vals":                   "变量",
	"key":                        "变量名",
	"value":                      "变量值",
	"order":                      "顺序",
	"params":                     "参数",
	"stages":                     "阶段",
	"jobs":                       "任务",
	"script":                     "脚本内容",
	"sql":                        "SQL 内容",
	"manual_exec_users":          "人工执行用户",
	"approve_users":              "审批人",
	"approval_nodes":             "审批节点",
	"services":                   "服务",
	"service_and_builds":         "构建对象",
	"default_service_and_builds": "默认构建对象",
	"repos":                      "代码仓库",
	"workflow":                   "工作流",
	"native_approval":            "原生审批",
	"lark_approval":              "飞书审批",
	"dingtalk_approval":          "钉钉审批",
	"workwx_approval":            "企业微信审批",
}

func GetReleasePlanVersionDiff(planID string, version int64) (*ReleasePlanVersionDiffResponse, error) {
	current, err := mongodb.NewReleasePlanVersionColl().Get(planID, version)
	if err != nil {
		return nil, errors.Wrap(err, "get version")
	}

	fromData, err := toGenericValue(current.BaseSnapshot)
	if err != nil {
		return nil, errors.Wrap(err, "convert base snapshot")
	}
	toData, err := toGenericValue(current.Snapshot)
	if err != nil {
		return nil, errors.Wrap(err, "convert current snapshot")
	}

	groupKey, groupName, groupType := releasePlanVersionDiffGroup(current.SectionKey, current.SectionName)

	rawEntries := make([]*releasePlanRawDiffEntry, 0)
	diffReleasePlanValues(releasePlanDiffContext{GroupType: groupType}, "", fromData, toData, &rawEntries)

	groupMap := map[string]*ReleasePlanVersionDiffGroup{}
	groupOrder := make([]string, 0)
	for _, entry := range rawEntries {
		if shouldIgnoreReleasePlanDiffPath(entry.Path) {
			continue
		}
		taskName, taskType := classifyReleasePlanDiffTask(entry.Path)
		group, exists := groupMap[groupKey]
		if !exists {
			group = &ReleasePlanVersionDiffGroup{
				GroupKey:  groupKey,
				GroupName: groupName,
				GroupType: groupType,
				Changes:   make([]*ReleasePlanVersionDiffChange, 0),
			}
			groupMap[groupKey] = group
			groupOrder = append(groupOrder, groupKey)
		}

		change := &ReleasePlanVersionDiffChange{
			TaskName:   taskName,
			TaskType:   taskType,
			ChangeType: entry.ChangeType,
			Path:       entry.Path,
			Label:      buildReleasePlanDiffLabel(entry.Path),
		}
		if entry.ChangeType == releasePlanDiffChangeTypeOrder {
			change.BeforeOrder = entry.BeforeOrder
			change.AfterOrder = entry.AfterOrder
		} else if isMaskedReleasePlanDiffValue(entry.Before) || isMaskedReleasePlanDiffValue(entry.After) {
			change.Masked = true
		} else if isLargeTextReleasePlanDiffPath(entry.Path, entry.Before, entry.After) {
			change.LargeText = true
		} else {
			change.Before = normalizeReleasePlanDiffValue(entry.Before)
			change.After = normalizeReleasePlanDiffValue(entry.After)
		}
		group.Changes = append(group.Changes, change)
	}

	sort.Strings(groupOrder)
	groups := make([]*ReleasePlanVersionDiffGroup, 0, len(groupOrder))
	for _, key := range groupOrder {
		group := groupMap[key]
		sort.Slice(group.Changes, func(i, j int) bool {
			return group.Changes[i].Path < group.Changes[j].Path
		})
		groups = append(groups, group)
	}

	return &ReleasePlanVersionDiffResponse{
		PlanID:          planID,
		Version:         version,
		PreviousVersion: previousReleasePlanVersion(version),
		Groups:          groups,
	}, nil
}

func previousReleasePlanVersion(version int64) int64 {
	if version <= 1 {
		return 0
	}
	return version - 1
}

func toGenericValue(value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var resp interface{}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func diffReleasePlanValues(ctx releasePlanDiffContext, path string, left, right interface{}, entries *[]*releasePlanRawDiffEntry) {
	if shouldIgnoreReleasePlanDiffPath(path) {
		return
	}

	if equal, hashed := equalReleasePlanSubtreeByHash(left, right); hashed {
		if equal {
			return
		}
	} else if reflect.DeepEqual(left, right) {
		return
	}

	leftMap, leftIsMap := left.(map[string]interface{})
	rightMap, rightIsMap := right.(map[string]interface{})
	if leftIsMap || rightIsMap {
		keys := make([]string, 0)
		keySet := map[string]struct{}{}
		for key := range leftMap {
			keySet[key] = struct{}{}
		}
		for key := range rightMap {
			keySet[key] = struct{}{}
		}
		for key := range keySet {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			nextPath := joinReleasePlanDiffPath(path, key)
			diffReleasePlanValues(ctx, nextPath, leftMap[key], rightMap[key], entries)
		}
		return
	}

	leftList, leftIsList := left.([]interface{})
	rightList, rightIsList := right.([]interface{})
	if leftIsList || rightIsList {
		diffReleasePlanArray(ctx, path, leftList, rightList, entries)
		return
	}

	*entries = append(*entries, &releasePlanRawDiffEntry{
		Path:   path,
		Before: left,
		After:  right,
	})
}

func equalReleasePlanSubtreeByHash(left, right interface{}) (equal bool, hashed bool) {
	if !shouldUseReleasePlanSubtreeHash(left, right) {
		return false, false
	}

	leftHash, err := hashReleasePlanSubtree(left)
	if err != nil {
		return false, false
	}
	rightHash, err := hashReleasePlanSubtree(right)
	if err != nil {
		return false, false
	}
	return leftHash == rightHash, true
}

func shouldUseReleasePlanSubtreeHash(left, right interface{}) bool {
	switch leftValue := left.(type) {
	case map[string]interface{}:
		rightValue, ok := right.(map[string]interface{})
		if !ok {
			return false
		}
		return len(leftValue) >= releasePlanHashPruneMinMapKeys || len(rightValue) >= releasePlanHashPruneMinMapKeys
	case []interface{}:
		rightValue, ok := right.([]interface{})
		if !ok {
			return false
		}
		return len(leftValue) >= releasePlanHashPruneMinArrayItems || len(rightValue) >= releasePlanHashPruneMinArrayItems
	default:
		return false
	}
}

func hashReleasePlanSubtree(value interface{}) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func diffReleasePlanArray(ctx releasePlanDiffContext, path string, left, right []interface{}, entries *[]*releasePlanRawDiffEntry) {
	leftMap, leftOrdered, leftMapped := buildReleasePlanArrayMap(left)
	rightMap, rightOrdered, rightMapped := buildReleasePlanArrayMap(right)
	strategy := resolveReleasePlanArrayDiffStrategy(ctx, path, leftMapped, rightMapped)
	if strategy == releasePlanArrayDiffStrategyKeyedOrdered {
		if entry := buildReleasePlanArrayOrderChange(path, left, right, leftMap, leftOrdered, rightMap, rightOrdered); entry != nil {
			*entries = append(*entries, entry)
		}
	}
	if strategy == releasePlanArrayDiffStrategyKeyedOrdered || strategy == releasePlanArrayDiffStrategyKeyedUnordered {
		keySet := map[string]struct{}{}
		keys := make([]string, 0)
		for _, key := range leftOrdered {
			if _, exists := keySet[key]; !exists {
				keySet[key] = struct{}{}
				keys = append(keys, key)
			}
		}
		for _, key := range rightOrdered {
			if _, exists := keySet[key]; !exists {
				keySet[key] = struct{}{}
				keys = append(keys, key)
			}
		}
		for _, key := range keys {
			nextPath := fmt.Sprintf("%s[%s]", path, key)
			diffReleasePlanValues(ctx, nextPath, leftMap[key], rightMap[key], entries)
		}
		return
	}

	maxLen := len(left)
	if len(right) > maxLen {
		maxLen = len(right)
	}
	for i := 0; i < maxLen; i++ {
		nextPath := fmt.Sprintf("%s[%d]", path, i)
		var leftVal, rightVal interface{}
		if i < len(left) {
			leftVal = left[i]
		}
		if i < len(right) {
			rightVal = right[i]
		}
		diffReleasePlanValues(ctx, nextPath, leftVal, rightVal, entries)
	}
}

func resolveReleasePlanArrayDiffStrategy(ctx releasePlanDiffContext, path string, leftMapped, rightMapped bool) releasePlanArrayDiffStrategy {
	if !leftMapped || !rightMapped {
		return releasePlanArrayDiffStrategyIndex
	}
	if shouldTrackReleasePlanArrayOrder(ctx, path) {
		return releasePlanArrayDiffStrategyKeyedOrdered
	}
	return releasePlanArrayDiffStrategyKeyedUnordered
}

func shouldTrackReleasePlanArrayOrder(ctx releasePlanDiffContext, path string) bool {
	return ctx.GroupType == releasePlanVersionSectionJobsOrder && path == ""
}

func buildReleasePlanArrayOrderChange(
	path string,
	left, right []interface{},
	leftMap map[string]interface{},
	leftOrdered []string,
	rightMap map[string]interface{},
	rightOrdered []string,
) *releasePlanRawDiffEntry {
	if !hasReleasePlanArrayRelativeOrderChange(leftMap, leftOrdered, rightMap, rightOrdered) {
		return nil
	}

	return &releasePlanRawDiffEntry{
		Path:        joinReleasePlanDiffPath(path, "order"),
		ChangeType:  releasePlanDiffChangeTypeOrder,
		BeforeOrder: buildReleasePlanArrayOrderItems(left, leftOrdered),
		AfterOrder:  buildReleasePlanArrayOrderItems(right, rightOrdered),
	}
}

func hasReleasePlanArrayRelativeOrderChange(
	leftMap map[string]interface{},
	leftOrdered []string,
	rightMap map[string]interface{},
	rightOrdered []string,
) bool {
	leftShared := filterReleasePlanArrayOrderedKeys(leftOrdered, rightMap)
	rightShared := filterReleasePlanArrayOrderedKeys(rightOrdered, leftMap)
	return !reflect.DeepEqual(leftShared, rightShared)
}

func filterReleasePlanArrayOrderedKeys(orderedKeys []string, otherMap map[string]interface{}) []string {
	resp := make([]string, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		if _, exists := otherMap[key]; exists {
			resp = append(resp, key)
		}
	}
	return resp
}

func buildReleasePlanArrayOrderItems(values []interface{}, orderedKeys []string) []*ReleasePlanVersionDiffOrderItem {
	resp := make([]*ReleasePlanVersionDiffOrderItem, 0, len(values))
	for idx, item := range values {
		key := ""
		if idx < len(orderedKeys) {
			key = orderedKeys[idx]
		}
		resp = append(resp, buildReleasePlanArrayOrderItem(item, key))
	}
	return resp
}

func buildReleasePlanArrayOrderItem(item interface{}, key string) *ReleasePlanVersionDiffOrderItem {
	resp := &ReleasePlanVersionDiffOrderItem{Key: key}

	switch value := item.(type) {
	case map[string]interface{}:
		if id, ok := getStringField(value, "id"); ok {
			resp.ID = id
		}
		if name, ok := getStringField(value, "name"); ok {
			resp.Name = name
			return resp
		}
		if itemKey, ok := getStringField(value, "key"); ok {
			resp.Name = itemKey
			return resp
		}
		if service, ok := getStringField(value, "service_name"); ok {
			if module, ok := getStringField(value, "service_module"); ok {
				resp.Name = fmt.Sprintf("%s/%s", service, module)
			} else {
				resp.Name = service
			}
			return resp
		}
		if repo, ok := getStringField(value, "repo_name"); ok {
			namespace, _ := getStringField(value, "repo_namespace")
			remote, _ := getStringField(value, "remote_name")
			resp.Name = strings.Trim(strings.Trim(fmt.Sprintf("%s/%s/%s", namespace, repo, remote), "/"), "/")
			return resp
		}
		if target, ok := getStringField(value, "target"); ok {
			resp.Name = target
			return resp
		}
		if userID, ok := getStringField(value, "user_id"); ok {
			resp.Name = userID
			return resp
		}
	}

	if resp.Name == "" && key != "" {
		resp.Name = key
	}
	if resp.Name == "" {
		resp.Name = fmt.Sprintf("%v", item)
	}
	return resp
}

func buildReleasePlanArrayMap(values []interface{}) (map[string]interface{}, []string, bool) {
	result := make(map[string]interface{}, len(values))
	orderedKeys := make([]string, 0, len(values))
	for idx, item := range values {
		key, ok := getReleasePlanArrayItemKey(item)
		if !ok {
			return nil, nil, false
		}
		if _, exists := result[key]; exists {
			key = fmt.Sprintf("%s#%d", key, idx)
		}
		result[key] = item
		orderedKeys = append(orderedKeys, key)
	}
	return result, orderedKeys, true
}

func getReleasePlanArrayItemKey(item interface{}) (string, bool) {
	switch value := item.(type) {
	case map[string]interface{}:
		if name, ok := getStringField(value, "name"); ok {
			if jobType, ok := getStringField(value, "type"); ok {
				if id, ok := getStringField(value, "id"); ok {
					return fmt.Sprintf("%s|%s|%s", name, jobType, id), true
				}
				return fmt.Sprintf("%s|%s", name, jobType), true
			}
			if id, ok := getStringField(value, "id"); ok {
				return fmt.Sprintf("%s|%s", name, id), true
			}
			return name, true
		}
		if key, ok := getStringField(value, "key"); ok {
			return key, true
		}
		if service, ok := getStringField(value, "service_name"); ok {
			if module, ok := getStringField(value, "service_module"); ok {
				return fmt.Sprintf("%s/%s", service, module), true
			}
		}
		if repo, ok := getStringField(value, "repo_name"); ok {
			namespace, _ := getStringField(value, "repo_namespace")
			remote, _ := getStringField(value, "remote_name")
			return fmt.Sprintf("%s/%s/%s", namespace, repo, remote), true
		}
		if target, ok := getStringField(value, "target"); ok {
			return target, true
		}
		if userID, ok := getStringField(value, "user_id"); ok {
			return userID, true
		}
		if id, ok := getStringField(value, "id"); ok {
			return id, true
		}
		return "", false
	default:
		return "", false
	}
}

func getStringField(input map[string]interface{}, key string) (string, bool) {
	value, exists := input[key]
	if !exists {
		return "", false
	}
	str, ok := value.(string)
	return str, ok && str != ""
}

func joinReleasePlanDiffPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func shouldIgnoreReleasePlanDiffPath(path string) bool {
	if path == "" {
		return false
	}
	prefixes := []string{
		"id",
		"index",
		"version",
		"created_by",
		"create_time",
		"updated_by",
		"update_time",
		"status",
		"planning_time",
		"finish_planning_time",
		"approval_time",
		"executing_time",
		"success_time",
		"instance_code",
		"hook_settings",
		"wait_for_finish_planning_external_check_time",
		"wait_for_approve_external_check_time",
		"wait_for_execute_external_check_time",
		"wait_for_all_done_external_check_time",
		"external_check_failed_reason",
		"callback_description",
	}
	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, prefix+".") {
			return true
		}
	}

	suffixes := []string{
		".status",
		".last_status",
		".updated",
		".executed_by",
		".executed_time",
		".task_id",
		".hook_payload",
		".hash",
		".notification_id",
		".operation_time",
		".reject_or_approve",
		".approval_instance",
		".manual_exector_id",
		".manual_exector_name",
		".notification_sent",
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func classifyReleasePlanDiffTask(path string) (taskName, taskType string) {
	jobSegments := releasePlanBracketSegments(path, "jobs")
	if len(jobSegments) >= 2 {
		taskName, taskType = splitReleasePlanBracketKey(jobSegments[len(jobSegments)-1])
	}
	return
}

func releasePlanBracketSegments(path, prefix string) []string {
	resp := make([]string, 0)
	for _, segment := range strings.Split(path, ".") {
		if strings.HasPrefix(segment, prefix+"[") {
			resp = append(resp, segment)
		}
	}
	return resp
}

func splitReleasePlanBracketKey(segment string) (string, string) {
	primary := bracketPrimaryName(segment)
	parts := strings.Split(primary, "|")
	if len(parts) == 1 {
		return primary, ""
	}
	return parts[0], strings.Join(parts[1:], "|")
}

func bracketPrimaryName(segment string) string {
	start := strings.Index(segment, "[")
	end := strings.LastIndex(segment, "]")
	if start == -1 || end == -1 || end <= start+1 {
		return segment
	}
	return segment[start+1 : end]
}

func buildReleasePlanDiffLabel(path string) string {
	segments := strings.Split(path, ".")
	labels := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "spec" || segment == "workflow" {
			continue
		}
		label := segment
		switch {
		case strings.HasPrefix(segment, "jobs["):
			name, _ := splitReleasePlanBracketKey(segment)
			label = fmt.Sprintf("任务 %s", name)
		case strings.HasPrefix(segment, "stages["):
			label = fmt.Sprintf("阶段 %s", bracketPrimaryName(segment))
		case strings.HasPrefix(segment, "params["):
			label = fmt.Sprintf("参数 %s", bracketPrimaryName(segment))
		case strings.HasPrefix(segment, "key_vals["):
			label = fmt.Sprintf("变量 %s", bracketPrimaryName(segment))
		case strings.HasPrefix(segment, "services["):
			label = fmt.Sprintf("服务 %s", bracketPrimaryName(segment))
		case strings.Contains(segment, "["):
			fieldName := segment[:strings.Index(segment, "[")]
			label = fmt.Sprintf("%s %s", translateReleasePlanFieldLabel(fieldName), bracketPrimaryName(segment))
		default:
			label = translateReleasePlanFieldLabel(segment)
		}
		labels = append(labels, label)
	}
	if len(labels) == 0 {
		return path
	}
	return strings.Join(labels, " / ")
}

func translateReleasePlanFieldLabel(name string) string {
	if label, exists := releasePlanFieldLabels[name]; exists {
		return label
	}
	return strings.ReplaceAll(name, "_", " ")
}

func isMaskedReleasePlanDiffValue(value interface{}) bool {
	return isReleasePlanMaskedStorageValue(value)
}

func isLargeTextReleasePlanDiffPath(path string, before, after interface{}) bool {
	lowerPath := strings.ToLower(path)
	keywords := []string{"script", "sql", "content", "yaml", "json"}
	for _, keyword := range keywords {
		if strings.Contains(lowerPath, keyword) {
			return true
		}
	}

	if value, ok := before.(string); ok && len(value) > 256 {
		return true
	}
	if value, ok := after.(string); ok && len(value) > 256 {
		return true
	}
	return false
}

func normalizeReleasePlanDiffValue(value interface{}) interface{} {
	switch value.(type) {
	case nil, string, bool, float64:
		return value
	default:
		payload, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(payload)
	}
}
