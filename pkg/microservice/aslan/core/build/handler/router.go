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
	"github.com/koderover/zadig/pkg/types/permission"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	router.Use(gin2.Auth())

	build := router.Group("build")
	{
		build.GET("/:name", FindBuildModule)
		build.GET("", ListBuildModules)
		build.POST("", gin2.StoreProductName, gin2.IsHavePermission([]string{permission.BuildManageUUID}, permission.ContextKeyType), gin2.UpdateOperationLogStatus, CreateBuildModule)
		build.PUT("", gin2.StoreProductName, gin2.IsHavePermission([]string{permission.BuildManageUUID}, permission.ContextKeyType), gin2.UpdateOperationLogStatus, UpdateBuildModule)
		build.DELETE("", gin2.IsHavePermission([]string{permission.BuildDeleteUUID}, permission.QueryType), gin2.UpdateOperationLogStatus, DeleteBuildModule)
		build.POST("/targets", gin2.IsHavePermission([]string{permission.BuildManageUUID}, permission.QueryType), gin2.UpdateOperationLogStatus, UpdateBuildTargets)
	}

	target := router.Group("targets")
	{
		target.GET("", ListDeployTarget)
		target.GET("/:productName", ListBuildModulesForProduct)
	}
}
