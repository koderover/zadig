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
	// 查看html测试报告不做鉴权
	testReport := router.Group("report")
	{
		testReport.GET("", GetHTMLTestReport)
	}

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
		tester.POST("", GetTestProductName, gin2.UpdateOperationLogStatus, CreateTestModule)
		tester.PUT("", GetTestProductName, gin2.UpdateOperationLogStatus, UpdateTestModule)
		tester.GET("", ListTestModules)
		tester.GET("/:name", GetTestModule)
		tester.DELETE("/:name", gin2.UpdateOperationLogStatus, DeleteTestModule)
	}

	// ---------------------------------------------------------------------------------------
	// Code scan APIs
	// ---------------------------------------------------------------------------------------
	scanner := router.Group("scanning")
	{
		// code scan config apis
		scanner.POST("", GetScanningProductName, gin2.UpdateOperationLogStatus, CreateScanningModule)
		scanner.PUT("/:id", GetScanningProductName, gin2.UpdateOperationLogStatus, UpdateScanningModule)
		scanner.GET("", ListScanningModule)
		scanner.GET("/:id", GetScanningModule)
		scanner.DELETE("/:id", gin2.UpdateOperationLogStatus, DeleteScanningModule)

		// code scan tasks apis
		scanner.POST("/:id/task", gin2.UpdateOperationLogStatus, CreateScanningTask)
		scanner.GET("/:id/task", ListScanningTask)
		scanner.GET("/:id/task/:scan_id", GetScanningTask)
		scanner.DELETE("/:id/task/:scan_id", gin2.UpdateOperationLogStatus, CancelScanningTask)
		scanner.GET("/:id/task/:scan_id/sse", GetScanningTaskSSE)
	}

	testStat := router.Group("teststat")
	{
		// 供aslanx的enterprise模块的数据统计调用
		testStat.GET("", ListTestStat)
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
		testTask.POST("", gin2.UpdateOperationLogStatus, CreateTestTask)
		testTask.POST("/productName/:productName/id/:id/pipelines/:name/restart", gin2.UpdateOperationLogStatus, RestartTestTask)
		testTask.DELETE("/productName/:productName/id/:id/pipelines/:name", gin2.UpdateOperationLogStatus, CancelTestTaskV2)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline workspace 管理接口
	// ---------------------------------------------------------------------------------------
	workspace := router.Group("workspace")
	{
		workspace.GET("/workflow/:pipelineName/taskId/:taskId", GetTestArtifactInfo)
	}
}
