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
	"fmt"

	"github.com/koderover/zadig/v2/pkg/types"

	"github.com/gin-gonic/gin"

	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary Get Service render charts
// @Description Get service render charts
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName	query		string										true	"project name"
// @Param 	envName		query		string										false	"env name"
// @Param 	body 		body 		commonservice.GetSvcRenderRequest 			true 	"body"
// @Success 200 		{array} 	commonservice.HelmSvcRenderArg
// @Router /api/aslan/environment/production/rendersets/renderchart [post]
func GetServiceRenderCharts(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Query("envName")
	production := c.Query("production") == "true"

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	if envName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("envName can not be null!")
		return
	}

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

	arg := &commonservice.GetSvcRenderRequest{}
	if err := c.ShouldBindJSON(arg); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, _, ctx.RespErr = commonservice.GetSvcRenderArgs(projectKey, envName, arg.GetSvcRendersArgs, ctx.Logger)
}

// @Summary Get service variables
// @Description Get service variables
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName	query		string										true	"project name"
// @Param 	envName		query		string										false	"env name"
// @Param 	serviceName	query		string										true	"service name"
// @Success 200 		{array} 	commonservice.K8sSvcRenderArg
// @Router /api/aslan/environment/rendersets/variables [get]
func GetServiceVariables(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Query("envName")
	production := c.Query("production") == "true"

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		// this api is also used for creating workflow task, so we add additional bypass for it
		// TODO: authorization leak
		if !ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Execute {
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
	}

	ctx.Resp, _, ctx.RespErr = commonservice.GetK8sSvcRenderArgs(projectKey, envName, c.Query("serviceName"), production, ctx.Logger)
}

func GetProductDefaultValues(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Query("envName")
	production := c.Query("production") == "true"

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	if envName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("envName can not be null!")
		return
	}

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

	ctx.Resp, ctx.RespErr = service.GetDefaultValues(projectKey, envName, production, ctx.Logger)
}

func GetYamlContent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	arg := &service.YamlContentRequestArg{}

	if err := c.ShouldBindQuery(arg); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if arg.CodehostID == 0 && len(arg.RepoLink) == 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("neither codehost nor repo link is specified")
		return
	}

	if len(arg.ValuesPath) == 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("values path can't be empty")
		return
	}

	ctx.Resp, ctx.RespErr = service.GetMergedYamlContent(arg)
}

type getGlobalVariablesRespone struct {
	GlobalVariables []*commontypes.GlobalVariableKV `json:"global_variables"`
	Revision        int64                           `json:"revision"`
}

// @Summary Get global variable
// @Description Get global variable from environment, current only used for k8s project
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName	query		string										true	"project name"
// @Param 	envName 	query		string										true	"env name"
// @Success 200 		{object} 	getGlobalVariablesRespone
// @Router /api/aslan/environment/rendersets/globalVariables [get]
func GetGlobalVariables(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	envName := c.Query("envName")
	production := c.Query("production") == "true"

	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	if envName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("envName can not be null!")
		return
	}

	// TODO: Authorization leak
	// authorization checks
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if projectedAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		if production {
			if projectedAuthInfo.IsProjectAdmin {
				permitted = true
			}

			if projectedAuthInfo.ProductionEnv.View {
				permitted = true
			}

			collaborationViewProductionEnvPermitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionView)
			if err == nil && collaborationViewProductionEnvPermitted {
				permitted = true
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if projectedAuthInfo.IsProjectAdmin {
				permitted = true
			}

			if projectedAuthInfo.Env.View {
				permitted = true
			}

			collaborationViewEnvPermitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
			if err == nil && collaborationViewEnvPermitted {
				permitted = true
			}
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	resp := new(getGlobalVariablesRespone)

	resp.GlobalVariables, resp.Revision, ctx.RespErr = service.GetGlobalVariables(projectKey, envName, production, ctx.Logger)
	ctx.Resp = resp
}
