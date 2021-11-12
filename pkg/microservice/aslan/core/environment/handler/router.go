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

	gin2 "github.com/koderover/zadig/pkg/middleware/gin"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	// ---------------------------------------------------------------------------------------
	// Kube配置管理接口 ConfigMap
	// ---------------------------------------------------------------------------------------
	configmaps := router.Group("configmaps")
	{
		configmaps.GET("", ListConfigMaps)
		configmaps.PUT("", gin2.UpdateOperationLogStatus, UpdateConfigMap)
		configmaps.POST("", gin2.UpdateOperationLogStatus, RollBackConfigMap)
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
		export.GET("/service", ExportYaml)
		// export.GET("/pipelines/:name", ExportBuildYaml)
	}

	// ---------------------------------------------------------------------------------------
	// 更新容器镜像
	// ---------------------------------------------------------------------------------------
	image := router.Group("image")
	{
		image.POST("/deployment", gin2.UpdateOperationLogStatus, UpdateDeploymentContainerImage)
		image.POST("/statefulset", gin2.UpdateOperationLogStatus, UpdateStatefulSetContainerImage)
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
		kube.DELETE("/pods/:podName", gin2.UpdateOperationLogStatus, DeletePod)
		kube.GET("/pods/:podName/events", ListPodEvents)
		kube.GET("/workloads", ListWorkloads)
	}

	// ---------------------------------------------------------------------------------------
	// 产品管理接口(集成环境)
	// ---------------------------------------------------------------------------------------
	environments := router.Group("environments")
	{
		environments.GET("", ListProducts)
		environments.GET("/:productName/status", GetProductStatus)
		environments.POST("/:productName/auto", AutoCreateProduct)

		environments.PUT("/:productName/autoUpdate", gin2.UpdateOperationLogStatus, AutoUpdateProduct)
		environments.POST("", gin2.UpdateOperationLogStatus, CreateProduct)

		environments.POST("/:productName/helm", gin2.UpdateOperationLogStatus, CreateHelmProduct)
		environments.PUT("/:productName/multiHelmEnv", gin2.UpdateOperationLogStatus, UpdateMultiHelmEnv)

		environments.POST("/:productName", gin2.UpdateOperationLogStatus, UpdateProduct)
		environments.PUT("/:productName/envRecycle", gin2.UpdateOperationLogStatus, UpdateProductRecycleDay)

		environments.POST("/:productName/estimated-values", EstimatedValues)
		environments.PUT("/:productName/renderset", gin2.UpdateOperationLogStatus, UpdateHelmProductRenderset)

		environments.GET("/:productName/helmChartVersions", GetHelmChartVersions)
		//environments.PUT("/:productName", gin2.UpdateOperationLogStatus, EnvShare)

		environments.GET("/:productName", GetProduct)
		environments.GET("/:productName/productInfo", GetProductInfo)
		environments.GET("/:productName/ingressInfo", GetProductIngress)
		environments.GET("/:productName/helmRenderCharts", ListRenderCharts)
		environments.DELETE("/:productName", gin2.UpdateOperationLogStatus, DeleteProduct)

		environments.GET("/:productName/groups", ListGroups)
		environments.GET("/:productName/workloads", ListWorkloadsInEnv)

		environments.GET("/:productName/services/:serviceName", GetService)
		environments.PUT("/:productName/services/:serviceName/revision", gin2.UpdateOperationLogStatus, UpdateServiceRevision)
		environments.PUT("/:productName/services/:serviceName/:serviceType", gin2.UpdateOperationLogStatus, UpdateService)
		environments.POST("/:productName/services/:serviceName/restart", gin2.UpdateOperationLogStatus, RestartService)
		environments.POST("/:productName/services/:serviceName/restartNew", gin2.UpdateOperationLogStatus, RestartNewService)
		environments.POST("/:productName/services/:serviceName/scale/:number", gin2.UpdateOperationLogStatus, ScaleService)
		environments.POST("/:productName/services/:serviceName/scaleNew/:number", gin2.UpdateOperationLogStatus, ScaleNewService)
		environments.GET("/:productName/services/:serviceName/containers/:container/namespaces/:namespace", GetServiceContainer)

		environments.GET("/estimated-renderchart", GetEstimatedRenderCharts)
	}

	// ---------------------------------------------------------------------------------------
	// renderset相关接口
	// ---------------------------------------------------------------------------------------
	rendersets := router.Group("rendersets")
	{
		rendersets.GET("/renderchart", GetServiceRenderCharts)
		rendersets.GET("/default-values", GetProductDefaultValues)
		rendersets.GET("/yamlContent", GetYamlContent)
	}

	// ---------------------------------------------------------------------------------------
	// 环境版本接口
	// ---------------------------------------------------------------------------------------
	revision := router.Group("revision")
	{
		revision.GET("/products", ListProductsRevision)
	}
}
