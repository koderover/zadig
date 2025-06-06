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
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

type CodeHost struct {
	ID                 int                      `bson:"id"                              json:"id"                                 yaml:"id"`
	IntegrationLevel   setting.IntegrationLevel `bson:"integration_level"               json:"integration_level"                  yaml:"integration_level"`
	Project            string                   `bson:"project"                         json:"project"                            yaml:"project"`
	Type               string                   `bson:"type"                            json:"type"                               yaml:"type"`
	Address            string                   `bson:"address"                         json:"address"                            yaml:"address"`
	IsReady            string                   `bson:"is_ready"                        json:"is_ready"                           yaml:"is_ready"`
	AccessToken        string                   `bson:"access_token"                    json:"access_token"                       yaml:"access_token"`
	RefreshToken       string                   `bson:"refresh_token"                   json:"refresh_token"                      yaml:"refresh_token"`
	Namespace          string                   `bson:"namespace"                       json:"namespace"                          yaml:"namespace"`
	ApplicationId      string                   `bson:"application_id"                  json:"application_id"                     yaml:"application_id"`
	Region             string                   `bson:"region,omitempty"                json:"region,omitempty"                   yaml:"region,omitempty"`
	ClientSecret       string                   `bson:"client_secret"                   json:"client_secret"                      yaml:"client_secret"`
	Alias              string                   `bson:"alias,omitempty"                 json:"alias,omitempty"                    yaml:"alias,omitempty"`
	AuthType           types.AuthType           `bson:"auth_type,omitempty"             json:"auth_type,omitempty"                yaml:"auth_type,omitempty"`
	SSHKey             string                   `bson:"ssh_key,omitempty"               json:"ssh_key,omitempty"                  yaml:"ssh_key,omitempty"`
	PrivateAccessToken string                   `bson:"private_access_token,omitempty"  json:"private_access_token,omitempty"     yaml:"private_access_token,omitempty"`
	CreatedAt          int64                    `bson:"created_at"                      json:"created_at"                         yaml:"created_at"`
	UpdatedAt          int64                    `bson:"updated_at"                      json:"updated_at"                         yaml:"updated_at"`
	DeletedAt          int64                    `bson:"deleted_at"                      json:"deleted_at"                         yaml:"deleted_at"`
	EnableProxy        bool                     `bson:"enable_proxy"                    json:"enable_proxy"                       yaml:"enable_proxy"`
	DisableSSL         bool                     `bson:"disable_ssl"                     json:"disable_ssl"                        yaml:"disable_ssl"`
	// perforce Type parameters
	P4Host string `bson:"perforce_host"                   json:"perforce_host"          yaml:"perforce_host"`
	P4Port int    `bson:"perforce_port"                   json:"perforce_port"          yaml:"perforce_port"`
	// common parameters
	Username string `bson:"username,omitempty"              json:"username,omitempty"        yaml:"username,omitempty"`
	Password string `bson:"password,omitempty"              json:"password,omitempty"        yaml:"password,omitempty"`
}

func (CodeHost) TableName() string {
	return "code_host"
}
