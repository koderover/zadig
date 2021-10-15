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

	// 查看自定义变量是否被引用
	render := router.Group("renders")
	{
		render.GET("/render/:name/revision/:revision", GetRenderSetInfo)
		render.PUT("", UpdateRenderSet)
	}

	// ---------------------------------------------------------------------------------------
	// 项目管理接口
	// ---------------------------------------------------------------------------------------
	product := router.Group("products")
	{
		product.GET("/:name", GetProductTemplate)
		product.GET("/:name/services", GetProductTemplateServices)
		product.GET("", ListProductTemplate)
		product.GET("/:name/searching-rules", GetCustomMatchRules)
		product.PUT("/:name/searching-rules", gin2.StoreProductName, gin2.IsHavePermission([]string{permission.SuperUserUUID}, permission.ContextKeyType), gin2.UpdateOperationLogStatus, CreateOrUpdateMatchRules)
		product.POST("", gin2.StoreProductName, gin2.IsHavePermission([]string{permission.SuperUserUUID}, permission.ContextKeyType), gin2.UpdateOperationLogStatus, CreateProductTemplate)
		product.PUT("/:name", gin2.IsHavePermission([]string{permission.ServiceTemplateEditUUID}, permission.ParamType), gin2.UpdateOperationLogStatus, UpdateProductTemplate)
		product.PUT("/:name/:status", gin2.IsHavePermission([]string{permission.ServiceTemplateEditUUID}, permission.ParamType), gin2.UpdateOperationLogStatus, UpdateProductTmplStatus)
		product.PUT("", gin2.StoreProductName, gin2.IsHavePermission([]string{permission.SuperUserUUID}, permission.ContextKeyType), gin2.UpdateOperationLogStatus, UpdateProject)
		product.DELETE("/:name", gin2.IsHavePermission([]string{permission.SuperUserUUID}, permission.ParamType), gin2.UpdateOperationLogStatus, DeleteProductTemplate)
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

	project := router.Group("projects")
	{
		project.GET("", ListProjects)
	}
}
