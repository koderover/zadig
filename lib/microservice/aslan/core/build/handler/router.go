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
	router.Use(middleware.Auth())

	build := router.Group("build")
	{
		build.GET("/:name/:version", FindBuildModule)
		build.GET("", ListBuildModules)
		build.POST("", middleware.StoreProductName, middleware.IsHavePermission([]string{permission.BuildManageUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, CreateBuildModule)
		build.PUT("", middleware.StoreProductName, middleware.IsHavePermission([]string{permission.BuildManageUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, UpdateBuildModule)
		build.DELETE("", middleware.IsHavePermission([]string{permission.BuildDeleteUUID}, permission.QueryType), middleware.UpdateOperationLogStatus, DeleteBuildModule)
		build.POST("/targets", middleware.IsHavePermission([]string{permission.BuildManageUUID}, permission.QueryType), middleware.UpdateOperationLogStatus, UpdateBuildTargets)
	}

	target := router.Group("targets")
	{
		target.GET("", ListDeployTarget)
		target.GET("/:productName", ListBuildModulesForProduct)
	}
}
