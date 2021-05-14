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
	// ---------------------------------------------------------------------------------------
	// 对外公共接口
	// ---------------------------------------------------------------------------------------
	webhook := router.Group("webhook")
	{
		webhook.POST("/ci/webhook", ProcessGithub)
		webhook.POST("/githubWebHook", ProcessGithub)
		webhook.POST("/gitlabhook", ProcessGitlabHook)
		webhook.POST("/gerritHook", ProcessGerritHook)
	}

	router.Use(middleware.Auth())

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
		sse.GET("/tasks/id/:id/pipelines/:name", GetPipelineTaskSSE)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline Favorite 接口
	// ---------------------------------------------------------------------------------------
	favorite := router.Group("favorite")
	{
		favorite.POST("", middleware.UpdateOperationLogStatus, CreateFavoritePipeline)
		favorite.DELETE("/:productName/:name/:type", middleware.UpdateOperationLogStatus, DeleteFavoritePipeline)

	}

	// ---------------------------------------------------------------------------------------
	// 产品工作流模块接口
	// ---------------------------------------------------------------------------------------
	workflow := router.Group("workflow")
	{
		workflow.POST("/:productName/auto", AutoCreateWorkflow)
		workflow.POST("", GetWorkflowProductName, middleware.IsHavePermission([]string{permission.WorkflowCreateUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, CreateWorkflow)
		workflow.PUT("", GetWorkflowProductName, middleware.IsHavePermission([]string{permission.WorkflowUpdateUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, UpdateWorkflow)
		workflow.GET("", ListWorkflows)
		workflow.GET("/testName/:testName", ListAllWorkflows)
		workflow.GET("/find/:name", FindWorkflow)
		workflow.DELETE("/:name", GetProductNameByWorkflow, middleware.IsHavePermission([]string{permission.WorkflowDeleteUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, DeleteWorkflow)
		workflow.GET("/preset/:productName", PreSetWorkflow)

		workflow.PUT("/old/:old/new/:new", CopyWorkflow)
	}

	// ---------------------------------------------------------------------------------------
	// 产品工作流任务接口
	// ---------------------------------------------------------------------------------------
	workflowtask := router.Group("workflowtask")
	{
		//todo 修改权限的uuid
		workflowtask.GET("/targets/:productName/:namespace", GetWorkflowArgs)
		workflowtask.GET("/preset/:namespace/:workflowName", PresetWorkflowArgs)
		workflowtask.POST("", GetWorkflowTaskProductName, middleware.IsHavePermission([]string{permission.WorkflowTaskUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, CreateWorkflowTask)
		workflowtask.PUT("", GetWorkflowTaskProductName, middleware.IsHavePermission([]string{permission.WorkflowTaskUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, CreateArtifactWorkflowTask)
		workflowtask.GET("/max/:max/start/:start/pipelines/:name", ListWorkflowTasksResult)
		workflowtask.GET("/id/:id/pipelines/:name", GetWorkflowTask)
		workflowtask.POST("/id/:id/pipelines/:name/restart", GetWorkflowTaskProductNameByTask, middleware.IsHavePermission([]string{permission.WorkflowTaskUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, RestartWorkflowTask)
		workflowtask.DELETE("/id/:id/pipelines/:name", GetWorkflowTaskProductNameByTask, middleware.IsHavePermission([]string{permission.WorkflowTaskUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, CancelWorkflowTaskV2)
	}

	serviceTask := router.Group("servicetask")
	{
		serviceTask.GET("/workflows/:productName/:envName/:serviceName/:serviceType", ListServiceWorkflows)
	}
}
