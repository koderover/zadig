/*
Copyright 2023 The KodeRover Authors.

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
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/sets"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

// TODO: deal with openapi later
func generalOpenAPIRequestValidate(c *gin.Context) (string, string, error) {
	projectName := c.Query("projectKey")
	if projectName == "" {
		return "", "", errors.New("projectKey can't be empty")
	}

	envName := c.Param("name")
	if envName == "" {
		return "", "", errors.New("envKey can't be empty")
	}
	return projectName, envName, nil
}

func OpenAPIScaleWorkloads(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPIScaleServiceReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Logger.Errorf("CreateProductTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, req); err != nil {
		ctx.Logger.Errorf("CreateProductTemplate json.Unmarshal err : %v", err)
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName+"(openAPI)",
		req.ProjectKey, setting.OperationSceneEnv,
		"伸缩",
		"环境-服务",
		fmt.Sprintf("环境名称:%s,%s:%s", req.EnvName, req.WorkloadType, req.WorkloadName),
		string(data), types.RequestBodyTypeJSON, ctx.Logger, req.EnvName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}

	// input validation for OpenAPI
	err = req.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[req.ProjectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[req.ProjectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[req.ProjectKey].Env.ManagePods {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, req.ProjectKey, types.ResourceTypeEnvironment, req.EnvName, types.EnvActionManagePod)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.RespErr = service.OpenAPIScale(req, ctx.Logger)
}

func OpenAPIApplyYamlService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPIApplyYamlServiceReq)

	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}

	projectKey := c.Query("projectKey")

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		ctx.Logger.Errorf("CreateProductTemplate c.GetRawData() err : %v", err)
	}

	// input validation for OpenAPI
	err = req.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	allowedEnvs, found := internalhandler.GetResourcesInHeader(c)
	if found {
		allowedSet := sets.NewString(allowedEnvs...)
		if !allowedSet.Has(req.EnvName) {
			c.String(http.StatusForbidden, "not all input envs are allowed, allowed envs are %v", allowedEnvs)
			return
		}
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName+"(openAPI)", projectKey, setting.OperationSceneEnv, "更新", "测试环境", req.EnvName, string(data), types.RequestBodyTypeJSON, ctx.Logger, req.EnvName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, req.EnvName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	_, err = service.OpenAPIApplyYamlService(projectKey, req, false, ctx.RequestID, ctx.UserName, ctx.Logger)

	ctx.RespErr = err
}

func OpenAPIDeleteYamlServiceFromEnv(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPIDeleteYamlServiceFromEnvReq)
	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid request body")
		return
	}

	// input validation for OpenAPI
	err = req.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	svcsInSubEnvs, err := service.CheckServicesDeployedInSubEnvs(c, projectKey, req.EnvName, req.ServiceNames)
	if err != nil {
		ctx.RespErr = err
		return
	}

	if len(svcsInSubEnvs) > 0 {
		data := make(map[string]interface{}, len(svcsInSubEnvs))
		for k, v := range svcsInSubEnvs {
			data[k] = v
		}

		ctx.RespErr = e.NewWithExtras(e.ErrDeleteSvcHasSvcsInSubEnv, "", data)
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName+"(openAPI)", projectKey, setting.OperationSceneEnv, "删除", "测试环境的服务", fmt.Sprintf("%s:[%s]", req.EnvName, strings.Join(req.ServiceNames, ",")), "", types.RequestBodyTypeJSON, ctx.Logger, req.EnvName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, req.EnvName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.RespErr = service.DeleteProductServices(ctx.UserName, ctx.RequestID, req.EnvName, projectKey, req.ServiceNames, false, !req.NotDeleteResource, ctx.Logger)
}

func OpenAPIDeleteProductionYamlServiceFromEnv(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPIDeleteYamlServiceFromEnvReq)
	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid request body")
		return
	}
	// input validation for OpenAPI
	err = req.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName+"(openAPI)", projectKey, setting.OperationSceneEnv, "删除", "生产环境的服务", fmt.Sprintf("%s:[%s]", req.EnvName, strings.Join(req.ServiceNames, ",")), "", types.RequestBodyTypeJSON, ctx.Logger, req.EnvName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, req.EnvName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.DeleteProductServices(ctx.UserName, ctx.RequestID, req.EnvName, projectKey, req.ServiceNames, true, !req.NotDeleteResource, ctx.Logger)
}

func OpenAPIApplyProductionYamlService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPIApplyYamlServiceReq)
	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}
	// input validation for OpenAPI
	err = req.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	allowedEnvs, found := internalhandler.GetResourcesInHeader(c)
	if found {
		allowedSet := sets.NewString(allowedEnvs...)
		if !allowedSet.Has(req.EnvName) {
			c.String(http.StatusForbidden, "not all input envs are allowed, allowed envs are %v", allowedEnvs)
			return
		}
	}
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = err
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "(openAPI)"+"更新", "生产环境", req.EnvName, string(data), types.RequestBodyTypeJSON, ctx.Logger, req.EnvName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, req.EnvName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	_, err = service.OpenAPIApplyYamlService(projectKey, req, true, ctx.RequestID, ctx.UserName, ctx.Logger)
	ctx.RespErr = err
}

func OpenAPIUpdateCommonEnvCfg(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPIEnvCfgArgs)

	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrUpdateEnvCfg.AddErr(fmt.Errorf("failed to get request data err : %v", err))
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = e.ErrUpdateEnvCfg.AddErr(fmt.Errorf("failed to unmarshal request data err : %v", err))
		return
	}
	projectKey := c.Query("projectKey")
	args.ProductName = projectKey
	if err := args.Validate(); err != nil {
		ctx.RespErr = e.ErrUpdateEnvCfg.AddErr(fmt.Errorf("failed to validate request data err : %v", err))
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "(OpenAPI)"+"更新", "环境配置", fmt.Sprintf("%s:%s:%s", args.EnvName, args.CommonEnvCfgType, args.Name), string(data), types.RequestBodyTypeJSON, ctx.Logger, args.Name)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductName].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.RespErr = service.OpenAPIUpdateCommonEnvCfg(projectKey, args, ctx.UserName, ctx.Logger)
}

func OpenAPIUpdateProductionCommonEnvCfg(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPIEnvCfgArgs)

	data, err := c.GetRawData()
	if err != nil {
		msg := fmt.Errorf("failed to get request data err : %v", err)
		ctx.RespErr = e.ErrUpdateEnvCfg.AddErr(msg)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		msg := fmt.Errorf("failed to unmarshal request data err : %v", err)
		ctx.RespErr = e.ErrUpdateEnvCfg.AddErr(msg)
		return
	}
	projectKey := c.Query("projectKey")
	args.ProductName = projectKey
	if err := args.Validate(); err != nil {
		msg := fmt.Errorf("failed to validate request data err : %v", err)
		ctx.RespErr = e.ErrUpdateEnvCfg.AddErr(msg)
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "(OpenAPI)"+"更新", "生产环境配置", fmt.Sprintf("%s:%s:%s", args.EnvName, args.CommonEnvCfgType, args.Name), string(data), types.RequestBodyTypeJSON, ctx.Logger, args.Name)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductName].ProductionEnv.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPIUpdateCommonEnvCfg(projectKey, args, ctx.UserName, ctx.Logger)
}

func OpenAPICreateCommonEnvCfg(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPIEnvCfgArgs)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = err
		return
	}
	args.ProductName, args.EnvName, err = generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err := args.Validate(); err != nil {
		ctx.RespErr = err
		return
	}
	logData, err := json.Marshal(args)
	if err != nil {
		ctx.RespErr = err
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, args.ProductName, setting.OperationSceneEnv, "(OpenAPI)"+"新建", "环境配置", fmt.Sprintf("%s:%s:%s", args.EnvName, args.CommonEnvCfgType, args.Name), string(logData), types.RequestBodyTypeJSON, ctx.Logger, args.Name)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductName].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductName, types.ResourceTypeEnvironment, args.EnvName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPICreateCommonEnvCfg(args.ProductName, args, ctx.UserName, ctx.Logger)
}

func OpenAPIListProductionCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIListCommonEnvCfg(projectName, envName, c.Query("type"), true, ctx.Logger)
}

func OpenAPIGetProductionCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	cfgName := c.Param("cfgName")
	if cfgName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cfgName is empty")
		return
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIGetCommonEnvCfg(projectName, envName, c.Query("type"), cfgName, true, ctx.Logger)
}

func OpenAPIListCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIListCommonEnvCfg(projectName, envName, c.Query("type"), false, ctx.Logger)
}

func OpenAPIGetCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	cfgName := c.Param("cfgName")
	if cfgName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cfgName is empty")
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIGetCommonEnvCfg(projectName, envName, c.Query("type"), cfgName, false, ctx.Logger)
}

func OpenAPIDeleteCommonEnvCfg(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	cfgName := c.Param("cfgName")
	if cfgName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cfgName is empty")
		return
	}
	cfgType := c.Query("type")
	if cfgType == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("type is empty")
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "OpenAPI"+"删除", "测试环境配置", fmt.Sprintf("%s:%s:%s", envName, cfgType, cfgName), "", types.RequestBodyTypeJSON, ctx.Logger, envName)

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

	ctx.RespErr = service.OpenAPIDeleteCommonEnvCfg(projectName, envName, cfgType, cfgName, ctx.Logger)
}

func OpenAPIDeleteProductionEnvCommonEnvCfg(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	cfgName := c.Param("cfgName")
	if cfgName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cfgName is empty")
		return
	}
	cfgType := c.Query("type")
	if cfgType == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("type is empty")
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "OpenAPI"+"删除", "生产环境配置", fmt.Sprintf("%s:%s:%s", envName, cfgType, cfgName), "", types.RequestBodyTypeJSON, ctx.Logger, envName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].ProductionEnv.EditConfig {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPIDeleteProductionEnvCommonEnvCfg(projectName, envName, cfgType, cfgName, ctx.Logger)
}

func OpenAPICreateK8sEnv(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPICreateEnvArgs)

	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = err
		return
	}
	args.ProjectName = c.Query("projectKey")
	if err := args.Validate(); err != nil {
		ctx.RespErr = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, args.ProjectName, setting.OperationSceneEnv, "(OpenAPI)"+"创建", "测试环境", args.EnvName, string(data), types.RequestBodyTypeJSON, ctx.Logger, args.EnvName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].Env.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.OpenAPICreateK8sEnv(args, ctx.UserName, ctx.RequestID, ctx.Logger)
}

func OpenAPIDeleteProductionEnv(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"删除", "生产环境", envName, "", types.RequestBodyTypeJSON, ctx.Logger, envName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].ProductionEnv.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.DeleteProductionProduct(ctx.UserName, envName, projectName, ctx.RequestID, ctx.Logger)
}

func OpenAPICreateProductionEnv(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPICreateEnvArgs)

	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = err
		return
	}
	args.ProjectName = c.Query("projectKey")
	args.Production = true
	if err := args.Validate(); err != nil {
		ctx.RespErr = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, args.ProjectName, setting.OperationSceneEnv, "(OpenAPI)"+"创建", "生产环境", args.EnvName, string(data), types.RequestBodyTypeJSON, ctx.Logger, args.EnvName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].ProductionEnv.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPICreateProductionEnv(args, ctx.UserName, ctx.RequestID, ctx.Logger)
}

func OpenAPIDeleteEnv(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrDeleteResource.AddErr(err)
		return
	}
	isDelete, err := strconv.ParseBool(c.Query("isDelete"))
	if err != nil {
		ctx.RespErr = e.ErrDeleteResource.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"删除", "测试环境", envName, "", types.RequestBodyTypeJSON, ctx.Logger, envName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Env.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.DeleteProduct(ctx.UserName, envName, projectName, ctx.RequestID, isDelete, ctx.Logger)
}

func OpenAPIGetEnvDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.GetEnvDetail(projectName, envName, false, ctx.Logger)
}

func OpenAPIGetProductionEnvDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.GetEnvDetail(projectName, envName, true, ctx.Logger)
}

func OpenAPIUpdateEnvBasicInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.EnvBasicInfoArgs)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = err
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"更新", "测试环境", envName, string(data), types.RequestBodyTypeJSON, ctx.Logger, envName)

	ctx.RespErr = service.OpenAPIUpdateEnvBasicInfo(args, ctx.UserName, projectName, envName, false, ctx.Logger)
}

func OpenAPIUpdateYamlServices(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPIServiceVariablesReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = err
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"环境管理-更新服务", "测试环境", envName, string(data), types.RequestBodyTypeJSON, ctx.Logger, envName)

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

	ctx.RespErr = service.OpenAPIUpdateYamlService(args, ctx.UserName, ctx.RequestID, projectName, envName, false, ctx.Logger)
}

func OpenAPIGetEnvGlobalVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIGetGlobalVariables(projectName, envName, false, ctx.Logger)
}

func OpenAPIGetProductionEnvGlobalVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIGetGlobalVariables(projectName, envName, true, ctx.Logger)
}

// @Summary OpenAPI Update K8S Environment Global Variables
// @Description OpenAPI Update K8S Environment Global Variables
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	projectKey	query		string									true	"project key"
// @Param 	name 		path		string									true	"env name"
// @Param 	body 		body 		service.OpenAPIEnvGlobalVariables 		true 	"body"
// @Success 200
// @Router /openapi/environments/{name}/variable [put]
func OpenAPIUpdateGlobalVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.OpenAPIEnvGlobalVariables)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = err
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"更新", "测试环境管理-更新全局变量", envName, string(data), types.RequestBodyTypeJSON, ctx.Logger, envName)

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       projectName,
		EnvName:    envName,
		Production: util.GetBoolPointer(false),
	})
	if err != nil {
		ctx.RespErr = fmt.Errorf("GetProductEnv envName:%s productName: %s error, error msg:%s", envName, projectName, err)
		return
	}

	ctx.RespErr = service.UpdateProductGlobalVariables(projectName, envName, ctx.UserName, ctx.RequestID, env.UpdateTime, args.GlobalVariables, false, ctx.Logger)
}

func OpenAPIUpdateProductionYamlServices(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPIServiceVariablesReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = err
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"环境管理-更新服务", "生产环境", envName, string(data), types.RequestBodyTypeJSON, ctx.Logger, envName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].ProductionEnv.EditConfig {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPIUpdateYamlService(args, ctx.UserName, ctx.RequestID, projectName, envName, true, ctx.Logger)
}

// @Summary OpenAPI Update Production K8S Environment Global Variables
// @Description OpenAPI Update Production K8S Environment Global Variables
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	projectKey	query		string									true	"project key"
// @Param 	name 		path		string									true	"env name"
// @Param 	body 		body 		service.OpenAPIEnvGlobalVariables 		true 	"body"
// @Success 200
// @Router /openapi/environments/production/{name}/variable [put]
func OpenAPIUpdateProductionGlobalVariables(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPIEnvGlobalVariables)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, &args); err != nil {
		ctx.RespErr = err
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"更新", "生产环境管理-更新全局变量", envName, string(data), types.RequestBodyTypeJSON, ctx.Logger, envName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].ProductionEnv.EditConfig {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       projectName,
		EnvName:    envName,
		Production: util.GetBoolPointer(true),
	})
	if err != nil {
		ctx.RespErr = fmt.Errorf("GetProductEnv envName:%s productName: %s error, error msg:%s", envName, projectName, err)
		return
	}

	ctx.RespErr = service.UpdateProductGlobalVariables(projectName, envName, ctx.UserName, ctx.RequestID, env.UpdateTime, args.GlobalVariables, true, ctx.Logger)
}

func OpenAPIUpdateProductionEnvBasicInfo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.EnvBasicInfoArgs)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = err
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"更新", "生产环境", envName, string(data), types.RequestBodyTypeJSON, ctx.Logger, envName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPIUpdateEnvBasicInfo(args, ctx.UserName, projectName, envName, true, ctx.Logger)
}

func OpenAPIListEnvs(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey is empty")
		return
	}

	hasPermission := false
	envFilter := make([]string, 0)

	if ctx.Resources.IsSystemAdmin {
		hasPermission = true
	}

	if projectInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		if projectInfo.IsProjectAdmin ||
			projectInfo.Env.View {
			hasPermission = true
		}
	}

	permittedEnv, _ := internalhandler.ListCollaborationEnvironmentsPermission(ctx.UserID, projectKey)
	if !hasPermission && permittedEnv != nil && len(permittedEnv.ReadEnvList) > 0 {
		hasPermission = true
		envFilter = permittedEnv.ReadEnvList
	}

	if !hasPermission {
		ctx.Resp = []*service.OpenAPIListEnvBrief{}
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIListEnvs(ctx.UserID, projectKey, envFilter, false, ctx.Logger)
}

func OpenAPIListProductionEnvs(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey is empty")
		return
	}

	hasPermission := false
	envFilter := make([]string, 0)

	if ctx.Resources.IsSystemAdmin {
		hasPermission = true
	}

	if projectInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		if projectInfo.IsProjectAdmin ||
			projectInfo.ProductionEnv.View {
			hasPermission = true
		}
	}

	permittedEnv, _ := internalhandler.ListCollaborationEnvironmentsPermission(ctx.UserID, projectKey)
	if permittedEnv != nil && len(permittedEnv.ReadEnvList) > 0 {
		hasPermission = true
		envFilter = permittedEnv.ReadEnvList
	}

	if !hasPermission {
		ctx.Resp = []*service.OpenAPIListEnvBrief{}
		return
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIListProductionEnvs(ctx.UserID, projectKey, envFilter, ctx.Logger)
}

func OpenAPIRestartService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName is empty")
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "OpenAPI"+"重启", "环境-服务", fmt.Sprintf("环境名称:%s,服务名称:%s", envName, serviceName), "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Env.ManagePods {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, types.EnvActionManagePod)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.RespErr = service.OpenAPIRestartService(projectName, envName, serviceName, false, ctx.Logger)
}

func OpenAPIProductionRestartService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName is empty")
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "OpenAPI"+"重启", "环境-服务", fmt.Sprintf("环境名称:%s,服务名称:%s", envName, serviceName), "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].ProductionEnv.ManagePods {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionManagePod)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPIRestartService(projectName, envName, serviceName, true, ctx.Logger)
}

func OpenAPICheckWorkloadsK8sServices(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.CheckWorkloadsK8sServices(c, envName, projectKey, false)
}

func OpenAPIEnableBaseEnv(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "OpenAPI-开启自测模式", "环境", envName, "", types.RequestBodyTypeJSON, ctx.Logger, envName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.EnableBaseEnv(c, envName, projectKey)
}

func OpenAPIDsiableBaseEnv(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv,
		"OpenAPI-关闭自测模式", "环境", envName,
		"", types.RequestBodyTypeJSON, ctx.Logger, envName)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.DisableBaseEnv(c, envName, projectKey)
}

type OpenAPIShareEnvReadyResponse struct {
	IsReady bool                       `json:"is_ready"`
	Checks  OpenAPIShareEnvReadyChecks `json:"checks"`
}

type OpenAPIShareEnvReadyChecks struct {
	NamespaceHasIstioLabel  bool `json:"namespace_has_istio_label"`
	VirtualServicesDeployed bool `json:"virtualservice_deployed"`
	PodsHaveIstioProxy      bool `json:"pods_have_istio_proxy"`
	WorkloadsReady          bool `json:"workloads_ready"`
	WorkloadsHaveK8sService bool `json:"workloads_have_k8s_service"`
}

func OpenAPICheckShareEnvReady(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	origResp, err := service.CheckShareEnvReady(c, envName, c.Param("op"), projectKey)
	if err != nil {
		ctx.RespErr = err
		return
	}

	resp := OpenAPIShareEnvReadyResponse{
		IsReady: origResp.IsReady,
		Checks: OpenAPIShareEnvReadyChecks{
			NamespaceHasIstioLabel:  origResp.Checks.NamespaceHasIstioLabel,
			VirtualServicesDeployed: origResp.Checks.VirtualServicesDeployed,
			PodsHaveIstioProxy:      origResp.Checks.PodsHaveIstioProxy,
			WorkloadsReady:          origResp.Checks.WorkloadsReady,
			WorkloadsHaveK8sService: origResp.Checks.WorkloadsHaveK8sService,
		},
	}
	ctx.Resp = resp

	return
}

type OpenAPIGetPortalServiceResponse struct {
	DefaultGatewayAddress string                           `json:"default_gateway_address"`
	Servers               []OpenAPISetPortalServiceRequest `json:"servers"`
}

func OpenAPIGetPortalService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName is empty")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	origResp, err := service.GetPortalService(c, projectKey, envName, serviceName)
	if err != nil {
		ctx.RespErr = err
		return
	}

	resp := OpenAPIGetPortalServiceResponse{
		DefaultGatewayAddress: origResp.DefaultGatewayAddress,
		Servers:               []OpenAPISetPortalServiceRequest{},
	}
	for _, r := range origResp.Servers {
		resp.Servers = append(resp.Servers, OpenAPISetPortalServiceRequest{
			Host:         r.Host,
			PortNumber:   r.PortNumber,
			PortProtocol: r.PortProtocol,
		})
	}

	return
}

type OpenAPISetPortalServiceRequest struct {
	Host         string `json:"host"`
	PortNumber   uint32 `json:"port_number"`
	PortProtocol string `json:"port_protocol"`
}

func OpenAPISetPortalService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName is empty")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	req := []OpenAPISetPortalServiceRequest{}
	err = c.ShouldBindJSON(&req)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	origReq := []service.SetupPortalServiceRequest{}
	for _, r := range req {
		origReq = append(origReq, service.SetupPortalServiceRequest{
			Host:         r.Host,
			PortNumber:   r.PortNumber,
			PortProtocol: r.PortProtocol,
		})
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.SetupPortalService(c, projectKey, envName, serviceName, origReq)
	return
}
