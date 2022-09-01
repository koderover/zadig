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

package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	templateservice "github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CreateYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &template.YamlTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "创建", "模板-YAML", req.Name, string(bs), ctx.Logger)

	ctx.Err = templateservice.CreateYamlTemplate(req, ctx.Logger)
}

func UpdateYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &template.YamlTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "模板-YAML", req.Name, string(bs), ctx.Logger)

	ctx.Err = templateservice.UpdateYamlTemplate(c.Param("id"), req, ctx.Logger)
}

func UpdateYamlTemplateVariable(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &template.YamlTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "模板-YAML-变量", req.Name, string(bs), ctx.Logger)

	ctx.Err = templateservice.UpdateYamlTemplateVariable(c.Param("id"), req, ctx.Logger)
}

type listYamlQuery struct {
	PageSize int `json:"page_size" form:"page_size,default=100"`
	PageNum  int `json:"page_num"  form:"page_num,default=1"`
}

type ListYamlResp struct {
	SystemVariables []*commonmodels.ChartVariable `json:"system_variables"`
	YamlTemplates   []*template.YamlListObject    `json:"yaml_template"`
	Total           int                           `json:"total"`
}

func ListYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// Query Verification
	args := &listYamlQuery{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	systemVariables := templateservice.GetSystemDefaultVariables()
	YamlTemplateList, total, err := templateservice.ListYamlTemplate(args.PageNum, args.PageSize, ctx.Logger)
	resp := ListYamlResp{
		SystemVariables: systemVariables,
		YamlTemplates:   YamlTemplateList,
		Total:           total,
	}
	ctx.Resp = resp
	ctx.Err = err
}

func GetYamlTemplateDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetYamlTemplateDetail(c.Param("id"), ctx.Logger)
}

func DeleteYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "模板-YAML", c.Param("id"), "", ctx.Logger)

	ctx.Err = templateservice.DeleteYamlTemplate(c.Param("id"), ctx.Logger)
}

func GetYamlTemplateReference(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetYamlTemplateReference(c.Param("id"), ctx.Logger)
}

func SyncYamlTemplateReference(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "同步", "模板-YAML", c.Param("id"), "", ctx.Logger)

	ctx.Err = templateservice.SyncYamlTemplateReference(ctx.UserName, c.Param("id"), ctx.Logger)
}

type getYamlTemplateVariablesReq struct {
	Content      string `json:"content"`
	VariableYaml string `json:"variable_yaml"`
}

func GetYamlTemplateVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &getYamlTemplateVariablesReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Resp, ctx.Err = template.GetYamlVariables(req.Content, ctx.Logger)
}

func ValidateTemplateVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &getYamlTemplateVariablesReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = templateservice.ValidateVariable(req.Content, req.VariableYaml)
}
