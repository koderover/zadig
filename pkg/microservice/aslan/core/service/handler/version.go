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
	"github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
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

// @Summary Diff Service Versions
// @Description Diff Service Versions
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	versionA		query		int								true	"version a"
// @Param 	versionB		query		int								true	"version b"
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

	versionA, err := strconv.ParseInt(c.Query("versionA"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid versionA: %s", err))
		return
	}
	versionB, err := strconv.ParseInt(c.Query("versionB"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid versionA: %s", err))
		return
	}

	ctx.Resp, ctx.Err = service.DiffServiceVersions(ctx, projectKey, serviceName, versionA, versionB, false, ctx.Logger)
}

// @Summary Diff Production Service Versions
// @Description Diff Production Service Versions
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	versionA		query		int								true	"version a"
// @Param 	versionB		query		int								true	"version b"
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

	versionA, err := strconv.ParseInt(c.Query("versionA"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid versionA: %s", err))
		return
	}
	versionB, err := strconv.ParseInt(c.Query("versionB"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid versionA: %s", err))
		return
	}

	ctx.Resp, ctx.Err = service.DiffServiceVersions(ctx, projectKey, serviceName, versionA, versionB, true, ctx.Logger)
}

// @Summary Rollback Service Version
// @Description Rollback Service Version
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	version	 		query		int								true	"version"
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

	version, err := strconv.ParseInt(c.Query("version"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid versionA: %s", err))
		return
	}

	ctx.Err = service.RollbackServiceVersion(ctx, projectKey, serviceName, version, false, ctx.Logger)
}

// @Summary Rollback Production Service Version
// @Description Rollback Service Version
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	version	 		query		int								true	"version"
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

	version, err := strconv.ParseInt(c.Query("version"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid versionA: %s", err))
		return
	}

	ctx.Err = service.RollbackServiceVersion(ctx, projectKey, serviceName, version, true, ctx.Logger)
}
