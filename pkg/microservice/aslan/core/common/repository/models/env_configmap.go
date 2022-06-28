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

// Note this file should be deleted in future

package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type EnvConfigMap struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	ProductName    string             `bson:"product_name"              json:"product_name"`
	CreateTime     int64              `bson:"create_time"               json:"create_time"`
	UpdateUserName string             `bson:"update_user_name"          json:"update_user_name"`
	Namespace      string             `bson:"namespace,omitempty"       json:"namespace,omitempty"`
	EnvName        string             `bson:"env_name"                  json:"env_name"`
	Name           string             `bson:"name"                      json:"name"`
	YamlData       string             `bson:"yaml_data"                 json:"yaml_data"`
}

func (EnvConfigMap) TableName() string {
	return "env_configmap"
}
