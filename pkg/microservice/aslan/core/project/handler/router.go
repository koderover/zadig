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
		product.GET("/:name/searching-rules", GetCustomMatchRules)
		product.PUT("/:name/searching-rules", CreateOrUpdateMatchRules)
		product.POST("", CreateProductTemplate)
		product.PUT("/:name", UpdateProductTemplate)
		product.PUT("/:name/:status", UpdateProductTmplStatus)
		product.PATCH("/:name", UpdateServiceOrchestration)
		product.PUT("", UpdateProject)
		product.PUT("/:name/type", TransferProject)
		product.DELETE("/:name", DeleteProductTemplate)
	}

	production := router.Group("production/products")
	{
		production.PATCH("/:name", UpdateProductionServiceOrchestration)
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

	pms := router.Group("pms")
	{
		pms.GET("", ListPMHosts)
		pms.GET("/:id", GetPMHost)
		pms.GET("/labels", ListLabels)
		pms.POST("", CreatePMHost)
		pms.POST("/batch", BatchCreatePMHost)
		pms.PUT("/:id", UpdatePMHost)
		pms.DELETE("/:id", DeletePMHost)
	}

	variables := router.Group("variablesets")
	{
		variables.GET("", ListVariableSets)
		variables.GET("/:id", GetVariableSet)
		variables.POST("", CreateVariableSet)
		variables.PUT("/:id", UpdateVariableSet)
		variables.DELETE("/:id", DeleteVariableSet)
	}
}

type OpenAPIRouter struct{}

func (*OpenAPIRouter) Inject(router *gin.RouterGroup) {
	product := router.Group("project")
	{
		product.POST("", OpenAPICreateProductTemplate)
	}
}
