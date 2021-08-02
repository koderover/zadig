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
	router.Use(gin2.Auth())

	log := router.Group("log")
	{
		log.GET("/pipelines/:pipelineName/tasks/:taskId/service/:serviceName", GetBuildJobContainerLogs)
		log.GET("/workflow/:pipelineName/tasks/:taskId/service/:serviceName", GetWorkflowBuildJobContainerLogs)
		log.GET("/pipelines/:pipelineName/tasks/:taskId/tests/:testName", GetTestJobContainerLogs)
		log.GET("/workflow/:pipelineName/tasks/:taskId/tests/:testName/service/:serviceName", GetWorkflowTestJobContainerLogs)
		log.GET("/pods/:podName/containers/:containerName", GetContainerLogs)
	}

	sse := router.Group("sse")
	{
		sse.GET("/pods/:podName/containers/:containerName", GetContainerLogsSSE)
		sse.GET("/build/:pipelineName/:taskId/:lines", GetBuildJobContainerLogsSSE)
		sse.GET("/workflow/build/:pipelineName/:taskId/:lines/:serviceName", GetWorkflowBuildJobContainerLogsSSE)
		sse.GET("/test/:pipelineName/:taskId/:testName/:lines", GetTestJobContainerLogsSSE)
		sse.GET("/workflow/test/:pipelineName/:taskId/:testName/:lines/:serviceName", GetWorkflowTestJobContainerLogsSSE)
		sse.GET("/service/build/:serviceName/:envName/:productName", GetServiceJobContainerLogsSSE)
	}
}
