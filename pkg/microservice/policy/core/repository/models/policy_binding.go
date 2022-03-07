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

// PolicyBinding references a Policy, but does not contain it. It adds who information via Subjects.
// PolicyBinding in a given namespace only have effect in that namespace.
// for a cluster scoped PolicyBinding, namespace is empty.
type PolicyBinding struct {
	Name      string `bson:"name"      json:"name"`
	Namespace string `bson:"namespace"  json:"namespace"`

	// Subjects holds references to the objects the Policy applies to.
	Subjects []*Subject `bson:"subjects"    json:"subjects"`

	// PolicyRef can reference a namespaced or cluster scoped Policy.
	PolicyRef *PolicyRef           `bson:"policy_ref"  json:"policy_ref"`
	Type      setting.ResourceType `bson:"type"        json:"type"`
}

// PolicyRef contains information that points to the policy being used
type PolicyRef struct {
	Name string `bson:"name" json:"name"`

	// Namespace of the referenced object. if the object is cluster scoped, namespace is empty.
	Namespace string `bson:"namespace" json:"namespace"`
}

func (PolicyBinding) TableName() string {
	return "policybinding"
}
