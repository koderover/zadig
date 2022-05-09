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

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
)

type Role struct {
	Name  string  `json:"name"`
	Rules []*Rule `json:"rules,omitempty"`
	// if the role is created by user , set the type to "custom"
	// if the role is created by system , set the type  to "system"
	Type setting.ResourceType `json:"type,omitempty"`
	// frontend default select flag
	Select    bool   `json:"select,omitempty"`
	Namespace string `json:"namespace"`
	Desc      string `json:"desc,omitempty"`
}

func CreateRole(ns string, role *Role, _ *zap.SugaredLogger) error {
	obj := &models.Role{
		Name:      role.Name,
		Namespace: ns,
		Type:      role.Type,
		Desc:      role.Desc,
	}

	for _, r := range role.Rules {
		obj.Rules = append(obj.Rules, &models.Rule{
			Verbs:           r.Verbs,
			Kind:            r.Kind,
			Resources:       r.Resources,
			MatchAttributes: r.MatchAttributes,
		})
	}

	return mongodb.NewRoleColl().Create(obj)
}

func UpdateRole(ns string, role *Role, _ *zap.SugaredLogger) error {
	obj := &models.Role{
		Name:      role.Name,
		Namespace: ns,
	}

	for _, r := range role.Rules {
		obj.Rules = append(obj.Rules, &models.Rule{
			Verbs:     r.Verbs,
			Kind:      r.Kind,
			Resources: r.Resources,
		})
	}
	return mongodb.NewRoleColl().UpdateRole(obj)
}

func UpdateOrCreateRole(ns string, role *Role, _ *zap.SugaredLogger) error {
	obj := &models.Role{
		Name:      role.Name,
		Desc:      role.Desc,
		Namespace: ns,
		Type:      role.Type,
	}

	for _, r := range role.Rules {
		obj.Rules = append(obj.Rules, &models.Rule{
			Verbs:           r.Verbs,
			Kind:            r.Kind,
			Resources:       r.Resources,
			MatchAttributes: r.MatchAttributes,
		})
	}
	return mongodb.NewRoleColl().UpdateOrCreate(obj)
}

func ListRoles(projectName string, _ *zap.SugaredLogger) ([]*Role, error) {
	var roles []*Role
	projectRoles, err := mongodb.NewRoleColl().ListBy(projectName)
	if err != nil {
		return nil, err
	}
	for _, v := range projectRoles {
		// frontend doesn't need to see contributor role
		if v.Name == string(setting.Contributor) {
			continue
		}
		tmpRole := Role{Select: false, Name: v.Name, Type: v.Type, Desc: v.Desc, Namespace: v.Namespace}
		if v.Name == string(setting.ReadProjectOnly) {
			tmpRole.Select = true
		}
		roles = append(roles, &tmpRole)
	}
	return roles, nil
}

func GetRole(ns, name string, _ *zap.SugaredLogger) (*Role, error) {
	r, found, err := mongodb.NewRoleColl().Get(ns, name)
	if err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("role %s not found", name)
	}

	res := &Role{
		Name: r.Name,
	}
	for _, ru := range r.Rules {
		res.Rules = append(res.Rules, &Rule{
			Verbs:     ru.Verbs,
			Kind:      ru.Kind,
			Resources: ru.Resources,
		})
	}

	return res, nil
}

func DeleteRole(name string, projectName string, logger *zap.SugaredLogger) error {
	err := mongodb.NewRoleColl().Delete(name, projectName)
	if err != nil {
		logger.Errorf("Failed to delete role %s in project %s, err: %s", name, projectName, err)
		return err
	}

	return mongodb.NewRoleBindingColl().DeleteByRole(name, projectName)
}

func DeleteRoles(names []string, projectName string, logger *zap.SugaredLogger) error {
	if len(names) == 0 {
		return nil
	}
	if projectName == "" {
		return fmt.Errorf("projectName is empty")
	}

	if names[0] == "*" {
		names = []string{}
	}

	err := mongodb.NewRoleColl().DeleteMany(names, projectName)
	if err != nil {
		logger.Errorf("Failed to delete roles %s in project %s, err: %s", names, projectName, err)
		return err
	}

	return mongodb.NewRoleBindingColl().DeleteByRoles(names, projectName)
}

func ListUserAllRolesByRoleBindings(roleBindings []*models.RoleBinding) ([]*models.Role, error) {
	var roles []*models.Role
	for _, v := range roleBindings {
		tmpRoles, err := mongodb.NewRoleColl().ListBySpaceAndName(v.RoleRef.Namespace, v.RoleRef.Name)
		if err != nil {
			continue
		}
		roles = append(roles, tmpRoles...)
	}
	return roles, nil
}
