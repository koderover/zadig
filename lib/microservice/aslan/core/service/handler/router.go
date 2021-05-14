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

	k8s := router.Group("services")
	{
		k8s.GET("", ListServiceTemplate)
		k8s.GET("/:name/:type", GetServiceTemplate)
		k8s.GET("/:name", GetServiceTemplateOption)
		k8s.POST("", GetServiceTemplateProductName, middleware.IsHavePermission([]string{permission.ServiceTemplateManageUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, CreateServiceTemplate)
		k8s.PUT("", GetServiceTemplateObjectProductName, middleware.IsHavePermission([]string{permission.ServiceTemplateManageUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, UpdateServiceTemplate)
		k8s.PUT("/yaml/validator", YamlValidator)
		k8s.DELETE("/:name/:type", middleware.IsHavePermission([]string{permission.ServiceTemplateDeleteUUID}, permission.QueryType), middleware.UpdateOperationLogStatus, DeleteServiceTemplate)
		k8s.GET("/:name/:type/ports", ListServicePort)
	}

	name := router.Group("name")
	{
		name.GET("", ListServiceTemplateNames)
	}

	loader := router.Group("loader")
	{
		loader.GET("/preload/:codehostId/:branchName", PreloadServiceTemplate)
		loader.POST("/load/:codehostId/:branchName", LoadServiceTemplate)
		loader.GET("/validateUpdate/:codehostId/:branchName", ValidateServiceUpdate)
	}
}
