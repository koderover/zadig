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
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	commontypes "github.com/koderover/zadig/pkg/microservice/aslan/core/common/types"
	svcservice "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func ListServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")

	// authorization check
	// either they have the authorization, or they are system admins/project admins.
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Service.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = commonservice.ListServiceTemplate(projectName, ctx.Logger)
}

func ListWorkloadTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Env.View {
			ctx.UnAuthorized = true
			return
		}
	}

	// anyone with a token should be able to use this API
	ctx.Resp, ctx.Err = commonservice.ListWorkloadTemplate(projectName, c.Query("env"), ctx.Logger)
}

func GetServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Service.View {
			ctx.UnAuthorized = true
			return
		}
	}

	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.Err = commonservice.GetServiceTemplate(c.Param("name"), c.Param("type"), projectName, setting.ProductStatusDeleting, revision, ctx.Logger)
}

func GetServiceTemplateOption(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// TODO: Authorization leak
	// this API is sometimes used in edit/create workflow scenario, thus giving the edit/create workflow permission
	// authorization check
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	}

	if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectName]; ok {
		// first check if the user is projectAdmin
		if projectAuthInfo.IsProjectAdmin {
			permitted = true
		}

		// then check if user has edit workflow permission
		if projectAuthInfo.Env.EditConfig ||
			projectAuthInfo.Service.View {
			permitted = true
		}

		// finally check if the permission is given by collaboration mode
		collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectName, types.ResourceTypeEnvironment, types.EnvActionEditConfig)
		if err == nil && collaborationAuthorizedEdit {
			permitted = true
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.Err = svcservice.GetServiceTemplateOption(c.Param("name"), projectName, revision, ctx.Logger)
}

type createServiceTemplateRequest struct {
	ProductName        string                           `json:"product_name" binding:"required"`
	ServiceName        string                           `json:"service_name" binding:"required"`
	Source             string                           `json:"source" binding:"required"`
	Type               string                           `json:"type" binding:"required"`
	Visibility         string                           `json:"visibility" binding:"required"`
	Yaml               string                           `json:"yaml" binding:"required"`
	VariableYaml       string                           `json:"variable_yaml"`
	ServiceVariableKVs []*commontypes.ServiceVariableKV `json:"service_variable_kvs"`
}

// @Summary Create service template
// @Description Create service template
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	force	query		bool							true	"is force to create service template"
// @Param 	body 	body 		createServiceTemplateRequest 	true 	"body"
// @Success 200 	{object} 	svcservice.ServiceOption
// @Router /api/aslan/service/services [post]
func CreateServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(createServiceTemplateRequest)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateServiceTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateServiceTemplate json.Unmarshal err : %v", err)
	}
	// insert operation logs
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", "项目管理-服务", fmt.Sprintf("服务名称:%s", args.ServiceName), string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	projectName := args.ProductName

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		// TODO: Authorization leak
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Service.Create &&
			!ctx.Resources.ProjectAuthInfo[projectName].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ServiceTmpl json args")
		return
	}

	force, err := strconv.ParseBool(c.Query("force"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("force params error")
		return
	}

	svc := new(commonmodels.Service)
	svc.CreateBy = ctx.UserName
	svc.ProductName = args.ProductName
	svc.ServiceName = args.ServiceName
	svc.Source = args.Source
	svc.Type = args.Type
	svc.VariableYaml = args.VariableYaml
	svc.ServiceVariableKVs = args.ServiceVariableKVs
	svc.Yaml = args.Yaml

	ctx.Resp, ctx.Err = svcservice.CreateServiceTemplate(ctx.UserName, svc, force, ctx.Logger)
}

// UpdateServiceTemplate TODO figure out in which scene this function will be used
//func UpdateServiceTemplate(c *gin.Context) {
//	ctx, err := internalhandler.NewContextWithAuthorization(c)
//	defer func() { internalhandler.JSONResponse(c, ctx) }()
//
//	if err != nil {
//		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
//		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
//		ctx.UnAuthorized = true
//		return
//	}
//
//	args := new(commonservice.ServiceTmplObject)
//	if err := c.ShouldBindJSON(args); err != nil {
//		ctx.Err = err
//		return
//	}
//	if args.Username != "system" {
//		internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-服务", fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceName, args.Revision), "", ctx.Logger)
//	}
//
//	// authorization checks
//	projectName := args.ProductName
//
//	if !ctx.Resources.IsSystemAdmin {
//		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
//			ctx.UnAuthorized = true
//			return
//		}
//		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
//			!ctx.Resources.ProjectAuthInfo[projectName].Service.Edit {
//			ctx.UnAuthorized = true
//			return
//		}
//	}
//
//	args.Username = ctx.UserName
//	ctx.Err = svcservice.UpdateServiceVisibility(args)
//}

type updateServiceVariableRequest struct {
	VariableYaml       string                           `json:"variable_yaml" binding:"required"`
	ServiceVariableKVs []*commontypes.ServiceVariableKV `json:"service_variable_kvs" binding:"required"`
}

// @Summary Update service varaible
// @Description Update service varaible
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	name		path		string							true	"service name"
// @Param 	projectName	query		string							true	"project name"
// @Param 	body  		body 		updateServiceVariableRequest 	true 	"body"
// @Success 200
// @Router /api/aslan/service/services/{name}/variable [put]
func UpdateServiceVariable(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(updateServiceVariableRequest)
	servceTmplObjectargs := new(commonservice.ServiceTmplObject)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	// authorization
	projectName := c.Query("projectName")
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

	servceTmplObjectargs.ProductName = c.Query("projectName")
	servceTmplObjectargs.ServiceName = c.Param("name")
	servceTmplObjectargs.Username = ctx.UserName
	servceTmplObjectargs.VariableYaml = req.VariableYaml
	servceTmplObjectargs.ServiceVariableKVs = req.ServiceVariableKVs

	internalhandler.InsertOperationLog(c, ctx.UserName, servceTmplObjectargs.ProductName, "更新", "项目管理-服务变量", fmt.Sprintf("服务名称:%s", servceTmplObjectargs.ServiceName), "", ctx.Logger)

	ctx.Err = svcservice.UpdateServiceVariables(servceTmplObjectargs)
}

func UpdateServiceHealthCheckStatus(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonservice.ServiceTmplObject)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	// authorization
	projectName := args.ProductName
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Env.EditConfig {
			ctx.UnAuthorized = true
			return
		}
	}

	if args.Username != "system" {
		internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-服务", fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceName, args.Revision), "", ctx.Logger)
	}
	args.Username = ctx.UserName
	ctx.Err = svcservice.UpdateServiceHealthCheckStatus(args)
}

type ValidatorResp struct {
	Message string `json:"message"`
}

func YamlValidator(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(svcservice.YamlValidatorReq)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid yaml args")
		return
	}
	resp := make([]*ValidatorResp, 0)
	errMsgList := svcservice.YamlValidator(args)
	for _, errMsg := range errMsgList {
		resp = append(resp, &ValidatorResp{Message: errMsg})
	}
	ctx.Resp = resp
}

func HelmReleaseNaming(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")

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

	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be nil")
		return
	}

	args := new(svcservice.ReleaseNamingRule)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid yaml args")
		return
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "修改", "项目管理-服务", args.ServiceName, string(bs), ctx.Logger)

	ctx.Err = svcservice.UpdateReleaseNamingRule(ctx.UserName, ctx.RequestID, projectName, args, ctx.Logger)
}

func DeleteServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Service.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "删除", "项目管理-服务", c.Param("name"), "", ctx.Logger)

	ctx.Err = svcservice.DeleteServiceTemplate(c.Param("name"), c.Param("type"), projectName, c.DefaultQuery("isEnvTemplate", "true"), c.DefaultQuery("visibility", "public"), ctx.Logger)
}

func ListServicePort(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.Err = svcservice.ListServicePort(c.Param("name"), c.Param("type"), c.Query("projectName"), setting.ProductStatusDeleting, revision, ctx.Logger)
}

func UpdateWorkloads(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(svcservice.UpdateWorkloadsArgs)

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateWorkloads c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateWorkloads json.Unmarshal err : %v", err)
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv, "配置", "环境", c.Query("env"), string(data), ctx.Logger, c.Query("env"))

	err = c.ShouldBindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid UpdateWorkloadsArgs")
		return
	}
	projectName := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Env.EditConfig {
			ctx.UnAuthorized = true
			return
		}
	}

	env := c.Query("env")
	if projectName == "" || env == "" {
		ctx.Err = e.ErrInvalidParam
		return
	}
	allowedEnvs, found := internalhandler.GetResourcesInHeader(c)
	if found {
		allowedSet := sets.NewString(allowedEnvs...)
		if !allowedSet.Has(env) {
			c.String(http.StatusForbidden, "not all input envs are allowed, allowed envs are %v", allowedEnvs)
			return
		}
	}

	ctx.Err = svcservice.UpdateWorkloads(c, ctx.RequestID, ctx.UserName, projectName, env, *args, ctx.Logger)
}

func CreateK8sWorkloads(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(svcservice.K8sWorkloadsArgs)

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateK8sWorkloads c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateK8sWorkloads json.Unmarshal err : %v", err)
	}

	projectName := c.Query("projectName")
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "新增", "环境", args.EnvName, string(data), ctx.Logger, args.EnvName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Env.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid K8sWorkloadsArgs args")
		return
	}

	ctx.Err = svcservice.CreateK8sWorkLoads(c, ctx.RequestID, ctx.UserName, args, ctx.Logger)
}

func GetServiceTemplateProductName(c *gin.Context) {
	args := new(commonmodels.Service)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("c.GetRawData() err : %v", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("json.Unmarshal err : %v", err)
		return
	}
	c.Set("productName", args.ProductName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	c.Next()
}

func CreatePMService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Param("productName")

	args := new(svcservice.ServiceTmplBuildObject)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreatePMService c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreatePMService json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "新增", "项目管理-物理机部署服务", fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceTmplObject.ServiceName, args.ServiceTmplObject.Revision), string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Service.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid service json args")
		return
	}
	if args.Build.Name == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("构建名称不能为空!")
		return
	}

	for _, heathCheck := range args.ServiceTmplObject.HealthChecks {
		if heathCheck.TimeOut < 2 || heathCheck.TimeOut > 60 {
			ctx.Err = e.ErrInvalidParam.AddDesc("超时时间必须在2-60之间")
			return
		}
		if heathCheck.Interval != 0 {
			if heathCheck.Interval < 2 || heathCheck.Interval > 60 {
				ctx.Err = e.ErrInvalidParam.AddDesc("间隔时间必须在2-60之间")
				return
			}
		}
		if heathCheck.HealthyThreshold != 0 {
			if heathCheck.HealthyThreshold < 2 || heathCheck.HealthyThreshold > 10 {
				ctx.Err = e.ErrInvalidParam.AddDesc("健康阈值必须在2-10之间")
				return
			}
		}
		if heathCheck.UnhealthyThreshold != 0 {
			if heathCheck.UnhealthyThreshold < 2 || heathCheck.UnhealthyThreshold > 10 {
				ctx.Err = e.ErrInvalidParam.AddDesc("不健康阈值必须在2-10之间")
				return
			}
		}
	}

	ctx.Err = svcservice.CreatePMService(ctx.UserName, args, ctx.Logger)
}

func UpdatePmServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Param("productName")

	args := new(commonservice.ServiceTmplBuildObject)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdatePmServiceTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdatePmServiceTemplate json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "更新", "项目管理-主机服务", fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceTmplObject.ServiceName, args.ServiceTmplObject.Revision), "", ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

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

	for _, heathCheck := range args.ServiceTmplObject.HealthChecks {
		if heathCheck.TimeOut < 2 || heathCheck.TimeOut > 60 {
			ctx.Err = e.ErrInvalidParam.AddDesc("超时时间必须在2-60之间")
			return
		}
		if heathCheck.Interval != 0 {
			if heathCheck.Interval < 2 || heathCheck.Interval > 60 {
				ctx.Err = e.ErrInvalidParam.AddDesc("间隔时间必须在2-60之间")
				return
			}
		}
		if heathCheck.HealthyThreshold != 0 {
			if heathCheck.HealthyThreshold < 2 || heathCheck.HealthyThreshold > 10 {
				ctx.Err = e.ErrInvalidParam.AddDesc("健康阈值必须在2-10之间")
				return
			}
		}
		if heathCheck.UnhealthyThreshold != 0 {
			if heathCheck.UnhealthyThreshold < 2 || heathCheck.UnhealthyThreshold > 10 {
				ctx.Err = e.ErrInvalidParam.AddDesc("不健康阈值必须在2-10之间")
				return
			}
		}
	}
	ctx.Err = commonservice.UpdatePmServiceTemplate(ctx.UserName, args, ctx.Logger)
}

// @Summary convert varaible kv and yaml
// @Description convert varaible kv and yaml
// @Tags service
// @Accept json
// @Produce json
// @Param body body commonservice.ConvertVaraibleKVAndYamlArgs true "body"
// @Success 200 {object} commonservice.ConvertVaraibleKVAndYamlArgs
// @Router /api/aslan/service/services/variable/convert [post]
func ConvertVaraibleKVAndYaml(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonservice.ConvertVaraibleKVAndYamlArgs)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ConvertVariableKVAndYamlArgs")
		return
	}

	resp, err := commonservice.ConvertVaraibleKVAndYaml(args)
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Resp = resp
}
