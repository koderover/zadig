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
// @Description 分页查询业务目录应用，支持按名称或唯一键模糊搜索、自定义字段过滤和排序；需要业务目录查看权限
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param body body service.OpenAPISearchApplicationsRequest true "分页、搜索、过滤和排序条件"
// @Success 200 {object} service.OpenAPISearchApplicationsResponse "查询成功，返回应用列表及分页信息"
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
// @Description 根据应用 ID 查询业务详情；需要业务目录查看权限
// @Tags OpenAPI
// @Produce json
// @Param id path string true "应用 ID"
// @Success 200 {object} service.OpenAPIApplication "查询成功，返回应用详情"
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
// @Description 创建业务目录应用；需要业务目录新建权限
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param body body service.OpenAPIApplicationRequest true "待创建的应用信息，name、key、project 必填"
// @Success 200 {object} service.OpenAPIApplication "创建成功，返回服务端生成 ID 和时间的应用对象"
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
// @Description 批量创建业务目录应用，所有应用校验通过后统一导入；需要业务目录新建权限
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param body body []service.OpenAPIApplicationRequest true "待创建的应用对象数组"
// @Success 200 "批量导入成功，无业务响应体"
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
// @Description 使用完整应用对象更新指定业务；应用 key 不允许修改，repository 使用代码源名称；需要业务目录编辑权限
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param id path string true "业务 ID"
// @Param body body service.OpenAPIApplicationRequest true "更新后的完整应用对象，name、key、project 必填"
// @Success 200 "更新成功，无业务响应体"
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
// @Description 删除指定业务并解除测试服务和生产服务关联；需要业务目录删除权限
// @Tags OpenAPI
// @Produce json
// @Param id path string true "业务 ID"
// @Success 200 "删除成功，无业务响应体"
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
// @Description 查询应用关联服务所在环境的项目、环境、部署类型、状态、镜像、Chart 版本和更新时间；需要业务目录查看权限
// @Tags OpenAPI
// @Produce json
// @Param id path string true "业务 ID"
// @Success 200 {array} service.GetBizDirServiceDetailResponse "查询成功，返回关联环境详情列表"
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
// @Description 查询内置字段和自定义字段定义，用于组装应用 custom_fields 和查询过滤条件；需要业务目录查看权限
// @Tags OpenAPI
// @Produce json
// @Success 200 {array} service.OpenAPIFieldDefinition "查询成功，返回业务模型字段定义列表"
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
// @Description 创建自定义业务模型字段；单选和多选类型必须提供 options，多选类型不支持 unique；需要业务目录编辑权限
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param body body service.OpenAPIFieldDefinitionRequest true "字段定义，key、name、type 必填"
// @Success 200 {object} service.OpenAPIFieldDefinition "创建成功，返回包含 ID、来源和时间的字段定义"
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
// @Description 更新指定业务模型字段；单选和多选类型必须提供 options，多选类型不支持 unique；需要业务目录编辑权限
// @Tags OpenAPI
// @Accept json
// @Param id path string true "字段 ID"
// @Param body body service.OpenAPIFieldDefinitionRequest true "更新后的完整字段定义，key、name、type 必填"
// @Success 200 "更新成功，无业务响应体"
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
// @Description 删除字段定义，并清理所有应用中的对应 custom_fields 数据和唯一索引；需要业务目录删除权限
// @Tags OpenAPI
// @Param id path string true "字段 ID"
// @Success 200 "删除成功，无业务响应体"
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
