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

		product.GET("/:name/globalVariables", GetGlobalVariables)
		product.PUT("/:name/globalVariables", UpdateGlobalVariables)
		product.GET("/:name/globalVariableCandidates", GetGlobalVariableCandidates)

		product.GET("/:name/productionGlobalVariables", GetProductionGlobalVariables)
		product.PUT("/:name/productionGlobalVariables", UpdateProductionGlobalVariables)
		product.GET("/:name/productionGlobalVariableCandidates", GetProductionGlobalVariableCandidates)
	}

	group := router.Group("group")
	{
		group.POST("", CreateProjectGroup)
		group.PUT("", UpdateProjectGroup)
		group.DELETE("", DeleteProjectGroup)
		group.GET("", ListProjectGroups)
		group.GET("/preset", GetPresetProjectGroup)
	}

	bizdir := router.Group("bizdir")
	{
		bizdir.GET("", GetBizDirProject)
		bizdir.GET("/services", GetBizDirProjectServices)
		bizdir.GET("/service/detail", GetBizDirServiceDetail)
		bizdir.GET("/search/project", SearchBizDirByProject)
		bizdir.GET("/search/service", SearchBizDirByService)
	}

	production := router.Group("production/products")
	{
		production.PATCH("/:name", UpdateProductionServiceOrchestration)
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
		pms.GET("/:id/agent/access", GetAgentAccessCmd)
		pms.PUT("/:id/agent/offline", OfflineVM)
		pms.PUT("/:id/agent/recovery", RecoveryVM)
		pms.PUT("/:id/agent/upgrade", UpgradeAgent)
	}

	variables := router.Group("variablesets")
	{
		variables.GET("", ListVariableSets)
		variables.GET("/:id", GetVariableSet)
		variables.POST("", CreateVariableSet)
		variables.PUT("/:id", UpdateVariableSet)
		variables.DELETE("/:id", DeleteVariableSet)
	}

	integration := router.Group("integration")
	{
		codehost := integration.Group(":name/codehosts")
		{
			codehost.GET("", ListProjectCodeHost)
			codehost.DELETE("/:id", DeleteCodeHost)
			codehost.POST("", CreateProjectCodeHost)
			codehost.PATCH("/:id", UpdateProjectCodeHost)
			codehost.GET("/:id", GetProjectCodeHost)
			codehost.GET("/available", ListAvailableCodeHost)
		}
	}

}

type OpenAPIRouter struct{}

func (*OpenAPIRouter) Inject(router *gin.RouterGroup) {
	product := router.Group("project")
	{
		product.POST("", OpenAPICreateProductTemplate)
		product.POST("/init/yaml", OpenAPIInitializeYamlProject)
		product.POST("/init/helm", OpenAPIInitializeHelmProject)
		product.GET("", OpenAPIListProject)
		product.GET("/detail", OpenAPIGetProjectDetail)
		product.DELETE("", OpenAPIDeleteProject)
		product.GET("/globalVariable", OpenAPIGetGlobalVariables)
	}
}
