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

import "go.mongodb.org/mongo-driver/bson/primitive"

// JenkinsIntegration table is used to store only jenkins integration info
// Since 3.0.0 it will also store BlueKing CI/CD tools info.
type JenkinsIntegration struct {
	// general fields
	ID        primitive.ObjectID `bson:"_id,omitempty"         json:"id,omitempty"`
	Name      string             `bson:"name"                  json:"name"`
	Type      string             `bson:"type"                  json:"type"`
	UpdateBy  string             `bson:"update_by"             json:"update_by"`
	UpdatedAt int64              `bson:"updated_at"            json:"updated_at"`
	// jenkins specific fields
	URL      string `bson:"url"                   json:"url,omitempty"`
	Username string `bson:"username"              json:"username,omitempty"`
	Password string `bson:"password"              json:"password,omitempty"`
	// BlueKing specific fields
	Host       string `bson:"host"        json:"host,omitempty"`
	AppCode    string `bson:"app_code"    json:"app_code,omitempty"`
	AppSecret  string `bson:"app_secret"  json:"app_secret,omitempty"`
	BKUserName string `bson:"bk_username" json:"bk_username,omitempty"`
}

func (j JenkinsIntegration) TableName() string {
	return "jenkins_integration"
}
