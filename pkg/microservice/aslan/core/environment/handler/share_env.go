/*
Copyright 2022 The KodeRover Authors.

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

	"github.com/gin-gonic/gin"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func CheckWorkloadsK8sServices(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
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
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.EditConfig {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionEditConfig)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.CheckWorkloadsK8sServices(c, envName, projectKey, production)
}

func EnableBaseEnv(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	if production {
		ctx.RespErr = fmt.Errorf("production environment not support")
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "开启自测模式", "环境", envName, "", types.RequestBodyTypeJSON, ctx.Logger, envName)

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

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.EnableBaseEnv(c, envName, projectKey)
}

func DisableBaseEnv(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	if production {
		ctx.RespErr = fmt.Errorf("production environment not support")
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv,
		"关闭自测模式", "环境", envName,
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

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.DisableBaseEnv(c, envName, projectKey)
}

func CheckShareEnvReady(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	if production {
		ctx.RespErr = fmt.Errorf("production environment not support")
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

	ctx.Resp, ctx.RespErr = service.CheckShareEnvReady(c, envName, c.Param("op"), projectKey)
}

// @Summary Get Portal Service for Share Env
// @Description Get Portal Service for Share Env
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string									true	"project name"
// @Param 	name			path		string									true	"env name"
// @Param 	serviceName		path		string									true	"service name"
// @Success 200 			{object} 	service.GetPortalServiceResponse
// @Router /api/aslan/environment/environments/{name}/share/portal/{serviceName} [get]
func GetPortalService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	serviceName := c.Param("serviceName")
	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	if production {
		ctx.RespErr = fmt.Errorf("production environment not support")
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.View {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.GetPortalService(c, projectKey, envName, serviceName)
	return
}

// @Summary Setup Portal Service for Share Env
// @Description Setup Portal Service for Share Env
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string									true	"project name"
// @Param 	name			path		string									true	"env name"
// @Param 	serviceName		path		string									true	"service name"
// @Param 	body 			body 		[]service.SetupPortalServiceRequest 	true 	"body"
// @Success 200
// @Router /api/aslan/environment/environments/{name}/share/portal/{serviceName} [post]
func SetupPortalService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	serviceName := c.Param("serviceName")
	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	if production {
		ctx.RespErr = fmt.Errorf("production environment not support")
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

	req := []service.SetupPortalServiceRequest{}
	err = c.ShouldBindJSON(&req)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.SetupPortalService(c, projectKey, envName, serviceName, req)
	return
}
