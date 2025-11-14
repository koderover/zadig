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
	proxy := router.Group("proxy")
	{
		proxy.GET("/config", GetProxyConfig)
	}

	// ---------------------------------------------------------------------------------------
	// 安装脚本管理接口
	// ---------------------------------------------------------------------------------------
	install := router.Group("install")
	{
		install.POST("", CreateInstall)
		install.PUT("", UpdateInstall)
		install.GET("/:name/:version", GetInstall)
		install.GET("", ListInstalls)
		install.PUT("/delete", DeleteInstall)
	}

	// ---------------------------------------------------------------------------------------
	// 代理管理接口
	// ---------------------------------------------------------------------------------------
	proxyManage := router.Group("proxyManage")
	{
		proxyManage.GET("", ListProxies)
		proxyManage.GET("/:id", GetProxy)
		proxyManage.POST("", CreateProxy)
		proxyManage.PUT("/:id", UpdateProxy)
		proxyManage.DELETE("/:id", DeleteProxy)

		proxyManage.POST("/connectionTest", TestConnection)
	}

	registry := router.Group("registry")
	{
		registry.GET("/project", ListRegistries)
		// 获取默认的镜像仓库配置，用于kodespace CLI调用
		registry.GET("/namespaces/default", GetDefaultRegistryNamespace)
		registry.GET("/namespaces/specific/:id", GetRegistryNamespace)
		registry.GET("/namespaces", ListRegistryNamespaces)
		registry.POST("/namespaces", CreateRegistryNamespace)
		registry.POST("/validate", ValidateRegistryNamespace)
		registry.PUT("/namespaces/:id", UpdateRegistryNamespace)

		registry.DELETE("/namespaces/:id", DeleteRegistryNamespace)
		registry.GET("/release/repos", ListAllRepos)
		registry.POST("/images", ListImages)
		registry.GET("/images/repos/:name", ListRepoImages)
	}

	s3storage := router.Group("s3storage")
	{
		s3storage.GET("", ListS3Storage)
		s3storage.POST("", CreateS3Storage)
		s3storage.POST("/validate", ValidateS3Storage)
		s3storage.GET("/:id", GetS3Storage)
		s3storage.PUT("/:id", UpdateS3Storage)
		s3storage.DELETE("/:id", DeleteS3Storage)
		s3storage.POST("/:id/releases/search", ListTars)
		s3storage.GET("/project", ListS3StorageByProject)
	}

	//系统清理缓存
	cleanCache := router.Group("cleanCache")
	{
		cleanCache.POST("/oneClick", CleanImageCache)
		cleanCache.GET("/state", CleanCacheState)
		cleanCache.POST("/cron", SetCron)
		cleanCache.POST("/sharedStorage", CleanSharedStorage)
	}

	// security and privacy settings
	security := router.Group("security")
	{
		security.POST("", CreateOrUpdateSecuritySettings)
		security.GET("", GetSecuritySettings)
	}

	// ---------------------------------------------------------------------------------------
	// jenkins集成接口以及jobs和buildWithParameters接口
	// ---------------------------------------------------------------------------------------
	jenkins := router.Group("jenkins")
	{
		jenkins.GET("/exist", CheckJenkinsIntegration)
		jenkins.POST("/user/connection", TestJenkinsConnection)
		jenkins.GET("/jobNames/:id", ListJobNames)
		jenkins.GET("/buildArgs/:id/:jobName", ListJobBuildArgs)
	}

	cicd := router.Group("cicdTools")
	{
		cicd.POST("", CreateCICDTools)
		cicd.GET("", ListCICDTools)
		cicd.PUT("/:id", UpdateCICDTools)
		cicd.DELETE("/:id", DeleteCICDTools)
	}

	bk := router.Group("blueking")
	{
		bk.GET("/business", ListBluekingBusiness)
		bk.GET("/executionPlan", ListBlueKingExecutionPlan)
		bk.GET("/executionPlan/:id", GetBlueKingExecutionPlanDetail)
		bk.GET("/topology", GetBlueKingBusinessTopology)
		bk.GET("/hosts/topologyNode", ListServerByBlueKingTopologyNode)
		bk.GET("/hosts/business", ListServerByBlueKingBusiness)
	}

	//系统配额
	capacity := router.Group("capacity")
	{
		capacity.POST("", UpdateStrategy)
		capacity.GET("/target/:target", GetStrategy)
		capacity.POST("/gc", GarbageCollection)
		// 清理已被删除的工作流的所有缓存，暂时用于手动调用
		capacity.POST("/clean", CleanCache)
	}

	// workflow concurrency settings
	concurrency := router.Group("concurrency")
	{
		concurrency.GET("/workflow", GetWorkflowConcurrency)
		concurrency.POST("/workflow", UpdateWorkflowConcurrency)
	}

	// default login default login home page settings
	login := router.Group("login")
	{
		login.GET("/default", GetDefaultLogin)
		login.POST("/default", UpdateDefaultLogin)
	}

	// ---------------------------------------------------------------------------------------
	// 自定义镜像管理接口
	// ---------------------------------------------------------------------------------------
	basicImages := router.Group("basicImages")
	{
		basicImages.GET("", ListBasicImages)
		basicImages.GET("/:id", GetBasicImage)
		basicImages.POST("", CreateBasicImage)
		basicImages.PUT("/:id", UpdateBasicImage)
		basicImages.DELETE("/:id", DeleteBasicImage)
	}

	// ---------------------------------------------------------------------------------------
	// helm chart 集成
	// ---------------------------------------------------------------------------------------
	integration := router.Group("helm")
	{
		integration.GET("", ListHelmRepos)
		integration.GET("project", ListHelmReposByProject)
		integration.GET("/public", ListHelmReposPublic)
		integration.POST("", CreateHelmRepo)
		integration.POST("/validate", ValidateHelmRepo)
		integration.PUT("/:id", UpdateHelmRepo)
		integration.DELETE("/:id", DeleteHelmRepo)
		integration.GET("/:name/index", ListCharts)
	}

	// ---------------------------------------------------------------------------------------
	// ssh私钥管理接口
	// ---------------------------------------------------------------------------------------
	privateKey := router.Group("privateKey")
	{
		privateKey.GET("", ListPrivateKeys)
		privateKey.GET("/internal", ListPrivateKeysInternal)
		privateKey.GET("/:id", GetPrivateKey)
		privateKey.GET("/labels", ListLabels)
		privateKey.POST("", CreatePrivateKey)
		privateKey.POST("/batch", BatchCreatePrivateKey)
		privateKey.PUT("/:id", UpdatePrivateKey)
		privateKey.POST("/validate", ValidatePrivateKey)
		privateKey.DELETE("/:id", DeletePrivateKey)
	}

	rsaKey := router.Group("rsaKey")
	{
		rsaKey.GET("publicKey", GetRSAPublicKey)
		rsaKey.GET("decryptedText", GetTextFromEncryptedKey)
	}

	notification := router.Group("notification")
	{
		notification.GET("", PullNotify)
		notification.PUT("/read", ReadNotify)
		notification.POST("/delete", DeleteNotifies)
		notification.POST("/subscribe", UpsertSubscription)
		notification.PUT("/subscribe/:type", UpdateSubscribe)
		notification.DELETE("/unsubscribe/notifytype/:type", Unsubscribe)
		notification.GET("/subscribe", ListSubscriptions)
	}

	announcement := router.Group("announcement")
	{
		announcement.POST("", CreateAnnouncement)
		announcement.PUT("/update", UpdateAnnouncement)
		announcement.GET("/all", PullAllAnnouncement)
		announcement.GET("", PullNotifyAnnouncement)
		announcement.DELETE("/:id", DeleteAnnouncement)
	}

	operation := router.Group("operation")
	{
		operation.GET("", GetOperationLogs)
		operation.POST("", AddSystemOperationLog)
		operation.PUT("/:id", UpdateOperationLog)
	}

	// ---------------------------------------------------------------------------------------
	// system external link
	// ---------------------------------------------------------------------------------------
	externalLink := router.Group("externalLink")
	{
		externalLink.GET("", ListExternalLinks)
		externalLink.POST("", CreateExternalLink)
		externalLink.PUT("/:id", UpdateExternalLink)
		externalLink.DELETE("/:id", DeleteExternalLink)
	}

	// ---------------------------------------------------------------------------------------
	// system custom theme
	// ---------------------------------------------------------------------------------------
	theme := router.Group("theme")
	{
		theme.GET("", GetThemeInfos)
		theme.PUT("", UpdateThemeInfo)
	}

	// ---------------------------------------------------------------------------------------
	// system language settings
	// ---------------------------------------------------------------------------------------
	language := router.Group("language")
	{
		language.GET("", GetSystemLanguage)
		language.PUT("", SetSystemLanguage)
	}

	// ---------------------------------------------------------------------------------------
	// external system API
	// ---------------------------------------------------------------------------------------
	externalSystem := router.Group("external")
	{
		externalSystem.POST("", CreateExternalSystem)
		externalSystem.GET("", ListExternalSystem)
		externalSystem.GET("/:id", GetExternalSystemDetail)
		externalSystem.PUT("/:id", UpdateExternalSystem)
		externalSystem.DELETE("/:id", DeleteExternalSystem)
	}

	// ---------------------------------------------------------------------------------------
	// sonar integration API
	// ---------------------------------------------------------------------------------------
	sonar := router.Group("sonar")
	{
		sonar.POST("/integration", CreateSonarIntegration)
		sonar.PUT("/integration/:id", UpdateSonarIntegration)
		sonar.GET("/integration", ListSonarIntegration)
		sonar.GET("/integration/:id", GetSonarIntegration)
		sonar.DELETE("/integration/:id", DeleteSonarIntegration)
		sonar.POST("/validate", ValidateSonarInformation)
	}

	// ---------------------------------------------------------------------------------------
	// configuration management integration API
	// ---------------------------------------------------------------------------------------
	configuration := router.Group("configuration")
	{
		configuration.GET("", ListConfigurationManagement)
		configuration.POST("", CreateConfigurationManagement)
		configuration.PUT("/:id", UpdateConfigurationManagement)
		configuration.GET("/:id", GetConfigurationManagement)
		configuration.DELETE("/:id", DeleteConfigurationManagement)
		configuration.POST("/validate", ValidateConfigurationManagement)
		configuration.GET("/apollo/:id/app", ListApolloApps)
		configuration.GET("/apollo/:id/:app_id/env", ListApolloEnvAndClusters)
		configuration.GET("/apollo/:id/:app_id/config", ListApolloConfigByType)
		configuration.GET("/apollo/:id/:app_id/:env/:cluster/namespace", ListApolloNamespaces)
		configuration.GET("/apollo/:id/:app_id/:env/:cluster/namespace/:namespace", ListApolloConfig)

		configuration.GET("/nacos/:id/config", ListNacosConfigByType)
		configuration.GET("/nacos/:id/namespace/:namespace/group/:group_name/data/:data_name", GetNacosConfig)
	}

	imapp := router.Group("im_app")
	{
		imapp.GET("", ListIMApp)
		imapp.POST("", CreateIMApp)
		imapp.PUT("/:id", UpdateIMApp)
		imapp.DELETE("/:id", DeleteIMApp)
		imapp.POST("/validate", ValidateIMApp)
	}

	observability := router.Group("observability")
	{
		observability.GET("", ListObservability)
		observability = observability.Group("", isSystemAdmin)
		observability.GET("/detail", ListObservabilityDetail)
		observability.POST("", CreateObservability)
		observability.PUT("/:id", UpdateObservability)
		observability.DELETE("/:id", DeleteObservability)
		observability.POST("/validate", ValidateObservability)
	}

	lark := router.Group("lark")
	{
		lark.GET("/:id/department/:department_id", GetLarkDepartment)
		lark.GET("/:id/user", GetLarkUserID)
		lark.POST("/:id/webhook", LarkEventHandler)
		lark.GET("/:id/user_group", GetLarkUserGroups)
		lark.GET("/:id/user_group/members", GetLarkUserGroupMembers)

		lark.GET("/:id/chat", ListAvailableLarkChat)
		lark.GET("/:id/chat/search", SearchLarkChat)
		lark.GET("/:id/chat/:chat_id/members", ListLarkChatMembers)
	}

	dingtalk := router.Group("dingtalk")
	{
		dingtalk.GET("/:id/department/:department_id", GetDingTalkDepartment)
		dingtalk.GET("/:id/user", GetDingTalkUserID)
		dingtalk.POST("/:ak/webhook", DingTalkEventHandler)
	}

	workwx := router.Group("workwx")
	{
		workwx.GET("/:id/department", GetWorkWxDepartment)
		workwx.GET("/:id/user", GetWorkWxUsers)
		workwx.GET("/:id/webhook", ValidateWorkWXCallback)
		workwx.POST("/:id/webhook", WorkWXEventHandler)
	}

	pm := router.Group("project_management")
	{
		pm.GET("", ListProjectManagement)
		pm.GET("/project", ListProjectManagementForProject)
		pm.POST("", CreateProjectManagement)
		pm.POST("/validate", Validate)
		pm.PATCH("/:id", UpdateProjectManagement)
		pm.DELETE("/:id", DeleteProjectManagement)
		pm.GET("/:id/jira/project", ListJiraProjects)
		pm.GET("/:id/jira/board", ListJiraBoards)
		pm.GET("/:id/jira/board/:boardID/sprint", ListJiraSprints)
		pm.GET("/:id/jira/sprint/:sprintID", GetJiraSprint)
		pm.GET("/:id/jira/sprint/:sprintID/issue", ListJiraSprintIssues)
		pm.GET("/:id/jira/issue", SearchJiraIssues)
		pm.GET("/:id/jira/issue/jql", SearchJiraProjectIssuesWithJQL)
		pm.GET("/:id/jira/type", GetJiraTypes)
		pm.GET("/:id/jira/status", GetJiraProjectStatus)
		pm.GET("/:id/jira/allStatus", GetJiraAllStatus)
		pm.POST("/jira/webhook/:workflowName/:hookName", HandleJiraEvent)
		pm.POST("/meego/webhook/:workflowName/:hookName", HandleMeegoEvent)
	}
	// personal dashboard configuration
	dashboard := router.Group("dashboard")
	{
		// dashboard configuration
		dashboard.GET("/settings", GetDashboardConfiguration)
		dashboard.PUT("/settings", CreateOrUpdateDashboardConfiguration)

		// dashboard card API
		dashboard.GET("/workflow/running", GetRunningWorkflow)
		dashboard.GET("/workflow/mine", GetMyWorkflow)
		dashboard.GET("/environment/:name", GetMyEnvironment)
	}

	// initialization apis
	init := router.Group("initialization")
	{
		init.GET("/status", GetSystemInitializationStatus)
		init.POST("/user", InitializeUser)
	}

	// get nacos info
	nacos := router.Group("nacos")
	{
		nacos.GET("/:nacosID", ListNacosNamespace)
		nacos.GET("/:nacosID/namespace/:nacosNamespaceID", ListNacosConfig)
		nacos.GET("/:nacosID/namespace/:nacosNamespaceID/group", ListNacosGroup)
	}

	// feishu project management module
	meego := router.Group("meego")
	{
		meego.GET("/:id/projects", GetMeegoProjects)
		meego.GET("/:id/projects/:projectID/work_item/types", GetWorkItemTypeList)
		meego.GET("/:id/projects/:projectID/work_item", ListMeegoWorkItems)
		meego.GET("/:id/projects/:projectID/work_item/:workItemID/node", ListMeegoWorkItemNodes)
		meego.POST("/:id/projects/:projectID/work_item/:workItemID/node/:nodeID/operate", OperateMeegoWorkItemNode)
		meego.GET("/:id/projects/:projectID/work_item/:workItemID/transitions", ListAvailableWorkItemTransitions)
	}

	pingcode := router.Group("pingcode")
	{
		pingcode.GET("/:id/projects", ListPingCodeProjects)
		pingcode.GET("/:id/projects/:projectID/boards", ListPingCodeProjectBoards)
		pingcode.GET("/:id/projects/:projectID/sprints", ListPingCodeProjectSprints)
		pingcode.GET("/:id/projects/:projectID/workitem_types", ListPingCodeProjectWorkItemTypes)
		pingcode.GET("/:id/projects/:projectID/workitems", ListPingCodeProjectWorkItems)
		pingcode.GET("/:id/projects/:projectID/workitem/status", ListPingCodeProjectStatus)
	}

	tapd := router.Group("tapd")
	{
		tapd.GET("/:id/projects", ListTapdProjects)
		tapd.GET("/:id/projects/:projectID/iterations", ListTapdProjectIterations)
	}

	// guanceyun api
	guanceyun := router.Group("guanceyun")
	{
		guanceyun.GET("/:id/monitor", ListGuanceyunMonitor)
	}

	// grafana
	grafana := router.Group("grafana")
	{
		grafana.GET("/:id/alert", ListGrafanaAlert)
	}

	// personal favorite API
	favorite := router.Group("favorite")
	{
		favorite.POST("/:type/:name", CreateFavorite)
		favorite.DELETE("/:type/:name", DeleteFavorite)
	}

	// ---------------------------------------------------------------------------------------
	// external system API
	// ---------------------------------------------------------------------------------------
	llm := router.Group("llm")
	{
		llm.POST("/integration", CreateLLMIntegration)
		llm.GET("/integration", ListLLMIntegration)
		llm.GET("/integration/check", CheckLLMIntegration)
		llm.POST("/integration/validate", ValidateLLMIntegration)
		llm.GET("/integration/:id", GetLLMIntegration)
		llm.PUT("/integration/:id", UpdateLLMIntegration)
		llm.DELETE("/integration/:id", DeleteLLMIntegration)
	}

	// ---------------------------------------------------------------------------------------
	// webhook config
	// ---------------------------------------------------------------------------------------
	webhook := router.Group("webhook")
	{
		webhook.GET("/config", GetWebhookConfig)
	}

	// ---------------------------------------------------------------------------------------
	// custom navigation (system level)
	// ---------------------------------------------------------------------------------------
	navigation := router.Group("navigation")
	{
		navigation.GET("", GetSystemNavigation)
		navigation.PUT("", UpdateSystemNavigation)
	}

	// ---------------------------------------------------------------------------------------
	// database instance
	// ---------------------------------------------------------------------------------------
	dbs := router.Group("dbinstance")
	{
		dbs.POST("", CreateDBInstance)
		dbs.GET("", ListDBInstanceInfo)
		dbs.GET("/project", ListDBInstancesInfoByProject)
		dbs.GET("/detail", ListDBInstance)
		dbs.GET("/:id", GetDBInstance)
		dbs.PUT("/:id", UpdateDBInstance)
		dbs.DELETE("/:id", DeleteDBInstance)
		dbs.POST("/validate", ValidateDBInstance)
	}

	// ---------------------------------------------------------------------------------------
	// service label management
	// ---------------------------------------------------------------------------------------
	labels := router.Group("labels")
	{
		labels.GET("", ListServiceLabelSettings)
		labels.POST("", CreateServiceLabelSetting)
		labels.PUT("/:id", UpdateServiceLabelSetting)
		labels.DELETE("/:id", DeleteServiceLabelSetting)

	}

	// ---------------------------------------------------------------------------------------
	// idp plugin management API
	// ---------------------------------------------------------------------------------------
	plugin := router.Group("plugin")
	{
		plugin.GET("", ListPlugins)
		plugin.POST("", CreatePlugin)
		plugin.PUT("/:id", UpdatePlugin)
		plugin.DELETE("/:id", DeletePlugin)
		// download plugin file by id
		plugin.GET("/:id/file", GetPluginFile)
	}

	sae := router.Group("sae")
	{
		sae.POST("", CreateSAE)
		sae.GET("", ListSAE)
		sae.GET("/detail", ListSAEInfo)
		sae.GET("/:id", GetSAE)
		sae.PUT("/:id", UpdateSAE)
		sae.DELETE("/:id", DeleteSAE)
		sae.POST("/validate", ValidateSAE)
	}

	// ---------------------------------------------------------------------------------------
	// temporary file upload API (multi-part upload for large files)
	// ---------------------------------------------------------------------------------------
	tempFile := router.Group("tempFile")
	{
		tempFile.POST("/initiate", InitiateUpload)
		tempFile.POST("/upload/:sessionId/:partNumber", UploadPart)
		tempFile.POST("/complete/:sessionId", CompleteUpload)
		tempFile.GET("/status/:sessionId", GetUploadStatus)
		tempFile.GET("/fileInfo/:fileId", GetTemporaryFileInfo)
		tempFile.GET("/download/:fileId", DownloadTemporaryFile)
		tempFile.POST("/cleanup", CleanupExpiredUploads)
	}
}

type OpenAPIRouter struct{}

func (*OpenAPIRouter) Inject(router *gin.RouterGroup) {
	reg := router.Group("registry")
	{
		reg.POST("", OpenAPICreateRegistry)
		reg.GET("", OpenAPIListRegistry)
		reg.GET("/:id", OpenAPIGetRegistry)
		reg.PUT("/:id", OpenAPIUpdateRegistry)
	}

	cluster := router.Group("cluster")
	{
		cluster.POST("", OpenAPICreateCluster)
		cluster.GET("", OpenAPIListCluster)
		cluster.PUT("/:id", OpenAPIUpdateCluster)
		cluster.DELETE("/:id", OpenAPIDeleteCluster)
	}

	operation := router.Group("operation")
	{
		operation.GET("", OpenAPIGetOperationLogs)
		operation.GET("/env", OpenAPIGetEnvOperationLogs)
	}
}
