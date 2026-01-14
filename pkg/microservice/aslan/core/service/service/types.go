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

package service

import (
	"encoding/json"
	"fmt"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/git"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

type LoadSource string

const (
	LoadFromRepo          LoadSource = "repo" //exclude gerrit
	LoadFromGerrit        LoadSource = "gerrit"
	LoadFromPublicRepo    LoadSource = "publicRepo"
	LoadFromChartTemplate LoadSource = "chartTemplate"
	LoadFromChartRepo     LoadSource = "chartRepo"
)

type HelmLoadSource struct {
	Source LoadSource `json:"source"`
}

type HelmServiceCreationArgs struct {
	HelmLoadSource
	Name           string                  `json:"name"`
	CreatedBy      string                  `json:"createdBy"`
	RequestID      string                  `json:"-"`
	AutoSync       bool                    `json:"auto_sync"`
	CreateFrom     interface{}             `json:"createFrom"`
	ValuesData     *service.ValuesDataArgs `json:"valuesData"`
	CreationDetail interface{}             `json:"-"`
	Production     bool                    `json:"production"`
}

type BulkHelmServiceCreationArgs struct {
	HelmLoadSource
	CreateFrom interface{}             `json:"createFrom"`
	CreatedBy  string                  `json:"createdBy"`
	RequestID  string                  `json:"-"`
	ValuesData *service.ValuesDataArgs `json:"valuesData"`
	AutoSync   bool                    `json:"auto_sync"`
	Production bool                    `json:"production"`
}

type FailedService struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type BulkHelmServiceCreationResponse struct {
	SuccessServices []string         `json:"successServices"`
	FailedServices  []*FailedService `json:"failedServices"`
}

type CreateFromRepo struct {
	CodehostID int      `json:"codehostID"`
	Owner      string   `json:"owner"`
	Namespace  string   `json:"namespace"`
	Repo       string   `json:"repo"`
	Branch     string   `json:"branch"`
	Paths      []string `json:"paths"`
}

type CreateFromPublicRepo struct {
	RepoLink string   `json:"repoLink"`
	Paths    []string `json:"paths"`
}

type CreateFromChartTemplate struct {
	TemplateName string      `json:"templateName"`
	ValuesYAML   string      `json:"valuesYAML"`
	Variables    []*Variable `json:"variables"`
}

type CreateFromChartRepo struct {
	ChartRepoName string `json:"chartRepoName"`
	ChartName     string `json:"chartName"`
	ChartVersion  string `json:"chartVersion"`
}

func PublicRepoToPrivateRepoArgs(args *CreateFromPublicRepo) (*CreateFromRepo, error) {
	if args.RepoLink == "" {
		return nil, fmt.Errorf("empty link")
	}
	owner, repo, err := git.ParseOwnerAndRepo(args.RepoLink)
	if err != nil {
		return nil, err
	}

	return &CreateFromRepo{
		Owner: owner,
		Repo:  repo,
		Paths: args.Paths,
	}, nil
}

func (a *HelmServiceCreationArgs) UnmarshalJSON(data []byte) error {
	s := &HelmLoadSource{}
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}

	switch s.Source {
	case LoadFromRepo:
		a.CreateFrom = &CreateFromRepo{}
	case LoadFromPublicRepo:
		a.CreateFrom = &CreateFromPublicRepo{}
	case LoadFromChartTemplate:
		a.CreateFrom = &CreateFromChartTemplate{}
	case LoadFromChartRepo:
		a.CreateFrom = &CreateFromChartRepo{}
	}

	type tmp HelmServiceCreationArgs

	return json.Unmarshal(data, (*tmp)(a))
}

func (a *BulkHelmServiceCreationArgs) UnmarshalJSON(data []byte) error {
	s := &HelmLoadSource{}
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}

	if s.Source == LoadFromChartTemplate {
		a.CreateFrom = &CreateFromChartTemplate{}
	}

	type tmp BulkHelmServiceCreationArgs

	return json.Unmarshal(data, (*tmp)(a))
}

type LoadServiceFromYamlTemplateReq struct {
	ServiceName        string                           `json:"service_name"`
	ProjectName        string                           `json:"project_name"`
	TemplateID         string                           `json:"template_id"`
	AutoSync           bool                             `json:"auto_sync"`
	VariableYaml       string                           `json:"variable_yaml"`
	ServiceVariableKVs []*commontypes.ServiceVariableKV `json:"service_variable_kvs"`
}

type OpenAPILoadServiceFromYamlTemplateReq struct {
	Production   bool         `json:"production"`
	ServiceName  string       `json:"service_name"`
	ProjectKey   string       `json:"project_key"`
	TemplateName string       `json:"template_name"`
	AutoSync     bool         `json:"auto_sync"`
	VariableYaml util.KVInput `json:"variable_yaml"`
}

type OpenAPIUpdateServiceConfigArgs struct {
	ProjectName string `json:"project_name" `
	ServiceName string `json:"service_name"`
	Type        string `json:"type"`
	Yaml        string `json:"yaml"`
}

func (o *OpenAPIUpdateServiceConfigArgs) Validate() error {
	if o.ProjectName == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if o.ServiceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if o.Type == "" {
		return fmt.Errorf("type cannot be empty")
	}

	if o.Yaml == "" {
		return fmt.Errorf("yaml cannot be empty")
	}

	return nil
}

func (req *OpenAPILoadServiceFromYamlTemplateReq) Validate() error {
	if req.ServiceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if req.ProjectKey == "" {
		return fmt.Errorf("project key cannot be empty")
	}

	if req.TemplateName == "" {
		return fmt.Errorf("template name cannot be empty")
	}

	return nil
}

type OpenAPICreateYamlServiceReq struct {
	ServiceName  string                           `json:"service_name"`
	Production   bool                             `json:"production"`
	Yaml         string                           `json:"yaml"`
	VariableYaml []*commontypes.ServiceVariableKV `json:"variable_yaml"`
}

func (req *OpenAPICreateYamlServiceReq) Validate() error {
	if req.ServiceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if req.Yaml == "" {
		return fmt.Errorf("yaml cannot be empty")
	}

	return nil
}

type OpenAPIGetYamlServiceResp struct {
	ServiceName        string                           `json:"service_name"`
	Source             string                           `json:"source"`
	Type               string                           `json:"type"`
	TemplateName       string                           `json:"template_name"`
	CreatedBy          string                           `json:"created_by"`
	CreatedTime        int64                            `json:"created_time"`
	Yaml               string                           `json:"yaml"`
	Containers         []*commonmodels.Container        `json:"containers"`
	ServiceVariableKvs []*commontypes.ServiceVariableKV `json:"service_variable_kvs"`
}

type OpenAPIServiceBrief struct {
	ServiceName string            `json:"service_name"`
	Source      string            `json:"source"`
	Type        string            `json:"type"`
	Containers  []*ContainerBrief `json:"containers"`
}

type ContainerBrief struct {
	Name      string `json:"name"`
	Image     string `json:"image"`
	ImageName string `json:"image_name"`
}

type OpenAPIUpdateServiceVariableRequest struct {
	ServiceVariableKVs []*commontypes.ServiceVariableKV `json:"service_variable_kvs" binding:"required"`
}

type OpenAPILoadHelmServiceFromTemplateReq struct {
	// 是否为生产服务
	Production bool `json:"production"`
	// 服务名称
	ServiceName string `json:"service_name" binding:"required"`
	// 项目标识
	ProjectKey string `json:"project_key" binding:"required"`
	// 模板名称
	TemplateName string `json:"template_name" binding:"required"`
	// 是否自动同步
	AutoSync bool `json:"auto_sync"`
	// Values YAML
	ValuesYaml string `json:"values_yaml"`
	// 变量
	Variables util.KVInput `json:"variables"`
}

func (req *OpenAPILoadHelmServiceFromTemplateReq) Validate() error {
	if req.ServiceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if req.ProjectKey == "" {
		return fmt.Errorf("project key cannot be empty")
	}

	if req.TemplateName == "" {
		return fmt.Errorf("template name cannot be empty")
	}

	return nil
}
