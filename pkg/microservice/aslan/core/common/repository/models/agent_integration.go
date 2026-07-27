/*
Copyright 2026 The KodeRover Authors.

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

	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

type AgentAuthType string

const (
	AgentAuthTypeAPIKey AgentAuthType = "api_key"
	AgentAuthTypeAKSK   AgentAuthType = "ak_sk"
)

type AgentIntegration struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description" json:"description"`
	BaseURL     string             `bson:"base_url" json:"base_url"`
	Protocol    llm.Protocol       `bson:"protocol" json:"protocol"`
	Model       string             `bson:"model" json:"model"`
	AuthType    AgentAuthType      `bson:"auth_type" json:"auth_type"`
	APIKey      string             `bson:"api_key,omitempty" json:"api_key,omitempty"`
	AccessKey   string             `bson:"access_key,omitempty" json:"access_key,omitempty"`
	SecretKey   string             `bson:"secret_key,omitempty" json:"secret_key,omitempty"`
	UpdatedBy   string             `bson:"updated_by" json:"updated_by"`
	UpdateTime  int64              `bson:"update_time" json:"update_time"`
}

func (AgentIntegration) TableName() string {
	return "agent_integration"
}
