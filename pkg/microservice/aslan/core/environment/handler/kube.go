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
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

// kubernetes resources apis will not have authorization for now

func ListKubeEvents(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Query("projectName")
	name := c.Query("name")
	rtype := c.Query("type")

	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !(ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
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
			if !(ctx.Resources.ProjectAuthInfo[projectKey].Env.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}
	ctx.Resp, ctx.RespErr = service.ListKubeEvents(envName, productName, name, rtype, ctx.Logger)
}

func ListAvailableNamespaces(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	clusterID := c.Query("clusterId")
	listType := c.Query("type")
	ctx.Resp, ctx.RespErr = service.ListAvailableNamespaces(clusterID, listType, ctx.Logger)
}

func DeletePod(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	podName := c.Param("podName")
	envName := c.Query("envName")
	projectName := c.Query("projectName")

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv,
		"重启", "环境-服务实例", fmt.Sprintf("环境名称:%s,pod名称:%s",
			c.Query("envName"), c.Param("podName")), "", types.RequestBodyTypeJSON, ctx.Logger, envName)

	production := c.Query("production") == "true"
	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !(ctx.Resources.ProjectAuthInfo[projectName].ProductionEnv.ManagePods ||
				ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionManagePod)
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
			if !(ctx.Resources.ProjectAuthInfo[projectName].Env.ManagePods ||
				ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, types.EnvActionManagePod)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.RespErr = service.DeletePod(envName, projectName, podName, production, ctx.Logger)
}

func ListPodEvents(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Query("envName")
	projectKey := c.Query("projectName")
	podName := c.Param("podName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !(ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
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
			if !(ctx.Resources.ProjectAuthInfo[projectKey].Env.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.ListPodEvents(envName, projectKey, podName, production, ctx.Logger)
}

func ListNodes(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.ListAvailableNodes(c.Query("clusterId"), ctx.Logger)
}

func DownloadFileFromPod(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Query("envName")
	projectKey := c.Query("projectName")
	podName := c.Param("podName")
	filePath := c.Query("path")
	container := c.Query("container")
	production := c.Query("production") == "true"

	if len(container) == 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("container can't be nil")
		return
	}
	if len(filePath) == 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("file path can't be nil")
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
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.DebugPod {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionDebug)
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
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.DebugPod {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionDebug)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	fileBytes, path, err := service.DownloadFile(envName, projectKey, podName, container, filePath, production, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	fileName := filepath.Base(path)
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Data(http.StatusOK, "application/octet-stream", fileBytes)
}

func ListNamespace(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.ListNamespace(c.Param("clusterID"), ctx.Logger)
}

func ListDeploymentNames(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.ListDeploymentNames(c.Param("clusterID"), c.Param("namespace"), ctx.Logger)
}

func ListWorkloadsInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.ListWorkloadsInfo(c.Param("clusterID"), c.Param("namespace"), ctx.Logger)
}

// @Summary Get Pods Info
// @Description Get Pods Info
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName 	query		string										true	"projectName"
// @Param 	envName 		query		string										true	"envName"
// @Success 200 			{array} 	service.ListPodsInfoRespone
// @Router /api/aslan/environment/kube/pods [get]
func ListPodsInfo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be empty")
		return
	}
	envName := c.Query("envName")
	if envName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("envName can't be empty")
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
			if !(ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
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
			if !(ctx.Resources.ProjectAuthInfo[projectKey].Env.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.ListPodsInfo(projectKey, envName, production, ctx.Logger)
}

// @Summary Get Pods Detail Info
// @Description Get Pods Detail Info
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName 	query		string										true	"projectName"
// @Param 	envName 		query		string										true	"envName"
// @Param 	podName 		path		string										true	"podName"
// @Success 200 			{object} 	resource.Pod
// @Router /api/aslan/environment/kube/pods/{podName} [get]
func GetPodsDetailInfo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be empty")
		return
	}
	envName := c.Query("envName")
	if envName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("envName can't be empty")
		return
	}
	podName := c.Param("podName")
	if podName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("podName can't be empty")
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
			if !(ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
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
			if !(ctx.Resources.ProjectAuthInfo[projectKey].Env.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.GetPodDetailInfo(projectKey, envName, podName, production, ctx.Logger)
}

func ListCustomWorkload(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.ListCustomWorkload(c.Param("clusterID"), c.Param("namespace"), ctx.Logger)
}

func ListCanaryDeploymentServiceInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.ListCanaryDeploymentServiceInfo(c.Param("clusterID"), c.Param("namespace"), ctx.Logger)
}

func ListAllK8sResourcesInNamespace(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.ListAllK8sResourcesInNamespace(c.Param("clusterID"), c.Param("namespace"), ctx.Logger)
}

func ListK8sResOverview(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	queryParam := &service.FetchResourceArgs{}
	err = c.BindQuery(queryParam)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	queryParam.Production = production
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !(ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, queryParam.EnvName, types.ProductionEnvActionView)
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
			if !(ctx.Resources.ProjectAuthInfo[projectKey].Env.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, queryParam.EnvName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	queryParam.ResourceTypes = c.Param("workloadType")
	if len(queryParam.ResourceTypes) == 0 {
		queryParam.ResourceTypes = c.Param("resourceType")
	}
	ctx.Resp, ctx.RespErr = service.ListK8sResOverview(queryParam, ctx.Logger)
}

func GetK8sResourceYaml(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	queryParam := &service.FetchResourceArgs{}
	err = c.BindQuery(queryParam)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	queryParam.Production = production
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !(ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, queryParam.EnvName, types.ProductionEnvActionView)
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
			if !(ctx.Resources.ProjectAuthInfo[projectKey].Env.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, queryParam.EnvName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.GetK8sResourceYaml(queryParam, ctx.Logger)
}

func GetK8sWorkflowDetail(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	defer func() { internalhandler.JSONResponse(c, ctx) }()

	queryParam := &service.FetchResourceArgs{}
	err = c.BindQuery(queryParam)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	queryParam.Production = production
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !(ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, queryParam.EnvName, types.ProductionEnvActionView)
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
			if !(ctx.Resources.ProjectAuthInfo[projectKey].Env.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, queryParam.EnvName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	workloadType := c.Param("workloadType")
	workloadName := c.Param("workloadName")

	ctx.Resp, ctx.RespErr = service.GetWorkloadDetail(queryParam, workloadType, workloadName, ctx.Logger)
}

// @Summary Get Resource Deploy Status
// @Description Get Resource Deploy Status
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string									true	"project name"
// @Param 	body 			body 		service.K8sDeployStatusCheckRequest 	true 	"body"
// @Success 200 			{array}  	service.ServiceDeployStatus
// @Router /api/aslan/environment/kube/k8s/resources [post]
func GetResourceDeployStatus(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	request := &service.K8sDeployStatusCheckRequest{}
	err = c.BindJSON(request)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	envName := request.EnvName

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !(ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
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
			if !(ctx.Resources.ProjectAuthInfo[projectKey].Env.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.GetResourceDeployStatus(projectKey, request, production, ctx.Logger)
}

func GetReleaseDeployStatus(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	request := &service.HelmDeployStatusCheckRequest{}
	err = c.BindJSON(request)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	envName := request.EnvName

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !(ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
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
			if !(ctx.Resources.ProjectAuthInfo[projectKey].Env.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.GetReleaseDeployStatus(projectKey, production, request)
}

// @Summary Get Release Instance Deploy Status
// @Description Get Release Instance Deploy Status
// @Tags 	environment
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string										true	"project name"
// @Param 	body 			body 		service.HelmDeployStatusCheckRequest 		true 	"body"
// @Success 200 			{array}  	service.GetReleaseInstanceDeployStatusResponse
// @Router /api/aslan/environment/kube/helm/releaseInstances [post]
func GetReleaseInstanceDeployStatus(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	request := &service.HelmDeployStatusCheckRequest{}
	err = c.BindJSON(request)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	envName := request.EnvName

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !(ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
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
			if !(ctx.Resources.ProjectAuthInfo[projectKey].Env.View ||
				ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.GetReleaseInstanceDeployStatus(projectKey, production, request)
}

type OpenAPIListKubeEventResponse struct {
	Reason string `json:"reason,omitempty"`
	// A human-readable description of the status of this operation.
	Message string `json:"message,omitempty" protobuf:"bytes,4,opt,name=message"`
	// The time at which the event was first recorded. (Time of server receipt is in TypeMeta.)
	FirstSeen int64 `json:"first_seen,omitempty"`
	// The time at which the most recent occurrence of this event was recorded.
	LastSeen int64 `json:"last_seen,omitempty"`
	// The number of times this event has occurred.
	Count int32 `json:"count,omitempty"`
	// Type of this event (Normal, Warning), new types could be added in the future
	Type string `json:"type,omitempty"`
}

func OpenAPIListKubeEvents(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Query("projectKey")
	name := c.Query("name")
	rtype := c.Query("type")

	origResp, err := service.ListKubeEvents(envName, productName, name, rtype, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	resp := make([]*OpenAPIListKubeEventResponse, 0)
	for _, or := range origResp {
		resp = append(resp, &OpenAPIListKubeEventResponse{
			Reason:    or.Reason,
			Message:   or.Message,
			FirstSeen: or.FirstSeen,
			LastSeen:  or.LastSeen,
			Count:     or.Count,
			Type:      or.Type,
		})
	}
	ctx.Resp = resp
	return
}
