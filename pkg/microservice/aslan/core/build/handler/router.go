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
	build := router.Group("build")
	{
		build.GET("/:name", FindBuildModule)
		build.GET("", ListBuildModules)
		build.GET("/serviceModule", ListBuildModulesByServiceModule)
		build.POST("", CreateBuildModule)
		build.PUT("", UpdateBuildModule)
		build.DELETE("", DeleteBuildModule)
		build.POST("/targets", UpdateBuildTargets)
	}

	deploy := router.Group("deploy")
	{
		deploy.GET("/:name", FindDeploy)
		deploy.POST("", CreateDeploy)
		deploy.PUT("", UpdateDeploy)
		deploy.DELETE("", DeleteDeploy)
	}

	target := router.Group("targets")
	{
		target.GET("", ListDeployTarget)
		target.GET("/:productName", ListBuildModulesForProduct)
	}
}

type OpenAPIRouter struct{}

func (*OpenAPIRouter) Inject(router *gin.RouterGroup) {
	build := router.Group("")
	{
		build.POST("", OpenAPICreateBuildModule)
		build.PUT("", OpenAPIUpdateBuildModule)
		build.DELETE("", OpenAPIDeleteBuildModule)
		build.GET("", OpenAPIListBuildModules)
		build.GET("/:name/detail", OpenAPIGetBuildModule)

		build.PUT("/:name/template", OpenAPIUpdateBuildModuleFromTemplate)
	}
}
