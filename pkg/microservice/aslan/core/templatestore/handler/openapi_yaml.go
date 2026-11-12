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
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/template"
	templateservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/templatestore/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type openAPIYamlTemplatePageQuery struct {
	PageNum  int `form:"pageNum,default=1"`
	PageSize int `form:"pageSize,default=20"`
}

type openAPIYamlTemplateList struct {
	Total         int                        `json:"total"`
	YamlTemplates []*template.YamlListObject `json:"yaml_templates"`
}

// @Summary List K8s YAML service templates
// @Description List K8s YAML service templates with their exact names. Available to authenticated users, matching the web endpoint.
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param pageNum query int false "Page number, greater than 0" default(1)
// @Param pageSize query int false "Page size, greater than 0" default(20)
// @Success 200 {object} openAPIYamlTemplateList
// @Router /openapi/templates/yaml [get]
func OpenAPIListYamlTemplates(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	query := new(openAPIYamlTemplatePageQuery)
	if err := c.ShouldBindQuery(query); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if query.PageNum <= 0 || query.PageSize <= 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("pageNum and pageSize must be greater than 0")
		return
	}
	templates, total, err := templateservice.ListYamlTemplate(query.PageNum, query.PageSize, ctx.Logger)
	ctx.Resp = &openAPIYamlTemplateList{Total: total, YamlTemplates: templates}
	if err != nil {
		ctx.RespErr = e.ErrListTemplate.AddErr(err)
	}
}

// @Summary Get K8s YAML service template
// @Description Get a K8s YAML service template with its content and variable definitions. Available to authenticated users, matching the web endpoint.
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param id path string true "template id"
// @Success 200 {object} template.YamlDetail
// @Router /openapi/templates/yaml/{id} [get]
func OpenAPIGetYamlTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !primitive.IsValidObjectID(c.Param("id")) {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid yaml template id")
		return
	}
	ctx.Resp, ctx.RespErr = templateservice.GetYamlTemplateDetail(c.Param("id"), ctx.Logger)
	if errors.Is(ctx.RespErr, mongo.ErrNoDocuments) {
		ctx.RespErr = e.ErrNotFound.AddErr(ctx.RespErr)
	} else if ctx.RespErr != nil {
		ctx.RespErr = e.ErrGetTemplate.AddErr(ctx.RespErr)
	}
}
