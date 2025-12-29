/*
Copyright 2024 The KodeRover Authors.

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

import "go.mongodb.org/mongo-driver/bson/primitive"

// KeyVaultItem represents a key-value pair in the keyvault
// Uniqueness constraint: (group, key, project_name) must be unique
type KeyVaultItem struct {
	ID primitive.ObjectID `bson:"_id,omitempty"          json:"id,omitempty"`

	Group       string `bson:"group"                  json:"group"`
	Key         string `bson:"key"                    json:"key"`
	Value       string `bson:"value"                  json:"value,omitempty"`
	IsSensitive bool   `bson:"is_sensitive"           json:"is_sensitive"`
	Description string `bson:"description"            json:"description"`

	ProjectName  string `bson:"project_name"           json:"project_name"`
	IsSystemWide bool   `bson:"is_system_wide"         json:"is_system_wide"`

	CreatedBy string `bson:"created_by"             json:"created_by"`
	CreatedAt int64  `bson:"created_at"             json:"created_at"`
	UpdatedBy string `bson:"updated_by"             json:"updated_by"`
	UpdatedAt int64  `bson:"updated_at"             json:"updated_at"`
}

func (KeyVaultItem) TableName() string {
	return "keyvault_item"
}
