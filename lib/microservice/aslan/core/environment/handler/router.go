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
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/lib/microservice/aslan/middleware"
	"github.com/koderover/zadig/lib/types/permission"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	router.Use(middleware.Auth())

	// ---------------------------------------------------------------------------------------
	// Kube配置管理接口 ConfigMap
	// ---------------------------------------------------------------------------------------
	configmaps := router.Group("configmaps")
	{
		configmaps.GET("", ListConfigMaps)
		configmaps.PUT("", middleware.StoreProductName, middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, UpdateConfigMap)
		configmaps.POST("", middleware.StoreProductName, middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, RollBackConfigMap)
	}

	// ---------------------------------------------------------------------------------------
	// 定时任务管理接口
	// ---------------------------------------------------------------------------------------
	cron := router.Group("cron")
	{
		cron.GET("/cleanproduct", CleanProductCronJob)
	}

	// ---------------------------------------------------------------------------------------
	// 模板diff信息接口
	// ---------------------------------------------------------------------------------------
	productDiff := router.Group("diff")
	{
		// productDiff.GET("/products/:productName/services/:serviceName/configs/:configName", ConfigDiff)
		productDiff.GET("/products/:productName/service/:serviceName", ServiceDiff)
	}

	// ---------------------------------------------------------------------------------------
	// 导出管理接口
	// ---------------------------------------------------------------------------------------
	export := router.Group("export")
	{
		export.GET("/service", middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID}, permission.QueryType), ExportYaml)
		// export.GET("/pipelines/:name", ExportBuildYaml)
	}

	// ---------------------------------------------------------------------------------------
	// 更新容器镜像
	// ---------------------------------------------------------------------------------------
	image := router.Group("image")
	{
		// image.POST("", middleware.StoreProductName, middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, UpdateContainerImage)
		image.POST("/deployment", middleware.StoreProductName, middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID, permission.TestUpdateEnvUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, UpdateDeploymentContainerImage)
		image.POST("/statefulset", middleware.StoreProductName, middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID, permission.TestUpdateEnvUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, UpdateStatefulSetContainerImage)
	}

	// 查询环境创建时的服务和变量信息
	productInit := router.Group("init_info")
	{
		productInit.GET("/:name", GetInitProduct)
	}

	// Kubernetes 资源操作
	kube := router.Group("kube")
	{
		kube.GET("/available_namespaces", ListAvailableNamespaces)
		kube.GET("/events", ListKubeEvents)

		kube.POST("/pods", ListServicePods)
		kube.DELETE("/pods/:podName", middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID, permission.TestUpdateEnvUUID}, permission.QueryType), middleware.UpdateOperationLogStatus, DeletePod)
		kube.GET("/pods/:podName/events", ListPodEvents)
	}

	// ---------------------------------------------------------------------------------------
	// 产品管理接口(集成环境)
	// ---------------------------------------------------------------------------------------
	environments := router.Group("environments")
	{
		environments.GET("", ListProducts)
		environments.POST("/:productName/auto", middleware.IsHavePermission([]string{permission.TestEnvCreateUUID}, permission.ParamType), AutoCreateProduct)
		environments.PUT("/:productName/autoUpdate", middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID}, permission.ParamType), middleware.UpdateOperationLogStatus, AutoUpdateProduct)
		environments.POST("", middleware.StoreProductName, middleware.IsHavePermission([]string{permission.TestEnvCreateUUID, permission.ProdEnvCreateUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, CreateProduct)
		environments.POST("/:productName", middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID}, permission.ParamType), middleware.UpdateOperationLogStatus, UpdateProduct)
		environments.GET("/:productName", GetProduct)
		environments.GET("/:productName/productInfo", GetProductInfo)
		environments.GET("/:productName/ingressInfo", GetProductIngress)
		environments.GET("/:productName/helmRenderCharts", ListRenderCharts)
		environments.DELETE("/:productName", middleware.IsHavePermission([]string{permission.TestEnvDeleteUUID, permission.ProdEnvDeleteUUID}, permission.ParamType), middleware.UpdateOperationLogStatus, DeleteProduct)

		environments.GET("/:productName/groups", ListGroups)
		environments.GET("/:productName/groups/:source", ListGroupsBySource)

		environments.GET("/:productName/services/:serviceName", GetService)
		environments.PUT("/:productName/services/:serviceName/:serviceType", middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID}, permission.ParamType), middleware.UpdateOperationLogStatus, UpdateService)
		environments.POST("/:productName/services/:serviceName/restart", middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID}, permission.ParamType), middleware.UpdateOperationLogStatus, RestartService)
		environments.POST("/:productName/services/:serviceName/restartNew", middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID, permission.TestUpdateEnvUUID}, permission.ParamType), middleware.UpdateOperationLogStatus, RestartNewService)
		environments.POST("/:productName/services/:serviceName/scale/:number", middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID}, permission.ParamType), middleware.UpdateOperationLogStatus, ScaleService)
		environments.POST("/:productName/services/:serviceName/scaleNew/:number", middleware.IsHavePermission([]string{permission.TestEnvManageUUID, permission.ProdEnvManageUUID}, permission.ParamType), middleware.UpdateOperationLogStatus, ScaleNewService)
		environments.GET("/:productName/services/:serviceName/containers/:container/namespaces/:namespace", GetServiceContainer)
	}

	// ---------------------------------------------------------------------------------------
	// 环境版本接口
	// ---------------------------------------------------------------------------------------
	revision := router.Group("revision")
	{
		revision.GET("/products", ListProductsRevision)
	}

	// ---------------------------------------------------------------------------------------
	// Server Sent Events 接口
	// ---------------------------------------------------------------------------------------
	sse := router.Group("sse")
	{
		sse.GET("/products", ListProductsSSE)
	}
}
