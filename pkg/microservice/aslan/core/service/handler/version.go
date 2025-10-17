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
	"github.com/koderover/zadig/v2/pkg/types"
)

// @Summary List Service Versions
// @Description List Service Versions
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	production		query		bool							true	"is production"
// @Success 200 			{array}  	service.ListServiceVersionsResponse
// @Router /api/aslan/service/version/{serviceName} [get]
func ListServiceVersions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.View {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	ctx.Resp, ctx.RespErr = service.ListServiceVersions(ctx, projectKey, serviceName, production, ctx.Logger)
}

// @Summary Get Service Version Yaml
// @Description Get Service Versions Yaml
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	revision		path		string							true	"revision"
// @Param 	production		query		bool							true	"is production"
// @Success 200 			{object}  	service.GetServiceVersionYamlResponse
// @Router /api/aslan/service/version/{serviceName}/revision/{revision} [get]
func GetServiceVersionYaml(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.View {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	revision, err := strconv.ParseInt(c.Param("revision"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revison: %s", err))
		return
	}

	ctx.Resp, ctx.RespErr = service.GetServiceVersionYaml(ctx, projectKey, serviceName, revision, production, ctx.Logger)
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
// @Param 	production		query		bool							true	"is production"
// @Success 200 			{object}  	service.ListServiceVersionsResponse
// @Router /api/aslan/service/version/{serviceName}/diff [get]
func DiffServiceVersions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.View {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	revisionA, err := strconv.ParseInt(c.Query("revisionA"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revisionA: %s", err))
		return
	}
	revisionB, err := strconv.ParseInt(c.Query("revisionB"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revisionB: %s", err))
		return
	}

	ctx.Resp, ctx.RespErr = service.DiffServiceVersions(ctx, projectKey, serviceName, revisionA, revisionB, production, ctx.Logger)
}

// @Summary Rollback Service Version
// @Description Rollback Service Version
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	serviceName		path		string							true	"service name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	revision	 	query		int								true	"revision"
// @Param 	production		query		bool							true	"is production"
// @Success 200
// @Router /api/aslan/service/version/{serviceName}/rollback [post]
func RollbackServiceVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Edit {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.Edit {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
		ctx.RespErr = err
		return
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	revision, err := strconv.ParseInt(c.Query("revision"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revision: %s", err))
		return
	}

	if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
		ctx.RespErr = err
		return
	}

	function := "服务"
	if production {
		function = "生产服务"
	}

	detail := fmt.Sprintf("服务: %s, 版本: %d", serviceName, revision)
	detailEn := fmt.Sprintf("Service: %s, Version: %d", serviceName, revision)
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneService, "回滚", function, detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = service.RollbackServiceVersion(ctx, projectKey, serviceName, revision, production, ctx.Logger)
}
