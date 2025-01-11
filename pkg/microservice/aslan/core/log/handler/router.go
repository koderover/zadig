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
	{
		router.GET("/pods/:name", GetContainerLogs)
	}

	log := router.Group("log")
	{
		log.GET("/testing/:test_name/tasks/:task_id", GetTestingContainerLogs)
		log.GET("/scanning/:id/task/:scan_id", GetScanningContainerLogs)
		log.GET("/v4/workflow/:workflowName/tasks/:taskID/jobs/:jobName", GetWorkflowV4JobContainerLogs)
		log.POST("/ai/workflow/:workflowName/tasks/:taskID/jobs/:jobName", AIAnalyzeBuildLog)
	}

	sse := router.Group("sse")
	{
		sse.GET("/pods/:podName/containers/:containerName", GetContainerLogsSSE)
		sse.GET("/testing/:test_name/tasks/:task_id", GetTestingContainerLogsSSE)
		sse.GET("/scanning/:id/task/:scan_id", GetScanningContainerLogsSSE)
		sse.GET("/v4/workflow/:workflowName/:taskID/:jobName/:lines", GetWorkflowJobContainerLogsSSE)
		sse.GET("/jenkins/:id/:jobName/:jobID", GetJenkinsJobContainerLogsSSE)
	}
}

type OpenAPIRouter struct{}

func (*OpenAPIRouter) Inject(router *gin.RouterGroup) {
	sse := router.Group("sse")
	{
		sse.GET("/pods/:podName/containers/:containerName", OpenAPIGetContainerLogsSSE)
	}
}
