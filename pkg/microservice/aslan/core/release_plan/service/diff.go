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
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
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

type releasePlanArrayDiffRuleMatchType int

const (
	releasePlanArrayDiffRuleMatchTypeExact releasePlanArrayDiffRuleMatchType = iota
	releasePlanArrayDiffRuleMatchTypeSafeSuffix
)

type releasePlanArrayKeyBuilder func(item interface{}) (string, bool)

type releasePlanArrayDiffRule struct {
	GroupType      string
	Path           string
	ParentJobTypes map[string]struct{}
	MatchType      releasePlanArrayDiffRuleMatchType
	Strategy       releasePlanArrayDiffStrategy
	BuildKey       releasePlanArrayKeyBuilder
}

var releasePlanWorkflowJobArrayRulePrefixes = []string{
	"spec.workflow.stages.jobs",
	"spec.workflow.jobs",
}

func newReleasePlanExactArrayRule(groupType, path string, strategy releasePlanArrayDiffStrategy, buildKey releasePlanArrayKeyBuilder) releasePlanArrayDiffRule {
	return releasePlanArrayDiffRule{
		GroupType: groupType,
		Path:      path,
		MatchType: releasePlanArrayDiffRuleMatchTypeExact,
		Strategy:  strategy,
		BuildKey:  buildKey,
	}
}

func newReleasePlanTypedExactArrayRule(groupType, path string, parentJobTypes []config.JobType, strategy releasePlanArrayDiffStrategy, buildKey releasePlanArrayKeyBuilder) releasePlanArrayDiffRule {
	rule := newReleasePlanExactArrayRule(groupType, path, strategy, buildKey)
	rule.ParentJobTypes = make(map[string]struct{}, len(parentJobTypes))
	for _, jobType := range parentJobTypes {
		rule.ParentJobTypes[string(jobType)] = struct{}{}
	}
	return rule
}

func buildReleasePlanWorkflowJobArrayRulePath(prefix, pathSuffix string) string {
	if pathSuffix == "" {
		return prefix
	}
	return prefix + "." + pathSuffix
}

func appendReleasePlanWorkflowJobExactArrayRules(rules []releasePlanArrayDiffRule, pathSuffix string, strategy releasePlanArrayDiffStrategy, buildKey releasePlanArrayKeyBuilder) []releasePlanArrayDiffRule {
	for _, prefix := range releasePlanWorkflowJobArrayRulePrefixes {
		rules = append(rules, newReleasePlanExactArrayRule("job", buildReleasePlanWorkflowJobArrayRulePath(prefix, pathSuffix), strategy, buildKey))
	}
	return rules
}

func appendReleasePlanWorkflowJobTypedExactArrayRules(rules []releasePlanArrayDiffRule, pathSuffix string, parentJobTypes []config.JobType, strategy releasePlanArrayDiffStrategy, buildKey releasePlanArrayKeyBuilder) []releasePlanArrayDiffRule {
	for _, prefix := range releasePlanWorkflowJobArrayRulePrefixes {
		rules = append(rules, newReleasePlanTypedExactArrayRule("job", buildReleasePlanWorkflowJobArrayRulePath(prefix, pathSuffix), parentJobTypes, strategy, buildKey))
	}
	return rules
}

func newReleasePlanSafeSuffixArrayRule(groupType, path string, strategy releasePlanArrayDiffStrategy, buildKey releasePlanArrayKeyBuilder) releasePlanArrayDiffRule {
	return releasePlanArrayDiffRule{
		GroupType: groupType,
		Path:      path,
		MatchType: releasePlanArrayDiffRuleMatchTypeSafeSuffix,
		Strategy:  strategy,
		BuildKey:  buildKey,
	}
}

var releasePlanArrayExactRules = func() []releasePlanArrayDiffRule {
	rules := []releasePlanArrayDiffRule{
		newReleasePlanExactArrayRule("plan", "jobs", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByNameTypeID),
		newReleasePlanExactArrayRule(releasePlanVersionSectionJobsOrder, "", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByNameID),
		newReleasePlanExactArrayRule("job", "spec.workflow.params", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByNameType),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByName),
		newReleasePlanExactArrayRule("job", "spec.workflow.share_storages", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByName),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.default_services", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_config.default_services", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_and_builds", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.default_service_and_builds", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_and_builds_options", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_and_vm_deploys", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.default_service_and_vm_deploys", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_and_vm_deploys_options", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_and_tests", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_test_options", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_and_scannings", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_scanning_options", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.target_services", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_and_image", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.gray_services", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceName),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.source_service", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_trigger_workflow", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByWorkflowTrigger),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.fixed_workflow_list", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByFixedWorkflowTrigger),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_trigger_workflow.params", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByNameType),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.fixed_workflow_list.params", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByNameType),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.test_modules", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByName),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.test_module_options", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByName),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.scannings", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByName),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.scanning_options", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByName),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.services.service_and_image", []config.JobType{config.JobK8sBlueGreenDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByServiceModuleNameOnly),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.service_options.service_and_image", []config.JobType{config.JobK8sBlueGreenDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByServiceModuleNameOnly),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.gray_services.service_and_image", []config.JobType{config.JobMseGrayRelease}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByServiceModuleNameOnly),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.targets", []config.JobType{config.JobCustomDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByTarget),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.target_options", []config.JobType{config.JobCustomDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByTarget),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.targets", []config.JobType{config.JobZadigDistributeImage}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.target_options", []config.JobType{config.JobZadigDistributeImage}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByServiceModule),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.targets", []config.JobType{config.JobK8sBlueGreenDeploy, config.JobK8sCanaryDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByK8sTarget),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.target_options", []config.JobType{config.JobK8sCanaryDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByK8sTarget),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.targets", []config.JobType{config.JobK8sGrayRelease}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByGrayReleaseTarget),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.target_options", []config.JobType{config.JobK8sGrayRelease}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByGrayReleaseTarget),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.targets", []config.JobType{config.JobK8sGrayRollback}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByGrayRollbackTarget),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.target_options", []config.JobType{config.JobK8sGrayRollback}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByGrayRollbackTarget),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.targets", []config.JobType{config.JobIstioRelease, config.JobIstioRollback}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByIstioTarget),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.target_options", []config.JobType{config.JobIstioRelease, config.JobIstioRollback}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByIstioTarget),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.jobs", []config.JobType{config.JobJenkins}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByJobName),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.job_options", []config.JobType{config.JobJenkins}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByJobName),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.jobs.parameters", []config.JobType{config.JobJenkins}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByName),
		newReleasePlanTypedExactArrayRule("job", "spec.workflow.stages.jobs.spec.job_options.parameters", []config.JobType{config.JobJenkins}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByName),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.alerts", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByNameID),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.alert_options", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByNameID),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.monitors", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByNameID),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.mail_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByUserID),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.mail_notification_config.target_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByUserID),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.lark_group_notification_config.at_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.lark_person_notification_config.target_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.native_approval.approve_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByUserID),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.dingtalk_approval.approval_nodes.approve_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.lark_approval.approve_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.lark_approval.approval_nodes.approve_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.lark_approval.approval_nodes.cc_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
		newReleasePlanExactArrayRule("approval", "native_approval.approve_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByUserID),
		newReleasePlanExactArrayRule("approval", "dingtalk_approval.approval_nodes.approve_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
		newReleasePlanExactArrayRule("approval", "lark_approval.approve_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
		newReleasePlanExactArrayRule("approval", "lark_approval.approval_nodes.approve_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
		newReleasePlanExactArrayRule("approval", "lark_approval.approval_nodes.cc_users", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByThirdPartyUserID),
		newReleasePlanExactArrayRule("approval", "lark_approval.approval_nodes.approve_groups", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByApprovalGroup),
		newReleasePlanExactArrayRule("approval", "lark_approval.approval_nodes.cc_groups", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByApprovalGroup),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.lark_approval.approval_nodes.approve_groups", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByApprovalGroup),
		newReleasePlanExactArrayRule("job", "spec.workflow.stages.jobs.spec.lark_approval.approval_nodes.cc_groups", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByApprovalGroup),
		newReleasePlanExactArrayRule("metadata", "jira_sprint_association.sprints", releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByJiraSprint),
	}
	rules = appendReleasePlanWorkflowJobExactArrayRules(rules, "", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByNameType)
	rules = appendReleasePlanWorkflowJobTypedExactArrayRules(rules, "spec.services", []config.JobType{config.JobZadigDeploy, config.JobK8sBlueGreenDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByServiceName)
	rules = appendReleasePlanWorkflowJobTypedExactArrayRules(rules, "spec.services", []config.JobType{config.JobFreestyle}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByServiceModule)
	rules = appendReleasePlanWorkflowJobTypedExactArrayRules(rules, "spec.service_options", []config.JobType{config.JobK8sBlueGreenDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByServiceName)
	rules = appendReleasePlanWorkflowJobTypedExactArrayRules(rules, "spec.service_config.services", []config.JobType{config.JobSAEDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByServiceModule)
	rules = appendReleasePlanWorkflowJobTypedExactArrayRules(rules, "spec.services.modules", []config.JobType{config.JobZadigDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByServiceModuleNameOnly)
	rules = appendReleasePlanWorkflowJobTypedExactArrayRules(rules, "spec.service_variable_config", []config.JobType{config.JobZadigDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByServiceName)
	rules = appendReleasePlanWorkflowJobTypedExactArrayRules(rules, "spec.service_variable_config.modules", []config.JobType{config.JobZadigDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByServiceModuleNameOnly)
	rules = appendReleasePlanWorkflowJobTypedExactArrayRules(rules, "spec.service_variable_config.variable_configs", []config.JobType{config.JobZadigDeploy}, releasePlanArrayDiffStrategyKeyedOrdered, buildReleasePlanArrayKeyByVariableConfig)
	return rules
}()

var releasePlanArraySafeSuffixRules = []releasePlanArrayDiffRule{
	newReleasePlanSafeSuffixArrayRule("job", "params", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByNameType),
	newReleasePlanSafeSuffixArrayRule("job", "repos", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByRepo),
	newReleasePlanSafeSuffixArrayRule("job", "code_info", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByRepo),
	newReleasePlanSafeSuffixArrayRule("job", "key_vals", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByKey),
	newReleasePlanSafeSuffixArrayRule("job", "envs", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByKey),
	newReleasePlanSafeSuffixArrayRule("job", "custom_envs", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByKey),
	newReleasePlanSafeSuffixArrayRule("job", "custom_annotations", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByKey),
	newReleasePlanSafeSuffixArrayRule("job", "custom_labels", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByKey),
	newReleasePlanSafeSuffixArrayRule("job", "variable_kvs", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByKey),
	newReleasePlanSafeSuffixArrayRule("job", "kv", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByKey),
	newReleasePlanSafeSuffixArrayRule("job", "original_config", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByKey),
	newReleasePlanSafeSuffixArrayRule("job", "service_and_builds", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
	newReleasePlanSafeSuffixArrayRule("job", "service_and_vm_deploys", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
	newReleasePlanSafeSuffixArrayRule("job", "service_and_tests", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
	newReleasePlanSafeSuffixArrayRule("job", "service_and_scannings", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
	newReleasePlanSafeSuffixArrayRule("job", "target_services", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
	newReleasePlanSafeSuffixArrayRule("job", "service_and_image", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByServiceModule),
	newReleasePlanSafeSuffixArrayRule("job", "test_modules", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByName),
	newReleasePlanSafeSuffixArrayRule("job", "scannings", releasePlanArrayDiffStrategyKeyedUnordered, buildReleasePlanArrayKeyByName),
}

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
		PreviousVersion: current.PreviousVersion,
		Groups:          groups,
	}, nil
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
	case sectionKey == releasePlanVersionSectionMetadata:
		return planSnapshot["metadata"]
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
	rule := matchReleasePlanArrayDiffRule(ctx, path)
	if rule == nil || rule.Strategy == releasePlanArrayDiffStrategyIndex {
		diffReleasePlanArrayByIndex(ctx, path, left, right, entries)
		return
	}

	leftMap, leftOrdered, leftMapped := buildReleasePlanArrayMap(left, rule.BuildKey)
	rightMap, rightOrdered, rightMapped := buildReleasePlanArrayMap(right, rule.BuildKey)
	if !leftMapped || !rightMapped {
		diffReleasePlanArrayByIndex(ctx, path, left, right, entries)
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
			diffReleasePlanValues(ctx, nextPath, leftMap[key], rightMap[key], entries)
		}
		return
	}

	diffReleasePlanArrayByIndex(ctx, path, left, right, entries)
}

func shouldSkipReleasePlanWorkflowTaskPresenceChange(path string, left, right interface{}) bool {
	if left != nil && right != nil {
		return false
	}
	normalizedPath := normalizeReleasePlanDiffPath(path)
	return normalizedPath == "spec.workflow.jobs" || strings.HasSuffix(normalizedPath, ".spec.workflow.jobs")
}

func diffReleasePlanArrayByIndex(ctx releasePlanDiffContext, path string, left, right []interface{}, entries *[]*releasePlanRawDiffEntry) {
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

type releasePlanArrayRuleLookupContext struct {
	GroupType     string
	Path          string
	ParentJobType string
}

func matchReleasePlanArrayDiffRule(ctx releasePlanDiffContext, path string) *releasePlanArrayDiffRule {
	lookupContexts := buildReleasePlanArrayRuleLookupContexts(ctx, path)
	for _, lookup := range lookupContexts {
		for idx := range releasePlanArrayExactRules {
			rule := &releasePlanArrayExactRules[idx]
			if rule.GroupType != lookup.GroupType {
				continue
			}
			if !matchesReleasePlanParentJobType(rule, lookup.ParentJobType) {
				continue
			}
			if rule.Path == lookup.Path {
				return rule
			}
		}
	}
	for _, lookup := range lookupContexts {
		for idx := range releasePlanArraySafeSuffixRules {
			rule := &releasePlanArraySafeSuffixRules[idx]
			if rule.GroupType != lookup.GroupType {
				continue
			}
			if lookup.Path == rule.Path || strings.HasSuffix(lookup.Path, "."+rule.Path) {
				return rule
			}
		}
	}
	return nil
}

func matchesReleasePlanParentJobType(rule *releasePlanArrayDiffRule, parentJobType string) bool {
	if len(rule.ParentJobTypes) == 0 {
		return true
	}
	_, ok := rule.ParentJobTypes[parentJobType]
	return ok
}

func buildReleasePlanArrayRuleLookupContexts(ctx releasePlanDiffContext, path string) []releasePlanArrayRuleLookupContext {
	normalizedPath := normalizeReleasePlanDiffPath(path)
	parentJobType := extractReleasePlanParentJobType(path)
	resp := []releasePlanArrayRuleLookupContext{{
		GroupType:     ctx.GroupType,
		Path:          normalizedPath,
		ParentJobType: parentJobType,
	}}

	if ctx.GroupType != "plan" {
		return resp
	}

	// Nested arrays under the plan snapshot still belong to job/approval/metadata structures.
	if strings.HasPrefix(normalizedPath, "jobs.") {
		resp = append(resp, releasePlanArrayRuleLookupContext{
			GroupType:     "job",
			Path:          strings.TrimPrefix(normalizedPath, "jobs."),
			ParentJobType: parentJobType,
		})
	}
	if strings.HasPrefix(normalizedPath, "approval.") {
		resp = append(resp, releasePlanArrayRuleLookupContext{
			GroupType:     "approval",
			Path:          strings.TrimPrefix(normalizedPath, "approval."),
			ParentJobType: parentJobType,
		})
	}
	if strings.HasPrefix(normalizedPath, "metadata.") {
		resp = append(resp, releasePlanArrayRuleLookupContext{
			GroupType:     "metadata",
			Path:          strings.TrimPrefix(normalizedPath, "metadata."),
			ParentJobType: parentJobType,
		})
	}
	return resp
}

func extractReleasePlanParentJobType(path string) string {
	parentJobType := ""
	searchPath := path
	for {
		idx := strings.Index(searchPath, "jobs[")
		if idx < 0 {
			return parentJobType
		}
		searchPath = searchPath[idx+len("jobs["):]
		endIdx := strings.IndexByte(searchPath, ']')
		if endIdx < 0 {
			return parentJobType
		}
		key := searchPath[:endIdx]
		if jobType, ok := extractReleasePlanJobTypeFromArrayKey(key); ok {
			parentJobType = jobType
		}
		searchPath = searchPath[endIdx+1:]
	}
}

func extractReleasePlanJobTypeFromArrayKey(key string) (string, bool) {
	parts := strings.Split(key, "|")
	if len(parts) < 2 {
		return "", false
	}
	return trimReleasePlanArrayDuplicateSuffix(parts[1]), true
}

func trimReleasePlanArrayDuplicateSuffix(value string) string {
	idx := strings.LastIndex(value, "#")
	if idx < 0 || idx == len(value)-1 {
		return value
	}
	for _, ch := range value[idx+1:] {
		if ch < '0' || ch > '9' {
			return value
		}
	}
	return value[:idx]
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

func buildReleasePlanArrayKeyByNameType(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	name, ok := getStringField(value, "name")
	if !ok {
		return "", false
	}
	itemType, ok := getStringField(value, "type")
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s|%s", name, itemType), true
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

func buildReleasePlanArrayKeyByName(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	return getStringField(value, "name")
}

func buildReleasePlanArrayKeyByKey(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	return getStringField(value, "key")
}

func buildReleasePlanArrayKeyByTarget(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	return getStringField(value, "target")
}

func buildReleasePlanArrayKeyByServiceModule(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	service, ok := getStringField(value, "service_name")
	if !ok {
		return "", false
	}
	module, ok := getStringField(value, "service_module")
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s/%s", service, module), true
}

func buildReleasePlanArrayKeyByServiceModuleNameOnly(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	if module, ok := getStringField(value, "service_module"); ok {
		return module, true
	}
	return getStringField(value, "name")
}

func buildReleasePlanArrayKeyByServiceName(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	return getStringField(value, "service_name")
}

func buildReleasePlanArrayKeyByVariableConfig(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	variableKey, ok := getStringField(value, "variable_key")
	if !ok {
		return "", false
	}
	source, _ := getStringField(value, "source")
	return fmt.Sprintf("%s|%s", variableKey, source), true
}

func buildReleasePlanArrayKeyByWorkflowTrigger(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	serviceName, ok := getStringField(value, "service_name")
	if !ok {
		return "", false
	}
	workflowName, ok := getStringField(value, "workflow_name")
	if !ok {
		return "", false
	}
	projectName, ok := getStringField(value, "project_name")
	if !ok {
		return "", false
	}
	serviceModule, _ := getStringField(value, "service_module")
	return fmt.Sprintf("%s|%s|%s|%s", serviceName, serviceModule, workflowName, projectName), true
}

func buildReleasePlanArrayKeyByFixedWorkflowTrigger(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	workflowName, ok := getStringField(value, "workflow_name")
	if !ok {
		return "", false
	}
	projectName, ok := getStringField(value, "project_name")
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s|%s", workflowName, projectName), true
}

func buildReleasePlanArrayKeyByJobName(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	return getStringField(value, "job_name")
}

func buildReleasePlanArrayKeyByRepo(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	repo, ok := getStringField(value, "repo_name")
	if !ok {
		return "", false
	}
	namespace, _ := getStringField(value, "repo_namespace")
	remote, _ := getStringField(value, "remote_name")
	return fmt.Sprintf("%s/%s/%s", namespace, repo, remote), true
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

func buildReleasePlanArrayKeyByK8sTarget(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	serviceName, ok := getStringField(value, "k8s_service_name")
	if !ok {
		return "", false
	}
	workloadName, ok := getStringField(value, "workload_name")
	if !ok {
		return "", false
	}
	containerName, ok := getStringField(value, "container_name")
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s|%s|%s", serviceName, workloadName, containerName), true
}

func buildReleasePlanArrayKeyByGrayReleaseTarget(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	workloadType, ok := getStringField(value, "workload_type")
	if !ok {
		return "", false
	}
	workloadName, ok := getStringField(value, "workload_name")
	if !ok {
		return "", false
	}
	containerName, ok := getStringField(value, "container_name")
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s|%s|%s", workloadType, workloadName, containerName), true
}

func buildReleasePlanArrayKeyByGrayRollbackTarget(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	workloadType, ok := getStringField(value, "workload_type")
	if !ok {
		return "", false
	}
	workloadName, ok := getStringField(value, "workload_name")
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s|%s", workloadType, workloadName), true
}

func buildReleasePlanArrayKeyByIstioTarget(item interface{}) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}
	virtualServiceName, ok := getStringField(value, "virtual_service_name")
	if !ok {
		return "", false
	}
	workloadName, ok := getStringField(value, "workload_name")
	if !ok {
		return "", false
	}
	containerName, ok := getStringField(value, "container_name")
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s|%s|%s", virtualServiceName, workloadName, containerName), true
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

func classifyReleasePlanDiffTask(path string) (taskName, taskType string) {
	jobSegments := releasePlanBracketSegments(path, "jobs")
	if len(jobSegments) >= 1 {
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
