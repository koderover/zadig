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

	// 查看自定义变量是否被引用
	render := router.Group("renders")
	{
		// render.POST("/data", CheckRenderDataStatus)
		// render.GET("/keys", ListTmplRenderKeys)
		// render.GET("", ListRenderSets)
		// render.GET("/render/:name", GetRenderSet)
		render.GET("/render/:name/revision/:revision", GetRenderSetInfo)
		// render.GET("/covered/:productName/:renderName", ValidateRenderSet)
		// render.POST("", CreateRenderSet)
		render.PUT("", UpdateRenderSet)
		// render.PUT("/default", SetDefaultRenderSet)
		// render.PUT("relate/:productName/:renderName", RelateRender)
	}

	// ---------------------------------------------------------------------------------------
	// 项目管理接口
	// ---------------------------------------------------------------------------------------
	product := router.Group("products")
	{
		product.GET("/:name", GetProductTemplate)
		product.GET("/:name/services", GetProductTemplateServices)
		product.GET("", ListProductTemplate)
		product.POST("", middleware.StoreProductName, middleware.IsHavePermission([]string{permission.SuperUserUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, CreateProductTemplate)
		product.PUT("/:name", middleware.IsHavePermission([]string{permission.ServiceTemplateEditUUID}, permission.ParamType), middleware.UpdateOperationLogStatus, UpdateProductTemplate)
		product.PUT("/:name/:status", middleware.IsHavePermission([]string{permission.ServiceTemplateEditUUID}, permission.ParamType), middleware.UpdateOperationLogStatus, UpdateProductTmplStatus)
		product.PUT("", middleware.StoreProductName, middleware.IsHavePermission([]string{permission.SuperUserUUID}, permission.ContextKeyType), middleware.UpdateOperationLogStatus, UpdateProject)
		product.DELETE("/:name", middleware.IsHavePermission([]string{permission.SuperUserUUID}, permission.ParamType), middleware.UpdateOperationLogStatus, DeleteProductTemplate)
	}

	openSource := router.Group("opensource")
	{
		openSource.POST("/:productName/fork", ForkProduct)
		openSource.DELETE("/:productName/fork", UnForkProduct)
	}

	template := router.Group("templates")
	{
		template.GET("/info", ListTemplatesHierachy)
	}
}
