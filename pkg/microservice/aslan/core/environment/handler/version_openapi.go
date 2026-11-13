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

// @Summary 获取环境服务版本列表
// @Description 获取指定环境服务的全部历史版本，按版本号从小到大排列
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param name path string true "环境标识"
// @Param serviceName path string true "服务名称；独立 Chart 填写 Release 名称"
// @Param projectKey query string true "项目标识"
// @Param production query bool false "是否为生产环境，默认为 false"
// @Param isHelmChart query bool false "serviceName 是否为独立 Chart 的 Release 名称；不传时由服务端识别，同名时必须传入"
// @Success 200 {array} service.ListEnvServiceVersionsResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /openapi/environments/{name}/version/{serviceName} [get]
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

// @Summary 获取环境服务版本详情
// @Description 获取指定环境服务版本的配置和镜像
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param name path string true "环境标识"
// @Param serviceName path string true "服务名称；独立 Chart 填写 Release 名称"
// @Param revision path int true "环境服务版本号"
// @Param projectKey query string true "项目标识"
// @Param production query bool false "是否为生产环境，默认为 false"
// @Param isHelmChart query bool false "serviceName 是否为独立 Chart 的 Release 名称；不传时由服务端识别，同名时必须传入"
// @Success 200 {object} service.GetEnvServiceVersionYamlResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /openapi/environments/{name}/version/{serviceName}/revision/{revision} [get]
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

// @Summary 对比环境服务版本
// @Description 获取两个环境服务版本的配置和镜像，供调用方对比差异
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param name path string true "环境标识"
// @Param serviceName path string true "服务名称；独立 Chart 填写 Release 名称"
// @Param projectKey query string true "项目标识"
// @Param revisionA query int true "版本 A 的版本号，-1 表示最新版本"
// @Param revisionB query int true "版本 B 的版本号，-1 表示最新版本"
// @Param production query bool false "是否为生产环境，默认为 false"
// @Param isHelmChart query bool false "serviceName 是否为独立 Chart 的 Release 名称；不传时由服务端识别，同名时必须传入"
// @Success 200 {object} service.DiffEnvServiceVersionsResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /openapi/environments/{name}/version/{serviceName}/diff [get]
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

// @Summary 检查环境服务版本是否可回滚
// @Description 检查目标版本使用的镜像是否仍存在于已集成的镜像仓库中
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param name path string true "环境标识"
// @Param serviceName path string true "服务名称；独立 Chart 填写 Release 名称"
// @Param projectKey query string true "项目标识"
// @Param revision query int true "要回滚到的目标版本号"
// @Param production query bool false "是否为生产环境，默认为 false"
// @Param isHelmChart query bool false "serviceName 是否为独立 Chart 的 Release 名称；不传时由服务端识别，同名时必须传入"
// @Success 200 {object} service.CheckRollbackEnvServiceVersionResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /openapi/environments/{name}/version/{serviceName}/rollback/check [get]
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

// OpenAPIRollbackEnvServiceVersionRequest carries the rollback action; the target service is located by path and query.
type OpenAPIRollbackEnvServiceVersionRequest struct {
	Revision         int64  `json:"revision"`
	OverrideResource bool   `json:"override_resource"`
	Detail           string `json:"detail"`
}

// @Summary 回滚环境服务版本
// @Description 将环境中的服务回滚到指定的历史版本。Helm 服务和独立 Chart 等待 Release 升级完成后返回，升级失败时返回错误；不等待 Pod 就绪
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param name path string true "环境标识"
// @Param serviceName path string true "服务名称；独立 Chart 填写 Release 名称"
// @Param projectKey query string true "项目标识"
// @Param production query bool false "是否为生产环境，默认为 false"
// @Param isHelmChart query bool false "serviceName 是否为独立 Chart 的 Release 名称；不传时由服务端识别，同名时必须传入"
// @Param body body OpenAPIRollbackEnvServiceVersionRequest true "回滚请求"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /openapi/environments/{name}/version/{serviceName}/rollback [post]
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

	req := new(OpenAPIRollbackEnvServiceVersionRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid rollback request body: %v", err))
		return
	}
	if req.Revision < 1 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("revision must be a positive integer")
		return
	}

	detail := fmt.Sprintf("环境: %s, 服务: %s, 版本: %d", params.envName, params.serviceName, req.Revision)
	detailEn := fmt.Sprintf("Environment: %s, Service: %s, Version: %d", params.envName, params.serviceName, req.Revision)
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName+"(OpenAPI)", params.projectKey, setting.OperationSceneEnv, "回滚", "环境-服务", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger, params.envName)

	rollbackStatus, err := commonservice.RollbackEnvServiceVersion(ctx, params.projectKey, params.envName, params.serviceName, req.Revision, params.isHelmChart, params.production, req.OverrideResource, req.Detail, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	// Unlike the console, OpenAPI callers have no notification channel, so wait for the Helm upgrade result.
	if rollbackStatus != nil && rollbackStatus.HelmDeployStatusChan != nil {
		select {
		case ok := <-rollbackStatus.HelmDeployStatusChan:
			if !ok {
				ctx.RespErr = e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to upgrade helm release: %w", rollbackStatus.HelmDeployErr))
				return
			}
		case <-c.Request.Context().Done():
			ctx.RespErr = e.ErrRollbackEnvServiceVersion.AddDesc("request canceled before the helm release upgrade finished; the upgrade continues in the background")
			return
		}
	}
	ctx.Resp = gin.H{"message": "success"}
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
