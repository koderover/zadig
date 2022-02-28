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

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/label"
)

// GetPermission user's permission for frontend
func GetPermission(ns, uid string, logger *zap.SugaredLogger) (map[string][]string, error) {
	//1.get user's all role in some project
	roleBindings, err := ListUserAllRoleBindings(ns, uid)
	if err != nil {
		logger.Errorf("ListUserAllRoleBindings err:%s", err)
		return nil, err
	}
	roles, err := ListUserAllRolesByRoleBindings(roleBindings)
	if err != nil {
		return nil, err
	}
	//2.get user's all rules
	rolesSet := map[string]sets.String{}
	rolesResp := map[string][]string{}
	for _, role := range roles {
		for _, rule := range role.Rules {
			for _, resource := range rule.Resources {
				if verbsSet, ok := rolesSet[resource]; ok {
					verbsSet.Insert(rule.Verbs...)
					rolesSet[resource] = verbsSet
				} else {
					rolesSet[resource] = sets.NewString(rule.Verbs...)
				}
			}
		}
	}
	for k, v := range rolesSet {
		rolesResp[k] = v.List()
	}
	return rolesResp, nil
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
			if rule.Resources[0] == resourceType && resourceType != string(config.ResourceTypeProduct) {
				for k, v := range resourceM {
					resourceM[k] = v.Insert(rule.Verbs...)
				}
			}
			if (rule.Resources[0] == "Environment" || rule.Resources[0] == "ProductionEnvironment") && resourceType == string(config.ResourceTypeProduct) {
				var rs []label.Resource
				for _, v := range resources {
					r := label.Resource{
						Name:        v,
						ProjectName: projectName,
						Type:        string(config.ResourceTypeProduct),
					}
					rs = append(rs, r)
				}

				labelRes, err := label.New().ListLabelsByResources(label.ListLabelsByResourcesReq{rs})
				if err != nil {
					continue
				}
				for _, resource := range resources {
					resourceKey := fmt.Sprintf("%s-%s-%s", config.ResourceTypeProduct, projectName, resource)
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
