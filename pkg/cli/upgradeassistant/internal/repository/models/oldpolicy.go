/*
Copyright 2022 The KodeRover Authors.

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

type PolicyMeta struct {
	Resource    string            `bson:"resource"    json:"resource"`
	Alias       string            `bson:"alias"       json:"alias"`
	Description string            `bson:"description" json:"description"`
	Rules       []*PolicyMetaRule `bson:"rules"       json:"rules"`
}

type Role struct {
	Name      string  `bson:"name"      json:"name"`
	Namespace string  `bson:"namespace" json:"namespace"`
	Rules     []*Rule `bson:"rules"     json:"rules"`
}

type Rule struct {
	// Verbs is a list of http methods or resource actions that apply to ALL the Resources contained in this rule. '*' represents all methods.
	Verbs           []string         `bson:"verbs"               json:"verbs"`
	Resources       []string         `bson:"resources"           json:"resources"`
	Kind            string           `bson:"kind"                json:"kind"`
	MatchAttributes []MatchAttribute `bson:"match_attributes"    json:"match_attributes"`
}

type MatchAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type RoleBinding struct {
	Name      string `bson:"name" json:"name"`
	Namespace string `bson:"namespace" json:"namespace"`

	// RoleRef can reference a namespaced or cluster scoped Role.
	RoleRef *RoleRef             `bson:"role_ref" json:"roleRef"`
	Type    setting.ResourceType `bson:"type" json:"type"`
}

// RoleRef contains information that points to the role being used
type RoleRef struct {
	Name string `bson:"name" json:"name"`

	// Namespace of the referenced object. if the object is cluster scoped, namespace is empty.
	Namespace string `bson:"namespace" json:"namespace"`
}

type PolicyMetaRule struct {
	Action      string        `bson:"action"      json:"action"`
	Alias       string        `bson:"alias"       json:"alias"`
	Description string        `bson:"description" json:"description"`
	Rules       []*ActionRule `bson:"rules"       json:"rules"`
}

type ActionRule struct {
	Method          string      `bson:"method"                     json:"method"`
	Endpoint        string      `bson:"endpoint"                   json:"endpoint"`
	ResourceType    string      `bson:"resource_type,omitempty"    json:"resourceType,omitempty"`
	IDRegex         string      `bson:"id_regex,omitempty"         json:"idRegex,omitempty"`
	MatchAttributes []Attribute `bson:"match_attributes,omitempty" json:"matchAttributes,omitempty"`
}

type Attribute struct {
	Key   string `bson:"key"   json:"key"`
	Value string `bson:"value" json:"value"`
}
