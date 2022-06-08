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
	ID          int    `bson:"id"                   json:"id"`
	Type        string `bson:"type"                 json:"type"`
	Address     string `bson:"address"              json:"address"`
	Namespace   string `bson:"namespace"            json:"namespace"`
	AccessToken string `bson:"access_token"         json:"accessToken"`
	EnableProxy bool   `bson:"enable_proxy"         json:"enable_proxy"`
	Alias       string `bson:"alias"                json:"alias"`
	Username    string `bson:"username"             json:"username"`
	AccessKey   string `bson:"application_id"       json:"application_id"`
}

func (CodeHost) TableName() string {
	return "code_host"
}
