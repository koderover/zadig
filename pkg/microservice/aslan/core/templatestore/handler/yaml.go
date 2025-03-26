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
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/template"
	templateservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/templatestore/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

// @Summary Create yaml template
// @Description Create yaml template
// @Tags 	template
// @Accept 	json
// @Produce json
// @Param 	body 	body 		template.YamlTemplate		true 	"body"
// @Success 200
// @Router /api/aslan/template/yaml [post]
func CreateYamlTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := &template.YamlTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "创建", "模板-YAML", req.Name, string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = templateservice.CreateYamlTemplate(req, ctx.Logger)
}

// @Summary Update yaml template
// @Description Update yaml template
// @Tags 	template
// @Accept 	json
// @Produce json
// @Param 	id		path		string						true	"template id"
// @Param 	body 	body 		template.YamlTemplate		true 	"body"
// @Success 200
// @Router /api/aslan/template/yaml/{id} [put]
func UpdateYamlTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := &template.YamlTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "模板-YAML", req.Name, string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = templateservice.UpdateYamlTemplate(c.Param("id"), req, ctx.Logger)
}

// @Summary Update yaml template variable
// @Description Update yaml template variable
// @Tags 	template
// @Accept 	json
// @Produce json
// @Param 	id		path		string						true	"template id"
// @Param 	body 	body 		template.YamlTemplate		true 	"body"
// @Success 200
// @Router /api/aslan/template/yaml/{id}/variable [put]
func UpdateYamlTemplateVariable(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := &template.YamlTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "模板-YAML-变量", req.Name, string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = templateservice.UpdateYamlTemplateVariable(c.Param("id"), req, ctx.Logger)
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
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// TODO: Authorization leak
	// comment: since currently there are multiple functionalities that wish to used this API without authorization,
	// we temporarily disabled the permission checks for this API.

	// authorization check
	//if !ctx.Resources.IsSystemAdmin {
	//	if !ctx.Resources.SystemActions.Template.View {
	//		ctx.UnAuthorized = true
	//		return
	//	}
	//}

	// Query Verification
	args := &listYamlQuery{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = err
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
	ctx.RespErr = err
}

// @Summary Get yaml template detail
// @Description Get yaml template detail
// @Tags 	template
// @Accept 	json
// @Produce json
// @Param 	id		path		string						true	"template id"
// @Success 200 	{object} 	template.YamlDetail
// @Router /api/aslan/template/yaml/{id} [get]
func GetYamlTemplateDetail(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// TODO: Authorization leak
	// comment: since currently there are multiple functionalities that wish to used this API without authorization,
	// we temporarily disabled the permission checks for this API.

	// authorization check
	//if !ctx.Resources.IsSystemAdmin {
	//	if !ctx.Resources.SystemActions.Template.View {
	//		ctx.UnAuthorized = true
	//		return
	//	}
	//}

	ctx.Resp, ctx.RespErr = templateservice.GetYamlTemplateDetail(c.Param("id"), ctx.Logger)
}

func DeleteYamlTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "模板-YAML", c.Param("id"), "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = templateservice.DeleteYamlTemplate(c.Param("id"), ctx.Logger)
}

func GetYamlTemplateReference(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = templateservice.GetYamlTemplateReference(c.Param("id"), ctx.Logger)
}

func SyncYamlTemplateReference(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "同步", "模板-YAML", c.Param("id"), "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = templateservice.SyncYamlTemplateReference(ctx.UserName, c.Param("id"), ctx.Logger)
}

type getYamlTemplateVariablesReq struct {
	Content      string `json:"content" binding:"required"`
	VariableYaml string `json:"variable_yaml" binding:"required"`
}

// @Summary Validate template varaibles
// @Description Validate template varaibles
// @Tags service
// @Accept json
// @Produce json
// @Param body body getYamlTemplateVariablesReq true "body"
// @Success 200
// @Router /api/aslan/template/yaml/validateVariable [post]
func ValidateTemplateVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &getYamlTemplateVariablesReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = templateservice.ValidateVariable(req.Content, req.VariableYaml)
}

// DEPRECATED since 1.18, now we auto extract varialbes when save yaml content
func ExtractTemplateVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &getYamlTemplateVariablesReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = templateservice.ExtractVariable(req.VariableYaml)
}

// DEPRECATED, since 1.18
func GetFlatKvs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &getYamlTemplateVariablesReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = templateservice.FlattenKvs(req.VariableYaml)
}
