/*
Copyright 2024 The KodeRover Authors.

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

import "github.com/gin-gonic/gin"

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	lark := router.Group("lark")
	{
		lark.POST("login", LarkLogin)
		lark.GET("config/auth", GetLarkAuthConfig)
		lark.PUT("config/auth", UpdateLarkAuthConfig)
		lark.GET("config/workflow", GetLarkWorkflowConfig)
		lark.PUT("config/workflow", UpdateLarkWorkflowConfig)
		lark.GET("workitem/type", GetLarkWorkitemType)
		lark.GET("workitem/type/:workitemTypeKey", GetLarkWorkitemTypeDetail)
		lark.GET("workitem/type/:workitemTypeKey/template", GetLarkWorkitemTypeTemplate)
		lark.GET("workitem/type/:workitemTypeKey/template/:templateID/node", GetLarkWorkitemTypeNodes)
		lark.GET("workitem/:workitemTypeKey/:workItemID/workflow", GetLarkWorkitemWorkflow)
		lark.POST("workitem/:workitemTypeKey/:workItemID/workflow", ExecuteLarkWorkitemWorkflow)
		lark.GET("workitem/:workitemTypeKey/:workItemID/workflow/:workflowName/task", ListLarkWorkitemWorkflowTask)
	}
}
