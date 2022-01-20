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
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
)

// GetPermission user's permission for frontend
func GetPermission(ns, uid string, log *zap.SugaredLogger) (map[string][]string, error) {
	roles := []*models.Role{}
	var metas []mongodb.RoleBindingMeta
	roleBindingReadOnlyMeta := mongodb.RoleBindingMeta{
		Uid:       "*",
		Namespace: ns,
	}
	roleBindingsAdminMeta := mongodb.RoleBindingMeta{
		Uid:       uid,
		Namespace: "*",
	}
	roleBindingCommonMeta := mongodb.RoleBindingMeta{
		Uid:       uid,
		Namespace: ns,
	}
	metas = append(metas, roleBindingReadOnlyMeta, roleBindingsAdminMeta, roleBindingCommonMeta)
	roleBindings, err := mongodb.NewRoleBindingColl().ListByRoleBindingOpt(mongodb.ListRoleBindingsOpt{RoleBindingMetas: metas})
	if err != nil {
		return nil, err
	}
	for _, v := range roleBindings {
		tmpRoles, err := mongodb.NewRoleColl().ListBySpaceAndName(v.RoleRef.Namespace, v.RoleRef.Name)
		if err != nil {
			continue
		}
		roles = append(roles, tmpRoles...)
	}
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

func GetResourcesPermission(uid string, projectName string, resourceType string, resources []string, log *zap.SugaredLogger) (map[string][]string, error) {

	// 1. get all policyBindings
	policyBindings, err := ListPolicyBindings(projectName, uid, log)
	if err != nil {
		return nil, err
	}
	var policies []*Policy
	for _, v := range policyBindings {
		policy, err := GetPolicy(projectName, v.Policy, log)
		if err != nil {
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
	resourceRes := make(map[string][]string)
	for k, v := range resourceM {
		resourceRes[k] = v.List()
	}
	return resourceRes, nil
}
