/*
Copyright 2025 The KodeRover Authors.

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
	application := router.Group("applications")
	{
		application.POST("", CreateApplication)
		application.POST("/bulk", BulkCreateApplications)
		application.GET("/:id", GetApplication)
		application.PUT("/:id", UpdateApplication)
		application.DELETE("/:id", DeleteApplication)
		// this is a very wrong api, we shouldn't be able to search envs from applications. but for now we just have to live with it.
		// TODO: remove this api
		application.GET("/:id/envs", ListApplicationEnvs)

		application.POST("/search", SearchApplications)
	}

	fields := router.Group("fields")
	{
		fields.POST("", CreateFieldDefinition)
		fields.GET("", ListFieldDefinitions)
		fields.PUT("/:id", UpdateFieldDefinition)
		fields.DELETE("/:id", DeleteFieldDefinition)
	}
}
