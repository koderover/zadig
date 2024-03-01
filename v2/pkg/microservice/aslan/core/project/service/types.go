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

package service

import (
	"errors"
	"regexp"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/util"
)

type OpenAPICreateProductReq struct {
	ProjectName string             `json:"project_name"`
	ProjectKey  string             `json:"project_key"`
	IsPublic    bool               `json:"is_public"`
	Description string             `json:"description"`
	ProjectType config.ProjectType `json:"project_type"`
}

func (req OpenAPICreateProductReq) Validate() error {
	if req.ProjectName == "" {
		return errors.New("project_name cannot be empty")
	}

	match, err := regexp.MatchString(setting.ProjectKeyRegEx, req.ProjectKey)
	if err != nil || !match {
		return errors.New(`project key should match regex: ^[a-z-\\d]+$`)
	}

	switch req.ProjectType {
	case config.ProjectTypeLoaded, config.ProjectTypeYaml, config.ProjectTypeHelm, config.ProjectTypeVM:
		break
	default:
		return errors.New("unsupported project type")
	}

	return nil
}

type OpenAPIInitializeProjectReq struct {
	ProjectName string               `json:"project_name"`
	ProjectKey  string               `json:"project_key"`
	IsPublic    bool                 `json:"is_public"`
	Description string               `json:"description"`
	ServiceList []*ServiceDefinition `json:"service_list"`
	EnvList     []*EnvDefinition     `json:"env_list"`
}

type ServiceDefinition struct {
	Source       string       `json:"source"`
	ServiceName  string       `json:"service_name"`
	TemplateName string       `json:"template_name"`
	VariableYaml util.KVInput `json:"variable_yaml"`
	AutoSync     bool         `json:"auto_sync"`
	Yaml         string       `json:"yaml"`
	ValuesYaml   string       `json:"values_yaml"`
}

type EnvDefinition struct {
	EnvName     string `json:"env_key"`
	ClusterName string `json:"cluster_name"`
	Namespace   string `json:"namespace"`
}

type OpenAPIListProjectReq struct {
	PageSize int64 `json:"pageSize" form:"pageSize,default=20"`
	PageNum  int64 `json:"pageNum"  form:"pageNum,default=1"`
}

type OpenAPIProjectDetailResp struct {
	ProjectName string `json:"project_name"`
	ProjectKey  string `json:"project_key"`
	IsPublic    bool   `json:"is_public"`
	Desc        string `json:"desc"`
	DeployType  string `json:"deploy_type"`
	CreateTime  int64  `json:"create_time"`
	CreatedBy   string `json:"created_by"`
}

type OpenAPIProjectListResp struct {
	Total    int             `json:"total"`
	Projects []*ProjectBrief `json:"projects"`
}

type ProjectBrief struct {
	ProjectName string `json:"project_name"`
	ProjectKey  string `json:"project_key"`
	Description string `json:"description"`
	DeployType  string `json:"deploy_type"`
}

type EnvInfo struct {
	Name       string `json:"name"`
	Production bool   `json:"production"`
	Status     string `json:"status"`
}

type Service struct {
	Name       string `json:"name"`
	CreatedBy  string `json:"created_by"`
	CreateTime int64  `json:"create_time"`
	Source     string `json:"source"`
	Production bool   `json:"production"`
}

func (req OpenAPIInitializeProjectReq) Validate() error {
	if req.ProjectName == "" {
		return errors.New("project_name cannot be empty")
	}

	match, err := regexp.MatchString(setting.ProjectKeyRegEx, req.ProjectKey)
	if err != nil || !match {
		return errors.New(`project key should match regex: ^[a-z-\\d]+$`)
	}

	if len(req.ServiceList) == 0 {
		return errors.New("initializing a project with no services is not allowed")
	}

	for _, serviceDef := range req.ServiceList {
		if serviceDef.ServiceName == "" {
			return errors.New("service_name cannot be empty")
		}
		switch serviceDef.Source {
		case config.SourceFromTemplate:
			if serviceDef.TemplateName == "" {
				return errors.New("template_name cannot be empty when the service source is template")
			}
		case config.SourceFromYaml:
			if serviceDef.Yaml == "" {
				return errors.New("yaml cannot be empty when the service source is yaml")
			}
		default:
			return errors.New("source of a service can only be of template or yaml")
		}
	}

	return nil
}

type ProjectGroupArgs struct {
	GroupID     string   `json:"group_id"`
	GroupName   string   `json:"group_name"`
	ProjectKeys []string `json:"project_keys"`
}

func (args *ProjectGroupArgs) Validate() error {
	if args.GroupName == "" {
		return errors.New("group_name cannot be empty")
	}

	return nil
}

type ProjectGroupPreset struct {
	GroupID   string                  `json:"group_id"`
	GroupName string                  `json:"group_name"`
	Projects  []*ProjectGroupRelation `json:"projects"`
}

type ProjectGroupRelation struct {
	ProjectName string `json:"project_name"`
	ProjectKey  string `json:"project_key"`
	DeployType  string `json:"deploy_type"`
	Enabled     bool   `json:"enabled"`
}
