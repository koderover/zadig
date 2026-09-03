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
	CodehostName string `json:"codehost_name"`
	RepoOwner    string `json:"repo_owner"`
	RepoName     string `json:"repo_name"`
}

type OpenAPIApplicationRepository struct {
	CodehostID int    `json:"codehost_id"`
	RepoOwner  string `json:"repo_owner"`
	RepoName   string `json:"repo_name"`
}

type OpenAPIApplicationRequest struct {
	Name                  string                               `json:"name"`
	Key                   string                               `json:"key"`
	Project               string                               `json:"project"`
	Type                  string                               `json:"type"`
	Owner                 string                               `json:"owner"`
	Description           string                               `json:"description"`
	Repository            *OpenAPIApplicationRepositoryRequest `json:"repository,omitempty"`
	TestingServiceName    string                               `json:"testing_service_name"`
	ProductionServiceName string                               `json:"production_service_name"`
	CustomFields          map[string]interface{}               `json:"custom_fields"`
}

type OpenAPIApplication struct {
	ID                    string                        `json:"id"`
	Name                  string                        `json:"name"`
	Key                   string                        `json:"key"`
	Project               string                        `json:"project"`
	Type                  string                        `json:"type"`
	Owner                 string                        `json:"owner"`
	Description           string                        `json:"description"`
	Repository            *OpenAPIApplicationRepository `json:"repository,omitempty"`
	TestingServiceName    string                        `json:"testing_service_name"`
	ProductionServiceName string                        `json:"production_service_name"`
	CustomFields          map[string]interface{}        `json:"custom_fields"`
	CreateTime            int64                         `json:"create_time"`
	UpdateTime            int64                         `json:"update_time"`
}

type OpenAPIApplicationFilter struct {
	Field string      `json:"field"`
	Verb  string      `json:"verb"`
	Value interface{} `json:"value"`
}

type OpenAPISearchApplicationsRequest struct {
	Page      int64                       `json:"page"`
	PageSize  int64                       `json:"page_size"`
	Query     string                      `json:"query"`
	Filters   []*OpenAPIApplicationFilter `json:"filters"`
	SortBy    string                      `json:"sort_by"`
	SortOrder string                      `json:"sort_order"`
}

type OpenAPISearchApplicationsResponse struct {
	Items    []*OpenAPIApplication `json:"items"`
	Page     int64                 `json:"page"`
	PageSize int64                 `json:"page_size"`
	Total    int64                 `json:"total"`
}

type OpenAPIFieldDefinitionRequest struct {
	Key         string                            `json:"key"`
	Name        string                            `json:"name"`
	Type        config.ApplicationCustomFieldType `json:"type"`
	Default     interface{}                       `json:"default"`
	Options     []string                          `json:"options,omitempty"`
	Unique      bool                              `json:"unique"`
	Required    bool                              `json:"required"`
	ShowInList  bool                              `json:"show_in_list"`
	Description string                            `json:"description"`
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
