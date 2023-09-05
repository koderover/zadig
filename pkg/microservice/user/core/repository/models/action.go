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

type Action struct {
	ID       uint   `gorm:"primarykey"      json:"id"`
	Name     string `gorm:"column:name"     json:"name"`
	Action   string `gorm:"column:action"   json:"action"`
	Resource string `gorm:"column:resource" json:"resource"`
	Scope    int    `gorm:"column:scope"    json:"scope"`

	RoleActionBindings []RoleActionBinding `gorm:"foreignKey:action_id;constraint:OnDelete:CASCADE;"`
}

func (Action) TableName() string {
	return "action"
}
