/*
Copyright 2023 The KodeRover Authors.

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

// RoleRef contains information that points to the role being used
type RoleRef struct {
	Name string `bson:"name" json:"name"`

	// Namespace of the referenced object. if the object is cluster scoped, namespace is empty.
	Namespace string `bson:"namespace" json:"namespace"`
}

// NewRoleBinding is the structure for role binding in mysql, after version 1.7
type NewRoleBinding struct {
	ID     uint   `gorm:"primary" json:"id"`
	UID    string `gorm:"column:uid" json:"uid"`
	RoleID uint   `gorm:"column:role_id" json:"role_id"`
}

// RoleBindingDetail is the structure that has actual user info and role info details.
type RoleBindingDetail struct {
	User *User
	Role *Role
}

func (RoleBinding) TableName() string {
	return "rolebinding"
}

func (NewRoleBinding) TableName() string {
	return "role_binding"
}
