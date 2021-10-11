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

const (
	MethodAll    = "*"
	KindResource = "resource"
)

// Role is a namespaced or cluster scoped, logical grouping of PolicyRules that can be referenced as a unit by a RoleBinding.
// for a cluster scoped Role, namespace is empty.
type Role struct {
	Name      string        `bson:"name"      json:"name"`
	Namespace string        `bson:"namespace" json:"namespace"`
	Rules     []*PolicyRule `bson:"rules"     json:"rules"`
	Kind      string        `bson:"kind"     json:"kind"`
}

// PolicyRule holds information that describes a policy rule, but does not contain information
// about whom the rule applies to.
// If Kind is "resource", verbs are resource actions, while resources are resource names
type PolicyRule struct {
	// Verbs is a list of http methods or resource actions that apply to ALL the Resources contained in this rule. '*' represents all methods.
	Verbs []string `bson:"verbs" json:"verbs"`

	// Resources is a list of resources this rule applies to. '*' represents all resources.
	Resources []string `bson:"resources" json:"resources"`
}

func (Role) TableName() string {
	return "policy_role"
}
