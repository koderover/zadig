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
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	envservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type openAPIEnvServiceVersionParams struct {
	projectKey  string
	envName     string
	serviceName string
	production  bool
	isHelmChart bool
}

func OpenAPIListEnvServiceVersions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	params, ok := parseOpenAPIEnvServiceVersionParams(c, ctx, false, false)
	if !ok {
		return
	}
	ctx.Resp, ctx.RespErr = commonservice.ListEnvServiceVersions(ctx, params.projectKey, params.envName, params.serviceName, params.isHelmChart, params.production, ctx.Logger)
}

func OpenAPIGetEnvServiceVersionYaml(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	params, ok := parseOpenAPIEnvServiceVersionParams(c, ctx, false, false)
	if !ok {
		return
	}
	revision, ok := parseOpenAPIRevision(ctx, "revision", c.Param("revision"))
	if !ok {
		return
	}
	ctx.Resp, ctx.RespErr = commonservice.GetEnvServiceVersionYaml(ctx, params.projectKey, params.envName, params.serviceName, revision, params.isHelmChart, params.production, ctx.Logger)
}

func OpenAPIDiffEnvServiceVersions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	// same as the internal API: users who can execute workflows may also diff versions
	params, ok := parseOpenAPIEnvServiceVersionParams(c, ctx, false, true)
	if !ok {
		return
	}
	revisionA, ok := parseOpenAPIRevision(ctx, "revisionA", c.Query("revisionA"))
	if !ok {
		return
	}
	revisionB, ok := parseOpenAPIRevision(ctx, "revisionB", c.Query("revisionB"))
	if !ok {
		return
	}
	ctx.Resp, ctx.RespErr = commonservice.DiffEnvServiceVersions(ctx, params.projectKey, params.envName, params.serviceName, revisionA, revisionB, params.isHelmChart, params.production, ctx.Logger)
}

func OpenAPICheckRollbackEnvServiceVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	params, ok := parseOpenAPIEnvServiceVersionParams(c, ctx, true, false)
	if !ok {
		return
	}
	revision, ok := parseOpenAPIRevision(ctx, "revision", c.Query("revision"))
	if !ok {
		return
	}
	ctx.Resp, ctx.RespErr = commonservice.CheckRollbackEnvServiceVersion(params.projectKey, params.envName, params.serviceName, revision, params.isHelmChart, params.production, ctx.Logger)
}

func OpenAPIRollbackEnvServiceVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	params, ok := parseOpenAPIEnvServiceVersionParams(c, ctx, true, false)
	if !ok {
		return
	}
	revision, ok := parseOpenAPIRevision(ctx, "revision", c.Query("revision"))
	if !ok {
		return
	}
	overrideResource, ok := parseOpenAPIOptionalBool(ctx, c, "overrideResource")
	if !ok {
		return
	}

	// the request body is optional for OpenAPI callers
	req := new(RollbackEnvServiceVersionRequest)
	if err := c.ShouldBindJSON(req); err != nil && !errors.Is(err, io.EOF) {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid rollback request body")
		return
	}

	detail := fmt.Sprintf("环境: %s, 服务: %s, 版本: %d", params.envName, params.serviceName, revision)
	detailEn := fmt.Sprintf("Environment: %s, Service: %s, Version: %d", params.envName, params.serviceName, revision)
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName+"(OpenAPI)", params.projectKey, setting.OperationSceneEnv, "回滚", "环境-服务", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger, params.envName)

	// same as the internal API: Helm releases are upgraded asynchronously after this returns
	_, ctx.RespErr = commonservice.RollbackEnvServiceVersion(ctx, params.projectKey, params.envName, params.serviceName, revision, params.isHelmChart, params.production, overrideResource, req.Detail, ctx.Logger)
	if ctx.RespErr == nil {
		ctx.Resp = gin.H{"message": "success"}
	}
}

// parseOpenAPIEnvServiceVersionParams validates the common params, checks permission and license,
// and resolves isHelmChart from the env when the caller does not specify it.
func parseOpenAPIEnvServiceVersionParams(c *gin.Context, ctx *internalhandler.Context, rollback, allowWorkflowExecute bool) (*openAPIEnvServiceVersionParams, bool) {
	params := &openAPIEnvServiceVersionParams{
		projectKey:  c.Query("projectKey"),
		envName:     c.Param("name"),
		serviceName: c.Param("serviceName"),
	}
	if params.projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return nil, false
	}
	if params.envName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("envName cannot be empty")
		return nil, false
	}
	if params.serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName cannot be empty")
		return nil, false
	}
	// reject malformed values instead of silently falling back to the test env
	production, ok := parseOpenAPIOptionalBool(ctx, c, "production")
	if !ok {
		return nil, false
	}
	params.production = production

	if !authorizeOpenAPIEnvServiceVersion(ctx, params.projectKey, params.envName, params.production, rollback, allowWorkflowExecute) {
		return nil, false
	}
	// same as the internal API: rollback always needs the license, while viewing production
	// versions needs it only for non system admins
	if rollback || (params.production && !ctx.Resources.IsSystemAdmin) {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return nil, false
		}
	}

	if _, specified := c.GetQuery("isHelmChart"); specified {
		params.isHelmChart, ok = parseOpenAPIOptionalBool(ctx, c, "isHelmChart")
		return params, ok
	}
	isHelmChart, err := envservice.ResolveEnvServiceIsHelmChart(params.projectKey, params.envName, params.serviceName, params.production)
	if err != nil {
		ctx.RespErr = err
		return nil, false
	}
	params.isHelmChart = isHelmChart
	return params, true
}

// parseOpenAPIOptionalBool returns false when the query is absent and rejects malformed values.
func parseOpenAPIOptionalBool(ctx *internalhandler.Context, c *gin.Context, name string) (bool, bool) {
	raw, specified := c.GetQuery(name)
	if !specified {
		return false, true
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid %s: %s", name, raw))
		return false, false
	}
	return value, true
}

func parseOpenAPIRevision(ctx *internalhandler.Context, name, raw string) (int64, bool) {
	revision, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid %s: %s", name, raw))
		return 0, false
	}
	return revision, true
}

func authorizeOpenAPIEnvServiceVersion(ctx *internalhandler.Context, projectKey, envName string, production, rollback, allowWorkflowExecute bool) bool {
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

	allowed, action := projectAuth.Env.View, types.EnvActionView
	switch {
	case production && rollback:
		allowed, action = projectAuth.ProductionEnv.Rollback, types.ProductionEnvActionRollback
	case production:
		allowed, action = projectAuth.ProductionEnv.View, types.ProductionEnvActionView
	case rollback:
		allowed, action = projectAuth.Env.Rollback, types.EnvActionRollback
	}
	if !rollback && allowWorkflowExecute && projectAuth.Workflow.Execute {
		allowed = true
	}
	if !allowed {
		permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, action)
		allowed = err == nil && permitted
	}
	ctx.UnAuthorized = !allowed
	return allowed
}
