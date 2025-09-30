/*
Copyright 2025 The KodeRover Authors.

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
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LarkPluginAuthConfig struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"                json:"id,omitempty"`
	WorkspaceID  string             `bson:"workspace_id"                 json:"workspace_id"`
	ZadigAddress string             `bson:"zadig_address"                json:"zadig_address"`
	ApiToken     string             `bson:"api_token"                    json:"api_token"`
	UpdateTime   int64              `bson:"update_time"                  json:"update_time"`
}

func (LarkPluginAuthConfig) TableName() string {
	return "lark_plugin_auth_config"
}
