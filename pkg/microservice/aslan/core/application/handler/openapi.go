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

package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/application/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// OpenAPISearchApplications godoc
// @Summary 查询业务目录应用列表
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param body body service.OpenAPISearchApplicationsRequest true "查询条件"
// @Success 200 {object} service.OpenAPISearchApplicationsResponse
// @Router /openapi/application/applications/search [post]
func OpenAPISearchApplications(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.BusinessDirectory.View {
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPISearchApplicationsRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.RespErr = service.SearchApplicationsOpenAPI(req, ctx.Logger)
}

// OpenAPIGetApplication godoc
// @Summary 查询指定业务详情
// @Tags OpenAPI
// @Produce json
// @Param id path string true "业务 ID"
// @Success 200 {object} service.OpenAPIApplication
// @Router /openapi/application/applications/{id} [get]
func OpenAPIGetApplication(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.BusinessDirectory.View {
		ctx.UnAuthorized = true
		return
	}
	ctx.Resp, ctx.RespErr = service.GetApplicationOpenAPI(c.Param("id"))
}

// OpenAPICreateApplication godoc
// @Summary 创建业务
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param body body service.OpenAPIApplicationRequest true "业务对象"
// @Success 200 {object} service.OpenAPIApplication
// @Router /openapi/application/applications [post]
func OpenAPICreateApplication(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.BusinessDirectory.Create {
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPIApplicationRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.RespErr = service.CreateApplicationOpenAPI(req, ctx.Logger)
}

// OpenAPIBulkCreateApplications godoc
// @Summary 批量导入业务
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param body body []service.OpenAPIApplicationRequest true "业务对象数组"
// @Success 200
// @Router /openapi/application/applications/bulk [post]
func OpenAPIBulkCreateApplications(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.BusinessDirectory.Create {
		ctx.UnAuthorized = true
		return
	}

	var reqs []*service.OpenAPIApplicationRequest
	if err := c.ShouldBindJSON(&reqs); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err := service.BulkCreateApplicationsOpenAPI(reqs, ctx.Logger); err != nil {
		ctx.RespErr = err
		return
	}
}

// OpenAPIUpdateApplication godoc
// @Summary 编辑业务
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param id path string true "业务 ID"
// @Param body body service.OpenAPIApplicationRequest true "更新后的业务对象"
// @Success 200
// @Router /openapi/application/applications/{id} [put]
func OpenAPIUpdateApplication(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.BusinessDirectory.Edit {
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPIApplicationRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err := service.UpdateApplicationOpenAPI(c.Param("id"), req, ctx.Logger); err != nil {
		ctx.RespErr = err
		return
	}
}

// OpenAPIDeleteApplication godoc
// @Summary 删除业务
// @Tags OpenAPI
// @Produce json
// @Param id path string true "业务 ID"
// @Success 200
// @Router /openapi/application/applications/{id} [delete]
func OpenAPIDeleteApplication(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.BusinessDirectory.Delete {
		ctx.UnAuthorized = true
		return
	}
	if err := service.DeleteApplication(c.Param("id"), ctx.Logger); err != nil {
		ctx.RespErr = err
		return
	}
}

// OpenAPIListApplicationEnvs godoc
// @Summary 查询业务关联环境
// @Tags OpenAPI
// @Produce json
// @Param id path string true "业务 ID"
// @Success 200 {array} service.GetBizDirServiceDetailResponse
// @Router /openapi/application/applications/{id}/envs [get]
func OpenAPIListApplicationEnvs(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.BusinessDirectory.View {
		ctx.UnAuthorized = true
		return
	}
	ctx.Resp, ctx.RespErr = service.ListApplicationEnvs(c.Param("id"), ctx.Logger)
}

// OpenAPIListFieldDefinitions godoc
// @Summary 查询业务模型字段列表
// @Tags OpenAPI
// @Produce json
// @Success 200 {array} service.OpenAPIFieldDefinition
// @Router /openapi/application/fields [get]
func OpenAPIListFieldDefinitions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.BusinessDirectory.View {
		ctx.UnAuthorized = true
		return
	}
	ctx.Resp, ctx.RespErr = service.ListFieldDefinitions(ctx.Logger)
}

// OpenAPICreateFieldDefinition godoc
// @Summary 创建业务模型字段
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param body body service.OpenAPIFieldDefinitionRequest true "字段定义"
// @Success 200 {object} service.OpenAPIFieldDefinition
// @Router /openapi/application/fields [post]
func OpenAPICreateFieldDefinition(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.BusinessDirectory.Edit {
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPIFieldDefinitionRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.RespErr = service.CreateFieldDefinitionOpenAPI(req, ctx.Logger)
}

// OpenAPIUpdateFieldDefinition godoc
// @Summary 编辑业务模型字段
// @Tags OpenAPI
// @Accept json
// @Param id path string true "字段 ID"
// @Param body body service.OpenAPIFieldDefinitionRequest true "更新后的字段定义"
// @Success 200
// @Router /openapi/application/fields/{id} [put]
func OpenAPIUpdateFieldDefinition(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.BusinessDirectory.Edit {
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPIFieldDefinitionRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.RespErr = service.UpdateFieldDefinitionOpenAPI(c.Param("id"), req, ctx.Logger)
}

// OpenAPIDeleteFieldDefinition godoc
// @Summary 删除业务模型字段
// @Tags OpenAPI
// @Param id path string true "字段 ID"
// @Success 200
// @Router /openapi/application/fields/{id} [delete]
func OpenAPIDeleteFieldDefinition(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.BusinessDirectory.Delete {
		ctx.UnAuthorized = true
		return
	}
	ctx.RespErr = service.DeleteFieldDefinition(c.Param("id"), ctx.Logger)
}
