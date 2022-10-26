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
	// 对外公共接口
	// ---------------------------------------------------------------------------------------
	webhook := router.Group("webhook")
	{
		webhook.POST("", ProcessWebHook)
	}

	build := router.Group("build")
	{
		build.GET("/:name/:version/to/subtasks", BuildModuleToSubTasks)
	}

	// ---------------------------------------------------------------------------------------
	// Server Sent Events 接口
	// ---------------------------------------------------------------------------------------
	sse := router.Group("sse")
	{
		sse.GET("/workflows/id/:id/pipelines/:name", GetWorkflowTaskSSE)
		sse.GET("/tasks/running", RunningPipelineTasksSSE)
		sse.GET("/tasks/pending", PendingPipelineTasksSSE)
		sse.GET("/workflowTasks/running", RunningWorkflowTasksSSE)
		sse.GET("/workflowTasks/pending", PendingWorkflowTasksSSE)
		sse.GET("/tasks/id/:id/pipelines/:name", GetPipelineTaskSSE)
		sse.GET("/workflowtask/v3/id/:id/name/:name", GetWorkflowTaskV3SSE)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline 管理接口
	// ---------------------------------------------------------------------------------------
	pipeline := router.Group("v2/pipelines")
	{
		pipeline.GET("", ListPipelines)
		pipeline.GET("/:name", GetPipeline)
		pipeline.POST("", GetPipelineProductName, UpsertPipeline)
		pipeline.POST("/old/:old/new/:new", GetProductNameByPipeline, CopyPipeline)
		pipeline.PUT("/rename/:old/:new", GetProductNameByPipeline, RenamePipeline)
		pipeline.DELETE("/:name", GetProductNameByPipeline, DeletePipeline)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline 状态接口
	// ---------------------------------------------------------------------------------------
	statusV2 := router.Group("v2/status")
	{
		statusV2.GET("/preview", ListPipelinesPreview)
		statusV2.GET("/task/info", FindTasks)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline 任务管理接口
	// ---------------------------------------------------------------------------------------
	taskV2 := router.Group("v2/tasks")
	{
		taskV2.POST("", GetProductNameByPipelineTask, CreatePipelineTask)
		taskV2.GET("/max/:max/start/:start/pipelines/:name", ListPipelineTasksResult)
		taskV2.GET("/id/:id/pipelines/:name", GetPipelineTask)
		taskV2.POST("/id/:id/pipelines/:name/restart", GetProductNameByPipeline, RestartPipelineTask)
		taskV2.DELETE("/id/:id/pipelines/:name", GetProductNameByPipeline, CancelTaskV2)
		taskV2.GET("/pipelines/:name/products", ListPipelineUpdatableProductNames)
		taskV2.GET("/file", GetPackageFile)
		taskV2.GET("/workflow/:pipelineName/taskId/:taskId", GetArtifactFile)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline Favorite 接口
	// ---------------------------------------------------------------------------------------
	favorite := router.Group("favorite")
	{
		favorite.POST("", CreateFavoritePipeline)
		favorite.DELETE("/:productName/:name/:type", DeleteFavoritePipeline)
		favorite.GET("", ListFavoritePipelines)
	}

	// ---------------------------------------------------------------------------------------
	// 产品工作流模块接口
	// ---------------------------------------------------------------------------------------
	workflow := router.Group("workflow")
	{
		workflow.POST("/:productName/auto", AutoCreateWorkflow)
		workflow.POST("", GetWorkflowProductName, CreateWorkflow)
		workflow.PUT("/:workflowName", GetWorkflowProductName, UpdateWorkflow)
		workflow.GET("", ListWorkflows)
		workflow.GET("/testName/:testName", ListTestWorkflows)
		workflow.GET("/find/:name", FindWorkflow)
		workflow.DELETE("/:name", GetProductNameByWorkflow, DeleteWorkflow)
		workflow.GET("/preset/:productName", PreSetWorkflow)

		workflow.PUT("/old/:old/new/:new/:newDisplay", CopyWorkflow)
	}

	// ---------------------------------------------------------------------------------------
	// 产品工作流任务接口
	// ---------------------------------------------------------------------------------------
	workflowtask := router.Group("workflowtask")
	{
		//todo 修改权限的uuid
		workflowtask.GET("/targets/:productName/:namespace", GetWorkflowArgs)
		workflowtask.GET("/preset/:namespace/:workflowName", PresetWorkflowArgs)
		workflowtask.POST("/:id", CreateWorkflowTask)
		workflowtask.PUT("/:id", CreateArtifactWorkflowTask)
		workflowtask.GET("/max/:max/start/:start/pipelines/:name", ListWorkflowTasksResult)
		workflowtask.GET("/filters/pipelines/:name", GetFiltersPipeline)
		workflowtask.GET("/id/:id/pipelines/:name", GetWorkflowTask)
		workflowtask.POST("/id/:id/pipelines/:name/restart", RestartWorkflowTask)
		workflowtask.DELETE("/id/:id/pipelines/:name", CancelWorkflowTaskV2)
		workflowtask.GET("/callback/id/:id/name/:name", GetWorkflowTaskCallback)
	}

	serviceTask := router.Group("servicetask")
	{
		serviceTask.GET("/workflows/:productName/:envName/:serviceName/:serviceType", ListServiceWorkflows)
	}

	// ---------------------------------------------------------------------------------------
	// 新版本 通用工作流（暂命名） 接口
	// ---------------------------------------------------------------------------------------
	workflowV3 := router.Group("v3")
	{
		workflowV3.POST("", CreateWorkflowV3)
		workflowV3.GET("", ListWorkflowsV3)
		workflowV3.GET("/:id", GetWorkflowV3Detail)
		workflowV3.PUT("/:id", UpdateWorkflowV3)
		workflowV3.DELETE("/:id", DeleteWorkflowV3)
		workflowV3.GET("/:id/args", GetWorkflowV3Args)
	}

	// ---------------------------------------------------------------------------------------
	// workflow v3 任务接口
	// ---------------------------------------------------------------------------------------
	taskV3 := router.Group("v3/workflowtask")
	{
		taskV3.POST("", CreateWorkflowTaskV3)
		taskV3.POST("/id/:id/name/:name/restart", RestartWorkflowTaskV3)
		taskV3.DELETE("/id/:id/name/:name", CancelWorkflowTaskV3)
		taskV3.GET("/max/:max/start/:start/name/:name", ListWorkflowV3TasksResult)
		taskV3.GET("/id/:id/name/:name", GetWorkflowTaskV3)
		taskV3.GET("/callback/id/:id/name/:name", GetWorkflowTaskV3Callback)
	}

	// ---------------------------------------------------------------------------------------
	// 新版本 通用工作流（暂命名） 接口
	// ---------------------------------------------------------------------------------------
	workflowV4 := router.Group("v4")
	{
		workflowV4.POST("", CreateWorkflowV4)
		workflowV4.GET("", ListWorkflowV4)
		workflowV4.POST("/lint", LintWorkflowV4)
		workflowV4.GET("/name/:name", FindWorkflowV4)
		workflowV4.PUT("/:name", UpdateWorkflowV4)
		workflowV4.DELETE("/:name", DeleteWorkflowV4)
		workflowV4.GET("/preset/:name", GetWorkflowV4Preset)
		workflowV4.GET("/webhook/preset", GetWebhookForWorkflowV4Preset)
		workflowV4.GET("/webhook", ListWebhookForWorkflowV4)
		workflowV4.POST("/webhook/:workflowName", CreateWebhookForWorkflowV4)
		workflowV4.PUT("/webhook/:workflowName", UpdateWebhookForWorkflowV4)
		workflowV4.DELETE("/webhook/:workflowName/trigger/:triggerName", DeleteWebhookForWorkflowV4)
		workflowV4.GET("/cron/preset", GetCronForWorkflowV4Preset)
		workflowV4.GET("/cron", ListCronForWorkflowV4)
		workflowV4.POST("/cron/:workflowName", CreateCronForWorkflowV4)
		workflowV4.PUT("/cron", UpdateCronForWorkflowV4)
		workflowV4.DELETE("/cron/:workflowName/trigger/:cronID", DeleteCronForWorkflowV4)
		workflowV4.POST("/patch", GetPatchParams)

	}

	// ---------------------------------------------------------------------------------------
	// workflow v4 任务接口
	// ---------------------------------------------------------------------------------------
	taskV4 := router.Group("v4/workflowtask")
	{
		taskV4.POST("", CreateWorkflowTaskV4)
		taskV4.GET("", ListWorkflowTaskV4)
		taskV4.GET("/workflow/:workflowName/task/:taskID", GetWorkflowTaskV4)
		taskV4.DELETE("/workflow/:workflowName/task/:taskID", CancelWorkflowTaskV4)
		taskV4.GET("/clone/workflow/:workflowName/task/:taskID", CloneWorkflowTaskV4)
		taskV4.POST("/approve", ApproveStage)
		taskV4.GET("/workflow/:workflowName/taskId/:taskId/job/:jobName", GetWorkflowV4ArtifactFileContent)
	}

	// ---------------------------------------------------------------------------------------
	// workflow view 接口
	// ---------------------------------------------------------------------------------------
	view := router.Group("view")
	{
		view.POST("", CreateWorkflowView)
		view.GET("", ListWorkflowViewNames)
		view.GET("/preset", GetWorkflowViewPreset)
		view.DELETE("/:projectName/:viewName", DeleteWorkflowView)
		view.PUT("", UpdateWorkflowView)
	}

	// ---------------------------------------------------------------------------------------
	// plugin repo 接口
	// ---------------------------------------------------------------------------------------
	plugin := router.Group("plugin")
	{
		plugin.GET("/template", ListPluginTemplates)
		plugin.POST("", UpsertUserPluginRepository)
		plugin.POST("/enterprise", UpsertEnterprisePluginRepository)
		plugin.GET("", ListUnofficalPluginRepositories)
		plugin.DELETE("/:id", DeletePluginRepo)
	}

	bundles := router.Group("bundle-resources")
	{
		bundles.GET("", GetBundleResources)
	}
}

type OpenAPIRouter struct{}

func (*OpenAPIRouter) Inject(router *gin.RouterGroup) {
	// custom workflow apis
	custom := router.Group("custom")
	{
		custom.POST("/task", CreateCustomWorkflowTask)
		custom.GET("/task", OpenAPIGetWorkflowTaskV4)
		custom.DELETE("/task", OpenAPICancelWorkflowTaskV4)
		custom.POST("/task/approve", ApproveStage)
	}
}
