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
		configmaps.GET("/:name", ListConfigMaps)
		configmaps.POST("", RollBackConfigMap)
		configmaps.GET("/migrate", MigrateHistoryConfigMaps)
	}
	secrets := router.Group("secrets")
	{
		secrets.GET("/:name", ListSecrets)
	}
	ingresses := router.Group("ingresses")
	{
		ingresses.GET("/:name", ListIngresses)
	}
	pvcs := router.Group("pvcs")
	{
		pvcs.GET("/:name", ListPvcs)
	}
	commonEnvCfgs := router.Group("envcfgs")
	{
		commonEnvCfgs.GET("/:name/cfg/:objectName", ListCommonEnvCfgHistory)
		commonEnvCfgs.GET("", ListLatestEnvCfg)
		commonEnvCfgs.PUT("/:name/:type/:objectName/sync", SyncEnvResource)
		commonEnvCfgs.PUT("/:name", UpdateCommonEnvCfg)
		commonEnvCfgs.POST("/:name", CreateCommonEnvCfg)
		commonEnvCfgs.DELETE("/:name/cfg/:objectName", DeleteCommonEnvCfg)
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
		image.POST("/cronjob/:envName", UpdateCronJobContainerImage)
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

		kube.GET("/workloads", ListWorkloads)
		kube.GET("/nodes", ListNodes)
		kube.GET("/pods", ListPodsInfo)
		kube.GET("/pods/:podName", GetPodsDetailInfo)
		kube.DELETE("/:env/pods/:podName", DeletePod)
		kube.GET("/pods/:podName/events", ListPodEvents)

		kube.POST("/k8s/resources", GetResourceDeployStatus)
		kube.POST("/helm/releases", GetReleaseDeployStatus)
		kube.POST("/helm/releaseInstances", GetReleaseInstanceDeployStatus)

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

		environments.GET("/:name", GetEnvironment)
		environments.PUT("/:name/envRecycle", UpdateProductRecycleDay)
		environments.PUT("/:name/alias", UpdateProductAlias)
		environments.POST("/:name/affectedservices", AffectedServices)
		environments.POST("/:name/estimated-values", EstimatedValues)

		environments.PUT("/:name/helm/default-values", UpdateHelmProductDefaultValues)
		environments.POST("/:name/helm/default-values/preview", PreviewHelmProductDefaultValues)

		environments.PUT("/:name/k8s/globalVariables", UpdateK8sProductGlobalVariables)
		environments.POST("/:name/k8s/globalVariables/preview", PreviewGlobalVariables)

		environments.GET("/:name/globalVariableCandidates", GetGlobalVariableCandidates)
		environments.PUT("/:name/helm/charts", UpdateHelmProductCharts)
		environments.PUT("/:name/syncVariables", SyncHelmProductRenderset)
		environments.DELETE("/:name", DeleteProduct)
		environments.GET("/:name/groups", ListGroups)
		environments.GET("/:name/workloads", ListWorkloadsInEnv)

		environments.GET("/:name/helm/releases", ListReleases)
		environments.DELETE("/:name/helm/releases", DeleteHelmReleases)
		environments.GET("/:name/helm/values", GetChartValues)
		environments.GET("/:name/helm/charts", GetChartInfos)
		environments.GET("/:name/helm/images", GetImageInfos)

		environments.GET("/:name/services", ListSvcsInEnv)
		environments.PUT("/:name/services", DeleteProductServices)
		environments.GET("/:name/services/:serviceName", GetService)
		environments.PUT("/:name/services/:serviceName", UpdateService)
		environments.GET("/:name/services/:serviceName/yaml", FetchServiceYaml)
		environments.POST("/:name/services/:serviceName/preview", PreviewService)
		environments.POST("/:name/services/preview/batch", BatchPreviewServices)
		environments.POST("/:name/services/:serviceName/restart", RestartService)
		environments.POST("/:name/services/:serviceName/restartNew", RestartWorkload)
		environments.POST("/:name/services/:serviceName/scaleNew", ScaleNewService)

		environments.POST("/:name/estimated-renderchart", GetEstimatedRenderCharts)

		environments.GET("/:name/check/workloads/k8services", CheckWorkloadsK8sServices)
		environments.POST("/:name/share/enable", EnableBaseEnv)
		environments.DELETE("/:name/share/enable", DisableBaseEnv)
		environments.GET("/:name/check/sharenv/:op/ready", CheckShareEnvReady)
		environments.GET("/:name/share/portal/:serviceName", GetPortalService)
		environments.POST("/:name/share/portal/:serviceName", SetupPortalService)

		environments.POST("/:name/istioGrayscale/enable", EnableIstioGrayscale)
		environments.DELETE("/:name/istioGrayscale/enable", DisableIstioGrayscale)
		environments.GET("/:name/check/istioGrayscale/:op/ready", CheckIstioGrayscaleReady)
		environments.GET("/:name/istioGrayscale/config", GetIstioGrayscaleConfig)
		environments.POST("/:name/istioGrayscale/config", SetIstioGrayscaleConfig)
		environments.GET("/:name/istioGrayscale/portal/:serviceName", GetIstioGrayscalePortalService)
		environments.POST("/:name/istioGrayscale/portal/:serviceName", SetupIstioGrayscalePortalService)

		environments.GET("/:name/services/:serviceName/pmexec", ConnectSshPmExec)
		environments.POST("/:name/services/:serviceName/execmd", ExecVmServiceCommand)

		// environments.POST("/:name/services/:serviceName/devmode/patch", PatchWorkload)
		// environments.POST("/:name/services/:serviceName/devmode/recover", RecoverWorkload)

		environments.GET("/:name/configs", GetEnvConfigs)
		environments.PUT("/:name/configs", UpdateEnvConfigs)
		environments.POST("/:name/analysis", RunAnalysis)
		environments.GET("/:name/analysis/cron", GetEnvAnalysisCron)
		environments.PUT("/:name/analysis/cron", UpsertEnvAnalysisCron)
		environments.GET("/analysis/history", GetEnvAnalysisHistory)

		environments.POST("/:name/sleep", EnvSleep)
		environments.GET("/:name/sleep/cron", GetEnvSleepCron)
		environments.PUT("/:name/sleep/cron", UpsertEnvSleepCron)

		environments.GET("/:name/version/:serviceName", ListEnvServiceVersions)
		environments.GET("/:name/version/:serviceName/revision/:revision", GetEnvServiceVersionYaml)
		environments.GET("/:name/version/:serviceName/diff", DiffEnvServiceVersions)
		environments.POST("/:name/version/:serviceName/rollback", RollbackEnvServiceVersion)

		environments.GET("sae", ListSAEEnvs)
		environments.POST("sae", CreateSAEEnv)
		environments.GET("sae/:name", GetSAEEnv)
		environments.DELETE("sae/:name", DeleteSAEEnv)
		environments.GET("sae/app", ListSAEApps)
		environments.GET("sae/namespace", ListSAENamespaces)
		environments.POST("sae/:name/app", AddSAEServiceToEnv)
		environments.PUT("sae/:name/app", DeleteSAEServiceFromEnv)
		environments.POST("sae/:name/app/:appID/serviceBind", BindSAEAppToService)
		environments.GET("sae/:name/app/:appID/versions", ListSAEAppVersion)
		environments.POST("sae/:name/app/:appID/restart", RestartSAEApp)
		environments.POST("sae/:name/app/:appID/rescale", RescaleSAEApp)
		environments.POST("sae/:name/app/:appID/rollback", RollbackSAEApp)
		environments.GET("sae/:name/app/:appID/instance", ListSAEAppInstances)
		environments.GET("sae/:name/app/:appID/changeOrder", ListSAEChangeOrder)
		environments.GET("sae/:name/app/:appID/changeOrder/:orderID", GetSAEChangeOrder)
		environments.POST("sae/:name/app/:appID/changeOrder/:orderID/abort", AbortSAEChangeOrder)
		environments.POST("sae/:name/app/:appID/changeOrder/:orderID/rollback", RollbackSAEChangeOrder)
		environments.POST("sae/:name/app/:appID/pipeline/:pipelineID/confirmPipelineBatch", ConfirmSAEPipelineBatch)
		environments.GET("sae/:name/app/:appID/pipeline/:pipelineID", GetSAEPipeline)
		environments.POST("sae/:name/app/:appID/instance/:instanceID/restart", RestartSAEAppInstance)
		environments.GET("sae/:name/app/:appID/instance/:instanceID/log", GetSAEAppInstanceLog)
	}

	// ---------------------------------------------------------------------------------------
	// renderset apis
	// ---------------------------------------------------------------------------------------
	rendersets := router.Group("rendersets")
	{
		rendersets.POST("/renderchart", GetServiceRenderCharts)
		rendersets.GET("/default-values", GetProductDefaultValues)
		rendersets.GET("/globalVariables", GetGlobalVariables)
		rendersets.GET("/yamlContent", GetYamlContent)
		rendersets.GET("/variables", GetServiceVariables)
	}

	// ---------------------------------------------------------------------------------------
	// 环境版本接口
	// ---------------------------------------------------------------------------------------
	revision := router.Group("revision")
	{
		revision.GET("/products", ListProductsRevision)
		revision.GET("/productsnaps", ListProductsRevisionSnaps)
	}

	initialize := router.Group("init")
	{
		initialize.POST("/type/:envType", InitializeEnv)
	}
}

type OpenAPIRouter struct{}

func (*OpenAPIRouter) Inject(router *gin.RouterGroup) {
	image := router.Group("image")
	{
		image.POST("/deployment/:envName", OpenAPIUpdateDeploymentContainerImage)
		image.POST("/statefulset/:envName", OpenAPIUpdateStatefulSetContainerImage)
		image.POST("/cronjob/:envName", OpenAPIUpdateCronJobContainerImage)
	}

	common := router.Group("")
	{
		common.GET("", OpenAPIListEnvs)

		common.POST("", OpenAPICreateK8sEnv)
		common.DELETE("/:name", OpenAPIDeleteEnv)
		common.GET("/:name", OpenAPIGetEnvDetail)
		common.PUT("/:name", OpenAPIUpdateEnvBasicInfo)

		common.POST("/scale", OpenAPIScaleWorkloads)
		common.GET("/service/yaml", OpenAPIGetEnvServiceYaml)
		common.GET("/service/values", OpenAPIGetEnvServiceValues)
		common.POST("/service/yaml", OpenAPIApplyYamlService)
		common.DELETE("/service/yaml", OpenAPIDeleteYamlServiceFromEnv)

		common.PUT("/envcfgs", OpenAPIUpdateCommonEnvCfg)
		common.POST("/:name/envcfgs", OpenAPICreateCommonEnvCfg)
		common.GET("/:name/envcfgs", OpenAPIListCommonEnvCfg)
		common.GET("/:name/envcfg/:cfgName", OpenAPIGetCommonEnvCfg)
		common.DELETE("/:name/envcfg/:cfgName", OpenAPIDeleteCommonEnvCfg)

		common.PUT("/:name/services", OpenAPIUpdateYamlServices)
		common.GET("/:name/variable", OpenAPIGetEnvGlobalVariables)
		common.PUT("/:name/variable", OpenAPIUpdateGlobalVariables)

		common.GET("/:name/services/:serviceName", OpenAPIGetService)
		common.POST("/:name/service/:serviceName/restart", OpenAPIRestartService)

		common.GET("/:name/check/workloads/k8services", OpenAPICheckWorkloadsK8sServices)
		common.POST("/:name/share/enable", OpenAPIEnableBaseEnv)
		common.DELETE("/:name/share/enable", OpenAPIDsiableBaseEnv)
		common.GET("/:name/check/sharenv/:op/ready", OpenAPICheckShareEnvReady)
		common.GET("/:name/share/portal/:serviceName", OpenAPIGetPortalService)
		common.POST("/:name/share/portal/:serviceName", OpenAPISetPortalService)
	}

	production := router.Group("production")
	{
		production.GET("", OpenAPIListProductionEnvs)

		production.DELETE("/:name", OpenAPIDeleteProductionEnv)
		production.POST("", OpenAPICreateProductionEnv)
		production.GET("/:name", OpenAPIGetProductionEnvDetail)
		production.PUT("/:name", OpenAPIUpdateProductionEnvBasicInfo)

		production.GET("/service/values", OpenAPIGetProductionEnvServiceValues)
		production.GET("/service/yaml", OpenAPIGetProductionEnvServiceYaml)
		production.POST("/service/yaml", OpenAPIApplyProductionYamlService)
		production.DELETE("/service/yaml", OpenAPIDeleteProductionYamlServiceFromEnv)

		production.PUT("/envcfgs", OpenAPIUpdateProductionCommonEnvCfg)
		production.POST("/:name/envcfgs", OpenAPICreateCommonEnvCfg)
		production.GET("/:name/envcfgs", OpenAPIListProductionCommonEnvCfg)
		production.GET("/:name/envcfg/:cfgName", OpenAPIGetProductionCommonEnvCfg)
		production.DELETE("/:name/envcfg/:cfgName", OpenAPIDeleteProductionEnvCommonEnvCfg)

		production.PUT("/:name/services", OpenAPIUpdateProductionYamlServices)
		production.GET("/:name/variable", OpenAPIGetProductionEnvGlobalVariables)
		production.PUT("/:name/variable", OpenAPIUpdateProductionGlobalVariables)

		production.GET("/:name/services/:serviceName", OpenAPIGetProductionService)
		production.POST("/:name/service/:serviceName/restart", OpenAPIProductionRestartService)
	}

	helm := router.Group("helm")
	{
		helm.POST("", OpenAPICreateHelmEnv)
		helm.POST("/:name/services", OpenAPIAddHelmServicesToEnv)
	}

	kube := router.Group("kube")
	{
		kube.GET("/events", OpenAPIListKubeEvents)
	}
}
