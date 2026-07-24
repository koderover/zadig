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
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

const (
	releasePlanDiffMaxDepth            = 50
	releasePlanDiffChangeTypeOrder     = "order_changed"
	releasePlanDiffDisplayApprovalSpec = "approval_spec"
	releasePlanDiffDisplayWorkflowSpec = "workflow_spec"
	releasePlanDiffDisplayMetadataSpec = "metadata_spec"
)

type ReleasePlanVersionDiffResponse struct {
	PlanID          string                         `json:"plan_id"`
	Version         int64                          `json:"version"`
	PreviousVersion int64                          `json:"previous_version"`
	Groups          []*ReleasePlanVersionDiffGroup `json:"groups"`
}

type ReleasePlanVersionDiffGroup struct {
	GroupKey    string                          `json:"group_key"`
	GroupName   string                          `json:"group_name"`
	GroupType   string                          `json:"group_type"`
	DisplayMode string                          `json:"display_mode,omitempty"`
	BeforeSpec  interface{}                     `json:"before_spec,omitempty"`
	AfterSpec   interface{}                     `json:"after_spec,omitempty"`
	Changes     []*ReleasePlanVersionDiffChange `json:"changes"`
}

type ReleasePlanVersionDiffOrderItem struct {
	Key  string `json:"key,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type ReleasePlanVersionDiffChange struct {
	ChangeType  string                             `json:"change_type,omitempty"`
	Path        string                             `json:"path,omitempty"`
	Label       string                             `json:"label"`
	Before      interface{}                        `json:"before,omitempty"`
	After       interface{}                        `json:"after,omitempty"`
	BeforeOrder []*ReleasePlanVersionDiffOrderItem `json:"before_order,omitempty"`
	AfterOrder  []*ReleasePlanVersionDiffOrderItem `json:"after_order,omitempty"`
	LargeText   bool                               `json:"large_text,omitempty"`
	Masked      bool                               `json:"masked,omitempty"`
}

type ReleasePlanVersionMetadataDiffItem struct {
	Key       string      `json:"key"`
	Label     string      `json:"label"`
	Value     interface{} `json:"value"`
	ValueType string      `json:"value_type"`
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

type releasePlanMetadataDiffField struct {
	Key       string
	Label     string
	ValueType string
}

type releasePlanArrayDiffStrategy int

const (
	releasePlanArrayDiffStrategyIndex releasePlanArrayDiffStrategy = iota
	releasePlanArrayDiffStrategyKeyedUnordered
	releasePlanArrayDiffStrategyKeyedOrdered
)

type releasePlanArrayKeyBuilder func(item interface{}) (string, bool)

type releasePlanArrayDiffRule struct {
	GroupType string
	Path      string
	Strategy  releasePlanArrayDiffStrategy
	BuildKey  releasePlanArrayKeyBuilder
}

func newReleasePlanExactArrayRule(groupType, path string, strategy releasePlanArrayDiffStrategy, buildKey releasePlanArrayKeyBuilder) releasePlanArrayDiffRule {
	return releasePlanArrayDiffRule{
		GroupType: groupType,
		Path:      path,
		Strategy:  strategy,
		BuildKey:  buildKey,
	}
}

var releasePlanArrayExactRules = []releasePlanArrayDiffRule{
	newReleasePlanExactArrayRule("plan", "jobs", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByNameTypeID),
	newReleasePlanExactArrayRule(releasePlanVersionSectionJobsOrder, "", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByNameID),
	newReleasePlanExactArrayRule("approval", "native_approval.approve_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByUserID),
	newReleasePlanExactArrayRule("approval", "dingtalk_approval.approval_nodes.approve_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
	newReleasePlanExactArrayRule("approval", "lark_approval.approve_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
	newReleasePlanExactArrayRule("approval", "lark_approval.approval_nodes.approve_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
	newReleasePlanExactArrayRule("approval", "lark_approval.approval_nodes.cc_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
	newReleasePlanExactArrayRule("approval", "lark_approval.approval_nodes.approve_groups", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByApprovalGroup),
	newReleasePlanExactArrayRule("approval", "lark_approval.approval_nodes.cc_groups", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByApprovalGroup),
	newReleasePlanExactArrayRule("metadata", "jira_sprint_association.sprints", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByJiraSprint),
}

// Keep only labels that are still used by path-based diff rendering.
// Workflow release jobs are now rendered from before_spec/after_spec directly.
var releasePlanFieldLabels = map[string]string{
	"name":                  "名称",
	"manager":               "负责人",
	"manager_id":            "负责人 ID",
	"start_time":            "开始时间",
	"end_time":              "结束时间",
	"schedule_execute_time": "定时执行时间",
	"description":           "需求关联",
	"approval":              "审批配置",
	"type":                  "类型",
	"enabled":               "是否启用",
	"content":               "内容",
	"remark":                "备注",
	"order":                 "顺序",
	"approve_users":         "审批人",
	"approval_nodes":        "审批节点",
	"native_approval":       "原生审批",
	"lark_approval":         "飞书审批",
	"dingtalk_approval":     "钉钉审批",
	"workwx_approval":       "企业微信审批",
}

var releasePlanMetadataDiffFields = []releasePlanMetadataDiffField{
	{Key: "name", Label: "名称", ValueType: "text"},
	{Key: "manager", Label: "负责人", ValueType: "text"},
	{Key: "start_time", Label: "开始时间", ValueType: "time"},
	{Key: "end_time", Label: "结束时间", ValueType: "time"},
	{Key: "schedule_execute_time", Label: "定时执行时间", ValueType: "time"},
	{Key: "description", Label: "需求关联", ValueType: "rich_text"},
	{Key: "jira_sprint_association", Label: "关联冲刺", ValueType: "jira_sprint_association"},
}

func GetReleasePlanVersionDiff(planID string, version int64) (*ReleasePlanVersionDiffResponse, error) {
	current, err := mongodb.NewReleasePlanVersionColl().Get(planID, version)
	if err != nil {
		return nil, errors.Wrap(err, "get version")
	}

	fromData, hasBaseSnapshot, err := releasePlanVersionBaseSnapshotAsGenericValue(current)
	if err != nil {
		return nil, errors.Wrap(err, "convert base snapshot")
	}

	var previous *models.ReleasePlanVersion
	if !hasBaseSnapshot && current.PreviousVersion > 0 {
		previous, err = mongodb.NewReleasePlanVersionColl().Get(planID, current.PreviousVersion)
		if err != nil {
			return nil, errors.Wrap(err, "get previous version")
		}
		fromData = comparableReleasePlanVersionSnapshot(previous, current.SectionKey)
	}

	toData, err := toReleasePlanGenericValue(current.Snapshot)
	if err != nil {
		return nil, errors.Wrap(err, "convert current snapshot")
	}

	return &ReleasePlanVersionDiffResponse{
		PlanID:          planID,
		Version:         version,
		PreviousVersion: current.PreviousVersion,
		Groups:          buildReleasePlanVersionDiffGroups(current, fromData, toData),
	}, nil
}

func buildReleasePlanVersionDiffGroups(current *models.ReleasePlanVersion, fromData, toData interface{}) []*ReleasePlanVersionDiffGroup {
	if current.SectionKey == releasePlanVersionSectionPlan && current.Verb == VerbCreate {
		return buildReleasePlanCreateVersionDiffGroups(current, fromData, toData)
	}

	groupKey, groupName, groupType := releasePlanVersionDiffGroup(current.SectionKey, current.SectionName)
	groups := buildReleasePlanSectionVersionDiffGroups(groupKey, groupName, groupType, current.SectionKey, current.Verb, fromData, toData)
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].GroupKey < groups[j].GroupKey
	})
	return groups
}

func buildReleasePlanCreateVersionDiffGroups(current *models.ReleasePlanVersion, fromData, toData interface{}) []*ReleasePlanVersionDiffGroup {
	fromPlan := releasePlanVersionDiffPlanSnapshot(fromData)
	toPlan := releasePlanVersionDiffPlanSnapshot(toData)

	groupMap := map[string]*ReleasePlanVersionDiffGroup{}
	groupOrder := make([]string, 0)
	appendReleasePlanVersionDiffGroup(groupMap, &groupOrder, releasePlanVersionSectionPlan, releasePlanVersionSectionName(releasePlanVersionSectionPlan, current.SectionName), releasePlanVersionSectionGroupType(releasePlanVersionSectionPlan), releasePlanVersionSectionPlan, current.Verb, fromPlan, toPlan)
	if hasReleasePlanSnapshotChanges(fromPlan["approval"], toPlan["approval"]) {
		appendReleasePlanVersionDiffGroup(groupMap, &groupOrder, releasePlanVersionSectionApproval, releasePlanVersionSectionName(releasePlanVersionSectionApproval, ""), releasePlanVersionSectionGroupType(releasePlanVersionSectionApproval), releasePlanVersionSectionApproval, current.Verb, fromPlan["approval"], toPlan["approval"])
	}
	appendReleasePlanCreateJobVersionDiffGroups(groupMap, &groupOrder, current.Verb, fromPlan["jobs"], toPlan["jobs"])

	return releasePlanVersionDiffGroupsFromMap(groupMap, groupOrder)
}

func buildReleasePlanSectionVersionDiffGroups(groupKey, groupName, groupType, sectionKey, verb string, fromData, toData interface{}) []*ReleasePlanVersionDiffGroup {
	groupMap := map[string]*ReleasePlanVersionDiffGroup{}
	groupOrder := make([]string, 0)
	appendReleasePlanVersionDiffGroup(groupMap, &groupOrder, groupKey, groupName, groupType, sectionKey, verb, fromData, toData)
	return releasePlanVersionDiffGroupsFromMap(groupMap, groupOrder)
}

func appendReleasePlanVersionDiffGroup(groupMap map[string]*ReleasePlanVersionDiffGroup, groupOrder *[]string, groupKey, groupName, groupType, sectionKey, verb string, fromData, toData interface{}) {
	displayMode, beforeSpec, afterSpec := releasePlanVersionDiffDisplaySpec(sectionKey, groupType, verb, fromData, toData)

	rawEntries := make([]*releasePlanRawDiffEntry, 0)
	if shouldBuildReleasePlanPathDiff(displayMode) {
		// Workflow release jobs are rendered from full preset specs on the frontend.
		// Keep path-level diff for simple sections only.
		diffReleasePlanValues(releasePlanDiffContext{GroupType: groupType}, "", fromData, toData, &rawEntries)
	}

	if shouldAddReleasePlanVersionDiffDisplaySpec(displayMode, beforeSpec, afterSpec) {
		group := ensureReleasePlanVersionDiffGroup(groupMap, groupOrder, groupKey, groupName, groupType)
		group.DisplayMode = displayMode
		group.BeforeSpec = sanitizeReleasePlanValueForDisplay(beforeSpec)
		group.AfterSpec = sanitizeReleasePlanValueForDisplay(afterSpec)
	}
	for _, entry := range rawEntries {
		if shouldIgnoreReleasePlanDiffPath(entry.Path) {
			continue
		}
		group := ensureReleasePlanVersionDiffGroup(groupMap, groupOrder, groupKey, groupName, groupType)

		change := &ReleasePlanVersionDiffChange{
			ChangeType: entry.ChangeType,
			Path:       entry.Path,
			Label:      buildReleasePlanDiffLabel(entry.Path),
		}
		if entry.ChangeType == releasePlanDiffChangeTypeOrder {
			change.BeforeOrder = entry.BeforeOrder
			change.AfterOrder = entry.AfterOrder
		} else if isReleasePlanMaskedStorageValue(entry.Before) || isReleasePlanMaskedStorageValue(entry.After) {
			change.Masked = true
		} else if isLargeTextReleasePlanDiffPath(entry.Path, entry.Before, entry.After) {
			change.LargeText = true
		} else {
			change.Before = normalizeReleasePlanDiffValue(entry.Before)
			change.After = normalizeReleasePlanDiffValue(entry.After)
		}
		group.Changes = append(group.Changes, change)
	}
}

func appendReleasePlanCreateJobVersionDiffGroups(groupMap map[string]*ReleasePlanVersionDiffGroup, groupOrder *[]string, verb string, fromData, toData interface{}) {
	fromJobs, fromOrder := releasePlanVersionDiffJobsByID(fromData)
	toJobs, toOrder := releasePlanVersionDiffJobsByID(toData)
	for _, jobID := range mergeReleasePlanVersionDiffJobOrder(toOrder, fromOrder) {
		sectionKey := releasePlanVersionSectionJobPrefix + jobID
		groupName := releasePlanVersionDiffJobName(toJobs[jobID], fromJobs[jobID])
		appendReleasePlanVersionDiffGroup(groupMap, groupOrder, sectionKey, releasePlanVersionSectionName(sectionKey, groupName), releasePlanVersionSectionGroupType(sectionKey), sectionKey, verb, releasePlanVersionDiffCreateJobSnapshot(fromJobs[jobID]), releasePlanVersionDiffCreateJobSnapshot(toJobs[jobID]))
	}
}

func releasePlanVersionDiffCreateJobSnapshot(job map[string]interface{}) interface{} {
	if job == nil {
		return nil
	}
	if jobType, ok := getStringField(job, "type"); ok && jobType == string(config.JobText) {
		return releasePlanVersionDiffTextJobSnapshot(job)
	}
	return job
}

func releasePlanVersionDiffTextJobSnapshot(job map[string]interface{}) map[string]interface{} {
	resp := make(map[string]interface{}, len(job))
	for key, value := range job {
		if key == "spec" {
			resp[key] = releasePlanVersionDiffTextJobSpecSnapshot(value)
			continue
		}
		resp[key] = value
	}
	return resp
}

func releasePlanVersionDiffTextJobSpecSnapshot(spec interface{}) interface{} {
	specMap, ok := getMapField(spec)
	if !ok {
		return spec
	}

	resp := make(map[string]interface{}, len(specMap))
	for key, value := range specMap {
		if key == "content" {
			resp[key] = normalizeReleasePlanRichTextComparableValue(value)
			continue
		}
		resp[key] = value
	}
	return resp
}

func releasePlanVersionDiffPlanSnapshot(value interface{}) map[string]interface{} {
	snapshot, ok := getMapField(value)
	if !ok {
		return map[string]interface{}{}
	}
	return snapshot
}

func releasePlanVersionDiffJobsByID(value interface{}) (map[string]map[string]interface{}, []string) {
	jobs := map[string]map[string]interface{}{}
	order := make([]string, 0)
	items, ok := value.([]interface{})
	if !ok {
		return jobs, order
	}
	for _, item := range items {
		job, ok := getMapField(item)
		if !ok {
			continue
		}
		jobID, ok := getStringField(job, "id")
		if !ok {
			continue
		}
		if _, exists := jobs[jobID]; exists {
			continue
		}
		jobs[jobID] = job
		order = append(order, jobID)
	}
	return jobs, order
}

func mergeReleasePlanVersionDiffJobOrder(primary, secondary []string) []string {
	merged := make([]string, 0, len(primary)+len(secondary))
	seen := map[string]struct{}{}
	for _, jobID := range primary {
		if _, exists := seen[jobID]; exists {
			continue
		}
		seen[jobID] = struct{}{}
		merged = append(merged, jobID)
	}
	for _, jobID := range secondary {
		if _, exists := seen[jobID]; exists {
			continue
		}
		seen[jobID] = struct{}{}
		merged = append(merged, jobID)
	}
	return merged
}

func releasePlanVersionDiffJobName(values ...map[string]interface{}) string {
	for _, value := range values {
		name, ok := getStringField(value, "name")
		if ok {
			return name
		}
	}
	return ""
}

func releasePlanVersionDiffGroupsFromMap(groupMap map[string]*ReleasePlanVersionDiffGroup, groupOrder []string) []*ReleasePlanVersionDiffGroup {
	groups := make([]*ReleasePlanVersionDiffGroup, 0, len(groupOrder))
	for _, key := range groupOrder {
		group := groupMap[key]
		sort.Slice(group.Changes, func(i, j int) bool {
			return group.Changes[i].Path < group.Changes[j].Path
		})
		groups = append(groups, group)
	}
	return groups
}

func ensureReleasePlanVersionDiffGroup(groupMap map[string]*ReleasePlanVersionDiffGroup, groupOrder *[]string, groupKey, groupName, groupType string) *ReleasePlanVersionDiffGroup {
	if group, exists := groupMap[groupKey]; exists {
		return group
	}

	group := &ReleasePlanVersionDiffGroup{
		GroupKey:  groupKey,
		GroupName: groupName,
		GroupType: groupType,
		Changes:   make([]*ReleasePlanVersionDiffChange, 0),
	}
	groupMap[groupKey] = group
	*groupOrder = append(*groupOrder, groupKey)
	return group
}

func shouldAddReleasePlanVersionDiffDisplaySpec(displayMode string, beforeSpec, afterSpec interface{}) bool {
	if displayMode == "" {
		return false
	}
	if displayMode == releasePlanDiffDisplayMetadataSpec {
		beforeItems, _ := beforeSpec.([]*ReleasePlanVersionMetadataDiffItem)
		afterItems, _ := afterSpec.([]*ReleasePlanVersionMetadataDiffItem)
		return len(beforeItems) > 0 || len(afterItems) > 0
	}
	return !reflect.DeepEqual(beforeSpec, afterSpec)
}

func releasePlanVersionDiffDisplaySpec(sectionKey, groupType, verb string, fromData, toData interface{}) (string, interface{}, interface{}) {
	switch groupType {
	case "approval":
		if fromData == nil && toData == nil {
			return "", nil, nil
		}
		return releasePlanDiffDisplayApprovalSpec, fromData, toData
	case "metadata":
		beforeSpec, afterSpec := releasePlanVersionDiffMetadataSpec(fromData, toData)
		return releasePlanDiffDisplayMetadataSpec, beforeSpec, afterSpec
	case "job":
		if !isReleasePlanWorkflowJobSnapshot(fromData) && !isReleasePlanWorkflowJobSnapshot(toData) {
			return "", nil, nil
		}
		return releasePlanDiffDisplayWorkflowSpec, releasePlanVersionDiffWorkflowSpec(fromData), releasePlanVersionDiffWorkflowSpec(toData)
	default:
		if sectionKey == releasePlanVersionSectionPlan && verb == VerbCreate {
			beforeSpec, afterSpec := releasePlanVersionDiffMetadataSpec(fromData, toData)
			return releasePlanDiffDisplayMetadataSpec, beforeSpec, afterSpec
		}
		return "", nil, nil
	}
}

func shouldBuildReleasePlanPathDiff(displayMode string) bool {
	return displayMode == "" || displayMode == releasePlanDiffDisplayApprovalSpec
}

func isReleasePlanWorkflowJobSnapshot(value interface{}) bool {
	job, ok := getMapField(value)
	if !ok {
		return false
	}
	if jobType, ok := getStringField(job, "type"); ok && jobType == string(config.JobWorkflow) {
		return true
	}
	spec, ok := getMapField(job["spec"])
	if !ok {
		return false
	}
	_, exists := spec["workflow"]
	return exists
}

func releasePlanVersionDiffJobSpec(value interface{}) interface{} {
	job, ok := getMapField(value)
	if !ok {
		return nil
	}
	return job["spec"]
}

func releasePlanVersionDiffWorkflowSpec(value interface{}) interface{} {
	job, ok := getMapField(value)
	if !ok {
		return nil
	}

	resp := make(map[string]interface{}, 3)
	for _, key := range []string{"name", "manager"} {
		if item, exists := job[key]; exists {
			resp[key] = item
		}
	}
	if spec := releasePlanVersionDiffJobSpec(value); spec != nil {
		if workflowSpec, ok := getMapField(spec); ok {
			for key, item := range workflowSpec {
				resp[key] = item
			}
		}
	}
	if len(resp) == 0 {
		return nil
	}
	return resp
}

func releasePlanVersionDiffMetadataSpec(fromData, toData interface{}) ([]*ReleasePlanVersionMetadataDiffItem, []*ReleasePlanVersionMetadataDiffItem) {
	fromMetadata := releasePlanVersionDiffMetadataSnapshot(fromData)
	toMetadata := releasePlanVersionDiffMetadataSnapshot(toData)

	beforeSpec := make([]*ReleasePlanVersionMetadataDiffItem, 0, len(releasePlanMetadataDiffFields))
	afterSpec := make([]*ReleasePlanVersionMetadataDiffItem, 0, len(releasePlanMetadataDiffFields))
	for _, field := range releasePlanMetadataDiffFields {
		beforeValue := normalizeReleasePlanMetadataDiffValue(field.Key, fromMetadata[field.Key])
		afterValue := normalizeReleasePlanMetadataDiffValue(field.Key, toMetadata[field.Key])
		if reflect.DeepEqual(beforeValue, afterValue) {
			continue
		}
		beforeSpec = append(beforeSpec, newReleasePlanVersionMetadataDiffItem(field, beforeValue))
		afterSpec = append(afterSpec, newReleasePlanVersionMetadataDiffItem(field, afterValue))
	}
	return beforeSpec, afterSpec
}

func releasePlanVersionDiffMetadataSnapshot(value interface{}) map[string]interface{} {
	snapshot, ok := getMapField(value)
	if !ok {
		return map[string]interface{}{}
	}
	if metadata, ok := getMapField(snapshot["metadata"]); ok {
		return metadata
	}
	return snapshot
}

func newReleasePlanVersionMetadataDiffItem(field releasePlanMetadataDiffField, value interface{}) *ReleasePlanVersionMetadataDiffItem {
	return &ReleasePlanVersionMetadataDiffItem{
		Key:       field.Key,
		Label:     field.Label,
		Value:     value,
		ValueType: field.ValueType,
	}
}

func normalizeReleasePlanMetadataDiffValue(key string, value interface{}) interface{} {
	if value == nil {
		return nil
	}

	switch key {
	case "description":
		return normalizeReleasePlanMetadataRichTextValue(value)
	case "start_time", "end_time", "schedule_execute_time":
		return normalizeReleasePlanMetadataTimeValue(value)
	case "jira_sprint_association":
		return normalizeReleasePlanMetadataJiraSprintAssociationValue(value)
	}

	if str, ok := value.(string); ok && strings.TrimSpace(str) == "" {
		return nil
	}
	return value
}

func normalizeReleasePlanMetadataRichTextValue(value interface{}) interface{} {
	str, ok := value.(string)
	if !ok {
		return value
	}
	if isEmptyReleasePlanRichText(str) {
		return nil
	}
	return str
}

func isEmptyReleasePlanRichText(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}

	// Compact is only used for empty-rich-text detection; returned content stays unchanged.
	compact := strings.ToLower(strings.Join(strings.Fields(trimmed), ""))
	compact = strings.ReplaceAll(compact, "&nbsp;", "")
	compact = strings.ReplaceAll(compact, "\u00a0", "")
	switch compact {
	case "", "<p></p>", "<p><br></p>", "<p><br/></p>", "<br>", "<br/>":
		return true
	default:
		return false
	}
}

func normalizeReleasePlanMetadataJiraSprintAssociationValue(value interface{}) interface{} {
	association, ok := getMapField(value)
	if !ok {
		return value
	}
	if isEmptyReleasePlanJiraSprintAssociation(association) {
		return nil
	}
	return value
}

func isEmptyReleasePlanJiraSprintAssociation(value map[string]interface{}) bool {
	if value == nil {
		return true
	}
	if jiraID, ok := value["jira_id"].(string); ok && strings.TrimSpace(jiraID) != "" {
		return false
	}
	if sprints, ok := value["sprints"].([]interface{}); ok && len(sprints) > 0 {
		return false
	}
	return true
}

func normalizeReleasePlanMetadataTimeValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case float64:
		if typed == 0 {
			return nil
		}
		intValue := int64(typed)
		if float64(intValue) == typed {
			return intValue
		}
		return typed
	case int:
		if typed == 0 {
			return nil
		}
		return int64(typed)
	case int64:
		if typed == 0 {
			return nil
		}
		return typed
	case json.Number:
		intValue, err := typed.Int64()
		if err == nil {
			if intValue == 0 {
				return nil
			}
			return intValue
		}
		return value
	default:
		return value
	}
}

func releasePlanVersionBaseSnapshotAsGenericValue(version *models.ReleasePlanVersion) (interface{}, bool, error) {
	if version == nil || version.BaseSnapshot == nil {
		return nil, false, nil
	}

	value, err := toReleasePlanGenericValue(version.BaseSnapshot)
	if err != nil {
		return nil, true, err
	}
	return value, true, nil
}

func comparableReleasePlanVersionSnapshot(version *models.ReleasePlanVersion, sectionKey string) interface{} {
	if version == nil {
		return nil
	}

	switch {
	case version.SectionKey == sectionKey, sectionKey == releasePlanVersionSectionPlan:
		value, err := toReleasePlanGenericValue(version.Snapshot)
		if err != nil {
			return version.Snapshot
		}
		return value
	case version.SectionKey != releasePlanVersionSectionPlan:
		return nil
	default:
		return extractReleasePlanSectionSnapshot(version.Snapshot, sectionKey)
	}
}

func extractReleasePlanSectionSnapshot(snapshot interface{}, sectionKey string) interface{} {
	genericValue, err := toReleasePlanGenericValue(snapshot)
	if err != nil {
		return nil
	}

	planSnapshot, ok := genericValue.(map[string]interface{})
	if !ok {
		return nil
	}

	switch {
	case isReleasePlanVersionMetadataSection(sectionKey):
		metadata, ok := planSnapshot["metadata"].(map[string]interface{})
		if !ok {
			return nil
		}
		switch sectionKey {
		case releasePlanVersionSectionMetadata:
			return metadata
		case releasePlanCollabSectionMetadataName:
			return map[string]interface{}{
				"name": metadata["name"],
			}
		case releasePlanCollabSectionMetadataManager:
			return map[string]interface{}{
				"manager":    metadata["manager"],
				"manager_id": metadata["manager_id"],
			}
		case releasePlanCollabSectionMetadataTimeRange:
			return map[string]interface{}{
				"start_time": metadata["start_time"],
				"end_time":   metadata["end_time"],
			}
		case releasePlanCollabSectionMetadataScheduleExecute:
			return map[string]interface{}{
				"schedule_execute_time": metadata["schedule_execute_time"],
			}
		case releasePlanCollabSectionMetadataDescription:
			return map[string]interface{}{
				"description": metadata["description"],
			}
		case releasePlanCollabSectionMetadataJiraSprint:
			return map[string]interface{}{
				"jira_sprint_association": metadata["jira_sprint_association"],
			}
		default:
			return metadata
		}
	case sectionKey == releasePlanVersionSectionApproval:
		return planSnapshot["approval"]
	case sectionKey == releasePlanVersionSectionJobsOrder:
		return planSnapshot["jobs_order"]
	case strings.HasPrefix(sectionKey, releasePlanVersionSectionJobPrefix):
		jobID := strings.TrimPrefix(sectionKey, releasePlanVersionSectionJobPrefix)
		jobs, ok := planSnapshot["jobs"].([]interface{})
		if !ok {
			return nil
		}
		for _, item := range jobs {
			job, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if id, _ := job["id"].(string); id == jobID {
				return job
			}
		}
	}
	return nil
}

func diffReleasePlanValues(ctx releasePlanDiffContext, path string, left, right interface{}, entries *[]*releasePlanRawDiffEntry) {
	diffReleasePlanValuesWithDepth(ctx, path, 0, left, right, entries)
}

func diffReleasePlanValuesWithDepth(ctx releasePlanDiffContext, path string, depth int, left, right interface{}, entries *[]*releasePlanRawDiffEntry) {
	if shouldIgnoreReleasePlanDiffPath(path) {
		return
	}

	if reflect.DeepEqual(left, right) {
		return
	}

	if depth >= releasePlanDiffMaxDepth {
		*entries = append(*entries, &releasePlanRawDiffEntry{
			Path:   path,
			Before: left,
			After:  right,
		})
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
			diffReleasePlanValuesWithDepth(ctx, nextPath, depth+1, leftMap[key], rightMap[key], entries)
		}
		return
	}

	leftList, leftIsList := left.([]interface{})
	rightList, rightIsList := right.([]interface{})
	if leftIsList || rightIsList {
		diffReleasePlanArray(ctx, path, depth, leftList, rightList, entries)
		return
	}

	*entries = append(*entries, &releasePlanRawDiffEntry{
		Path:   path,
		Before: left,
		After:  right,
	})
}

func diffReleasePlanArray(ctx releasePlanDiffContext, path string, depth int, left, right []interface{}, entries *[]*releasePlanRawDiffEntry) {
	rule := matchReleasePlanArrayDiffRule(ctx, path)
	if rule == nil || rule.Strategy == releasePlanArrayDiffStrategyIndex {
		diffReleasePlanArrayByIndex(ctx, path, depth, left, right, entries)
		return
	}

	leftMap, leftOrdered, leftMapped := buildReleasePlanArrayMap(left, rule.BuildKey)
	rightMap, rightOrdered, rightMapped := buildReleasePlanArrayMap(right, rule.BuildKey)
	if !leftMapped || !rightMapped {
		diffReleasePlanArrayByIndex(ctx, path, depth, left, right, entries)
		return
	}

	strategy := rule.Strategy
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
			if shouldSkipReleasePlanWorkflowTaskPresenceChange(path, leftMap[key], rightMap[key]) {
				continue
			}
			nextPath := fmt.Sprintf("%s[%s]", path, key)
			diffReleasePlanValuesWithDepth(ctx, nextPath, depth+1, leftMap[key], rightMap[key], entries)
		}
		return
	}

	diffReleasePlanArrayByIndex(ctx, path, depth, left, right, entries)
}

func shouldSkipReleasePlanWorkflowTaskPresenceChange(path string, left, right interface{}) bool {
	if left != nil && right != nil {
		return false
	}
	normalizedPath := normalizeReleasePlanDiffPath(path)
	return normalizedPath == "spec.workflow.jobs" || strings.HasSuffix(normalizedPath, ".spec.workflow.jobs")
}

func diffReleasePlanArrayByIndex(ctx releasePlanDiffContext, path string, depth int, left, right []interface{}, entries *[]*releasePlanRawDiffEntry) {
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
		diffReleasePlanValuesWithDepth(ctx, nextPath, depth+1, leftVal, rightVal, entries)
	}
}

type releasePlanArrayRuleLookupContext struct {
	GroupType string
	Path      string
}

func matchReleasePlanArrayDiffRule(ctx releasePlanDiffContext, path string) *releasePlanArrayDiffRule {
	lookupContexts := buildReleasePlanArrayRuleLookupContexts(ctx, path)
	for _, lookup := range lookupContexts {
		for idx := range releasePlanArrayExactRules {
			rule := &releasePlanArrayExactRules[idx]
			if rule.GroupType != lookup.GroupType {
				continue
			}
			if rule.Path == lookup.Path {
				return rule
			}
		}
	}
	return nil
}

func buildReleasePlanArrayRuleLookupContexts(ctx releasePlanDiffContext, path string) []releasePlanArrayRuleLookupContext {
	normalizedPath := normalizeReleasePlanDiffPath(path)
	resp := []releasePlanArrayRuleLookupContext{{
		GroupType: ctx.GroupType,
		Path:      normalizedPath,
	}}

	if ctx.GroupType != "plan" {
		return resp
	}

	// Nested arrays under the plan snapshot still belong to approval/metadata structures.
	if strings.HasPrefix(normalizedPath, "approval.") {
		resp = append(resp, releasePlanArrayRuleLookupContext{
			GroupType: "approval",
			Path:      strings.TrimPrefix(normalizedPath, "approval."),
		})
	}
	if strings.HasPrefix(normalizedPath, "metadata.") {
		resp = append(resp, releasePlanArrayRuleLookupContext{
			GroupType: "metadata",
			Path:      strings.TrimPrefix(normalizedPath, "metadata."),
		})
	}
	return resp
}

func normalizeReleasePlanDiffPath(path string) string {
	if path == "" {
		return ""
	}

	builder := strings.Builder{}
	builder.Grow(len(path))
	inBracket := false
	for _, ch := range path {
		switch ch {
		case '[':
			inBracket = true
		case ']':
			inBracket = false
		default:
			if !inBracket {
				builder.WriteRune(ch)
			}
		}
	}
	return builder.String()
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
		if workflowName, ok := getStringField(value, "workflow_name"); ok {
			projectName, _ := getStringField(value, "project_name")
			serviceName, _ := getStringField(value, "service_name")
			serviceModule, _ := getStringField(value, "service_module")
			parts := make([]string, 0, 4)
			if projectName != "" {
				parts = append(parts, projectName)
			}
			if workflowName != "" {
				parts = append(parts, workflowName)
			}
			if serviceName != "" {
				parts = append(parts, serviceName)
			}
			if serviceModule != "" {
				parts = append(parts, serviceModule)
			}
			resp.Name = strings.Join(parts, "/")
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
		if module, ok := getStringField(value, "service_module"); ok {
			resp.Name = module
			return resp
		}
		if repo, ok := getStringField(value, "repo_name"); ok {
			namespace, _ := getStringField(value, "repo_namespace")
			remote, _ := getStringField(value, "remote_name")
			resp.Name = strings.Trim(fmt.Sprintf("%s/%s/%s", namespace, repo, remote), "/")
			return resp
		}
		if target, ok := getStringField(value, "target"); ok {
			resp.Name = target
			return resp
		}
		if targetName := buildReleasePlanTargetOrderName(value); targetName != "" {
			resp.Name = targetName
			return resp
		}
		if userID, ok := getStringField(value, "user_id"); ok {
			resp.Name = userID
			return resp
		}
		if groupName, ok := getStringField(value, "group_name"); ok {
			resp.Name = groupName
			return resp
		}
		if sprintName, ok := getStringField(value, "sprint_name"); ok {
			projectKey, _ := getStringField(value, "project_key")
			if projectKey != "" {
				resp.Name = fmt.Sprintf("%s/%s", projectKey, sprintName)
			} else {
				resp.Name = sprintName
			}
			return resp
		}
		if variableKey, ok := getStringField(value, "variable_key"); ok {
			resp.Name = variableKey
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

func buildReleasePlanTargetOrderName(value map[string]interface{}) string {
	if serviceName, ok := getStringField(value, "k8s_service_name"); ok {
		workloadName, _ := getStringField(value, "workload_name")
		containerName, _ := getStringField(value, "container_name")
		return joinReleasePlanOrderNameParts(serviceName, workloadName, containerName)
	}
	if virtualServiceName, ok := getStringField(value, "virtual_service_name"); ok {
		workloadName, _ := getStringField(value, "workload_name")
		containerName, _ := getStringField(value, "container_name")
		return joinReleasePlanOrderNameParts(virtualServiceName, workloadName, containerName)
	}
	if workloadType, ok := getStringField(value, "workload_type"); ok {
		workloadName, _ := getStringField(value, "workload_name")
		containerName, _ := getStringField(value, "container_name")
		return joinReleasePlanOrderNameParts(workloadType, workloadName, containerName)
	}
	return ""
}

func joinReleasePlanOrderNameParts(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, " / ")
}

func buildReleasePlanArrayMap(values []interface{}, buildKey releasePlanArrayKeyBuilder) (map[string]interface{}, []string, bool) {
	if buildKey == nil {
		return nil, nil, false
	}

	result := make(map[string]interface{}, len(values))
	orderedKeys := make([]string, 0, len(values))
	for idx, item := range values {
		key, ok := buildKey(item)
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

func buildReleasePlanArrayKeyByNameTypeID(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	name, ok := getStringField(value, "name")
	if !ok {
		return "", false
	}
	jobType, ok := getStringField(value, "type")
	if !ok {
		return "", false
	}
	id, ok := getStringField(value, "id")
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s|%s|%s", name, jobType, id), true
}

func buildReleasePlanArrayKeyByNameID(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	name, ok := getStringField(value, "name")
	if !ok {
		return "", false
	}
	id, ok := getStringField(value, "id")
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s|%s", name, id), true
}

func buildReleasePlanArrayKeyByUserID(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	return getStringField(value, "user_id")
}

func buildReleasePlanArrayKeyByThirdPartyUserID(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	if id, ok := getStringField(value, "id"); ok {
		return id, true
	}
	name, hasName := getStringField(value, "name")
	userID, hasUserID := getStringField(value, "user_id")
	if hasName && hasUserID {
		return fmt.Sprintf("%s|%s", name, userID), true
	}
	return "", false
}

func buildReleasePlanArrayKeyByApprovalGroup(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	if groupID, ok := getStringField(value, "group_id"); ok {
		return groupID, true
	}
	return getStringField(value, "group_name")
}

func buildReleasePlanArrayKeyByJiraSprint(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	projectKey, _ := getStringField(value, "project_key")
	if projectKey == "" {
		projectKey, _ = getStringField(value, "project_name")
	}
	if projectKey == "" {
		return "", false
	}
	boardID, ok := getNumberFieldString(value, "board_id")
	if !ok {
		return "", false
	}
	sprintID, ok := getNumberFieldString(value, "sprint_id")
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s|%s|%s", projectKey, boardID, sprintID), true
}

func getMapField(item interface{}) (map[string]interface{}, bool) {
	value, ok := item.(map[string]interface{})
	return value, ok
}

func getStringField(input map[string]interface{}, key string) (string, bool) {
	value, exists := input[key]
	if !exists {
		return "", false
	}
	str, ok := value.(string)
	return str, ok && str != ""
}

func getNumberFieldString(input map[string]interface{}, key string) (string, bool) {
	value, exists := input[key]
	if !exists {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		return typed, typed != ""
	case float64:
		intValue := int64(typed)
		if float64(intValue) != typed {
			return "", false
		}
		return strconv.FormatInt(intValue, 10), true
	case float32:
		intValue := int64(typed)
		if float32(intValue) != typed {
			return "", false
		}
		return strconv.FormatInt(intValue, 10), true
	case int:
		return strconv.Itoa(typed), true
	case int8:
		return strconv.FormatInt(int64(typed), 10), true
	case int16:
		return strconv.FormatInt(int64(typed), 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	case json.Number:
		if intValue, err := typed.Int64(); err == nil {
			return strconv.FormatInt(intValue, 10), true
		}
		return "", false
	default:
		return "", false
	}
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
	if isReleasePlanWorkflowJobStructureDiffPath(path) {
		return true
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

func isReleasePlanWorkflowJobStructureDiffPath(path string) bool {
	if !strings.HasPrefix(path, "spec.workflow.jobs[") {
		return false
	}
	if strings.Contains(path, "].spec.") {
		return false
	}
	return true
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
		if segment == "metadata" || segment == "spec" || segment == "workflow" {
			continue
		}
		label := segment
		switch {
		case strings.HasPrefix(segment, "jobs["):
			name, _ := splitReleasePlanBracketKey(segment)
			label = fmt.Sprintf("任务 %s", name)
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

func isLargeTextReleasePlanDiffPath(path string, before, after interface{}) bool {
	if shouldPreserveFullReleasePlanDiffValue(path) {
		return false
	}

	lowerPath := strings.ToLower(path)
	keywords := []string{"script", "sql", "yaml", "json"}
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

func shouldPreserveFullReleasePlanDiffValue(path string) bool {
	lowerPath := strings.ToLower(path)
	switch {
	case lowerPath == "description":
		return true
	case strings.HasSuffix(lowerPath, ".description"):
		return true
	case lowerPath == "spec.content":
		return true
	default:
		return false
	}
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
