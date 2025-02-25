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

	"github.com/koderover/zadig/v2/pkg/types"

	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
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
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Param("name")
	if envName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty name")
		return
	}
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionView)
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
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.View {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	isHelmChart, err := strconv.ParseBool(c.Query("isHelmChart"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid isHelmChart: %s", err))
		return
	}

	ctx.Resp, ctx.RespErr = commonservice.ListEnvServiceVersions(ctx, projectKey, envName, serviceName, isHelmChart, production, ctx.Logger)
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
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Param("name")
	if envName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty name")
		return
	}
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionView)
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
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.View {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
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
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revision: %s", err))
		return
	}

	isHelmChart, err := strconv.ParseBool(c.Query("isHelmChart"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid isHelmChart: %s", err))
		return
	}

	ctx.Resp, ctx.RespErr = commonservice.GetEnvServiceVersionYaml(ctx, projectKey, envName, serviceName, revision, isHelmChart, production, ctx.Logger)
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
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Param("name")
	if envName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty name")
		return
	}
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View || !ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Execute {
				envPermitted, envErr := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionView)
				workflowPermitted, workflowErr := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeWorkflow, envName, types.WorkflowActionRun)
				if envErr != nil || workflowErr != nil || !envPermitted || !workflowPermitted {
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
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.View || !ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Execute {
				envPermitted, envErr := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				workflowPermitted, workflowErr := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeWorkflow, envName, types.WorkflowActionRun)
				if envErr != nil || workflowErr != nil || !envPermitted || !workflowPermitted {
					ctx.UnAuthorized = true
					return
				}
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

	isHelmChart, err := strconv.ParseBool(c.Query("isHelmChart"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid isHelmChart: %s", err))
		return
	}

	ctx.Resp, ctx.RespErr = commonservice.DiffEnvServiceVersions(ctx, projectKey, envName, serviceName, revisionA, revisionB, isHelmChart, production, ctx.Logger)
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
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Param("name")
	if envName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty name")
		return
	}
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

	serviceName := c.Param("serviceName")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty serviceName")
		return
	}

	revision, err := strconv.ParseInt(c.Query("revision"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid revison: %s", err))
		return
	}

	isHelmChart, err := strconv.ParseBool(c.Query("isHelmChart"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("invalid isHelmChart: %s", err))
		return
	}

	if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
		ctx.RespErr = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "回滚", "环境-服务", fmt.Sprintf("环境: %s, 服务: %s, 版本: %d", envName, serviceName, revision), "", ctx.Logger, envName)

	_, ctx.RespErr = commonservice.RollbackEnvServiceVersion(ctx, projectKey, envName, serviceName, revision, isHelmChart, production, ctx.Logger)
}
