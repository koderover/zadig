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

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
)

type RoleBinding struct {
	Name   string `json:"name"`
	UID    string `json:"uid"`
	Role   string `json:"role"`
	Public bool   `json:"public"`
}

const SystemScope = "*"

func CreateRoleBindings(ns string, rbs []*RoleBinding, logger *zap.SugaredLogger) error {
	var objs []*models.RoleBinding
	for _, rb := range rbs {
		obj, err := createRoleBindingObject(ns, rb, logger)
		if err != nil {
			return err
		}

		objs = append(objs, obj)
	}

	return mongodb.NewRoleBindingColl().BulkCreate(objs)
}

func CreateOrUpdateSystemRoleBinding(ns string, rb *RoleBinding, logger *zap.SugaredLogger) error {

	obj, err := createRoleBindingObject(ns, rb, logger)
	if err != nil {
		return err
	}
	return mongodb.NewRoleBindingColl().UpdateOrCreate(obj)
}

func UpdateOrCreateRoleBinding(ns string, rb *RoleBinding, logger *zap.SugaredLogger) error {
	obj, err := createRoleBindingObject(ns, rb, logger)
	if err != nil {
		return err
	}
	return mongodb.NewRoleBindingColl().UpdateOrCreate(obj)
}

func ListRoleBindings(ns, uid string, _ *zap.SugaredLogger) ([]*RoleBinding, error) {
	var roleBindings []*RoleBinding
	modelRoleBindings, err := mongodb.NewRoleBindingColl().ListBy(ns, uid)
	if err != nil {
		return nil, err
	}

	for _, v := range modelRoleBindings {
		roleBindings = append(roleBindings, &RoleBinding{
			Name:   v.Name,
			Role:   v.RoleRef.Name,
			UID:    v.Subjects[0].UID,
			Public: v.RoleRef.Namespace == "",
		})
	}

	return roleBindings, nil
}

func ListRoleBindingsByRole(ns, roleName string, publicRole bool, _ *zap.SugaredLogger) ([]*RoleBinding, error) {
	var roleBindings []*RoleBinding

	roleNamespace := ns
	if publicRole {
		roleNamespace = ""
	}
	modelRoleBindings, err := mongodb.NewRoleBindingColl().List(&mongodb.ListOptions{RoleName: roleName, RoleNamespace: roleNamespace})
	if err != nil {
		return nil, err
	}

	for _, v := range modelRoleBindings {
		roleBindings = append(roleBindings, &RoleBinding{
			Name:   v.Name,
			Role:   v.RoleRef.Name,
			UID:    v.Subjects[0].UID,
			Public: v.RoleRef.Namespace == "",
		})
	}

	return roleBindings, nil
}

func DeleteRoleBinding(name string, projectName string, _ *zap.SugaredLogger) error {
	return mongodb.NewRoleBindingColl().Delete(name, projectName)
}

func DeleteRoleBindings(names []string, projectName string, userID string, _ *zap.SugaredLogger) error {

	if len(names) == 1 && names[0] == "*" {
		names = []string{}
	}

	return mongodb.NewRoleBindingColl().DeleteMany(names, projectName, userID)
}

func createRoleBindingObject(ns string, rb *RoleBinding, logger *zap.SugaredLogger) (*models.RoleBinding, error) {
	nsRole := ns
	if rb.Public {
		nsRole = ""
	}
	role, found, err := mongodb.NewRoleColl().Get(nsRole, rb.Role)
	if err != nil {
		logger.Errorf("Failed to get role %s in namespace %s, err: %s", rb.Role, nsRole, err)
		return nil, err
	} else if !found {
		logger.Errorf("Role %s is not found in namespace %s", rb.Role, nsRole)
		return nil, fmt.Errorf("role %s not found", rb.Role)
	}

	ensureRoleBindingName(ns, rb)

	return &models.RoleBinding{
		Name:      rb.Name,
		Namespace: ns,
		Subjects:  []*models.Subject{{Kind: models.UserKind, UID: rb.UID}},
		RoleRef: &models.RoleRef{
			Name:      role.Name,
			Namespace: role.Namespace,
		},
	}, nil
}

func ensureRoleBindingName(ns string, rb *RoleBinding) {
	if rb.Name != "" {
		return
	}

	nsRole := ns
	if rb.Public {
		nsRole = ""
	}

	rb.Name = config.RoleBindingNameFromUIDAndRole(rb.UID, setting.RoleType(rb.Role), nsRole)
}
