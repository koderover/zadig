/*
 * Copyright 2022 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
)

type ProjectManagement struct {
	ID             primitive.ObjectID  `bson:"_id,omitempty"       json:"id" `
	SystemIdentity string              `bson:"system_identity"     json:"system_identity"`
	Type           string              `bson:"type"                json:"type"`
	JiraAuthType   config.JiraAuthType `bson:"jira_auth_type" json:"jira_auth_type"`
	JiraHost       string              `bson:"jira_host"           json:"jira_host"`
	JiraUser       string              `bson:"jira_user"           json:"jira_user"`
	// JiraToken is used in place of password for basic auth with username
	JiraToken string `bson:"jira_token"          json:"jira_token"`
	// JiraPersonalAccessToken is used for bearer token
	JiraPersonalAccessToken string `bson:"jira_personal_access_token" json:"jira_personal_access_token"`
	MeegoHost               string `bson:"meego_host"          json:"meego_host"`
	MeegoPluginID           string `bson:"meego_plugin_id"     json:"meego_plugin_id"`
	MeegoPluginSecret       string `bson:"meego_plugin_secret" json:"meego_plugin_secret"`
	MeegoUserKey            string `bson:"meego_user_key"      json:"meego_user_key"`
	PingCodeAddress         string `bson:"ping_code_address"         json:"ping_code_address"`
	PingCodeClientID        string `bson:"ping_code_client_id"       json:"ping_code_client_id"`
	PingCodeClientSecret    string `bson:"ping_code_client_secret"   json:"ping_code_client_secret"`
	UpdatedAt               int64  `bson:"updated_at"                json:"updated_at"`
}

func (ProjectManagement) TableName() string {
	return "project_management"
}
