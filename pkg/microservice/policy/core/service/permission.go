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
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
)

func GetPermission(ns, uid string) (map[string][]string, error) {
	roles := []*models.Role{}
	rolebindingsReadOnly, err := mongodb.NewRoleBindingColl().ListBy(ns, "*")
	if err != nil {
		return nil, err
	}

	rolebindingsAdmin, err := mongodb.NewRoleBindingColl().ListBy("*", uid)
	if err != nil {
		return nil, err
	}

	// 1.2 get normal rolebindings
	rolebindingsNormal, err := mongodb.NewRoleBindingColl().ListBy(ns, uid)
	if err != nil {
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
	rolesResp := map[string][]string{}
	resourceSet := sets.NewString()
	for _, v := range roles {
		for _, vv := range v.Rules {
			if len(vv.Resources) > 0 {
				if resourceSet.Has(vv.Resources[0]) {
					vv.Verbs = append(rolesResp[vv.Resources[0]])
				}
				rolesResp[vv.Resources[0]] = vv.Verbs
			}
		}
	}
	return rolesResp, nil
}
