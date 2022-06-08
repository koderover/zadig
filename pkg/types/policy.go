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

package types

//PolicyMetaScope resource scope for permission
type PolicyMetaScope string

const (
	SystemScope  PolicyMetaScope = "system"
	ProjectScope PolicyMetaScope = "project"
)

type Policy struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	UpdateTime  int64   `json:"update_time"`
	Rules       []*Rule `json:"rules"`
}

type Rule struct {
	Verbs           []string         `json:"verbs"`
	Resources       []string         `json:"resources"`
	Kind            string           `json:"kind"`
	MatchAttributes []MatchAttribute `json:"match_attributes"`
}

type MatchAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type PolicyMeta struct {
	Resource    string      `json:"resource"`
	Alias       string      `json:"alias"`
	Description string      `json:"description"`
	Rules       []*RuleMeta `json:"rules"`
}

type RuleMeta struct {
	Action      string        `json:"action"`
	Alias       string        `json:"alias"`
	Description string        `json:"description"`
	Rules       []*ActionRule `json:"rules"`
}

type ActionRule struct {
	Method          string       `json:"method"`
	Endpoint        string       `json:"endpoint"`
	ResourceType    string       `json:"resourceType,omitempty"`
	IDRegex         string       `json:"idRegex,omitempty"`
	MatchAttributes []*Attribute `json:"matchAttributes,omitempty"`
	Filter          bool         `json:"filter,omitempty"`
}

type Attribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type PolicyRule struct {
	Methods  []string `json:"methods"`
	Endpoint string   `json:"endpoint"`
}
