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

	"github.com/koderover/zadig/pkg/microservice/aslan/middleware"
	"github.com/koderover/zadig/pkg/types/permission"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	router.Use(middleware.Auth())

	// ---------------------------------------------------------------------------------------
	// 系统测试接口
	// ---------------------------------------------------------------------------------------
	itReport := router.Group("itreport")
	{
		itReport.GET("/pipelines/:pipelineName/id/:id/names/:testName", GetLocalTestSuite)
		itReport.GET("/workflow/:pipelineName/id/:id/names/:testName/service/:serviceName", GetWorkflowLocalTestSuite)
		itReport.GET("/latest/service/:serviceName", GetTestLocalTestSuite)
	}

	// ---------------------------------------------------------------------------------------
	// 测试管理接口
	// ---------------------------------------------------------------------------------------
	tester := router.Group("test")
	{
		tester.POST("", GetTestProductName, middleware.IsHavePermission([]string{permission.TestManageUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, CreateTestModule)
		tester.PUT("", GetTestProductName, middleware.IsHavePermission([]string{permission.TestManageUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, UpdateTestModule)
		tester.GET("", ListTestModules)
		tester.GET("/:name", GetTestModule)
		tester.DELETE("/:name", middleware.IsHavePermission([]string{permission.TestDeleteUUID}, permission.QueryType), middleware.UpdateOperationLogStatus, DeleteTestModule)
	}

	testDetail := router.Group("testdetail")
	{
		testDetail.GET("", ListDetailTestModules)
	}

	// ---------------------------------------------------------------------------------------
	// test 任务接口
	// ---------------------------------------------------------------------------------------
	testTask := router.Group("testtask")
	{
		testTask.POST("", middleware.UpdateOperationLogStatus, CreateTestTask)
		testTask.POST("/productName/:productName/id/:id/pipelines/:name/restart", middleware.UpdateOperationLogStatus, RestartTestTask)
		testTask.DELETE("/productName/:productName/id/:id/pipelines/:name", middleware.UpdateOperationLogStatus, CancelTestTaskV2)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline workspace 管理接口
	// ---------------------------------------------------------------------------------------
	workspace := router.Group("workspace")
	{
		workspace.GET("/workflow/:pipelineName/taskId/:taskId", GetTestArtifactInfo)
	}
}
