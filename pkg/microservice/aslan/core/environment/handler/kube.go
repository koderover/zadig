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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

// kubernetes resources apis will not have authorization for now

type ListServicePodsArgs struct {
	serviceName string `json:"service_name"`
	ProductName string `json:"product_name"`
	EnvName     string `json:"env_name"`
}

func ListKubeEvents(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Query("projectName")
	name := c.Query("name")
	rtype := c.Query("type")

	ctx.Resp, ctx.Err = service.ListKubeEvents(envName, productName, name, rtype, ctx.Logger)
}

func ListAvailableNamespaces(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	clusterID := c.Query("clusterId")
	listType := c.Query("type")
	ctx.Resp, ctx.Err = service.ListAvailableNamespaces(clusterID, listType, ctx.Logger)
}

func ListServicePods(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(ListServicePodsArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = service.ListServicePods(
		args.ProductName,
		args.EnvName,
		args.serviceName,
		ctx.Logger,
	)
}

func DeletePod(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	podName := c.Param("podName")
	envName := c.Query("envName")
	productName := c.Query("projectName")

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv,
		"重启", "环境-服务实例", fmt.Sprintf("环境名称:%s,pod名称:%s",
			c.Query("envName"), c.Param("podName")), "", ctx.Logger, envName)
	ctx.Err = service.DeletePod(envName, productName, podName, ctx.Logger)
}

func ListPodEvents(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Query("projectName")
	podName := c.Param("podName")

	ctx.Resp, ctx.Err = service.ListPodEvents(envName, productName, podName, ctx.Logger)
}

func ListNodes(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListAvailableNodes(c.Query("clusterId"), ctx.Logger)
}

func DownloadFileFromPod(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Query("projectName")
	podName := c.Param("podName")
	filePath := c.Query("path")
	container := c.Query("container")

	if len(container) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("container can't be nil")
		return
	}
	if len(filePath) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("file path can't be nil")
		return
	}

	fileBytes, path, err := service.DownloadFile(envName, productName, podName, container, filePath, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	fileName := filepath.Base(path)
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Data(http.StatusOK, "application/octet-stream", fileBytes)
}

func ListNamespace(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListNamespace(c.Param("clusterID"), ctx.Logger)
}

func ListDeploymentNames(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListDeploymentNames(c.Param("clusterID"), c.Param("namespace"), ctx.Logger)
}

func ListWorkloadsInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListWorkloadsInfo(c.Param("clusterID"), c.Param("namespace"), ctx.Logger)
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

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be empty")
		return
	}
	envName := c.Query("envName")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can't be empty")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !(ctx.Resources.ProjectAuthInfo[projectKey].Env.View ||
			ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = service.ListPodsInfo(projectKey, envName, ctx.Logger)
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

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be empty")
		return
	}
	envName := c.Query("envName")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can't be empty")
		return
	}
	podName := c.Param("podName")
	if podName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("podName can't be empty")
		return
	}
	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !(ctx.Resources.ProjectAuthInfo[projectKey].Env.View ||
			ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin) {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = service.GetPodDetailInfo(projectKey, envName, podName, ctx.Logger)
}

func ListCustomWorkload(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListCustomWorkload(c.Param("clusterID"), c.Param("namespace"), ctx.Logger)
}

func ListCanaryDeploymentServiceInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListCanaryDeploymentServiceInfo(c.Param("clusterID"), c.Param("namespace"), ctx.Logger)
}

func ListAllK8sResourcesInNamespace(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListAllK8sResourcesInNamespace(c.Param("clusterID"), c.Param("namespace"), ctx.Logger)
}

func ListK8sResOverview(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	queryParam := &service.FetchResourceArgs{}
	err := c.BindQuery(queryParam)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	queryParam.ResourceTypes = c.Param("workloadType")
	if len(queryParam.ResourceTypes) == 0 {
		queryParam.ResourceTypes = c.Param("resourceType")
	}
	ctx.Resp, ctx.Err = service.ListK8sResOverview(queryParam, ctx.Logger)
}

func GetK8sResourceYaml(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	queryParam := &service.FetchResourceArgs{}
	err := c.BindQuery(queryParam)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.GetK8sResourceYaml(queryParam, ctx.Logger)
}

func GetK8sWorkflowDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	queryParam := &service.FetchResourceArgs{}
	err := c.BindQuery(queryParam)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	workloadType := c.Param("workloadType")
	workloadName := c.Param("workloadName")

	ctx.Resp, ctx.Err = service.GetWorkloadDetail(queryParam, workloadType, workloadName, ctx.Logger)
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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	request := &service.K8sDeployStatusCheckRequest{}
	err := c.BindJSON(request)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.GetResourceDeployStatus(c.Query("projectName"), request, ctx.Logger)
}

func GetReleaseDeployStatus(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	request := &service.HelmDeployStatusCheckRequest{}
	err := c.BindJSON(request)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.GetReleaseDeployStatus(c.Query("projectName"), request)
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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	request := &service.HelmDeployStatusCheckRequest{}
	err := c.BindJSON(request)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.GetReleaseInstanceDeployStatus(c.Query("projectName"), request)
}
