/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	aslanconfig "github.com/koderover/zadig/pkg/microservice/aslan/config"
	aslanmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	aslanmongo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/label"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetUserRulesByProject(uid string, projectName string, log *zap.SugaredLogger) (*GetUserRulesByProjectResp, error) {
	roleBindings, err := mongodb.NewRoleBindingColl().ListBy(projectName, uid)
	if err != nil {
		return nil, err
	}
	allUserRoleBingdins, err := mongodb.NewRoleBindingColl().ListBy(projectName, "*")
	if err != nil {
		return nil, err
	}
	roleBindings = append(roleBindings, allUserRoleBingdins...)
	roles, err := ListUserAllRolesByRoleBindings(roleBindings)
	if err != nil {
		return nil, err
	}
	roleMap := make(map[string]*models.Role)
	for _, role := range roles {
		roleMap[role.Name] = role
	}
	isSystemAdmin := false
	isProjectAdmin := false
	projectVerbSet := sets.NewString()
	for _, rolebinding := range roleBindings {
		if rolebinding.RoleRef.Name == string(setting.SystemAdmin) {
			isSystemAdmin = true
			continue
		} else if rolebinding.RoleRef.Name == string(setting.ProjectAdmin) {
			isProjectAdmin = true
			continue
		}
		var role *models.Role
		if roleRef, ok := roleMap[rolebinding.RoleRef.Name]; ok {
			role = roleRef
		} else {
			log.Errorf("roleMap has no role:%s", rolebinding.RoleRef.Name)
			return nil, fmt.Errorf("roleMap has no role:%s", rolebinding.RoleRef.Name)
		}
		for _, rule := range role.Rules {
			var ruleVerbs []string
			if rule.Resources[0] == "ProductionEnvironment" {
				for _, verb := range rule.Verbs {
					ruleVerbs = append(ruleVerbs, "production:"+verb)
				}
			} else {
				ruleVerbs = rule.Verbs
			}
			projectVerbSet.Insert(ruleVerbs...)
		}
	}

	policyBindings, err := mongodb.NewPolicyBindingColl().ListBy(projectName, uid)
	if err != nil {
		return nil, err
	}
	policies, err := ListUserAllPoliciesByPolicyBindings(policyBindings)
	if err != nil {
		return nil, err
	}
	policyMap := make(map[string]*models.Policy)
	for _, policy := range policies {
		policyMap[policy.Name] = policy
	}
	labelVerbMap := make(map[string][]string)

	for _, policyBinding := range policyBindings {
		var policy *models.Policy
		if policyRef, ok := policyMap[policyBinding.PolicyRef.Name]; ok {
			policy = policyRef
		} else {
			log.Errorf("policyMap has no policy:%s", policyBinding.PolicyRef.Name)
			return nil, fmt.Errorf("policyMap has no policy:%s", policyBinding.PolicyRef.Name)
		}

		for _, rule := range policy.Rules {
			for _, matchAttribute := range rule.MatchAttributes {
				labelKeyKey := rule.Resources[0] + ":" + matchAttribute.Key + ":" + matchAttribute.Value
				if verbs, ok := labelVerbMap[labelKeyKey]; ok {
					verbsSet := sets.NewString(verbs...)
					verbsSet.Insert(rule.Verbs...)
					labelVerbMap[labelKeyKey] = verbsSet.List()
				} else {
					labelVerbMap[labelKeyKey] = rule.Verbs
				}
			}
		}
	}
	var labels []label.Label
	for labelKey, _ := range labelVerbMap {
		keySplit := strings.Split(labelKey, ":")
		labels = append(labels, label.Label{
			Type:  keySplit[0],
			Key:   keySplit[1],
			Value: keySplit[2],
		})
	}
	req := label.ListResourcesByLabelsReq{
		LabelFilters: labels,
	}
	resp, err := label.New().ListResourcesByLabels(req)
	if err != nil {
		return nil, err
	}
	environmentVerbMap := make(map[string][]string)
	workflowVerbMap := make(map[string][]string)
	for labelKey, resources := range resp.Resources {
		for _, resource := range resources {
			resourceType := resource.Type
			if resource.Type == "CommonWorkflow" {
				resourceType = "Workflow"
			}
			if verbs, ok := labelVerbMap[resourceType+":"+labelKey]; ok {
				if resourceType == string(config.ResourceTypeEnvironment) {
					if resourceVerbs, rOK := environmentVerbMap[resource.Name]; rOK {
						verbSet := sets.NewString(resourceVerbs...)
						verbSet.Insert(verbs...)
						environmentVerbMap[resource.Name] = verbSet.List()
					} else {
						environmentVerbMap[resource.Name] = verbs
					}
				}
				if resourceType == string(config.ResourceTypeWorkflow) {
					if resourceVerbs, rOK := workflowVerbMap[resource.Name]; rOK {
						verbSet := sets.NewString(resourceVerbs...)
						verbSet.Insert(verbs...)
						workflowVerbMap[resource.Name] = verbSet.List()
					} else {
						workflowVerbMap[resource.Name] = verbs
					}
				}
			} else {
				log.Warnf("labelVerbMap key:%s not exist", resource.Type+":"+labelKey)
			}
		}
	}
	return &GetUserRulesByProjectResp{
		IsSystemAdmin:       isSystemAdmin,
		ProjectVerbs:        projectVerbSet.List(),
		IsProjectAdmin:      isProjectAdmin,
		EnvironmentVerbsMap: environmentVerbMap,
		WorkflowVerbsMap:    workflowVerbMap,
	}, nil
}

type GetUserRulesByProjectResp struct {
	IsSystemAdmin       bool                `json:"is_system_admin"`
	IsProjectAdmin      bool                `json:"is_project_admin"`
	ProjectVerbs        []string            `json:"project_verbs"`
	WorkflowVerbsMap    map[string][]string `json:"workflow_verbs_map"`
	EnvironmentVerbsMap map[string][]string `json:"environment_verbs_map"`
}

type GetUserRulesResp struct {
	IsSystemAdmin    bool                `json:"is_system_admin"`
	ProjectAdminList []string            `json:"project_admin_list"`
	ProjectVerbMap   map[string][]string `json:"project_verb_map"`
	SystemVerbs      []string            `json:"system_verbs"`
}

func GetUserRules(uid string, log *zap.SugaredLogger) (*GetUserRulesResp, error) {
	roleBindings, err := mongodb.NewRoleBindingColl().ListRoleBindingsByUIDs([]string{uid, "*"})
	if err != nil {
		log.Errorf("ListRoleBindingsByUIDs err:%s")
		return &GetUserRulesResp{}, err
	}
	if len(roleBindings) == 0 {
		log.Info("rolebindings == 0")
		return &GetUserRulesResp{}, nil
	}
	roles, err := ListUserAllRolesByRoleBindings(roleBindings)
	if err != nil {
		log.Errorf("ListUserAllRolesByRoleBindings err:%s", err)
		return &GetUserRulesResp{}, err
	}
	roleMap := make(map[string]*models.Role)
	for _, role := range roles {
		roleMap[role.Name] = role
	}
	isSystemAdmin := false
	projectAdminSet := sets.NewString()
	projectVerbMap := make(map[string][]string)
	systemVerbSet := sets.NewString()
	for _, rolebinding := range roleBindings {
		if rolebinding.RoleRef.Name == string(setting.SystemAdmin) {
			isSystemAdmin = true
			continue
		} else if rolebinding.RoleRef.Name == string(setting.ProjectAdmin) {
			projectAdminSet.Insert(rolebinding.Namespace)
			continue
		}
		var role *models.Role
		if roleRef, ok := roleMap[rolebinding.RoleRef.Name]; ok {
			role = roleRef
		} else {
			log.Errorf("roleMap has no role:%s", rolebinding.RoleRef.Name)
			return nil, fmt.Errorf("roleMap has no role:%s", rolebinding.RoleRef.Name)
		}
		if rolebinding.Namespace == "*" {
			for _, rule := range role.Rules {
				systemVerbSet.Insert(rule.Verbs...)
			}
		} else {
			if verbs, ok := projectVerbMap[rolebinding.Namespace]; ok {
				verbSet := sets.NewString(verbs...)
				for _, rule := range role.Rules {
					var ruleVerbs []string
					if rule.Resources[0] == "ProductionEnvironment" {
						for _, verb := range rule.Verbs {
							ruleVerbs = append(ruleVerbs, "production:"+verb)
						}
					} else {
						ruleVerbs = rule.Verbs
					}
					verbSet.Insert(ruleVerbs...)
				}
				projectVerbMap[rolebinding.Namespace] = verbSet.List()

			} else {
				verbSet := sets.NewString()
				for _, rule := range role.Rules {
					var ruleVerbs []string
					if rule.Resources[0] == "ProductionEnvironment" {
						for _, verb := range rule.Verbs {
							ruleVerbs = append(ruleVerbs, "production:"+verb)
						}
					} else {
						ruleVerbs = rule.Verbs
					}
					verbSet.Insert(ruleVerbs...)
				}
				projectVerbMap[rolebinding.Namespace] = verbSet.List()
			}
		}
	}
	return &GetUserRulesResp{
		IsSystemAdmin:    isSystemAdmin,
		ProjectVerbMap:   projectVerbMap,
		SystemVerbs:      systemVerbSet.List(),
		ProjectAdminList: projectAdminSet.List(),
	}, nil
}

// GetResourcesPermission get resources action list for frontend to show icon
func GetResourcesPermission(uid string, projectName string, resourceType string, resources []string, logger *zap.SugaredLogger) (map[string][]string, error) {
	// 1. get all policyBindings
	policyBindings, err := ListPolicyBindings(projectName, uid, logger)
	if err != nil {
		logger.Errorf("ListPolicyBindings err:%s", err)
		return nil, err
	}
	var policies []*Policy
	for _, v := range policyBindings {
		policy, err := GetPolicy(projectName, v.Policy, logger)
		if err != nil {
			logger.Warnf("GetPolicy err:%s", err)
			continue
		}
		policies = append(policies, policy)
	}

	queryResourceSet := sets.NewString(resources...)
	resourceM := make(map[string]sets.String)
	for _, v := range resources {
		resourceM[v] = sets.NewString()
	}
	for _, policy := range policies {
		for _, rule := range policy.Rules {
			if rule.Resources[0] == resourceType {
				for _, resource := range rule.RelatedResources {
					if queryResourceSet.Has(resource) {
						resourceM[resource] = resourceM[resource].Insert(rule.Verbs...)
					}
				}
			}

		}
	}
	// 2. get all roleBindings
	roleBindings, err := ListUserAllRoleBindings(projectName, uid)
	if err != nil {
		logger.Errorf("ListUserAllRoleBindings err:%s", err)
		return nil, err
	}
	roles, err := ListUserAllRolesByRoleBindings(roleBindings)
	if err != nil {
		logger.Errorf("ListUserAllRolesByRoleBindings err:%s", err)
		return nil, err
	}
	for _, role := range roles {
		if role.Name == string(setting.SystemAdmin) || role.Name == string(setting.ProjectAdmin) {
			for k, _ := range resourceM {
				resourceM[k] = sets.NewString("*")
			}
			break
		}
		for _, rule := range role.Rules {
			if rule.Resources[0] == resourceType && resourceType != string(config.ResourceTypeEnvironment) {
				for k, v := range resourceM {
					resourceM[k] = v.Insert(rule.Verbs...)
				}
			}
			if (rule.Resources[0] == "Environment" || rule.Resources[0] == "ProductionEnvironment") && resourceType == string(config.ResourceTypeEnvironment) {
				var rs []label.Resource
				for _, v := range resources {
					r := label.Resource{
						Name:        v,
						ProjectName: projectName,
						Type:        string(config.ResourceTypeEnvironment),
					}
					rs = append(rs, r)
				}

				labelRes, err := label.New().ListLabelsByResources(label.ListLabelsByResourcesReq{rs})
				if err != nil {
					continue
				}
				for _, resource := range resources {
					resourceKey := fmt.Sprintf("%s-%s-%s", config.ResourceTypeEnvironment, projectName, resource)
					if labels, ok := labelRes.Labels[resourceKey]; ok {
						for _, label := range labels {
							if label.Key == "production" {
								if rule.Resources[0] == "Environment" && label.Value == "false" {
									resourceM[resource] = resourceM[resource].Insert(rule.Verbs...)
								}
								if rule.Resources[0] == "ProductionEnvironment" && label.Value == "true" {
									resourceM[resource] = resourceM[resource].Insert(rule.Verbs...)
								}
							}
						}
					}
				}

			}
		}
	}
	resourceRes := make(map[string][]string)
	for k, v := range resourceM {
		resourceRes[k] = v.List()
	}
	return resourceRes, nil
}

func getRoleBindingVerbMapByResource(uid, resourceType string) (bool, map[string][]string, error) {
	roleBindings, err := mongodb.NewRoleBindingColl().ListRoleBindingsByUIDs([]string{uid, "*"})
	if err != nil {
		return false, nil, err
	}
	if len(roleBindings) == 0 {
		return false, nil, nil
	}
	roles, err := ListUserAllRolesByRoleBindings(roleBindings)
	if err != nil {
		return false, nil, err
	}
	roleMap := make(map[string]*models.Role)
	for _, role := range roles {
		roleMap[getRoleKey(role.Name, role.Namespace)] = role
	}
	var isSystemAdmin bool
	projectVerbMap := make(map[string][]string)

	for _, rolebinding := range roleBindings {
		if rolebinding.RoleRef.Name == string(setting.SystemAdmin) {
			isSystemAdmin = true
			continue
		} else if rolebinding.RoleRef.Name == string(setting.ProjectAdmin) {
			projectVerbMap[rolebinding.Namespace] = []string{"*"}
			continue
		}
		var role *models.Role
		if roleRef, ok := roleMap[getRoleKey(rolebinding.RoleRef.Name, rolebinding.RoleRef.Namespace)]; ok {
			role = roleRef
		} else {
			return false, projectVerbMap, fmt.Errorf("roleMap has no role:%s", rolebinding.RoleRef.Name)
		}
		if role.Name == "read-project-only" {
			continue
		}
		if rolebinding.Namespace != "*" {
			if verbs, ok := projectVerbMap[rolebinding.Namespace]; ok {
				verbSet := sets.NewString(verbs...)
				for _, rule := range role.Rules {
					if len(rule.MatchAttributes) > 0 && rule.MatchAttributes[0].Key == "placeholder" {
						continue
					}
					if rule.Resources[0] == resourceType {
						verbSet.Insert(rule.Verbs...)
					}
				}
				projectVerbMap[rolebinding.Namespace] = verbSet.List()

			} else {
				verbSet := sets.NewString()
				for _, rule := range role.Rules {
					if len(rule.MatchAttributes) > 0 && rule.MatchAttributes[0].Key == "placeholder" {
						continue
					}
					if rule.Resources[0] == resourceType {
						verbSet.Insert(rule.Verbs...)
					}
				}
				projectVerbMap[rolebinding.Namespace] = verbSet.List()
			}
		}
	}
	for project, verbs := range projectVerbMap {
		for _, verb := range verbs {
			if verb == "*" {
				projectVerbMap[project] = []string{"*"}
				break
			}
		}
		if len(verbs) == 0 {
			delete(projectVerbMap, project)
		}
	}
	return isSystemAdmin, projectVerbMap, nil
}

func getRoleKey(name, namespace string) string {
	return strings.Join([]string{name, namespace}, "@")
}

type ReleaseWorkflowResp struct {
	Name                 string                   `json:"name"`
	DisplayName          string                   `json:"display_name"`
	Category             setting.WorkflowCategory `json:"category"`
	Stages               []string                 `json:"stages"`
	Project              string                   `json:"project"`
	Description          string                   `json:"description"`
	CreatedBy            string                   `json:"createdBy"`
	CreateTime           int64                    `json:"createTime"`
	UpdatedBy            string                   `json:"updatedBy"`
	UpdateTime           int64                    `json:"updateTime"`
	RecentTask           *TaskInfo                `json:"recentTask"`
	RecentSuccessfulTask *TaskInfo                `json:"recentSuccessfulTask"`
	RecentFailedTask     *TaskInfo                `json:"recentFailedTask"`
	AverageExecutionTime float64                  `json:"averageExecutionTime"`
	SuccessRate          float64                  `json:"successRate"`
	NeverRun             bool                     `json:"never_run"`
	Verbs                []string                 `json:"verbs"`
}

type TaskInfo struct {
	TaskID       int64  `json:"taskID"`
	PipelineName string `json:"pipelineName"`
	Status       string `json:"status"`
	TaskCreator  string `json:"task_creator"`
	CreateTime   int64  `json:"create_time"`
}

func getRecentTaskV4Info(workflow *ReleaseWorkflowResp, tasks []*aslanmodels.WorkflowTask) {
	recentTask := &aslanmodels.WorkflowTask{}
	recentFailedTask := &aslanmodels.WorkflowTask{}
	recentSucceedTask := &aslanmodels.WorkflowTask{}
	workflow.NeverRun = true
	for _, task := range tasks {
		if task.WorkflowName != workflow.Name {
			continue
		}
		workflow.NeverRun = false
		if task.TaskID > recentTask.TaskID {
			recentTask = task
		}
		if task.Status == aslanconfig.StatusPassed && task.TaskID > recentSucceedTask.TaskID {
			recentSucceedTask = task
		}
		if task.Status == aslanconfig.StatusFailed && task.TaskID > recentFailedTask.TaskID {
			recentFailedTask = task
		}
	}
	if recentTask.TaskID > 0 {
		workflow.RecentTask = &TaskInfo{
			TaskID:       recentTask.TaskID,
			PipelineName: recentTask.WorkflowName,
			Status:       string(recentTask.Status),
			TaskCreator:  recentTask.TaskCreator,
			CreateTime:   recentTask.CreateTime,
		}
	}
	if recentSucceedTask.TaskID > 0 {
		workflow.RecentSuccessfulTask = &TaskInfo{
			TaskID:       recentSucceedTask.TaskID,
			PipelineName: recentSucceedTask.WorkflowName,
			Status:       string(recentSucceedTask.Status),
			TaskCreator:  recentSucceedTask.TaskCreator,
			CreateTime:   recentSucceedTask.CreateTime,
		}
	}
	if recentFailedTask.TaskID > 0 {
		workflow.RecentFailedTask = &TaskInfo{
			TaskID:       recentFailedTask.TaskID,
			PipelineName: recentFailedTask.WorkflowName,
			Status:       string(recentFailedTask.Status),
			TaskCreator:  recentFailedTask.TaskCreator,
			CreateTime:   recentFailedTask.CreateTime,
		}
	}
}

func getWorkflowStatMap(workflowNames []string, workflowType aslanconfig.PipelineType) map[string]*aslanmodels.WorkflowStat {
	workflowStats, err := aslanmongo.NewWorkflowStatColl().FindWorkflowStat(&aslanmongo.WorkflowStatArgs{Names: workflowNames, Type: string(workflowType)})
	if err != nil {
		log.Warnf("Failed to list workflow stats, err: %s", err)
	}
	workflowStatMap := make(map[string]*aslanmodels.WorkflowStat)
	for _, s := range workflowStats {
		workflowStatMap[s.Name] = s
	}
	return workflowStatMap
}

func setWorkflowStat(workflow *ReleaseWorkflowResp, statMap map[string]*aslanmodels.WorkflowStat) {
	if s, ok := statMap[workflow.Name]; ok {
		total := float64(s.TotalSuccess + s.TotalFailure)
		successful := float64(s.TotalSuccess)
		totalDuration := float64(s.TotalDuration)

		workflow.AverageExecutionTime = totalDuration / total
		workflow.SuccessRate = successful / total
	}
}

func setVerbToWorkflows(workflowsNameMap map[string]*ReleaseWorkflowResp, workflows []*ReleaseWorkflowResp, verbs []string) {
	for _, workflow := range workflows {
		workflow.Verbs = verbs
		workflowsNameMap[workflow.Name] = workflow
	}
}

func WorkflowToWorkflowResp(workflow *aslanmodels.WorkflowV4) *ReleaseWorkflowResp {
	stages := []string{}
	for _, stage := range workflow.Stages {
		if stage.Approval != nil && stage.Approval.Enabled {
			stages = append(stages, "人工审批")
		}
		stages = append(stages, stage.Name)
	}
	return &ReleaseWorkflowResp{
		Name:        workflow.Name,
		DisplayName: workflow.DisplayName,
		Category:    workflow.Category,
		Stages:      stages,
		Project:     workflow.Project,
		Description: workflow.Description,
		CreatedBy:   workflow.CreatedBy,
		CreateTime:  workflow.CreateTime,
		UpdatedBy:   workflow.UpdatedBy,
		UpdateTime:  workflow.UpdateTime,
	}
}

func GetUserReleaseWorkflows(uid string, log *zap.SugaredLogger) ([]*ReleaseWorkflowResp, error) {
	workflowResp := []*ReleaseWorkflowResp{}
	releaseWorkflows, _, err := aslanmongo.NewWorkflowV4Coll().List(&aslanmongo.ListWorkflowV4Option{Category: setting.ReleaseWorkflow}, 0, 0)
	if err != nil {
		log.Errorf("List reealse workflow err:%s", err)
		return workflowResp, err
	}
	workflowProjectMap := map[string][]*ReleaseWorkflowResp{}
	for _, workflow := range releaseWorkflows {
		workflowProjectMap[workflow.Project] = append(workflowProjectMap[workflow.Project], WorkflowToWorkflowResp(workflow))
	}
	workflowNameMap := map[string]*ReleaseWorkflowResp{}
	for _, workflow := range releaseWorkflows {
		workflowNameMap[workflow.Name] = WorkflowToWorkflowResp(workflow)
	}

	isSystemAdmin, projectVerbMap, err := getRoleBindingVerbMapByResource(uid, "Workflow")
	if err != nil {
		log.Errorf("getRoleBindingVerbMapByResource err:%s", err)
		return workflowResp, err
	}

	policyBindings, err := mongodb.NewPolicyBindingColl().ListByUser(uid)
	if err != nil {
		return nil, err
	}
	policies, err := ListUserAllPoliciesByPolicyBindings(policyBindings)
	if err != nil {
		return nil, err
	}
	policyMap := make(map[string]*models.Policy)
	for _, policy := range policies {
		policyMap[policy.Name] = policy
	}

	labelVerbMap := make(map[string][]string)

	for _, policyBinding := range policyBindings {
		var policy *models.Policy
		if policyRef, ok := policyMap[policyBinding.PolicyRef.Name]; ok {
			policy = policyRef
		} else {
			log.Errorf("policyMap has no policy:%s", policyBinding.PolicyRef.Name)
			return workflowResp, fmt.Errorf("policyMap has no policy:%s", policyBinding.PolicyRef.Name)
		}

		for _, rule := range policy.Rules {
			for _, matchAttribute := range rule.MatchAttributes {
				labelKeyKey := rule.Resources[0] + ":" + matchAttribute.Key + ":" + matchAttribute.Value
				if verbs, ok := labelVerbMap[labelKeyKey]; ok {
					verbsSet := sets.NewString(verbs...)
					verbsSet.Insert(rule.Verbs...)
					labelVerbMap[labelKeyKey] = verbsSet.List()
				} else {
					labelVerbMap[labelKeyKey] = rule.Verbs
				}
			}
		}
	}

	var labels []label.Label
	for labelKey, _ := range labelVerbMap {
		keySplit := strings.Split(labelKey, ":")
		labels = append(labels, label.Label{
			Type:  keySplit[0],
			Key:   keySplit[1],
			Value: keySplit[2],
		})
	}
	req := label.ListResourcesByLabelsReq{
		LabelFilters: labels,
	}
	resp, err := label.New().ListResourcesByLabels(req)
	if err != nil {
		return nil, err
	}
	workflowVerbMap := make(map[string][]string)
	for labelKey, resources := range resp.Resources {
		for _, resource := range resources {
			resourceType := resource.Type
			if resource.Type == "CommonWorkflow" {
				resourceType = "Workflow"
			}
			if verbs, ok := labelVerbMap[resourceType+":"+labelKey]; ok {
				if resourceType == string(config.ResourceTypeWorkflow) {
					if resourceVerbs, rOK := workflowVerbMap[resource.Name]; rOK {
						verbSet := sets.NewString(resourceVerbs...)
						verbSet.Insert(verbs...)
						workflowVerbMap[resource.Name] = verbSet.List()
					} else {
						workflowVerbMap[resource.Name] = verbs
					}
				}
			} else {
				log.Warnf("labelVerbMap key:%s not exist", resource.Type+":"+labelKey)
			}
		}
	}
	respMap := map[string]*ReleaseWorkflowResp{}

	for project, workflows := range workflowProjectMap {
		if isSystemAdmin {
			setVerbToWorkflows(respMap, workflows, []string{"*"})
			continue
		}
		if verbs, ok := projectVerbMap[project]; ok {
			setVerbToWorkflows(respMap, workflows, verbs)
		}
	}

	for worklfowName, verbs := range workflowVerbMap {
		workflow, ok := workflowNameMap[worklfowName]
		if !ok {
			continue
		}
		if _, ok := respMap[worklfowName]; !ok {
			setVerbToWorkflows(respMap, []*ReleaseWorkflowResp{workflow}, verbs)
		}
	}
	workflowNames := []string{}
	for wokflowName := range respMap {
		workflowNames = append(workflowNames, wokflowName)
	}
	tasks, _, err := aslanmongo.NewworkflowTaskv4Coll().List(&aslanmongo.ListWorkflowTaskV4Option{WorkflowNames: workflowNames})
	if err != nil {
		log.Errorf("fail to list workflow task :%v", err)
		return workflowResp, fmt.Errorf("fail to list workflow task :%v", err)
	}
	workflowStatMap := getWorkflowStatMap(workflowNames, aslanconfig.WorkflowTypeV4)

	for _, workflow := range respMap {
		workflowResp = append(workflowResp, workflow)
		getRecentTaskV4Info(workflow, tasks)
		setWorkflowStat(workflow, workflowStatMap)
	}
	return workflowResp, nil
}

type TestingOpt struct {
	Name        string                  `json:"name"`
	ProductName string                  `json:"product_name"`
	Desc        string                  `json:"desc"`
	UpdateTime  int64                   `json:"update_time"`
	UpdateBy    string                  `json:"update_by"`
	TestCaseNum int                     `json:"test_case_num,omitempty"`
	ExecuteNum  int                     `json:"execute_num,omitempty"`
	PassRate    float64                 `json:"pass_rate,omitempty"`
	AvgDuration float64                 `json:"avg_duration,omitempty"`
	Workflows   []*aslanmodels.Workflow `json:"workflows,omitempty"`
	Verbs       []string                `json:"verbs"`
}

func setVerbToTestings(workflowsNameMap map[string]*TestingOpt, testings []*TestingOpt, verbs []string) {
	for _, testing := range testings {
		testing.Verbs = verbs
		workflowsNameMap[testing.Name] = testing
	}
}

func ListTesting(uid string, log *zap.SugaredLogger) ([]*TestingOpt, error) {
	testingResp := []*TestingOpt{}
	allTestings := []*aslanmodels.Testing{}
	testings, err := aslanmongo.NewTestingColl().List(&aslanmongo.ListTestOption{TestType: "function"})
	if err != nil {
		log.Errorf("[Testing.List] error: %v", err)
		return nil, fmt.Errorf("list testing error: %v", err)
	}

	for _, testing := range testings {

		testTaskStat, _ := GetTestTask(testing.Name)
		if testTaskStat == nil {
			testTaskStat = new(aslanmodels.TestTaskStat)
		}
		testing.TestCaseNum = testTaskStat.TestCaseNum
		totalNum := testTaskStat.TotalSuccess + testTaskStat.TotalFailure
		testing.ExecuteNum = totalNum
		if totalNum != 0 {
			passRate := float64(testTaskStat.TotalSuccess) / float64(totalNum)
			testing.PassRate = decimal(passRate)
			avgDuration := float64(testTaskStat.TotalDuration) / float64(totalNum)
			testing.AvgDuration = decimal(avgDuration)
		}

		testing.Workflows, _ = ListAllWorkflows(testing.Name, log)

		allTestings = append(allTestings, testing)
	}

	testingOpts := make([]*TestingOpt, 0)
	for _, t := range allTestings {
		testingOpts = append(testingOpts, &TestingOpt{
			Name:        t.Name,
			ProductName: t.ProductName,
			Desc:        t.Desc,
			UpdateTime:  t.UpdateTime,
			UpdateBy:    t.UpdateBy,
			TestCaseNum: t.TestCaseNum,
			ExecuteNum:  t.ExecuteNum,
			PassRate:    t.PassRate,
			AvgDuration: t.AvgDuration,
			Workflows:   t.Workflows,
		})
	}
	isSystemAdmin, projectVerbMap, err := getRoleBindingVerbMapByResource(uid, "Test")
	if err != nil {
		log.Errorf("getRoleBindingVerbMapByResource err:%s", err)
		return testingResp, err
	}
	respMap := make(map[string]*TestingOpt)
	testingProjectMap := make(map[string][]*TestingOpt)
	for _, testing := range testingOpts {
		testingProjectMap[testing.ProductName] = append(testingProjectMap[testing.ProductName], testing)
	}
	for project, testings := range testingProjectMap {
		if isSystemAdmin {
			setVerbToTestings(respMap, testings, []string{"*"})
			continue
		}
		if verbs, ok := projectVerbMap[project]; ok {
			setVerbToTestings(respMap, testings, verbs)
		}
	}
	for _, testing := range respMap {
		testingResp = append(testingResp, testing)
	}

	return testingResp, nil
}

func GetTestTask(testName string) (*aslanmodels.TestTaskStat, error) {
	return aslanmongo.NewTestTaskStatColl().FindTestTaskStat(&aslanmongo.TestTaskStatOption{Name: testName})
}

func ListAllWorkflows(testName string, log *zap.SugaredLogger) ([]*aslanmodels.Workflow, error) {
	workflows, err := aslanmongo.NewWorkflowColl().ListByTestName(testName)
	if err != nil {
		log.Errorf("Workflow.List error: %v", err)
		return nil, fmt.Errorf("list workflow error: %s", err)
	}
	return workflows, nil
}

func decimal(value float64) float64 {
	value, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", value), 64)
	return value
}
