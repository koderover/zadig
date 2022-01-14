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

// PolicyBinding references a policy, but does not contain it. It adds who information via Subjects.
// PolicyBinding in a given namespace only have effect in that namespace.
// for a cluster scoped PolicyBinding, namespace is empty.
type PolicyDefineBinding struct {
	Name      string `bson:"name"      json:"name"`
	Namespace string `bson:"namespace" json:"namespace"`

	// Subjects holds references to the objects the role applies to.
	Subjects []*Subject `bson:"subjects" json:"subjects"`

	// RoleRef can reference a namespaced or cluster scoped Role.
	RoleRef *RoleRef `bson:"policy_ref" json:"roleRef"`
}

func (PolicyDefineBinding) TableName() string {
	return "policybinding"
}
