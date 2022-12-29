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
	projects := router.Group("projects")
	{
		projects.DELETE("/:name", DeleteProject)
		projects.GET("", ListProjects)
		projects.POST("", CreateProject)
		projects.PUT("/:name", UpdateProject)
	}

	workflows := router.Group("workflows")
	{
		workflows.GET("testName/:testName", ListTestWorkflows)
		workflows.GET("", ListWorkflows)
		workflows.GET("v3", ListWorkflowsV3)
		workflows.GET("all", ListAllWorkflows)
	}

	rolebindings := router.Group("bindings")
	{
		rolebindings.GET("", ListBindings)
	}
	users := router.Group("users")
	{
		users.POST("", SearchUsers)
		users.DELETE("/:id", DeleteUser)
	}
	downloads := router.Group("kubeconfig")
	{
		downloads.GET("", GetKubeConfig)
	}
	stat := router.Group("stat")
	{
		stat.GET("dashboard/overview", Overview)
		stat.GET("dashboard/deploy", Deploy)
		stat.GET("dashboard/test", Test)
		stat.GET("dashboard/build", Build)
	}
	testing := router.Group("testing")
	{
		testing.GET("", ListTestings)
	}
}
