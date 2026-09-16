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
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	envservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

func OpenAPIGetHelmServiceValues(c *gin.Context) {
	production := c.Query("production") == "true"
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	projectKey, envName, serviceName, valid := validateOpenAPIHelmValuesParams(c, ctx)
	if !valid || !validateOpenAPIHelmServiceType(c, ctx) || !authorizeOpenAPIHelmValues(ctx, projectKey, envName, production, false) || !checkOpenAPIHelmValuesLicense(ctx, production) {
		return
	}
	ctx.Resp, ctx.RespErr = envservice.GetHelmServiceValuesOpenAPI(projectKey, envName, serviceName, production)
}

func OpenAPIGetHelmReleaseValues(c *gin.Context) {
	production := c.Query("production") == "true"
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	projectKey, envName, serviceName, valid := validateOpenAPIHelmValuesParams(c, ctx)
	if !valid || !validateOpenAPIHelmServiceType(c, ctx) || !authorizeOpenAPIHelmValues(ctx, projectKey, envName, production, false) || !checkOpenAPIHelmValuesLicense(ctx, production) {
		return
	}
	values, err := commonservice.GetChartValues(projectKey, envName, serviceName, false, production, true)
	if err != nil {
		ctx.RespErr = e.ErrGetEnv.AddErr(err)
		return
	}
	ctx.Resp = &envservice.OpenAPIHelmReleaseValues{ValuesYAML: values.ValuesYaml}
}

func OpenAPIGetHelmValuesSource(c *gin.Context) {
	production := c.Query("production") == "true"
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	projectKey, envName, serviceName, valid := validateOpenAPIHelmValuesParams(c, ctx)
	if !valid || !authorizeOpenAPIHelmValues(ctx, projectKey, envName, production, false) || !checkOpenAPIHelmValuesLicense(ctx, production) {
		return
	}
	ctx.Resp, ctx.RespErr = envservice.GetHelmValuesSourceOpenAPI(projectKey, envName, serviceName, production)
}

func OpenAPIUpdateHelmValuesSource(c *gin.Context) {
	production := c.Query("production") == "true"
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	projectKey, envName, serviceName, valid := validateOpenAPIHelmValuesParams(c, ctx)
	if !valid {
		return
	}
	req := new(envservice.OpenAPIUpdateHelmValuesSourceReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid Helm Values source request body")
		return
	}
	if err := req.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if !authorizeOpenAPIHelmValues(ctx, projectKey, envName, production, true) || !checkOpenAPIHelmValuesLicense(ctx, production) {
		return
	}
	data, _ := json.Marshal(req)
	detail := fmt.Sprintf("%s:%s", envName, serviceName)
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName+"(OpenAPI)", projectKey, setting.OperationSceneEnv, "更新", "Helm Values 来源配置", detail, detail, string(data), types.RequestBodyTypeJSON, ctx.Logger, envName)
	ctx.RespErr = envservice.UpdateHelmValuesSourceOpenAPI(projectKey, envName, serviceName, ctx.UserName, production, req)
	if ctx.RespErr == nil {
		ctx.Resp = gin.H{"message": "success"}
	}
}

func OpenAPIDeleteHelmValuesSource(c *gin.Context) {
	production := c.Query("production") == "true"
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	projectKey, envName, serviceName, valid := validateOpenAPIHelmValuesParams(c, ctx)
	if !valid || !authorizeOpenAPIHelmValues(ctx, projectKey, envName, production, true) || !checkOpenAPIHelmValuesLicense(ctx, production) {
		return
	}
	detail := fmt.Sprintf("%s:%s", envName, serviceName)
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName+"(OpenAPI)", projectKey, setting.OperationSceneEnv, "删除", "Helm Values 来源配置", detail, detail, "", types.RequestBodyTypeJSON, ctx.Logger, envName)
	ctx.RespErr = envservice.UpdateHelmValuesSource(projectKey, envName, serviceName, ctx.UserName, production, false, &envservice.UpdateHelmValuesSourceArgs{})
	if ctx.RespErr == nil {
		ctx.Resp = gin.H{"message": "success"}
	}
}

func OpenAPIPreviewHelmServiceValues(c *gin.Context) {
	handleOpenAPIHelmServiceValuesUpdate(c, c.Query("production") == "true", true)
}

func OpenAPIUpdateHelmServiceValues(c *gin.Context) {
	handleOpenAPIHelmServiceValuesUpdate(c, c.Query("production") == "true", false)
}

func handleOpenAPIHelmServiceValuesUpdate(c *gin.Context, production, preview bool) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	projectKey, envName, serviceName, valid := validateOpenAPIHelmValuesParams(c, ctx)
	if !valid {
		return
	}
	req := new(envservice.OpenAPIUpdateHelmValuesReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid Helm Values update request body")
		return
	}
	if err := req.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if !authorizeOpenAPIHelmValues(ctx, projectKey, envName, production, true) || !checkOpenAPIHelmValuesLicense(ctx, production) {
		return
	}
	if preview {
		ctx.Resp, ctx.RespErr = envservice.PreviewHelmServiceValuesOpenAPI(projectKey, envName, serviceName, production, req, ctx.Logger)
		return
	}

	data, _ := json.Marshal(req)
	detail := fmt.Sprintf("%s:%s", envName, serviceName)
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName+"(OpenAPI)", projectKey, setting.OperationSceneEnv, "更新", "更新服务", detail, detail, string(data), types.RequestBodyTypeJSON, ctx.Logger, envName)
	ctx.RespErr = envservice.UpdateHelmServiceValuesOpenAPI(projectKey, envName, serviceName, ctx.UserName, ctx.RequestID, production, req, ctx.Logger)
	if ctx.RespErr == nil {
		ctx.Resp = gin.H{"message": "success"}
	}
}

func validateOpenAPIHelmValuesParams(c *gin.Context, ctx *internalhandler.Context) (string, string, string, bool) {
	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return "", "", "", false
	}
	envName := c.Param("name")
	if envName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("envName cannot be empty")
		return "", "", "", false
	}
	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName cannot be empty")
		return "", "", "", false
	}
	return projectKey, envName, serviceName, true
}

func validateOpenAPIHelmServiceType(c *gin.Context, ctx *internalhandler.Context) bool {
	serviceType, specified := c.GetQuery("type")
	if specified && serviceType != "service" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("type must be service")
		return false
	}
	return true
}

func authorizeOpenAPIHelmValues(ctx *internalhandler.Context, projectKey, envName string, production, edit bool) bool {
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	projectAuth, ok := ctx.Resources.ProjectAuthInfo[projectKey]
	if !ok {
		ctx.UnAuthorized = true
		return false
	}
	if projectAuth.IsProjectAdmin {
		return true
	}
	allowed := projectAuth.Env.View
	action := types.EnvActionView
	if production {
		allowed = projectAuth.ProductionEnv.View
		action = types.ProductionEnvActionView
	}
	if edit {
		allowed = projectAuth.Env.EditConfig
		action = types.EnvActionEditConfig
		if production {
			allowed = projectAuth.ProductionEnv.EditConfig
			action = types.ProductionEnvActionEditConfig
		}
	}
	if !allowed {
		permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, action)
		allowed = err == nil && permitted
	}
	ctx.UnAuthorized = !allowed
	return allowed
}

func checkOpenAPIHelmValuesLicense(ctx *internalhandler.Context, production bool) bool {
	if !production {
		return true
	}
	if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
		ctx.RespErr = err
		return false
	}
	return true
}
