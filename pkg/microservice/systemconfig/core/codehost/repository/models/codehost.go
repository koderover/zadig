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

type CodeHost struct {
	ID            int    `bson:"id"               json:"id"`
	Type          string `bson:"type"             json:"type"`
	Address       string `bson:"address"          json:"address"`
	IsReady       string `bson:"is_ready"         json:"is_ready"`
	AccessToken   string `bson:"access_token"     json:"access_token"`
	RefreshToken  string `bson:"refresh_token"    json:"refresh_token"`
	Namespace     string `bson:"namespace"        json:"namespace"`
	ApplicationId string `bson:"application_id"   json:"application_id"`
	Region        string `bson:"region"           json:"region,omitempty"`
	Username      string `bson:"username"         json:"username,omitempty"`
	Password      string `bson:"password"         json:"password,omitempty"`
	ClientSecret  string `bson:"client_secret"    json:"client_secret"`
	CreatedAt     int64  `bson:"created_at"       json:"created_at"`
	UpdatedAt     int64  `bson:"updated_at"       json:"updated_at"`
	DeletedAt     int64  `bson:"deleted_at"       json:"deleted_at"`
	EnableProxy   bool   `bson:"enable_proxy"     json:"enable_proxy"`
}

func (CodeHost) TableName() string {
	return "code_host"
}
