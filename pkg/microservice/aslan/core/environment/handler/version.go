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
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary List Environment Service Versions
// @Description List Environment Service Versions
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	name			path		string							true	"env name"
// @Param 	serviceName		path		string							true	"service name or release name when isHelmChart is true"
// @Param 	projectName		query		string							true	"project name"
// @Param 	isHelmChart		query		bool							true	"is helm chart type"
// @Success 200 			{array}  	service.ListEnvServiceVersionsResponse
// @Router /api/aslan/environment/environments/{name}/version/{serviceName} [get]
func ListEnvServiceVersions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.View {
			ctx.UnAuthorized = true
			return
		}
	}

	envName := c.Param("name")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty name")
		return
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	isHelmChart, err := strconv.ParseBool(c.Query("isHelmChart"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid isHelmChart: %s", err))
		return
	}

	ctx.Resp, ctx.Err = service.ListEnvServiceVersions(ctx, projectKey, envName, serviceName, isHelmChart, false, ctx.Logger)
}

// @Summary List Production Environment Service Versions
// @Description List Production Environment Service Versions
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	name			path		string							true	"env name"
// @Param 	serviceName		path		string							true	"service name or release name when isHelmChart is true"
// @Param 	projectName		query		string							true	"project name"
// @Param 	isHelmChart		query		bool							true	"is helm chart type"
// @Param 	releaseName		query		string							true	"release name"
// @Success 200 			{array}  	service.ListEnvServiceVersionsResponse
// @Router /api/aslan/environment/production/environments/{name}/version/{serviceName} [get]
func ListProductionEnvServiceVersions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View {
			ctx.UnAuthorized = true
			return
		}
	}

	envName := c.Param("name")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty name")
		return
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	isHelmChart, err := strconv.ParseBool(c.Query("isHelmChart"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid isHelmChart: %s", err))
		return
	}

	ctx.Resp, ctx.Err = service.ListEnvServiceVersions(ctx, projectKey, envName, serviceName, isHelmChart, true, ctx.Logger)
}

// @Summary Get Environment Service Version Yaml
// @Description Get Environment Service Version Yaml
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	name			path		string							true	"env name"
// @Param 	serviceName		path		string							true	"service name or release name when isHelmChart is true"
// @Param 	revision		path		string							true	"revision"
// @Param 	projectName		query		string							true	"project name"
// @Param 	isHelmChart		query		bool							true	"is helm chart type"
// @Param 	releaseName		query		string							true	"release name"
// @Success 200 			{array}  	service.GetEnvServiceVersionYamlResponse
// @Router /api/aslan/environment/environments/{name}/version/{serviceName}/revision/{revision} [get]
func GetEnvServiceVersionYaml(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.View {
			ctx.UnAuthorized = true
			return
		}
	}

	envName := c.Param("name")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty name")
		return
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	revision, err := strconv.ParseInt(c.Param("revision"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revision: %s", err))
		return
	}

	isHelmChart, err := strconv.ParseBool(c.Query("isHelmChart"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid isHelmChart: %s", err))
		return
	}

	ctx.Resp, ctx.Err = service.GetEnvServiceVersionYaml(ctx, projectKey, envName, serviceName, revision, isHelmChart, false, ctx.Logger)
}

// @Summary Get Production Environment Service Version Yaml
// @Description Get Production Environment Service Version Yaml
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	name			path		string							true	"env name"
// @Param 	serviceName		path		string							true	"service name or release name when isHelmChart is true"
// @Param 	revision		path		string							true	"revision"
// @Param 	projectName		query		string							true	"project name"
// @Param 	isHelmChart		query		bool							true	"is helm chart type"
// @Param 	releaseName		query		string							true	"release name"
// @Success 200 			{array}  	service.GetEnvServiceVersionYamlResponse
// @Router /api/aslan/environment/production/environments/{name}/version/{serviceName}/revision/{revision} [get]
func GetProductionEnvServiceVersionYaml(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View {
			ctx.UnAuthorized = true
			return
		}
	}

	envName := c.Param("name")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty name")
		return
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	revision, err := strconv.ParseInt(c.Param("revision"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revision: %s", err))
		return
	}

	isHelmChart, err := strconv.ParseBool(c.Query("isHelmChart"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid isHelmChart: %s", err))
		return
	}

	ctx.Resp, ctx.Err = service.GetEnvServiceVersionYaml(ctx, projectKey, envName, serviceName, revision, isHelmChart, true, ctx.Logger)
}

// @Summary Diff Environment Service Versions
// @Description Diff Environment Service Versions
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	name			path		string							true	"env name"
// @Param 	serviceName		path		string							true	"service name or release name when isHelmChart is true"
// @Param 	projectName		query		string							true	"project name"
// @Param 	revisionA		query		int								true	"revision a"
// @Param 	revisionB		query		int								true	"revision b"
// @Param 	isHelmChart		query		bool							true	"is helm chart type"
// @Param 	releaseName		query		string							true	"release name"
// @Success 200 			{object}  	service.ListEnvServiceVersionsResponse
// @Router /api/aslan/environment/environments/{name}/version/{serviceName}/diff [get]
func DiffEnvServiceVersions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.View {
			ctx.UnAuthorized = true
			return
		}
	}

	envName := c.Param("name")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty name")
		return
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	revisionA, err := strconv.ParseInt(c.Query("revisionA"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revisionA: %s", err))
		return
	}
	revisionB, err := strconv.ParseInt(c.Query("revisionB"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revisionB: %s", err))
		return
	}

	isHelmChart, err := strconv.ParseBool(c.Query("isHelmChart"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid isHelmChart: %s", err))
		return
	}

	ctx.Resp, ctx.Err = service.DiffEnvServiceVersions(ctx, projectKey, envName, serviceName, revisionA, revisionB, isHelmChart, false, ctx.Logger)
}

// @Summary Diff Production Environment Service Versions
// @Description Diff Production Environment Service Versions
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	name			path		string							true	"env name"
// @Param 	serviceName		path		string							true	"service name or release name when isHelmChart is true"
// @Param 	projectName		query		string							true	"project name"
// @Param 	revisionA		query		int								true	"revision a"
// @Param 	revisionB		query		int								true	"revision b"
// @Param 	isHelmChart		query		bool							true	"is helm chart type"
// @Param 	releaseName		query		string							true	"release name"
// @Success 200 			{object}  	service.ListEnvServiceVersionsResponse
// @Router /api/aslan/environment/production/environments/{name}/version/{serviceName}/diff [get]
func DiffProductionEnvServiceVersions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View {
			ctx.UnAuthorized = true
			return
		}
	}

	envName := c.Param("name")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty name")
		return
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	revisionA, err := strconv.ParseInt(c.Query("revisionA"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revisionA: %s", err))
		return
	}
	revisionB, err := strconv.ParseInt(c.Query("revisionB"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revisionB: %s", err))
		return
	}

	isHelmChart, err := strconv.ParseBool(c.Query("isHelmChart"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid isHelmChart: %s", err))
		return
	}

	ctx.Resp, ctx.Err = service.DiffEnvServiceVersions(ctx, projectKey, envName, serviceName, revisionA, revisionB, isHelmChart, true, ctx.Logger)
}

// @Summary Rollback Environment Service Version
// @Description Rollback Environment Service Version
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	name			path		string							true	"env name"
// @Param 	serviceName		path		string							true	"service name or release name when isHelmChart is true"
// @Param 	projectName		query		string							true	"project name"
// @Param 	revision	 	query		int								true	"revision"
// @Param 	isHelmChart		query		bool							true	"is helm chart type"
// @Success 200
// @Router /api/aslan/environment/environments/{name}/version/{serviceName}/rollback [post]
func RollbackEnvServiceVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			ctx.UnAuthorized = true
			return
		}
	}

	envName := c.Param("name")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty name")
		return
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	revision, err := strconv.ParseInt(c.Query("revision"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revison: %s", err))
		return
	}

	isHelmChart, err := strconv.ParseBool(c.Query("isHelmChart"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid isHelmChart: %s", err))
		return
	}

	if err := commonutil.CheckZadigXLicenseStatus(); err != nil {
		ctx.Err = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "回滚", "环境-服务", fmt.Sprintf("环境: %s, 服务: %s, 版本: %d", envName, serviceName, revision), "", ctx.Logger, envName)

	ctx.Err = service.RollbackEnvServiceVersion(ctx, projectKey, envName, serviceName, revision, isHelmChart, false, ctx.Logger)
}

// @Summary Rollback Production Environment Service Version
// @Description Rollback Production Environment Service Version
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	name			path		string							true	"env name"
// @Param 	serviceName		path		string							true	"service name or release name when isHelmChart is true"
// @Param 	projectName		query		string							true	"project name"
// @Param 	revision	 	query		int								true	"revision"
// @Param 	isHelmChart		query		bool							true	"is helm chart type"
// @Success 200
// @Router /api/aslan/environment/production/environments/{name}/version/{serviceName}/rollback [post]
func RollbackProductionEnvServiceVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.EditConfig {
			ctx.UnAuthorized = true
			return
		}
	}

	envName := c.Param("name")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty name")
		return
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	revision, err := strconv.ParseInt(c.Query("revision"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revision: %s", err))
		return
	}

	isHelmChart, err := strconv.ParseBool(c.Query("isHelmChart"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid isHelmChart: %s", err))
		return
	}

	if err := commonutil.CheckZadigXLicenseStatus(); err != nil {
		ctx.Err = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "回滚", "生产环境-服务", fmt.Sprintf("环境: %s, 服务: %s, 版本: %d", envName, serviceName, revision), "", ctx.Logger, envName)

	ctx.Err = service.RollbackEnvServiceVersion(ctx, projectKey, envName, serviceName, revision, isHelmChart, true, ctx.Logger)
}
