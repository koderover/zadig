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
	"github.com/koderover/zadig/v2/pkg/setting"
)

type ProjectManagement struct {
	ID             primitive.ObjectID            `bson:"_id,omitempty"       json:"id" `
	SystemIdentity string                        `bson:"system_identity"     json:"system_identity"`
	Type           setting.ProjectManagementType `bson:"type"                json:"type"`
	Spec           interface{}                   `bson:"spec"                json:"spec"`
	UpdatedAt      int64                         `bson:"updated_at"          json:"updated_at"`

	// Deprecated since 4.0.0
	JiraAuthType config.JiraAuthType `bson:"jira_auth_type,omitempty" json:"jira_auth_type,omitempty"`
	JiraHost     string              `bson:"jira_host,omitempty"           json:"jira_host,omitempty"`
	JiraUser     string              `bson:"jira_user,omitempty"           json:"jira_user,omitempty"`
	// JiraToken is used in place of password for basic auth with username
	JiraToken string `bson:"jira_token,omitempty"          json:"jira_token,omitempty"`
	// JiraPersonalAccessToken is used for bearer token
	JiraPersonalAccessToken string `bson:"jira_personal_access_token,omitempty" json:"jira_personal_access_token,omitempty"`
	MeegoHost               string `bson:"meego_host,omitempty"          json:"meego_host,omitempty"`
	MeegoPluginID           string `bson:"meego_plugin_id,omitempty"     json:"meego_plugin_id,omitempty"`
	MeegoPluginSecret       string `bson:"meego_plugin_secret,omitempty" json:"meego_plugin_secret,omitempty"`
	MeegoUserKey            string `bson:"meego_user_key,omitempty"      json:"meego_user_key,omitempty"`
}

type ProjectManagementJiraSpec struct {
	JiraAuthType config.JiraAuthType `bson:"jira_auth_type" json:"jira_auth_type"`
	JiraHost     string              `bson:"jira_host"           json:"jira_host"`
	JiraUser     string              `bson:"jira_user"           json:"jira_user"`
	// JiraToken is used in place of password for basic auth with username
	JiraToken string `bson:"jira_token"          json:"jira_token"`
	// JiraPersonalAccessToken is used for bearer token
	JiraPersonalAccessToken string `bson:"jira_personal_access_token" json:"jira_personal_access_token"`
}

type ProjectManagementMeegoSpec struct {
	MeegoHost         string `bson:"meego_host"          json:"meego_host"`
	MeegoPluginID     string `bson:"meego_plugin_id"     json:"meego_plugin_id"`
	MeegoPluginSecret string `bson:"meego_plugin_secret" json:"meego_plugin_secret"`
	MeegoUserKey      string `bson:"meego_user_key"      json:"meego_user_key"`
}

type ProjectManagementPingCodeSpec struct {
	PingCodeAddress      string `bson:"ping_code_address"         json:"ping_code_address"`
	PingCodeClientID     string `bson:"ping_code_client_id"       json:"ping_code_client_id"`
	PingCodeClientSecret string `bson:"ping_code_client_secret"   json:"ping_code_client_secret"`
}

type ProjectManagementTapdSpec struct {
	TapdAddress      string `bson:"tapd_address"         json:"tapd_address"`
	TapdClientID     string `bson:"tapd_client_id"       json:"tapd_client_id"`
	TapdClientSecret string `bson:"tapd_client_secret"   json:"tapd_client_secret"`
	TapdCompanyID    string `bson:"tapd_company_id"      json:"tapd_company_id"`
}

func (ProjectManagement) TableName() string {
	return "project_management"
}
