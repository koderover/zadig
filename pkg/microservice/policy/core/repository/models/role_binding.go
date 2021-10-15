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

type SubjectKind string

const (
	UserKind  SubjectKind = "user"
	GroupKind SubjectKind = "group"
)

// RoleBinding references a role, but does not contain it. It adds who information via Subjects.
// RoleBindings in a given namespace only have effect in that namespace.
// for a cluster scoped RoleBinding, namespace is empty.
type RoleBinding struct {
	Name      string `bson:"name"      json:"name"`
	Namespace string `bson:"namespace" json:"namespace"`

	// Subjects holds references to the objects the role applies to.
	Subjects []*Subject `bson:"subjects" json:"subjects"`

	// RoleRef can reference a namespaced or cluster scoped Role.
	RoleRef *RoleRef `bson:"role_ref" json:"roleRef"`
}

// Subject contains a reference to the object or user identities a role binding applies to.
type Subject struct {
	// Kind of object being referenced. allowed values are "User", "Group".
	Kind SubjectKind `bson:"kind" json:"kind"`
	// Name of the object being referenced.
	Name string `bson:"name" json:"name"`
}

// RoleRef contains information that points to the role being used
type RoleRef struct {
	Name string `bson:"name" json:"name"`

	// Namespace of the referenced object. if the object is cluster scoped, namespace is empty.
	Namespace string `bson:"namespace" json:"namespace"`
}

func (RoleBinding) TableName() string {
	return "rolebinding"
}
