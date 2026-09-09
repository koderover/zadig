/*
Copyright 2026 The KodeRover Authors.

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
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	codehostrepo "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type OpenAPIApplicationRepositoryRequest struct {
	// 代码源名称
	CodehostName string `json:"codehost_name"`
	// 仓库所属用户或组织
	RepoOwner string `json:"repo_owner"`
	// 仓库名称
	RepoName string `json:"repo_name"`
}

type OpenAPIApplicationRepository struct {
	// 代码源 ID
	CodehostID int `json:"codehost_id"`
	// 仓库所属用户或组织
	RepoOwner string `json:"repo_owner"`
	// 仓库名称
	RepoName string `json:"repo_name"`
}

type OpenAPIApplicationRequest struct {
	// 应用名称，必填
	Name string `json:"name"`
	// 应用唯一键，必填，创建后不可修改
	Key string `json:"key"`
	// 关联项目标识，必填
	Project string `json:"project"`
	// 应用类型，例如 service、component、application、library
	Type string `json:"type"`
	// 应用负责人用户 ID
	Owner string `json:"owner"`
	// 应用描述
	Description string `json:"description"`
	// 关联代码仓库；传入时 codehost_name、repo_owner、repo_name 必填
	Repository *OpenAPIApplicationRepositoryRequest `json:"repository,omitempty"`
	// 关联的测试服务名称
	TestingServiceName string `json:"testing_service_name"`
	// 关联的生产服务名称
	ProductionServiceName string `json:"production_service_name"`
	// 自定义字段，键来自业务模型字段定义
	CustomFields map[string]interface{} `json:"custom_fields"`
}

type OpenAPIApplication struct {
	// 应用 ID
	ID string `json:"id"`
	// 应用名称
	Name string `json:"name"`
	// 应用唯一键
	Key string `json:"key"`
	// 关联项目标识
	Project string `json:"project"`
	// 应用类型
	Type string `json:"type"`
	// 应用负责人用户 ID
	Owner string `json:"owner"`
	// 应用描述
	Description string `json:"description"`
	// 关联代码仓库
	Repository *OpenAPIApplicationRepository `json:"repository,omitempty"`
	// 关联的测试服务名称
	TestingServiceName string `json:"testing_service_name"`
	// 关联的生产服务名称
	ProductionServiceName string `json:"production_service_name"`
	// 自定义字段
	CustomFields map[string]interface{} `json:"custom_fields"`
	// 创建时间，Unix 时间戳，单位为秒
	CreateTime int64 `json:"create_time"`
	// 更新时间，Unix 时间戳，单位为秒
	UpdateTime int64 `json:"update_time"`
}

type OpenAPIApplicationFilter struct {
	// 过滤字段；自定义字段格式为 custom_fields.{field_key}
	Field string `json:"field"`
	// 过滤操作符，具体取值取决于字段类型
	Verb string `json:"verb"`
	// 过滤值，类型必须与字段定义类型匹配
	Value interface{} `json:"value"`
}

type OpenAPISearchApplicationsRequest struct {
	// 页码，从 1 开始，默认 1
	Page int64 `json:"page"`
	// 每页数量，默认 20
	PageSize int64 `json:"page_size"`
	// 按应用名称或唯一键进行不区分大小写的模糊查询
	Query string `json:"query"`
	// 过滤条件列表
	Filters []*OpenAPIApplicationFilter `json:"filters"`
	// 排序字段，默认 update_time
	SortBy string `json:"sort_by"`
	// 排序方向，可选 asc、desc，默认 asc
	SortOrder string `json:"sort_order"`
}

type OpenAPISearchApplicationsResponse struct {
	// 当前页应用列表
	Items []*OpenAPIApplication `json:"items"`
	// 当前页码
	Page int64 `json:"page"`
	// 每页数量
	PageSize int64 `json:"page_size"`
	// 符合查询条件的应用总数
	Total int64 `json:"total"`
}

type OpenAPIFieldDefinitionRequest struct {
	// 字段唯一键，对应 custom_fields.{key}
	Key string `json:"key"`
	// 字段展示名称
	Name string `json:"name"`
	// 字段类型
	Type config.ApplicationCustomFieldType `json:"type"`
	// 默认值，类型必须与 type 匹配
	Default interface{} `json:"default"`
	// 单选或多选字段的可选项
	Options []string `json:"options,omitempty"`
	// 是否要求字段值唯一
	Unique bool `json:"unique"`
	// 是否必填
	Required bool `json:"required"`
	// 是否在应用列表中展示
	ShowInList bool `json:"show_in_list"`
	// 字段描述
	Description string `json:"description"`
}

type OpenAPIFieldDefinition = commonmodels.ApplicationFieldDefinition

func (req *OpenAPIApplicationRequest) validate() error {
	if req == nil {
		return e.ErrInvalidParam.AddDesc("empty body")
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Key) == "" || strings.TrimSpace(req.Project) == "" {
		return e.ErrInvalidParam.AddDesc("name, key, project are required")
	}
	if req.Repository != nil && (strings.TrimSpace(req.Repository.CodehostName) == "" || strings.TrimSpace(req.Repository.RepoOwner) == "" || strings.TrimSpace(req.Repository.RepoName) == "") {
		return e.ErrInvalidParam.AddDesc("repository.codehost_name, repository.repo_owner, repository.repo_name are required")
	}
	return nil
}

func applicationFromOpenAPI(req *OpenAPIApplicationRequest, resolveCodehostID func(string) (int, error)) (*commonmodels.Application, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}

	app := &commonmodels.Application{
		Name:                  req.Name,
		Key:                   req.Key,
		Project:               req.Project,
		Type:                  req.Type,
		Owner:                 req.Owner,
		Description:           req.Description,
		TestingServiceName:    req.TestingServiceName,
		ProductionServiceName: req.ProductionServiceName,
		CustomFields:          req.CustomFields,
	}
	if req.Repository == nil {
		return app, nil
	}

	codehostID, err := resolveCodehostID(req.Repository.CodehostName)
	if err != nil {
		return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("failed to find codehost %q: %s", req.Repository.CodehostName, err))
	}
	app.Repository = &commonmodels.ApplicationRepositoryRef{
		CodehostID: codehostID,
		RepoOwner:  req.Repository.RepoOwner,
		RepoName:   req.Repository.RepoName,
	}
	return app, nil
}

func applicationToOpenAPI(app *commonmodels.Application) *OpenAPIApplication {
	if app == nil {
		return nil
	}

	customFields := app.CustomFields
	if customFields == nil {
		customFields = map[string]interface{}{}
	}
	resp := &OpenAPIApplication{
		ID:                    app.ID.Hex(),
		Name:                  app.Name,
		Key:                   app.Key,
		Project:               app.Project,
		Type:                  app.Type,
		Owner:                 app.Owner,
		Description:           app.Description,
		TestingServiceName:    app.TestingServiceName,
		ProductionServiceName: app.ProductionServiceName,
		CustomFields:          customFields,
		CreateTime:            app.CreateTime,
		UpdateTime:            app.UpdateTime,
	}
	if app.Repository == nil {
		return resp
	}

	resp.Repository = &OpenAPIApplicationRepository{
		CodehostID: app.Repository.CodehostID,
		RepoOwner:  app.Repository.RepoOwner,
		RepoName:   app.Repository.RepoName,
	}
	return resp
}

func resolveCodehostID(name string) (int, error) {
	codehost, err := codehostrepo.NewCodehostColl().GetSystemCodeHostByAlias(name)
	if err != nil {
		return 0, err
	}
	return codehost.ID, nil
}

func CreateApplicationOpenAPI(req *OpenAPIApplicationRequest, logger *zap.SugaredLogger) (*OpenAPIApplication, error) {
	app, err := applicationFromOpenAPI(req, resolveCodehostID)
	if err != nil {
		return nil, err
	}
	created, err := CreateApplication(app, logger)
	if err != nil {
		return nil, err
	}
	return applicationToOpenAPI(created), nil
}

func BulkCreateApplicationsOpenAPI(reqs []*OpenAPIApplicationRequest, logger *zap.SugaredLogger) error {
	apps := make([]*commonmodels.Application, 0, len(reqs))
	for _, req := range reqs {
		app, err := applicationFromOpenAPI(req, resolveCodehostID)
		if err != nil {
			return err
		}
		apps = append(apps, app)
	}
	return BulkCreateApplications(apps, logger)
}

func GetApplicationOpenAPI(id string) (*OpenAPIApplication, error) {
	app, err := commonrepo.NewApplicationColl().GetByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return applicationToOpenAPI(app), nil
}

func UpdateApplicationOpenAPI(id string, req *OpenAPIApplicationRequest, logger *zap.SugaredLogger) error {
	app, err := applicationFromOpenAPI(req, resolveCodehostID)
	if err != nil {
		return err
	}
	return UpdateApplication(id, app, logger)
}

func SearchApplicationsOpenAPI(req *OpenAPISearchApplicationsRequest, logger *zap.SugaredLogger) (*OpenAPISearchApplicationsResponse, error) {
	filters := make([]*Filter, 0, len(req.Filters))
	for _, filter := range req.Filters {
		if filter == nil {
			continue
		}
		filters = append(filters, &Filter{Field: filter.Field, Verb: filter.Verb, Value: filter.Value})
	}
	searchReq := &SearchApplicationsRequest{
		Page:      req.Page,
		PageSize:  req.PageSize,
		Query:     req.Query,
		Filters:   filters,
		SortBy:    req.SortBy,
		SortOrder: req.SortOrder,
	}
	apps, total, err := SearchApplications(searchReq, logger)
	if err != nil {
		return nil, err
	}
	items := make([]*OpenAPIApplication, 0, len(apps))
	for _, app := range apps {
		items = append(items, applicationToOpenAPI(app))
	}
	return &OpenAPISearchApplicationsResponse{Items: items, Page: searchReq.Page, PageSize: searchReq.PageSize, Total: total}, nil
}

func CreateFieldDefinitionOpenAPI(req *OpenAPIFieldDefinitionRequest, logger *zap.SugaredLogger) (*commonmodels.ApplicationFieldDefinition, error) {
	return CreateFieldDefinition(&commonmodels.ApplicationFieldDefinition{
		Key:         req.Key,
		Name:        req.Name,
		Type:        req.Type,
		Default:     req.Default,
		Options:     req.Options,
		Unique:      req.Unique,
		Required:    req.Required,
		ShowInList:  req.ShowInList,
		Description: req.Description,
	}, logger)
}

func UpdateFieldDefinitionOpenAPI(id string, req *OpenAPIFieldDefinitionRequest, logger *zap.SugaredLogger) error {
	defs, err := ListFieldDefinitions(logger)
	if err != nil {
		return err
	}
	var existing *commonmodels.ApplicationFieldDefinition
	for _, def := range defs {
		if def.ID.Hex() == id {
			existing = def
			break
		}
	}
	if existing == nil {
		return fmt.Errorf("field definition %s not found", id)
	}

	def := &commonmodels.ApplicationFieldDefinition{
		Key:         req.Key,
		Name:        req.Name,
		Type:        req.Type,
		Default:     req.Default,
		Options:     req.Options,
		Unique:      req.Unique,
		Required:    req.Required,
		ShowInList:  req.ShowInList,
		Description: req.Description,
		Source:      existing.Source,
		CreateTime:  existing.CreateTime,
	}
	return UpdateFieldDefinition(id, def, logger)
}
