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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/templatestore/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type openAPIBuildTemplatePageQuery struct {
	PageNum  int `form:"pageNum,default=1"`
	PageSize int `form:"pageSize,default=20"`
}

func OpenAPIListBuildTemplates(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.Template.View {
		ctx.UnAuthorized = true
		return
	}

	query := new(openAPIBuildTemplatePageQuery)
	if err := c.ShouldBindQuery(query); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if query.PageNum <= 0 || query.PageSize <= 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("pageNum and pageSize must be greater than 0")
		return
	}
	ctx.Resp, ctx.RespErr = service.OpenAPIListBuildTemplates(query.PageNum, query.PageSize)
}

func OpenAPIGetBuildTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.Template.View {
		ctx.UnAuthorized = true
		return
	}
	if !primitive.IsValidObjectID(c.Param("id")) {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid build template id")
		return
	}
	ctx.Resp, ctx.RespErr = service.OpenAPIGetBuildTemplate(c.Param("id"), ctx.Logger)
}

func OpenAPICreateBuildTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.Template.Create {
		ctx.UnAuthorized = true
		return
	}

	data, err := internalhandler.GetRawData(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid build template request body")
		return
	}
	req := new(service.OpenAPIBuildTemplateInput)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid build template request body")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "新增", "模板-构建", req.Name, req.Name, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = service.OpenAPICreateBuildTemplate(req, ctx.UserName, ctx.Logger)
}

func OpenAPIUpdateBuildTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.Template.Edit {
		ctx.UnAuthorized = true
		return
	}
	if !primitive.IsValidObjectID(c.Param("id")) {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid build template id")
		return
	}

	data, err := internalhandler.GetRawData(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid build template request body")
		return
	}
	req := new(service.OpenAPIBuildTemplateInput)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid build template request body")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "更新", "模板-构建", c.Param("id"), c.Param("id"), string(data), types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = service.OpenAPIUpdateBuildTemplate(c.Param("id"), req, ctx.UserName, ctx.Logger)
}

func OpenAPIDeleteBuildTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.Template.Delete {
		ctx.UnAuthorized = true
		return
	}
	if !primitive.IsValidObjectID(c.Param("id")) {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid build template id")
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "删除", "模板-构建", c.Param("id"), c.Param("id"), "", types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = service.RemoveBuildTemplate(c.Param("id"), ctx.Logger)
}
