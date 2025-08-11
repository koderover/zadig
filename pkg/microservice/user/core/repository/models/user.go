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

type User struct {
	Model
	UID           string `gorm:"primary" json:"uid"`
	Name          string `json:"name"`
	IdentityType  string `gorm:"default:'unknown'" json:"identity_type"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	Account       string `json:"account"`
	APIToken      string `gorm:"api_token" json:"api_token"`
	LastLoginTime int64  `json:"last_login_time"`

	// used to mention the foreign key relationship between user and groupBinding
	// and specify the onDelete action.
	GroupBindings    []GroupBinding   `gorm:"foreignKey:UID;references:UID;constraint:OnDelete:CASCADE;" json:"-"`
	UserRoleBindings []NewRoleBinding `gorm:"foreignKey:UID;references:UID;constraint:OnDelete:CASCADE;" json:"-"`
}

// TableName sets the insert table name for this struct type
func (User) TableName() string {
	return "user"
}
