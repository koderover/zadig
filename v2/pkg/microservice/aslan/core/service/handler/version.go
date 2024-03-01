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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary List Service Versions
// @Description List Service Versions
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Success 200 			{array}  	service.ListServiceVersionsResponse
// @Router /api/aslan/service/version/{serviceName} [get]
func ListServiceVersions(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
			ctx.UnAuthorized = true
			return
		}
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	ctx.Resp, ctx.Err = service.ListServiceVersions(ctx, projectKey, serviceName, false, ctx.Logger)
}

// @Summary List Production Service Versions
// @Description List Production Service Versions
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Success 200 			{array}  	service.ListServiceVersionsResponse
// @Router /api/aslan/service/production/version/{serviceName} [get]
func ListProductionServiceVersions(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.View {
			ctx.UnAuthorized = true
			return
		}
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	ctx.Resp, ctx.Err = service.ListServiceVersions(ctx, projectKey, serviceName, true, ctx.Logger)
}

// @Summary Get Service Version Yaml
// @Description Get Service Versions Yaml
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	revision		path		string							true	"revision"
// @Success 200 			{object}  	service.GetServiceVersionYamlResponse
// @Router /api/aslan/service/version/{serviceName}/revision/{revision} [get]
func GetServiceVersionYaml(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
			ctx.UnAuthorized = true
			return
		}
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	revision, err := strconv.ParseInt(c.Param("revision"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revison: %s", err))
		return
	}

	ctx.Resp, ctx.Err = service.GetServiceVersionYaml(ctx, projectKey, serviceName, revision, false, ctx.Logger)
}

// @Summary Get Production Service Version Yaml
// @Description Get Production Service Versions Yaml
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	revision		path		string							true	"revision"
// @Success 200 			{object}  	service.GetServiceVersionYamlResponse
// @Router /api/aslan/service/production/version/{serviceName}/revision/{revision} [get]
func GetProductionServiceVersionYaml(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.View {
			ctx.UnAuthorized = true
			return
		}
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

	ctx.Resp, ctx.Err = service.GetServiceVersionYaml(ctx, projectKey, serviceName, revision, true, ctx.Logger)
}

// @Summary Diff Service Versions
// @Description Diff Service Versions
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	revisionA		query		int								true	"revision a"
// @Param 	revisionB		query		int								true	"revision b"
// @Success 200 			{object}  	service.ListServiceVersionsResponse
// @Router /api/aslan/service/version/{serviceName}/diff [get]
func DiffServiceVersions(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
			ctx.UnAuthorized = true
			return
		}
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

	ctx.Resp, ctx.Err = service.DiffServiceVersions(ctx, projectKey, serviceName, revisionA, revisionB, false, ctx.Logger)
}

// @Summary Diff Production Service Versions
// @Description Diff Production Service Versions
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	revisionA		query		int								true	"revision a"
// @Param 	revisionB		query		int								true	"revision b"
// @Success 200 			{object}  	service.ListServiceVersionsResponse
// @Router /api/aslan/service/production/version/{serviceName}/diff [get]
func DiffProductionServiceVersions(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.View {
			ctx.UnAuthorized = true
			return
		}
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

	ctx.Resp, ctx.Err = service.DiffServiceVersions(ctx, projectKey, serviceName, revisionA, revisionB, true, ctx.Logger)
}

// @Summary Rollback Service Version
// @Description Rollback Service Version
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	revision	 	query		int								true	"revision"
// @Success 200
// @Router /api/aslan/service/version/{serviceName}/rollback [post]
func RollbackServiceVersion(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
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

	if err := commonutil.CheckZadigXLicenseStatus(); err != nil {
		ctx.Err = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneService, "回滚", "服务", fmt.Sprintf("服务: %s, 版本: %d", serviceName, revision), "", ctx.Logger)

	ctx.Err = service.RollbackServiceVersion(ctx, projectKey, serviceName, revision, false, ctx.Logger)
}

// @Summary Rollback Production Service Version
// @Description Rollback Production SService Version
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	revision	 	query		int								true	"revision"
// @Success 200
// @Router /api/aslan/service/production/version/{serviceName}/rollback [post]
func RollbackProductionServiceVersion(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Edit {
			ctx.UnAuthorized = true
			return
		}
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

	if err := commonutil.CheckZadigXLicenseStatus(); err != nil {
		ctx.Err = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneService, "回滚", "生产服务", fmt.Sprintf("服务: %s, 版本: %d", serviceName, revision), "", ctx.Logger)

	ctx.Err = service.RollbackServiceVersion(ctx, projectKey, serviceName, revision, true, ctx.Logger)
}
