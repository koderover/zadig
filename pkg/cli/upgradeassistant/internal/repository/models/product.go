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
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commontypes "github.com/koderover/zadig/pkg/microservice/aslan/core/common/types"
	"go.mongodb.org/mongo-driver/bson/primitive"

	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
)

type ProductAuthType string
type ProductPermission string

// Vars do not save, only input parameters
type Product struct {
	ID             primitive.ObjectID              `bson:"_id,omitempty"             json:"id,omitempty"`
	ProductName    string                          `bson:"product_name"              json:"product_name"`
	CreateTime     int64                           `bson:"create_time"               json:"create_time"`
	UpdateTime     int64                           `bson:"update_time"               json:"update_time"`
	Namespace      string                          `bson:"namespace,omitempty"       json:"namespace,omitempty"`
	Status         string                          `bson:"status"                    json:"status"`
	Revision       int64                           `bson:"revision"                  json:"revision"`
	Enabled        bool                            `bson:"enabled"                   json:"enabled"`
	EnvName        string                          `bson:"env_name"                  json:"env_name"`
	BaseEnvName    string                          `bson:"-"                         json:"base_env_name"`
	UpdateBy       string                          `bson:"update_by"                 json:"update_by"`
	Visibility     string                          `bson:"-"                         json:"visibility"`
	Services       [][]*models.ProductService      `bson:"services"                  json:"services"`
	Render         *RenderInfo                     `bson:"render"                    json:"render"` // Deprecated in 1.19.0, will be removed in 1.20.0
	Error          string                          `bson:"error"                     json:"error"`
	ServiceRenders []*templatemodels.ServiceRender `bson:"-"                         json:"chart_infos,omitempty"`
	IsPublic       bool                            `bson:"is_public"                 json:"isPublic"`
	RoleIDs        []int64                         `bson:"role_ids"                  json:"roleIds"`
	ClusterID      string                          `bson:"cluster_id,omitempty"      json:"cluster_id,omitempty"`
	RecycleDay     int                             `bson:"recycle_day"               json:"recycle_day"`
	Source         string                          `bson:"source"                    json:"source"`
	IsOpenSource   bool                            `bson:"is_opensource"             json:"is_opensource"`
	RegistryID     string                          `bson:"registry_id"               json:"registry_id"`
	BaseName       string                          `bson:"base_name"                 json:"base_name"`
	// IsExisted is true if this environment is created from an existing one
	IsExisted bool `bson:"is_existed"                json:"is_existed"`
	// TODO: Deprecated: temp flag
	IsForkedProduct bool `bson:"-" json:"-"`

	// New Since v1.11.0.
	//ShareEnv ProductShareEnv `bson:"share_env" json:"share_env"`

	// New Since v1.13.0.
	//EnvConfigs []*CreateUpdateCommonEnvCfgArgs `bson:"-"   json:"env_configs,omitempty"`

	// New Since v1.16.0, used to determine whether to install resources
	ServiceDeployStrategy map[string]string `bson:"service_deploy_strategy" json:"service_deploy_strategy"`

	// New Since v.1.18.0, env configs
	//AnalysisConfig      *AnalysisConfig       `bson:"analysis_config"      json:"analysis_config"`
	//NotificationConfigs []*NotificationConfig `bson:"notification_configs" json:"notification_configs"`

	// New Since v1.19.0, env sleep configs
	PreSleepStatus map[string]int `bson:"pre_sleep_status" json:"pre_sleep_status"`

	// New Since v1.19.0, for env global variables
	// GlobalValues for helm projects
	DefaultValues string                     `bson:"default_values,omitempty"       json:"default_values,omitempty"`
	YamlData      *templatemodels.CustomYaml `bson:"yaml_data,omitempty"            json:"yaml_data,omitempty"`
	// GlobalValues for k8s projects
	GlobalVariables []*commontypes.GlobalVariableKV `bson:"global_variables,omitempty"     json:"global_variables,omitempty"`

	// For production environment
	Production bool   `json:"production" bson:"production"`
	Alias      string `json:"alias" bson:"alias"`
}

type RenderInfo struct {
	Name        string `bson:"name"                     json:"name"`
	Revision    int64  `bson:"revision"                 json:"revision"`
	ProductTmpl string `bson:"product_tmpl"             json:"product_tmpl"`
	Description string `bson:"description"              json:"description"`
}

type ProductAuth struct {
	Type        ProductAuthType     `bson:"type"          json:"type"`
	Name        string              `bson:"name"          json:"name"`
	Permissions []ProductPermission `bson:"permissions"   json:"permissions"`
}

// ImagePathSpec paths in yaml used to parse image
type ImagePathSpec struct {
	Repo  string `bson:"repo,omitempty"           json:"repo,omitempty"`
	Image string `bson:"image,omitempty"           json:"image,omitempty"`
	Tag   string `bson:"tag,omitempty"           json:"tag,omitempty"`
}

type Container struct {
	Name      string         `bson:"name"           json:"name"`
	Image     string         `bson:"image"          json:"image"`
	ImagePath *ImagePathSpec `bson:"image_path,omitempty"          json:"imagePath,omitempty"`
}

type EnvConfig struct {
	EnvName string   `bson:"env_name,omitempty" json:"env_name"`
	HostIDs []string `bson:"host_ids,omitempty" json:"host_ids"`
	Labels  []string `bson:"labels,omitempty"   json:"labels"`
}

type ProductService struct {
	ServiceName string       `bson:"service_name"               json:"service_name"`
	ProductName string       `bson:"product_name"               json:"product_name"`
	Type        string       `bson:"type"                       json:"type"`
	Revision    int64        `bson:"revision"                   json:"revision"`
	Containers  []*Container `bson:"containers"                 json:"containers,omitempty"`
	//Render      *RenderInfo  `bson:"render,omitempty"           json:"render,omitempty"` // Record the render information of each service to facilitate the update of a single service
	Render     *models.RenderInfo `bson:"render"`
	EnvConfigs []*EnvConfig       `bson:"-"                          json:"env_configs,omitempty"`
}

type ServiceConfig struct {
	ConfigName string `bson:"config_name"           json:"config_name"`
	Revision   int64  `bson:"revision"              json:"revision"`
}

func (*Product) TableName() string {
	return "product"
}

//func (p *Product) GetServiceMap() map[string]*ProductService {
//	ret := make(map[string]*ProductService)
//	for _, group := range p.Services {
//		for _, svc := range group {
//			ret[svc.ServiceName] = svc
//		}
//	}
//
//	return ret
//}
