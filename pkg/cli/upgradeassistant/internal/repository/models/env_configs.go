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

type EnvResource struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	Type           string             `bson:"type"                      json:"type"`
	ProductName    string             `bson:"product_name"              json:"product_name"`
	CreateTime     int64              `bson:"create_time"               json:"create_time"`
	UpdateUserName string             `bson:"update_user_name"          json:"update_user_name"`
	Namespace      string             `bson:"namespace,omitempty"       json:"namespace,omitempty"`
	Status         string             `bson:"status"                    json:"status"`
	DeletedAt      int64              `bson:"deleted_at"                json:"deleted_at" `
	EnvName        string             `bson:"env_name"                  json:"env_name"`
	Name           string             `bson:"name"                      json:"name"`
	YamlData       string             `bson:"yaml_data"                 json:"yaml_data"`
}

func (EnvResource) TableName() string {
	return "env_resource"
}

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

type EnvIngress struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	ProductName    string             `bson:"product_name"              json:"product_name"`
	CreateTime     int64              `bson:"create_time"               json:"create_time"`
	UpdateUserName string             `bson:"update_user_name"          json:"update_user_name"`
	Namespace      string             `bson:"namespace,omitempty"       json:"namespace,omitempty"`
	Status         string             `bson:"status"                    json:"status"`
	EnvName        string             `bson:"env_name"                  json:"env_name"`
	Name           string             `bson:"name"                      json:"name"`
	YamlData       string             `bson:"yaml_data"                 json:"yaml_data"`
}

func (EnvIngress) TableName() string {
	return "env_ingress"
}

type EnvPvc struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	ProductName    string             `bson:"product_name"              json:"product_name"`
	CreateTime     int64              `bson:"create_time"               json:"create_time"`
	UpdateUserName string             `bson:"update_user_name"          json:"update_user_name"`
	Namespace      string             `bson:"namespace,omitempty"       json:"namespace,omitempty"`
	Status         string             `bson:"status"                    json:"status"`
	EnvName        string             `bson:"env_name"                  json:"env_name"`
	Name           string             `bson:"name"                      json:"name"`
	YamlData       string             `bson:"yaml_data"                 json:"yaml_data"`
}

func (EnvPvc) TableName() string {
	return "env_pvc"
}

type EnvSecret struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	ProductName    string             `bson:"product_name"              json:"product_name"`
	CreateTime     int64              `bson:"create_time"               json:"create_time"`
	UpdateUserName string             `bson:"update_user_name"          json:"update_user_name"`
	Namespace      string             `bson:"namespace,omitempty"       json:"namespace,omitempty"`
	Status         string             `bson:"status"                    json:"status"`
	EnvName        string             `bson:"env_name"                  json:"env_name"`
	Name           string             `bson:"name"                      json:"name"`
	YamlData       string             `bson:"yaml_data"                 json:"yaml_data"`
}

func (EnvSecret) TableName() string {
	return "env_secret"
}
