/*
Copyright 2021 The KodeRover Authors.

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
	"github.com/koderover/zadig/v2/pkg/types"

	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func ListReleases(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")

	args := &service.HelmReleaseQueryArgs{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = err
		return
	}

	// TODO: Authorization leak
	// authorization checks
	production := c.Query("production") == "true"
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[args.ProjectName].ProductionEnv.View {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProjectName, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}

			if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[args.ProjectName].Env.View &&
				!ctx.Resources.ProjectAuthInfo[args.ProjectName].Version.Create {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProjectName, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.ListReleases(args, envName, production, ctx.Logger)
}

func ListHelmReleaseDiffSummaries(c *gin.Context) {
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
	if projectKey == "" {
		ctx.RespErr = fmt.Errorf("projectName can't be empty")
		return
	}

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

			if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.View &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Version.Create {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.ListHelmReleaseDiffSummaries(projectKey, envName, production, ctx.Logger)
}

func GetHelmReleaseDiff(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	serviceOrReleaseName := c.Param("serviceOrReleaseName")
	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	isHelmChartDeployParam := c.Query("isHelmChartDeploy")
	if projectKey == "" {
		ctx.RespErr = fmt.Errorf("projectName can't be empty")
		return
	}
	if serviceOrReleaseName == "" {
		ctx.RespErr = fmt.Errorf("serviceOrReleaseName can't be empty")
		return
	}
	if isHelmChartDeployParam != "true" && isHelmChartDeployParam != "false" {
		ctx.RespErr = fmt.Errorf("isHelmChartDeploy must be true or false")
		return
	}
	isHelmChartDeploy := isHelmChartDeployParam == "true"

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

			if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.View &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Version.Create {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.GetHelmReleaseDiff(projectKey, envName, serviceOrReleaseName, production, isHelmChartDeploy, ctx.Logger)
}

func UpdateHelmValuesSource(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	serviceOrReleaseName := c.Param("serviceOrReleaseName")
	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	isHelmChartDeployParam := c.Query("isHelmChartDeploy")
	if projectKey == "" {
		ctx.RespErr = fmt.Errorf("projectName can't be empty")
		return
	}
	if serviceOrReleaseName == "" {
		ctx.RespErr = fmt.Errorf("serviceOrReleaseName can't be empty")
		return
	}
	if isHelmChartDeployParam != "true" && isHelmChartDeployParam != "false" {
		ctx.RespErr = fmt.Errorf("isHelmChartDeploy must be true or false")
		return
	}
	isHelmChartDeploy := isHelmChartDeployParam == "true"

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
			if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
				ctx.RespErr = err
				return
			}
		} else if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	args := new(service.UpdateHelmValuesSourceArgs)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	// The source snapshot is updated only by a successful import or sync, not by saving configuration.
	if args.ValuesData != nil {
		args.ValuesData.AutoSyncYaml = ""
	}
	data, _ := json.Marshal(args)
	detail := fmt.Sprintf("%s:%s", envName, serviceOrReleaseName)
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv, "更新", "Helm Values 来源配置", detail, detail, string(data), types.RequestBodyTypeJSON, ctx.Logger, envName)

	ctx.RespErr = service.UpdateHelmValuesSource(projectKey, envName, serviceOrReleaseName, ctx.UserName, production, isHelmChartDeploy, args)
}

// @Summary 获取Helm服务Chart Values
// @Description 获取Helm服务Chart Values
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName			query		string								true	"project name"
// @Param 	name				path		string								true	"env name"
// @Param 	serviceName			path		string								true	"service name"
// @Param 	isHelmChartDeploy	query		string								true	"isHelmChartDeploy"
// @Param 	releaseName			query		string								true	"release name"
// @Param 	production			query		string								true	"production"
// @Success 200 				{object}    commonservice.ValuesResp
// @Router /api/aslan/environment/environments/{name}/helm/values [get]
func GetChartValues(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectName")
	serviceName := c.Query("serviceName")
	isHelmChartDeploy := c.Query("isHelmChartDeploy")
	releaseName := c.Query("releaseName")
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

			if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
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

	if isHelmChartDeploy == "false" {
		ctx.Resp, ctx.RespErr = commonservice.GetChartValues(projectKey, envName, serviceName, false, production, true)
	} else {
		ctx.Resp, ctx.RespErr = commonservice.GetChartValues(projectKey, envName, releaseName, true, production, true)
	}
}

func GetChartInfos(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	servicesName := c.Query("serviceName")
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
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}

			if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
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

	ctx.Resp, ctx.RespErr = service.GetChartInfos(projectKey, envName, servicesName, production, ctx.Logger)
}

func GetImageInfos(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectName")
	servicesName := c.Query("serviceName")
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

			if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
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

	ctx.Resp, ctx.RespErr = service.GetImageInfos(projectKey, envName, servicesName, production, ctx.Logger)
}
