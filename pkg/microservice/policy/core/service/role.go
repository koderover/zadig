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

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
)

type Role struct {
	Name  string        `json:"name"`
	Rules []*PolicyRule `json:"rules"`
}

type PolicyRule struct {
	Verbs     []string `json:"verbs"`
	Resources []string `json:"resources"`
}

func CreateRole(ns string, role *Role, _ *zap.SugaredLogger) error {
	obj := &models.Role{
		Name:      role.Name,
		Namespace: ns,
	}

	for _, r := range role.Rules {
		obj.Rules = append(obj.Rules, &models.PolicyRule{
			Verbs:     r.Verbs,
			Resources: r.Resources,
		})
	}

	return mongodb.NewRoleColl().Create(obj)
}
