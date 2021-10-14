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

	harbor := router.Group("harbor")
	{
		harbor.GET("/project", ListHarborProjects)
		harbor.GET("/project/:project/charts", ListHarborChartRepos)
		harbor.GET("/project/:project/chart/:chart/versions", ListHarborChartVersions)
		harbor.GET("/project/:project/chart/:chart/version/:version", FindHarborChartDetail)
	}

	helm := router.Group("helm")
	{
		helm.GET("/:productName", ListHelmServices)
		helm.GET("/:productName/:serviceName/serviceModule", GetHelmServiceModule)
		helm.GET("/:productName/:serviceName/filePath", GetFilePath)
		helm.GET("/:productName/:serviceName/fileContent", GetFileContent)
		helm.POST("/services", gin2.IsHavePermission([]string{permission.ServiceTemplateManageUUID}, permission.ParamType), CreateOrUpdateHelmService)
		helm.PUT("/:productName", gin2.IsHavePermission([]string{permission.ServiceTemplateManageUUID}, permission.ParamType), UpdateHelmService)
	}

	k8s := router.Group("services")
	{
		k8s.GET("", ListServiceTemplate)
		k8s.GET("/:name/:type", GetServiceTemplate)
		k8s.GET("/:name", GetServiceTemplateOption)
		k8s.POST("", GetServiceTemplateProductName, gin2.IsHavePermission([]string{permission.ServiceTemplateManageUUID}, permission.ContextKeyType), gin2.UpdateOperationLogStatus, CreateServiceTemplate)
		k8s.PUT("", GetServiceTemplateObjectProductName, gin2.IsHavePermission([]string{permission.ServiceTemplateManageUUID}, permission.ContextKeyType), gin2.UpdateOperationLogStatus, UpdateServiceTemplate)
		k8s.PUT("/yaml/validator", YamlValidator)
		k8s.DELETE("/:name/:type", gin2.IsHavePermission([]string{permission.ServiceTemplateDeleteUUID}, permission.QueryType), gin2.UpdateOperationLogStatus, DeleteServiceTemplate)
		k8s.GET("/:name/:type/ports", ListServicePort)
	}

	workload := router.Group("workloads")
	{
		workload.POST("", CreateK8sWorkloads)
		workload.GET("", ListWorkloadTemplate)
		workload.PUT("", UpdateWorkloads)
	}

	name := router.Group("name")
	{
		name.GET("", ListAvailablePublicServices)
	}

	loader := router.Group("loader")
	{
		loader.GET("/preload/:codehostId/:branchName", PreloadServiceTemplate)
		loader.POST("/load/:codehostId/:branchName", LoadServiceTemplate)
		loader.GET("/validateUpdate/:codehostId/:branchName", ValidateServiceUpdate)
	}

	pm := router.Group("pm")
	{
		pm.POST("/:productName", gin2.IsHavePermission([]string{permission.ServiceTemplateManageUUID}, permission.ParamType), gin2.UpdateOperationLogStatus, CreatePMService)
		pm.PUT("/:productName", gin2.IsHavePermission([]string{permission.ServiceTemplateManageUUID}, permission.ParamType), gin2.UpdateOperationLogStatus, UpdatePmServiceTemplate)
	}
}
