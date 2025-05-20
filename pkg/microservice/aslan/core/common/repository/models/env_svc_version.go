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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
)

type EnvServiceVersion struct {
	ID             primitive.ObjectID             `bson:"_id,omitempty"             json:"id,omitempty"`
	ProductName    string                         `bson:"product_name"              json:"product_name"`
	EnvName        string                         `bson:"env_name"                  json:"env_name"`
	Namespace      string                         `bson:"namespace"                 json:"namespace"`
	Production     bool                           `bson:"production"                json:"production"`
	Revision       int64                          `bson:"revision"                  json:"revision"`
	Operation      config.EnvOperation            `bson:"operation"                 json:"operation"`
	Detail         string                         `bson:"detail"                    json:"detail"`
	ProductFeature *templatemodels.ProductFeature `bson:"product_feature" json:"product_feature"`
	Service        *ProductService                `bson:"service"                   json:"service"`
	// env global variables
	// GlobalValues for helm projects
	DefaultValues string                     `bson:"default_values,omitempty"       json:"default_values,omitempty"`
	YamlData      *templatemodels.CustomYaml `bson:"yaml_data,omitempty"            json:"yaml_data,omitempty"`
	// GlobalValues for k8s projects
	GlobalVariables []*commontypes.GlobalVariableKV `bson:"global_variables,omitempty"     json:"global_variables,omitempty"`
	CreateBy        string                          `bson:"create_by"                 json:"create_by"`
	CreateTime      int64                           `bson:"create_time"               json:"create_time"`
	UpdateTime      int64                           `bson:"update_time"               json:"update_time"`
}

func (EnvServiceVersion) TableName() string {
	return "env_service_version"
}
