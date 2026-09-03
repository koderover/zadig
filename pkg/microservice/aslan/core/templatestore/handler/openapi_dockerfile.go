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
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/template"
	templateservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/templatestore/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type openAPIDockerfilePageQuery struct {
	PageNum  int `form:"pageNum,default=1"`
	PageSize int `form:"pageSize,default=20"`
}

type openAPIDockerfileTemplateList struct {
	Total               int                              `json:"total"`
	DockerfileTemplates []*template.DockerfileListObject `json:"dockerfile_templates"`
}

func OpenAPIListDockerfileTemplates(c *gin.Context) {
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
	query := new(openAPIDockerfilePageQuery)
	if err := c.ShouldBindQuery(query); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if query.PageNum <= 0 || query.PageSize <= 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("pageNum and pageSize must be greater than 0")
		return
	}
	templates, total, err := templateservice.ListDockerfileTemplate(query.PageNum, query.PageSize, ctx.Logger)
	ctx.Resp = &openAPIDockerfileTemplateList{Total: total, DockerfileTemplates: templates}
	ctx.RespErr = err
}

func OpenAPIGetDockerfileTemplate(c *gin.Context) {
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
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid dockerfile template id")
		return
	}
	ctx.Resp, ctx.RespErr = template.GetDockerfileTemplateDetail(c.Param("id"), ctx.Logger)
}

func OpenAPICreateDockerfileTemplate(c *gin.Context) {
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
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid dockerfile template request body")
		return
	}
	req := new(template.DockerfileTemplate)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid dockerfile template request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || strings.TrimSpace(req.Content) == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("name and content cannot be empty")
		return
	}
	if err := templateservice.ValidateDockerfileTemplate(req.Content, ctx.Logger); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid dockerfile content")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "新建", "模板库-Dockerfile", req.Name, req.Name, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = templateservice.CreateDockerfileTemplate(req, ctx.Logger)
}

func OpenAPIUpdateDockerfileTemplate(c *gin.Context) {
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
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid dockerfile template id")
		return
	}
	data, err := internalhandler.GetRawData(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid dockerfile template request body")
		return
	}
	req := new(template.DockerfileTemplate)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid dockerfile template request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || strings.TrimSpace(req.Content) == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("name and content cannot be empty")
		return
	}
	if err := templateservice.ValidateDockerfileTemplate(req.Content, ctx.Logger); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid dockerfile content")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "更新", "模板库-Dockerfile", c.Param("id"), c.Param("id"), string(data), types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = templateservice.UpdateDockerfileTemplate(c.Param("id"), req, ctx.Logger)
}

func OpenAPIDeleteDockerfileTemplate(c *gin.Context) {
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
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid dockerfile template id")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "删除", "模板库-Dockerfile", c.Param("id"), c.Param("id"), "", types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = templateservice.DeleteDockerfileTemplate(c.Param("id"), ctx.Logger)
}
