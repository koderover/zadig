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
	"strconv"

	"github.com/koderover/zadig/v2/pkg/types"

	"github.com/gin-gonic/gin"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	internalresource "github.com/koderover/zadig/v2/pkg/shared/kube/resource"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary List services in env
// @Description List services in env
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	name 			path 		string 						 	true 	"env name"
// @Param 	projectName 	query 		string 						 	true 	"project name"
// @Success 200 			{object} 	commonservice.EnvServices
// @Router /api/aslan/environment/environments/{name}/services [get]
func ListSvcsInEnv(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectName")

	// TODO: Authorization leak
	// authorization checks
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if projectedAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		if projectedAuthInfo.IsProjectAdmin {
			permitted = true
		} else if projectedAuthInfo.Env.View ||
			projectedAuthInfo.Workflow.Execute {
			permitted = true
		} else if collaborationViewEnvPermitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView); err != nil && collaborationViewEnvPermitted {
			permitted = true
		} else {
			collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeWorkflow, types.WorkflowActionRun)
			if err == nil && collaborationAuthorizedEdit {
				permitted = true
			}
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = commonservice.ListServicesInEnv(envName, projectKey, nil, ctx.Logger)
}

func GetService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectName")
	serviceName := c.Param("serviceName")
	workLoadType := c.Query("workLoadType")
	production := c.Query("production") == "true"

	// authorization checks
	permitted := false
	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if projectedAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		if projectedAuthInfo.IsProjectAdmin {
			permitted = true
		}

		if production {
			if projectedAuthInfo.ProductionEnv.View ||
				projectedAuthInfo.ProductionEnv.ManagePods {
				permitted = true
			}

			readPermitted, _ := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionView)
			if readPermitted {
				permitted = true
			}

			editPermitted, _ := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionManagePod)
			if editPermitted {
				permitted = true
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if projectedAuthInfo.Env.View ||
				projectedAuthInfo.Env.ManagePods {
				permitted = true
			}

			readPermitted, _ := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
			if readPermitted {
				permitted = true
			}

			editPermitted, _ := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionManagePod)
			if editPermitted {
				permitted = true
			}
		}
	}
	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.GetService(envName, projectKey, serviceName, production, workLoadType, ctx.Logger)
}

func RestartService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectName")
	serviceName := c.Param("serviceName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.ManagePods {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionManagePod)
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
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.ManagePods {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionManagePod)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	args := &service.SvcOptArgs{
		EnvName:     envName,
		ProductName: projectKey,
		ServiceName: serviceName,
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv,
		"重启", "环境-服务", fmt.Sprintf("环境名称:%s,服务名称:%s", c.Param("name"), c.Param("serviceName")),
		"", types.RequestBodyTypeJSON, ctx.Logger, args.EnvName)
	ctx.RespErr = service.RestartService(args.EnvName, args, production, ctx.Logger)
}

type FetchServiceYamlResponse struct {
	Yaml string `json:"yaml"`
}

// @Summary Fetch Service Yaml
// @Description  Fetch Service Yaml
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Param 	name			path		string								true	"env name"
// @Param 	serviceName		path		string								true	"service name"
// @Success 200 			{object}    FetchServiceYamlResponse
// @Router /api/aslan/environment/environments/{name}/services/{serviceName}/yaml [get]
func FetchServiceYaml(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	serviceName := c.Param("serviceName")
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

	resp := new(FetchServiceYamlResponse)
	resp.Yaml, ctx.RespErr = service.FetchServiceYaml(projectKey, envName, serviceName, ctx.Logger)
	ctx.Resp = resp
}

// @Summary Preview service
// @Description Preview service
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Param 	name			path		string								true	"env name"
// @Param 	serviceName		path		string								true	"service name"
// @Param 	body 			body 		service.PreviewServiceArgs 			true 	"body"
// @Success 200 			{object} 	service.SvcDiffResult
// @Router /api/aslan/environment/environments/{name}/services/{serviceName}/preview [post]
func PreviewService(c *gin.Context) {
	// TODO: add authorization probably
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.PreviewServiceArgs)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	args.ProductName = c.Query("projectName")
	args.EnvName = c.Param("name")
	args.ServiceName = c.Param("serviceName")

	ctx.Resp, ctx.RespErr = service.PreviewService(args, ctx.Logger)
}

func BatchPreviewServices(c *gin.Context) {
	// TODO: add authorization probably
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := make([]*service.PreviewServiceArgs, 0)
	if err := c.BindJSON(&args); err != nil {
		ctx.Logger.Errorf("faield to bind args, err: %s", err)
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	production := c.Query("production") == "true"
	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	for _, arg := range args {
		arg.ProductName = c.Query("projectName")
		arg.EnvName = c.Param("name")
	}

	ctx.Resp, ctx.RespErr = service.BatchPreviewService(args, ctx.Logger)
}

// @Summary Update service
// @Description Update service
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Param 	name 			path		string								true	"env name"
// @Param 	serviceName	 	path		string								true	"service name"
// @Param 	body 			body 		service.SvcRevision 				true 	"body"
// @Success 200
// @Router /api/aslan/environment/environments/{name}/services/{serviceName} [put]
func UpdateService(c *gin.Context) {
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
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectKey, setting.OperationSceneEnv,
		"更新", "环境-单服务", fmt.Sprintf("环境名称:%s,服务名称:%s", envName, c.Param("serviceName")),
		"", types.RequestBodyTypeJSON, ctx.Logger, envName)

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

			if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
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

	svcRev := new(service.SvcRevision)
	if err := c.BindJSON(svcRev); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if c.Param("serviceName") != svcRev.ServiceName {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName not match")
		return
	}

	args := &service.SvcOptArgs{
		EnvName:           envName,
		ProductName:       projectKey,
		ServiceName:       c.Param("serviceName"),
		ServiceType:       svcRev.Type,
		ServiceRev:        svcRev,
		UpdateBy:          ctx.UserName,
		UpdateServiceTmpl: svcRev.UpdateServiceTmpl,
	}

	ctx.RespErr = service.UpdateService(args, ctx.Logger)
}

func RestartWorkload(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectName")
	serviceName := c.Param("serviceName")
	workloadType := c.Query("type")
	workloadName := c.Query("name")
	production := c.Query("production") == "true"

	args := &service.RestartScaleArgs{
		EnvName:     envName,
		ProductName: projectKey,
		ServiceName: serviceName,
		Type:        workloadType,
		Name:        workloadName,
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName,
		projectKey,
		setting.OperationSceneEnv,
		"重启",
		"环境-服务",
		fmt.Sprintf(
			"环境名称:%s,服务名称:%s,%s:%s", args.EnvName, args.ServiceName, args.Type, args.Name,
		),
		"", types.RequestBodyTypeJSON, ctx.Logger, args.EnvName,
	)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.ManagePods {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionManagePod)
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
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.ManagePods {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionManagePod)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.RespErr = service.RestartScale(args, production, ctx.Logger)
}

func ScaleNewService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.ScaleArgs)
	args.Type = setting.Deployment

	projectKey := c.Query("projectName")
	serviceName := c.Param("serviceName")
	envName := c.Param("name")
	resourceType := c.Query("type")
	name := c.Query("name")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.ManagePods {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionManagePod)
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
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.ManagePods {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionManagePod)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName,
		projectKey, setting.OperationSceneEnv,
		"伸缩",
		"环境-服务",
		fmt.Sprintf("环境名称:%s,%s:%s", envName, resourceType, name),
		"", types.RequestBodyTypeJSON, ctx.Logger, envName)

	number, err := strconv.Atoi(c.Query("number"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid number format")
		return
	}

	ctx.RespErr = service.Scale(&service.ScaleArgs{
		Type:        resourceType,
		ProductName: projectKey,
		EnvName:     envName,
		ServiceName: serviceName,
		Name:        name,
		Number:      number,
		Production:  production,
	}, ctx.Logger)
}

type OpenAPIGetServiceResponse struct {
	ServiceName string                       `json:"service_name"`
	Scales      []*internalresource.Workload `json:"scales"`
	Ingress     []*internalresource.Ingress  `json:"ingress"`
	Services    []*internalresource.Service  `json:"service_endpoints"`
	CronJobs    []*internalresource.CronJob  `json:"cron_jobs"`
	Namespace   string                       `json:"namespace"`
	EnvName     string                       `json:"env_name"`
	ProductName string                       `json:"product_name"`
	GroupName   string                       `json:"group_name"`
	Workloads   []*commonservice.Workload    `json:"-"`
}

func OpenAPIGetService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectKey")
	serviceName := c.Param("serviceName")
	workLoadType := c.Query("workLoadType")

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

	origResp, err := service.GetService(envName, projectKey, serviceName, false, workLoadType, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp = OpenAPIGetServiceResponse{
		ServiceName: origResp.ServiceName,
		Scales:      origResp.Scales,
		Ingress:     origResp.Ingress,
		Services:    origResp.Services,
		CronJobs:    origResp.CronJobs,
		Namespace:   origResp.Namespace,
		EnvName:     origResp.EnvName,
		ProductName: origResp.ProductName,
		GroupName:   origResp.GroupName,
		Workloads:   origResp.Workloads,
	}
	return
}

func OpenAPIGetProductionService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Param("name")
	projectKey := c.Query("projectKey")
	serviceName := c.Param("serviceName")
	workLoadType := c.Query("workLoadType")

	// TODO: Authorization leak
	// Authorization checks
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if projectedAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		if projectedAuthInfo.IsProjectAdmin {
			permitted = true
		}

		if projectedAuthInfo.ProductionEnv.View ||
			projectedAuthInfo.ProductionEnv.ManagePods {
			permitted = true
		}

		readPermitted, _ := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionView)
		if readPermitted {
			permitted = true
		}

		editPermitted, _ := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionManagePod)
		if editPermitted {
			permitted = true
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	origResp, err := service.GetService(envName, projectKey, serviceName, true, workLoadType, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp = OpenAPIGetServiceResponse{
		ServiceName: origResp.ServiceName,
		Scales:      origResp.Scales,
		Ingress:     origResp.Ingress,
		Services:    origResp.Services,
		CronJobs:    origResp.CronJobs,
		Namespace:   origResp.Namespace,
		EnvName:     origResp.EnvName,
		ProductName: origResp.ProductName,
		GroupName:   origResp.GroupName,
		Workloads:   origResp.Workloads,
	}
	return
}
