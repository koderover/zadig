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
)

type RoleBinding struct {
	Name    string   `json:"name"`
	User    string   `json:"user"`
	RoleRef *RoleRef `json:"roleRef"`
}

type RoleRef struct {
	Name   string `json:"name"`
	Global bool   `json:"global"`
}

func CreateRoleBinding(ns string, rb *RoleBinding, logger *zap.SugaredLogger) error {
	if ns == "" {
		logger.Errorf("Namespace is empty")
		return fmt.Errorf("empty namespace")
	}
	if rb.RoleRef == nil {
		logger.Errorf("RoleRef is empty")
		return fmt.Errorf("empty roleRef")
	}

	nsRole := ns
	if rb.RoleRef.Global {
		nsRole = ""
	}
	role, found, err := mongodb.NewRoleColl().Get(nsRole, rb.RoleRef.Name)
	if err != nil {
		logger.Errorf("Failed to get role %s in namespace %s, err: %s", rb.RoleRef.Name, nsRole, err)
		return err
	} else if !found {
		logger.Errorf("Role %s is not found in namespace %s", rb.RoleRef.Name, nsRole)
		return fmt.Errorf("role %s not found", rb.RoleRef.Name)
	}

	obj := &models.RoleBinding{
		Name:      rb.Name,
		Namespace: ns,
		Subjects:  []*models.Subject{{Kind: models.UserKind, Name: rb.User}},
		RoleRef: &models.RoleRef{
			Name:      role.Name,
			Namespace: role.Namespace,
		},
	}

	return mongodb.NewRoleBindingColl().Create(obj)
}
