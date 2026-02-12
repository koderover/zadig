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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	projectservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/project/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func GetProductTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productTemplatName := c.Param("name")
	ctx.Resp, ctx.RespErr = commonservice.GetProductTemplate(productTemplatName, ctx.Logger)
}

// TODO: no authorization whatsoever
func GetProductTemplateServices(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productTemplatName := c.Param("name")

	envType := types.EnvType(c.Query("envType"))
	isBaseEnvStr := c.Query("isBaseEnv")
	baseEnvName := c.Query("baseEnv")

	if envType == "" {
		envType = types.GeneralEnv
	}

	var isBaseEnv bool
	var err error
	if envType == types.ShareEnv {
		isBaseEnv, err = strconv.ParseBool(isBaseEnvStr)
		if err != nil {
			ctx.RespErr = fmt.Errorf("failed to parse %s to bool: %s", isBaseEnvStr, err)
			return
		}
	}

	ctx.Resp, ctx.RespErr = projectservice.GetProductTemplateServices(productTemplatName, envType, isBaseEnv, baseEnvName, ctx.Logger)
}

func CreateProductTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(template.Product)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateProductTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateProductTemplate json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", "项目管理-项目", args.ProductName, args.ProductName, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Project.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}

	args.UpdateBy = ctx.UserName
	ctx.RespErr = projectservice.CreateProductTemplate(args, ctx.Logger)
}

// UpdateProductTemplate ...
func UpdateProductTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(template.Product)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateProductTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateProductTemplate json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-项目环境模板或变量", args.ProductName, args.ProductName, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductName].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	args.UpdateBy = ctx.UserName
	ctx.RespErr = projectservice.UpdateProductTemplate(c.Param("name"), args, ctx.Logger)
}

// TODO: old API with no authorizations
func UpdateProductTmplStatus(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productName := c.Param("name")
	onboardingStatus := c.Param("status")

	ctx.RespErr = projectservice.UpdateProductTmplStatus(productName, onboardingStatus, ctx.Logger)
}

func TransferProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	productName := c.Param("name")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[productName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[productName].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	// license checks
	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = projectservice.TransferHostProject(ctx.UserName, productName, ctx.Logger)
}

func UpdateProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(template.Product)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateProject c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateProject json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-项目", args.ProductName, args.ProductName, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}
	args.UpdateBy = ctx.UserName
	productName := c.Query("projectName")
	if productName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can't be empty")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[productName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[productName].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = projectservice.UpdateProject(productName, args, ctx.Logger)
}

type UpdateOrchestrationServiceReq struct {
	Services           [][]string `json:"services"`
	ProductionServices [][]string `json:"production_services"`
}

func UpdateServiceOrchestration(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Param("name")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "更新", "项目管理-测试服务编排", projectName, projectName, "", types.RequestBodyTypeJSON, ctx.Logger)

	args := new(UpdateOrchestrationServiceReq)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid UpdateOrchestrationServiceReq json args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = projectservice.UpdateServiceOrchestration(projectName, args.Services, ctx.UserName, ctx.Logger)
}

func UpdateProductionServiceOrchestration(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Param("name")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "更新", "项目管理-生产服务编排", projectName, projectName, "", types.RequestBodyTypeJSON, ctx.Logger)

	args := new(UpdateOrchestrationServiceReq)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid UpdateOrchestrationServiceReq json args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = projectservice.UpdateProductionServiceOrchestration(projectName, args.ProductionServices, ctx.UserName, ctx.Logger)
}

func DeleteProductTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "删除", "项目管理-项目", projectKey, projectKey, "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	isDelete, err := strconv.ParseBool(c.Query("is_delete"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalidParam is_delete")
		return
	}
	ctx.RespErr = projectservice.DeleteProductTemplate(ctx.UserName, projectKey, ctx.RequestID, isDelete, ctx.Logger)
}

func ListTemplatesHierachy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = projectservice.ListTemplatesHierachy(ctx.UserName, ctx.Logger)
}

func GetCustomMatchRules(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = projectservice.GetCustomMatchRules(projectKey, ctx.Logger)
}

func CreateOrUpdateMatchRules(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "更新", "工程管理-项目", c.Param("name"), c.Param("name"), "", types.RequestBodyTypeJSON, ctx.Logger)

	args := new(projectservice.CustomParseDataArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateOrUpdateMatchRules c.GetRawData() err : %v", err)
		ctx.RespErr = e.ErrInvalidParam
		return
	}
	if err = json.Unmarshal(data, &args); err != nil {
		log.Errorf("CreateOrUpdateMatchRules json.Unmarshal err : %v", err)
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	ctx.RespErr = projectservice.UpdateCustomMatchRules(projectKey, ctx.UserName, ctx.RequestID, args.Rules)
}

// @Summary Get global variables
// @Description Get global variables
// @Tags 	project
// @Accept 	json
// @Produce json
// @Param 	name	path		string							true	"project name"
// @Success 200 	{array} 	commontypes.ServiceVariableKV
// @Router /api/aslan/project/products/{name}/globalVariables [get]
func GetGlobalVariables(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// TODO: Authorization leak
	// Authorization checks
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if projectedAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		if projectedAuthInfo.IsProjectAdmin {
			permitted = true
		} else if projectedAuthInfo.Service.Create ||
			projectedAuthInfo.Service.Edit ||
			projectedAuthInfo.Service.View ||
			projectedAuthInfo.Env.EditConfig ||
			projectedAuthInfo.Env.Create {
			permitted = true
		} else {
			permittedByCollaborationMode, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeEnvironment, types.EnvActionEditConfig)
			if err == nil {
				permitted = permittedByCollaborationMode
			}
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = projectservice.GetGlobalVariables(projectKey, false, ctx.Logger)
}

// @Summary Get global production_variables
// @Description Get global variables
// @Tags 	project
// @Accept 	json
// @Produce json
// @Param 	name	path		string							true	"project name"
// @Success 200 	{array} 	commontypes.ServiceVariableKV
// @Router /api/aslan/project/products/{name}/productionGlobalVariables [get]
func GetProductionGlobalVariables(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			// this api is called when the user is trying to
			// a. view and edit the production service's global variables
			// b. edit the value of the global variables in production env
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Create &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Edit &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.View &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.EditConfig {

			permittedByCollaborationMode, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeEnvironment, types.ProductionEnvActionEditConfig)
			if err != nil || !permittedByCollaborationMode {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Resp, ctx.RespErr = projectservice.GetGlobalVariables(projectKey, true, ctx.Logger)
}

type updateGlobalVariablesRequest struct {
	GlobalVariables []*commontypes.ServiceVariableKV `json:"global_variables"`
}

// @Summary Update global variables
// @Description Update global variables
// @Tags 	project
// @Accept 	json
// @Produce json
// @Param 	name	path		string							true	"project name"
// @Param 	body 	body 		updateGlobalVariablesRequest 	true 	"body"
// @Success 200
// @Router /api/aslan/project/products/{name}/globalVariables [put]
func UpdateGlobalVariables(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "更新", "工程管理-项目", c.Param("name"), c.Param("name"), "", types.RequestBodyTypeJSON, ctx.Logger)

	args := new(updateGlobalVariablesRequest)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid UpdateGlobalVariablesRequest json args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Create &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = projectservice.UpdateGlobalVariables(projectKey, ctx.UserName, args.GlobalVariables, false)
}

// @Summary Update production_global variables
// @Description Update global variables
// @Tags 	project
// @Accept 	json
// @Produce json
// @Param 	name	path		string							true	"project name"
// @Param 	body 	body 		updateGlobalVariablesRequest 	true 	"body"
// @Success 200
// @Router /api/aslan/project/products/{name}/productionGlobalVariables [put]
func UpdateProductionGlobalVariables(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "更新", "工程管理-项目", c.Param("name"), c.Param("name"), "", types.RequestBodyTypeJSON, ctx.Logger)

	args := new(updateGlobalVariablesRequest)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid UpdateGlobalVariablesRequest json args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Create &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = projectservice.UpdateGlobalVariables(projectKey, ctx.UserName, args.GlobalVariables, true)
}

// @Summary Get global variable candidates
// @Description Get global variable candidates
// @Tags 	project
// @Accept 	json
// @Produce json
// @Param 	name	path		string												true	"project name"
// @Success 200 	{array} 	projectservice.GetGlobalVariableCandidatesRespone
// @Router /api/aslan/project/products/{name}/globalVariableCandidates [get]
func GetGlobalVariableCandidates(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Create &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = projectservice.GetGlobalVariableCandidates(projectKey, false, ctx.Logger)
}

// @Summary Get production_global variable candidates
// @Description Get global variable candidates
// @Tags 	project
// @Accept 	json
// @Produce json
// @Param 	name	path		string												true	"project name"
// @Success 200 	{array} 	projectservice.GetGlobalVariableCandidatesRespone
// @Router /api/aslan/project/products/{name}/globalProductionGlobalVariables [get]
func GetProductionGlobalVariableCandidates(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Create &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = projectservice.GetGlobalVariableCandidates(c.Param("name"), true, ctx.Logger)
}

func CreateProjectGroup(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Project.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	// license checks
	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	args := new(projectservice.ProjectGroupArgs)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Logger.Errorf("failed to get raw data from request, error: %v", err)
		ctx.RespErr = e.ErrCreateProjectGroup.AddDesc(err.Error())
		return
	}

	if err = json.Unmarshal(data, args); err != nil {
		ctx.Logger.Errorf("failed to unmarshal data, error: %v", err)
		ctx.RespErr = e.ErrCreateProjectGroup.AddDesc(err.Error())
		return
	}
	if err := args.Validate(); err != nil {
		ctx.RespErr = e.ErrCreateProjectGroup.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "分组", args.GroupName, args.GroupName, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = projectservice.CreateProjectGroup(args, ctx.UserName, ctx.Logger)
}

func UpdateProjectGroup(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Project.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	// license checks
	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	args := new(projectservice.ProjectGroupArgs)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Logger.Errorf("failed to get raw data from request, error: %v", err)
		ctx.RespErr = e.ErrUpdateProjectGroup.AddDesc(err.Error())
		return
	}

	if err = json.Unmarshal(data, args); err != nil {
		ctx.Logger.Errorf("failed to unmarshal data, error: %v", err)
		ctx.RespErr = e.ErrUpdateProjectGroup.AddDesc(err.Error())
		return
	}
	if err := args.Validate(); err != nil {
		ctx.RespErr = e.ErrUpdateProjectGroup.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "编辑", "分组", args.GroupName, args.GroupName, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = projectservice.UpdateProjectGroup(args, ctx.UserName, ctx.Logger)
}

func DeleteProjectGroup(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Project.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	// license checks
	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	groupName, err := url.QueryUnescape(c.Query("groupName"))
	if err != nil {
		ctx.RespErr = e.ErrDeleteProjectGroup.AddErr(err)
		return
	}
	if groupName == "" {
		ctx.RespErr = e.ErrDeleteProjectGroup.AddErr(errors.New("group name is empty"))
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "分组", groupName, groupName, groupName, types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = projectservice.DeleteProjectGroup(groupName, ctx.Logger)
}

func ListProjectGroups(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = projectservice.ListProjectGroupNames()
}

func GetPresetProjectGroup(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// license checks
	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	groupName, err := url.QueryUnescape(c.Query("groupName"))
	if err != nil {
		ctx.RespErr = e.ErrDeleteProjectGroup.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = projectservice.GetProjectGroupRelation(groupName, ctx.Logger)
}
