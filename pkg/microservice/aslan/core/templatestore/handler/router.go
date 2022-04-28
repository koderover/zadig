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
	// chart templates
	// ---------------------------------------------------------------------------------------
	install := router.Group("charts")
	{
		install.GET("", ListChartTemplates)
		install.GET("/:name", GetChartTemplate)
		install.GET("/:name/files", ListFiles)
		install.GET("/:name/variables", GetTemplateVariables)
		install.POST("", AddChartTemplate)
		install.PUT("/:name", UpdateChartTemplate)
		install.PUT("/:name/variables", UpdateChartTemplateVariables)
		install.DELETE("/:name", RemoveChartTemplate)
	}

	dockerfile := router.Group("dockerfile")
	{
		dockerfile.POST("", CreateDockerfileTemplate)
		dockerfile.PUT("/:id", UpdateDockerfileTemplate)
		dockerfile.GET("", ListDockerfileTemplate)
		dockerfile.GET("/:id", GetDockerfileTemplateDetail)
		dockerfile.DELETE("/:id", DeleteDockerfileTemplate)
		dockerfile.GET("/:id/reference", GetDockerfileTemplateReference)
		dockerfile.POST("/validation", ValidateDockerfileTemplate)
	}

	yaml := router.Group("yaml")
	{
		yaml.POST("", CreateYamlTemplate)
		yaml.PUT("/:id", UpdateYamlTemplate)
		yaml.GET("", ListYamlTemplate)
		yaml.GET("/:id", GetYamlTemplateDetail)
		yaml.DELETE("/:id", DeleteYamlTemplate)
		yaml.GET("/:id/reference", GetYamlTemplateReference)
		yaml.POST("/getVariables", GetYamlTemplateVariables)
	}

	build := router.Group("build")
	{
		build.POST("", AddBuildTemplate)
		build.PUT("/:id", UpdateBuildTemplate)
		build.GET("", ListBuildTemplates)
		build.GET("/:id", GetBuildTemplate)
		build.DELETE("/:id", RemoveBuildTemplate)
	}
}
