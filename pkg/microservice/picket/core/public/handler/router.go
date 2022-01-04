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

	dev := router.Group("")
	{
		dev.POST("/workflowTask/create", CreateWorkflowTask)
		dev.POST("/workflowTask/id/:id/pipelines/:name/cancel", CancelWorkflowTask)
		dev.POST("/workflowTask/id/:id/pipelines/:name/restart", RestartWorkflowTask)
		dev.GET("/workflowTask", ListWorkflowTask)
		dev.GET("/workflowTask/id/:id/pipelines/:name", GetWorkflowDetail)
		dev.GET("/dc/releases", ListDelivery)
		dev.GET("/dc/artifact", GetArtifactInfo)
	}
}
