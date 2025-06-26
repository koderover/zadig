/*
Copyright 2022 The KodeRover Authors.

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

type ConfigurationManagement struct {
	ID             primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	SystemIdentity string             `json:"system_identity" bson:"system_identity" binding:"required"`
	Type           string             `json:"type" bson:"type" binding:"required"`
	AuthConfig     interface{}        `json:"auth_config" bson:"auth_config"`
	ServerAddress  string             `json:"server_address" bson:"server_address" binding:"required"`
	UpdateTime     int64              `json:"update_time" bson:"update_time"`
}

func (ConfigurationManagement) TableName() string {
	return "configuration_management"
}

type ApolloConfig struct {
	ServerAddress string `json:"server_address"`
	*ApolloAuthConfig
}

type ApolloAuthConfig struct {
	// User is the username of apollo for update config
	User  string `json:"user"`
	Token string `json:"token" bson:"token"`
}
