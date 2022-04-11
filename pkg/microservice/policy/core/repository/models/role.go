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

package models

import (
	"github.com/koderover/zadig/pkg/setting"
)

// Role is a namespaced or cluster scoped, logical grouping of PolicyRules that can be referenced as a unit by a RoleBinding.
// for a cluster scoped Role, namespace is empty.
type Role struct {
	Name      string               `bson:"name"      json:"name"`
	Desc      string               `bson:"desc"      json:"desc"`
	Namespace string               `bson:"namespace" json:"namespace"`
	Rules     []*Rule              `bson:"rules"     json:"rules"`
	Type      setting.ResourceType `bson:"type"     json:"type"`
}

func (Role) TableName() string {
	return "role"
}
