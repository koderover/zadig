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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
)

type ProductAuthType string
type ProductPermission string

// Vars不做保存，只做input参数
type Product struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty"             json:"id,omitempty"`
	ProductName string              `bson:"product_name"              json:"product_name"`
	CreateTime  int64               `bson:"create_time"               json:"create_time"`
	UpdateTime  int64               `bson:"update_time"               json:"update_time"`
	Namespace   string              `bson:"namespace,omitempty"       json:"namespace,omitempty"`
	Status      string              `bson:"status"                    json:"status"`
	Revision    int64               `bson:"revision"                  json:"revision"`
	Enabled     bool                `bson:"enabled"                   json:"enabled"`
	EnvName     string              `bson:"env_name"                  json:"env_name"`
	BaseEnvName string              `bson:"-"                         json:"base_env_name"`
	UpdateBy    string              `bson:"update_by"                 json:"update_by"`
	Auth        []*ProductAuth      `bson:"auth"                      json:"auth"`
	Visibility  string              `bson:"-"                         json:"visibility"`
	Services    [][]*ProductService `bson:"services"                  json:"services"`
	Render      *RenderInfo         `bson:"render"                    json:"render"`
	Error       string              `bson:"error"                     json:"error"`
	//Vars           []*templatemodels.RenderKV      `bson:"_,omitempty"               json:"vars,omitempty"`
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
	ShareEnv ProductShareEnv `bson:"share_env" json:"share_env"`

	// New Since v1.13.0.
	EnvConfigs []*CreateUpdateCommonEnvCfgArgs `bson:"-"   json:"env_configs,omitempty"`

	// New Since v1.16.0, used to determine whether to install resources
	ServiceDeployStrategy map[string]string `bson:"service_deploy_strategy" json:"service_deploy_strategy"`

	// For production environment
	Production bool   `json:"production" bson:"production"`
	Alias      string `json:"alias" bson:"alias"`
}

type CreateUpdateCommonEnvCfgArgs struct {
	EnvName              string                        `json:"env_name"`
	ProductName          string                        `json:"product_name"`
	ServiceName          string                        `json:"service_name"`
	Name                 string                        `json:"name"`
	YamlData             string                        `json:"yaml_data"`
	RestartAssociatedSvc bool                          `json:"restart_associated_svc"`
	CommonEnvCfgType     config.CommonEnvCfgType       `json:"common_env_cfg_type"`
	Services             []string                      `json:"services"`
	GitRepoConfig        *templatemodels.GitRepoConfig `json:"git_repo_config"`
	SourceDetail         *CreateFromRepo               `json:"-"`
	AutoSync             bool                          `json:"auto_sync"`
	LatestEnvResource    *EnvResource                  `json:"-"`
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

type ProductService struct {
	ServiceName string       `bson:"service_name"               json:"service_name"`
	ProductName string       `bson:"product_name"               json:"product_name"`
	Type        string       `bson:"type"                       json:"type"`
	Revision    int64        `bson:"revision"                   json:"revision"`
	Containers  []*Container `bson:"containers"                 json:"containers,omitempty"`
	//Render      *RenderInfo  `bson:"render,omitempty"           json:"render,omitempty"` // Deprecated
	Error        string       `bson:"error,omitempty"            json:"error,omitempty"`
	EnvConfigs   []*EnvConfig `bson:"-"                          json:"env_configs,omitempty"`
	VariableYaml string       `bson:"-"                          json:"variable_yaml,omitempty"`
}

type ServiceConfig struct {
	ConfigName string `bson:"config_name"           json:"config_name"`
	Revision   int64  `bson:"revision"              json:"revision"`
}

type ProductShareEnv struct {
	Enable  bool   `bson:"enable"   json:"enable"`
	IsBase  bool   `bson:"is_base"  json:"is_base"`
	BaseEnv string `bson:"base_env" json:"base_env"`
}

func (Product) TableName() string {
	return "product"
}

// GetNamespace returns the default name of namespace created by zadig
func (p *Product) GetNamespace() string {
	return p.ProductName + "-env-" + p.EnvName
}

func (p *Product) GetGroupServiceNames() [][]string {
	var resp [][]string
	for _, group := range p.Services {
		services := make([]string, 0, len(group))
		for _, service := range group {
			services = append(services, service.ServiceName)
		}
		resp = append(resp, services)
	}
	return resp
}

func (p *Product) GetServiceMap() map[string]*ProductService {
	ret := make(map[string]*ProductService)
	for _, group := range p.Services {
		for _, svc := range group {
			ret[svc.ServiceName] = svc
		}
	}

	return ret
}

func (p *Product) GetProductSvcNames() []string {
	ret := make([]string, 0)
	for _, group := range p.Services {
		for _, svc := range group {
			ret = append(ret, svc.ServiceName)
		}
	}
	return ret
}

// EnsureRenderInfo For some old data, the render data mayby nil
func (p *Product) EnsureRenderInfo() {
	if p.Render != nil {
		return
	}
	p.Render = &RenderInfo{ProductTmpl: p.ProductName, Name: p.Namespace}
}
