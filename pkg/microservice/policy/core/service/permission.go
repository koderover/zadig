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
	rolebindingsReadOnly, err := mongodb.NewRoleBindingColl().ListBy(ns, "*")
	if err != nil {
		log.Errorf("list readonly RoleBinding err:%s,ns:%s,uid:*", err, ns)
		return nil, err
	}

	rolebindingsAdmin, err := mongodb.NewRoleBindingColl().ListBy("*", uid)
	if err != nil {
		log.Errorf("list admin RoleBinding err:%s,ns:*,uid:%s", err)
		return nil, err
	}

	// 1.2 get normal rolebindings
	rolebindingsNormal, err := mongodb.NewRoleBindingColl().ListBy(ns, uid)
	if err != nil {
		log.Errorf("list normal RoleBindings err:%s,ns:%s,uid:%s", err, ns, uid)
		return nil, err
	}

	rolebindings := append(rolebindingsReadOnly, rolebindingsNormal...)
	rolebindings = append(rolebindings, rolebindingsAdmin...)
	for _, v := range rolebindings {
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
