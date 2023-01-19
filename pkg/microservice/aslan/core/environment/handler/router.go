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
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	// ---------------------------------------------------------------------------------------
	// Kube配置管理接口 ConfigMap
	// ---------------------------------------------------------------------------------------
	configmaps := router.Group("configmaps")
	{
		configmaps.GET("/:envName", ListConfigMaps)
		configmaps.POST("", RollBackConfigMap)
		configmaps.GET("/migrate", MigrateHistoryConfigMaps)
	}

	secrets := router.Group("secrets")
	{
		secrets.GET("/:envName", ListSecrets)
	}
	ingresses := router.Group("ingresses")
	{
		ingresses.GET("/:envName", ListIngresses)
	}
	pvcs := router.Group("pvcs")
	{
		pvcs.GET("/:envName", ListPvcs)
	}

	commonEnvCfgs := router.Group("envcfgs")
	{
		commonEnvCfgs.GET("/:envName/cfg/:objectName", ListCommonEnvCfgHistory)
		commonEnvCfgs.GET("", ListLatestEnvCfg)
		commonEnvCfgs.PUT("/:envName/:type/:objectName/sync", SyncEnvResource)
		commonEnvCfgs.PUT("/:envName", UpdateCommonEnvCfg)
		commonEnvCfgs.POST("/:envName", CreateCommonEnvCfg)
		commonEnvCfgs.DELETE("/:envName/cfg/:objectName", DeleteCommonEnvCfg)
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
	}

	// ---------------------------------------------------------------------------------------
	// 更新容器镜像
	// ---------------------------------------------------------------------------------------
	image := router.Group("image")
	{
		image.POST("/deployment/:envName", UpdateDeploymentContainerImage)
		image.POST("/statefulset/:envName", UpdateStatefulSetContainerImage)
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
		kube.DELETE("/:env/pods/:podName", DeletePod)
		kube.GET("/pods/:podName/events", ListPodEvents)
		kube.GET("/workloads", ListWorkloads)
		kube.GET("/nodes", ListNodes)

		kube.POST("/k8s/resources", GetResourceDeployStatus)
		kube.POST("/helm/releases", GetReleaseDeployStatus)

		kube.POST("/:env/pods/:podName/debugcontainer", PatchDebugContainer)

		kube.GET("/pods/:podName/file", DownloadFileFromPod)
		kube.GET("/namespace/cluster/:clusterID", ListNamespace)
		kube.GET("/deployment/cluster/:clusterID/namespace/:namespace", ListDeploymentNames)
		kube.GET("/workload/cluster/:clusterID/namespace/:namespace", ListWorkloadsInfo)
		kube.GET("/custom_workload/cluster/:clusterID/namespace/:namespace", ListCustomWorkload)
		kube.GET("/canary_service/cluster/:clusterID/namespace/:namespace", ListCanaryDeploymentServiceInfo)
		kube.GET("/resources/cluster/:clusterID/namespace/:namespace", ListAllK8sResourcesInNamespace)

		kube.GET("/workloads/:workloadType", ListK8sResOverview)
		kube.GET("/workloads/:workloadType/:workloadName", GetK8sWorkflowDetail)
		kube.GET("/resources/:resourceType", ListK8sResOverview)
		kube.GET("/yaml", GetK8sResourceYaml)
	}

	operations := router.Group("operations")
	{
		operations.GET("", GetOperationLogs)
	}

	// ---------------------------------------------------------------------------------------
	// 产品管理接口(环境)
	// ---------------------------------------------------------------------------------------
	environments := router.Group("environments")
	{
		environments.GET("", ListProducts)
		environments.PUT("/:name", UpdateProduct)
		environments.PUT("/:name/registry", UpdateProductRegistry)
		environments.PUT("", UpdateMultiProducts)
		environments.POST("", CreateProduct)

		environments.GET("/:name", GetProduct)
		environments.PUT("/:name/envRecycle", UpdateProductRecycleDay)
		environments.PUT("/:name/alias", UpdateProductAlias)
		environments.POST("/:name/affectedservices", AffectedServices)
		environments.POST("/:name/estimated-values", EstimatedValues)
		environments.PUT("/:name/renderset", UpdateHelmProductRenderset)
		environments.PUT("/:name/helm/default-values", UpdateHelmProductDefaultValues)
		environments.PUT("/:name/k8s/default-values", UpdateK8sProductDefaultValues)
		environments.PUT("/:name/helm/charts", UpdateHelmProductCharts)
		environments.PUT("/:name/syncVariables", SyncHelmProductRenderset)
		environments.GET("/:name/helmChartVersions", GetHelmChartVersions)
		environments.GET("/:name/productInfo", GetProductInfo)
		environments.DELETE("/:name", DeleteProduct)
		environments.GET("/:name/groups", ListGroups)
		environments.GET("/:name/workloads", ListWorkloadsInEnv)

		environments.GET("/:name/helm/releases", ListReleases)
		environments.GET("/:name/helm/values", GetChartValues)
		environments.GET("/:name/helm/charts", GetChartInfos)
		environments.GET("/:name/helm/images", GetImageInfos)

		environments.PUT("/:name/services", DeleteProductServices)
		environments.GET("/:name/services/:serviceName", GetService)
		environments.PUT("/:name/services/:serviceName", UpdateService)
		environments.POST("/:name/services/:serviceName/restart", RestartService)
		environments.POST("/:name/services/:serviceName/restartNew", RestartWorkload)
		environments.POST("/:name/services/:serviceName/scaleNew", ScaleNewService)
		environments.GET("/:name/services/:serviceName/containers/:container", GetServiceContainer)

		environments.GET("/:name/estimated-renderchart", GetEstimatedRenderCharts)

		environments.GET("/:name/check/workloads/k8services", CheckWorkloadsK8sServices)
		environments.POST("/:name/share/enable", EnableBaseEnv)
		environments.DELETE("/:name/share/enable", DisableBaseEnv)
		environments.GET("/:name/check/sharenv/:op/ready", CheckShareEnvReady)

		environments.GET("/:name/services/:serviceName/pmexec", ConnectSshPmExec)

		environments.POST("/:name/services/:serviceName/devmode/patch", PatchWorkload)
		environments.POST("/:name/services/:serviceName/devmode/recover", RecoverWorkload)

	}

	// ---------------------------------------------------------------------------------------
	// renderset相关接口
	// ---------------------------------------------------------------------------------------
	rendersets := router.Group("rendersets")
	{
		rendersets.GET("/renderchart", GetServiceRenderCharts)
		rendersets.GET("/default-values", GetProductDefaultValues)
		rendersets.GET("/yamlContent", GetYamlContent)
		rendersets.GET("/variables", GetServiceVariables)
	}

	// ---------------------------------------------------------------------------------------
	// 环境版本接口
	// ---------------------------------------------------------------------------------------
	revision := router.Group("revision")
	{
		revision.GET("/products", ListProductsRevision)
	}

	bundles := router.Group("bundle-resources")
	{
		bundles.GET("", GetBundleResources)
	}
}
