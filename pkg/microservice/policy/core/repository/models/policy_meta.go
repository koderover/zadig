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

type PolicyMeta struct {
	Resource    string            `bson:"resource"    json:"resource"`
	Alias       string            `bson:"alias"       json:"alias"`
	Description string            `bson:"description" json:"description"`
	Rules       []*PolicyMetaRule `bson:"rules"       json:"rules"`
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
	ResourceType    string      `bson:"resource_type,omitempty"    json:"resource_type,omitempty"`
	IDRegex         string      `bson:"id_regex,omitempty"         json:"idRegex,omitempty"`
	MatchAttributes []Attribute `bson:"match_attributes,omitempty" json:"match_attributes,omitempty"`
}

type Attribute struct {
	Key   string `bson:"key"   json:"key"`
	Value string `bson:"value" json:"value"`
}

func (PolicyMeta) TableName() string {
	return "policy_meta"
}
