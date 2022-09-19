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
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/label"
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

//GetResourcesPermission get resources action list for frontend to show icon
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
