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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

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
		string(data), ctx.Logger, req.EnvName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}

	// input validation for OpenAPI
	err = req.Validate()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.OpenAPIScale(req, ctx.Logger)
}

func OpenAPIApplyYamlService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(service.OpenAPIApplyYamlServiceReq)

	if err := c.BindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}

	projectKey := c.Query("projectKey")

	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		ctx.Logger.Errorf("CreateProductTemplate c.GetRawData() err : %v", err)
	}

	// input validation for OpenAPI
	err = req.Validate()
	if err != nil {
		ctx.Err = err
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

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName+"(openAPI)", projectKey, setting.OperationSceneEnv, "更新", "测试环境", req.EnvName, string(data), ctx.Logger, req.EnvName)

	_, err = service.OpenAPIApplyYamlService(projectKey, req, false, ctx.RequestID, ctx.Logger)

	ctx.Err = err
}

func OpenAPIDeleteYamlServiceFromEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(service.OpenAPIDeleteYamlServiceFromEnvReq)

	if err := c.BindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid request body")
		return
	}

	// input validation for OpenAPI
	err := req.Validate()
	if err != nil {
		ctx.Err = err
		return
	}

	projectKey := c.Query("projectKey")

	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	svcsInSubEnvs, err := service.CheckServicesDeployedInSubEnvs(c, projectKey, req.EnvName, req.ServiceNames)
	if err != nil {
		ctx.Err = err
		return
	}

	if len(svcsInSubEnvs) > 0 {
		data := make(map[string]interface{}, len(svcsInSubEnvs))
		for k, v := range svcsInSubEnvs {
			data[k] = v
		}

		ctx.Err = e.NewWithExtras(e.ErrDeleteSvcHasSvcsInSubEnv, "", data)
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName+"(openAPI)", projectKey, setting.OperationSceneEnv, "删除", "测试环境的服务", fmt.Sprintf("%s:[%s]", req.EnvName, strings.Join(req.ServiceNames, ",")), "", ctx.Logger, req.EnvName)
	ctx.Err = service.DeleteProductServices(ctx.UserName, ctx.RequestID, req.EnvName, projectKey, req.ServiceNames, false, ctx.Logger)
}

func OpenAPIDeleteProductionYamlServiceFromEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(service.OpenAPIDeleteYamlServiceFromEnvReq)
	if err := c.BindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid request body")
		return
	}
	// input validation for OpenAPI
	err := req.Validate()
	if err != nil {
		ctx.Err = err
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName+"(openAPI)", projectKey, setting.OperationSceneEnv, "删除", "生产环境的服务", fmt.Sprintf("%s:[%s]", req.EnvName, strings.Join(req.ServiceNames, ",")), "", ctx.Logger, req.EnvName)
	ctx.Err = service.DeleteProductServices(ctx.UserName, ctx.RequestID, req.EnvName, projectKey, req.ServiceNames, true, ctx.Logger)
}

func OpenAPIApplyProductionYamlService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(service.OpenAPIApplyYamlServiceReq)
	if err := c.BindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}
	// input validation for OpenAPI
	err := req.Validate()
	if err != nil {
		ctx.Err = err
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
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
		ctx.Err = err
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "(openAPI)"+"更新", "生产环境", req.EnvName, string(data), ctx.Logger, req.EnvName)

	_, err = service.OpenAPIApplyYamlService(projectKey, req, true, ctx.RequestID, ctx.Logger)
	ctx.Err = err
}

func OpenAPIUpdateCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.OpenAPIEnvCfgArgs)

	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrUpdateEnvCfg.AddErr(fmt.Errorf("failed to get request data err : %v", err))
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.Err = e.ErrUpdateEnvCfg.AddErr(fmt.Errorf("failed to unmarshal request data err : %v", err))
		return
	}
	projectKey := c.Query("projectKey")
	args.ProductName = projectKey
	if err := args.Validate(); err != nil {
		ctx.Err = e.ErrUpdateEnvCfg.AddErr(fmt.Errorf("failed to validate request data err : %v", err))
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "(OpenAPI)"+"更新", "环境配置", fmt.Sprintf("%s:%s:%s", args.EnvName, args.CommonEnvCfgType, args.Name), string(data), ctx.Logger, args.Name)

	ctx.Err = service.OpenAPIUpdateCommonEnvCfg(projectKey, args, ctx.UserName, ctx.Logger)
}

func OpenAPIUpdateProductionCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.OpenAPIEnvCfgArgs)

	data, err := c.GetRawData()
	if err != nil {
		msg := fmt.Errorf("failed to get request data err : %v", err)
		ctx.Err = e.ErrUpdateEnvCfg.AddErr(msg)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		msg := fmt.Errorf("failed to unmarshal request data err : %v", err)
		ctx.Err = e.ErrUpdateEnvCfg.AddErr(msg)
		return
	}
	projectKey := c.Query("projectKey")
	args.ProductName = projectKey
	if err := args.Validate(); err != nil {
		msg := fmt.Errorf("failed to validate request data err : %v", err)
		ctx.Err = e.ErrUpdateEnvCfg.AddErr(msg)
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "(OpenAPI)"+"更新", "生产环境配置", fmt.Sprintf("%s:%s:%s", args.EnvName, args.CommonEnvCfgType, args.Name), string(data), ctx.Logger, args.Name)

	ctx.Err = service.OpenAPIUpdateCommonEnvCfg(projectKey, args, ctx.UserName, ctx.Logger)
}

func OpenAPICreateCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.OpenAPIEnvCfgArgs)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.Err = err
		return
	}
	args.ProductName, args.EnvName, err = generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err := args.Validate(); err != nil {
		ctx.Err = err
		return
	}
	logData, err := json.Marshal(args)
	if err != nil {
		ctx.Err = err
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, args.ProductName, setting.OperationSceneEnv, "(OpenAPI)"+"新建", "环境配置", fmt.Sprintf("%s:%s:%s", args.EnvName, args.CommonEnvCfgType, args.Name), string(logData), ctx.Logger, args.Name)

	ctx.Err = service.OpenAPICreateCommonEnvCfg(args.ProductName, args, ctx.UserName, ctx.Logger)
}

func OpenAPIListProductionCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.OpenAPIListCommonEnvCfg(projectName, envName, c.Query("type"), ctx.Logger)
}

func OpenAPIGetProductionCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	cfgName := c.Param("cfgName")
	if cfgName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("cfgName is empty")
		return
	}

	ctx.Resp, ctx.Err = service.OpenAPIGetCommonEnvCfg(projectName, envName, c.Query("type"), cfgName, ctx.Logger)
}

func OpenAPIListCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.OpenAPIListCommonEnvCfg(projectName, envName, c.Query("type"), ctx.Logger)
}

func OpenAPIGetCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	cfgName := c.Param("cfgName")
	if cfgName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("cfgName is empty")
		return
	}

	ctx.Resp, ctx.Err = service.OpenAPIGetCommonEnvCfg(projectName, envName, c.Query("type"), cfgName, ctx.Logger)
}

func OpenAPIDeleteCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	cfgName := c.Param("cfgName")
	if cfgName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("cfgName is empty")
		return
	}
	cfgType := c.Query("type")
	if cfgType == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("type is empty")
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "OpenAPI"+"删除", "测试环境配置", fmt.Sprintf("%s:%s:%s", envName, cfgType, cfgName), "", ctx.Logger, envName)

	ctx.Err = service.OpenAPIDeleteCommonEnvCfg(projectName, envName, cfgType, cfgName, ctx.Logger)
}

func OpenAPIDeleteProductionEnvCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	cfgName := c.Param("cfgName")
	if cfgName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("cfgName is empty")
		return
	}
	cfgType := c.Query("type")
	if cfgType == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("type is empty")
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "OpenAPI"+"删除", "生产环境配置", fmt.Sprintf("%s:%s:%s", envName, cfgType, cfgName), "", ctx.Logger, envName)

	ctx.Err = service.OpenAPIDeleteProductionEnvCommonEnvCfg(projectName, envName, cfgType, cfgName, ctx.Logger)
}

func OpenAPICreateK8sEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.OpenAPICreateEnvArgs)

	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.Err = err
		return
	}
	args.ProjectName = c.Query("projectKey")
	if err := args.Validate(); err != nil {
		ctx.Err = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, args.ProjectName, setting.OperationSceneEnv, "(OpenAPI)"+"创建", "测试环境", args.EnvName, string(data), ctx.Logger, args.EnvName)

	ctx.Err = service.OpenAPICreateK8sEnv(args, ctx.UserName, ctx.RequestID, ctx.Logger)
}

func OpenAPIDeleteProductionEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"删除", "生产环境", envName, "", ctx.Logger, envName)

	ctx.Err = service.DeleteProductionProduct(ctx.UserName, envName, projectName, ctx.RequestID, ctx.Logger)
}

func OpenAPICreateProductionEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.OpenAPICreateEnvArgs)

	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.Err = err
		return
	}
	args.ProjectName = c.Query("projectKey")
	args.Production = true
	if err := args.Validate(); err != nil {
		ctx.Err = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, args.ProjectName, setting.OperationSceneEnv, "(OpenAPI)"+"创建", "生产环境", args.EnvName, string(data), ctx.Logger, args.EnvName)

	ctx.Err = service.OpenAPICreateProductionEnv(args, ctx.UserName, ctx.RequestID, ctx.Logger)
}

func OpenAPIDeleteEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrDeleteResource.AddErr(err)
		return
	}
	isDelete, err := strconv.ParseBool(c.Query("isDelete"))
	if err != nil {
		ctx.Err = e.ErrDeleteResource.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"删除", "测试环境", envName, "", ctx.Logger, envName)

	ctx.Err = service.DeleteProduct(ctx.UserName, envName, projectName, ctx.RequestID, isDelete, ctx.Logger)
}

func OpenAPIGetEnvDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.GetEnvDetail(projectName, envName, ctx.Logger)
}

func OpenAPIGetProductionEnvDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.GetEnvDetail(projectName, envName, ctx.Logger)
}

func OpenAPIUpdateEnvBasicInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.EnvBasicInfoArgs)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.Err = err
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"更新", "测试环境", envName, string(data), ctx.Logger, envName)

	ctx.Err = service.OpenAPIUpdateEnvBasicInfo(args, ctx.UserName, projectName, envName, false, ctx.Logger)
}

func OpenAPIUpdateYamlServices(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.OpenAPIServiceVariablesReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.Err = err
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"环境管理-更新服务", "测试环境", envName, string(data), ctx.Logger, envName)

	ctx.Err = service.OpenAPIUpdateYamlService(args, ctx.UserName, ctx.RequestID, projectName, envName, false, ctx.Logger)
}

func OpenAPIGetEnvGlobalVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.OpenAPIGetGlobalVariables(projectName, envName, false, ctx.Logger)
}

func OpenAPIGetProductionEnvGlobalVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.OpenAPIGetGlobalVariables(projectName, envName, true, ctx.Logger)
}

func OpenAPIUpdateGlobalVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.OpenAPIEnvGlobalVariables)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.Err = err
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"更新", "测试环境管理-更新全局变量", envName, string(data), ctx.Logger, envName)

	ctx.Err = service.OpenAPIUpdateGlobalVariables(args, ctx.UserName, ctx.RequestID, projectName, envName, ctx.Logger)
}

func OpenAPIUpdateProductionYamlServices(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.OpenAPIServiceVariablesReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.Err = err
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"环境管理-更新服务", "生产环境", envName, string(data), ctx.Logger, envName)

	ctx.Err = service.OpenAPIUpdateYamlService(args, ctx.UserName, ctx.RequestID, projectName, envName, true, ctx.Logger)
}

func OpenAPIUpdateProductionGlobalVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.OpenAPIEnvGlobalVariables)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, &args); err != nil {
		ctx.Err = err
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"更新", "生产环境管理-更新全局变量", envName, string(data), ctx.Logger, envName)

	ctx.Err = service.OpenAPIUpdateGlobalVariables(args, ctx.UserName, ctx.RequestID, projectName, envName, ctx.Logger)
}

func OpenAPIUpdateProductionEnvBasicInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.EnvBasicInfoArgs)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.Err = err
		return
	}

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "(OpenAPI)"+"更新", "生产环境", envName, string(data), ctx.Logger, envName)

	ctx.Err = service.OpenAPIUpdateEnvBasicInfo(args, ctx.UserName, projectName, envName, true, ctx.Logger)
}

func OpenAPIListEnvs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectKey is empty")
		return
	}
	ctx.Resp, ctx.Err = service.OpenAPIListEnvs(projectKey, ctx.Logger)
}

func OpenAPIListProductionEnvs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectKey is empty")
		return
	}
	ctx.Resp, ctx.Err = service.OpenAPIListProductionEnvs(projectKey, ctx.Logger)
}

func OpenAPIRestartService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalOpenAPIRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("serviceName is empty")
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "OpenAPI"+"重启", "环境-服务", fmt.Sprintf("环境名称:%s,服务名称:%s", envName, serviceName), "", ctx.Logger)
	ctx.Err = service.OpenAPIRestartService(projectName, envName, serviceName, ctx.Logger)
}
