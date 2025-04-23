/*
Copyright 2024 The KodeRover Authors.

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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EnvInfo struct {
	ID            primitive.ObjectID      `bson:"_id,omitempty"     json:"id"`
	ProjectName   string                  `bson:"project_name"      json:"project_name"`
	EnvName       string                  `bson:"env_name"          json:"env_name"`
	EnvType       config.EnvType          `bson:"env_type"          json:"env_type"`
	Production    bool                    `bson:"production"        json:"production"`
	Operation     config.EnvOperation     `bson:"operation"         json:"operation"`
	OperationType config.EnvOperationType `bson:"operation_type"    json:"operation_type"`
	ServiceName   string                  `bson:"service_name"      json:"service_name"`
	ServiceType   config.ServiceType      `bson:"service_type"      json:"service_type"`
	OriginService *ProductService         `bson:"origin_service"    json:"origin_service"`
	UpdateService *ProductService         `bson:"update_service"    json:"update_service"`
	OriginSaeApp  *SAEApplication         `bson:"origin_sae_app"    json:"origin_sae_app"`
	UpdateSaeApp  *SAEApplication         `bson:"update_sae_app"    json:"update_sae_app"`
	Detail        string                  `bson:"detail"            json:"detail"`
	CreatedBy     types.UserBriefInfo     `bson:"created_by"        json:"created_by"`
	CreatTime     int64                   `bson:"create_time"       json:"create_time"`
}

func (EnvInfo) TableName() string {
	return "env_info"
}
