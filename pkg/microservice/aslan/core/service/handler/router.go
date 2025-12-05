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
		helm.PUT("/:serviceName/file", UpdateFileContent)
		helm.POST("/services", CreateOrUpdateHelmService)
		helm.PUT("/services", UpdateHelmService)
		helm.POST("/services/bulk", CreateOrUpdateBulkHelmServices)
		helm.PUT("/services/releaseNaming", HelmReleaseNaming)
	}

	k8s := router.Group("services")
	{
		k8s.GET("", ListServiceTemplate)
		k8s.GET("/:name/:type", GetServiceTemplate)
		k8s.GET("/:name", GetServiceTemplateOption)
		k8s.POST("", GetServiceTemplateProductName, CreateServiceTemplate)
		k8s.PUT("/:name/variable", UpdateServiceVariable)
		k8s.PUT("", UpdateServiceTemplate)
		k8s.PUT("/yaml/validator", YamlValidator)
		k8s.DELETE("/:name/:type", DeleteServiceTemplate)
		k8s.GET("/:name/environments/deployable", GetDeployableEnvs)
		k8s.POST("/variable/convert", ConvertVaraibleKVAndYaml)

		k8s.GET("/kube/workloads", GetKubeWorkloads)
		k8s.POST("/yaml", LoadKubeWorkloadsYaml)
	}

	labels := router.Group("labels")
	{
		// service labeling
		labels.POST("", AddServiceLabel)
		labels.PUT("/:id", GetLabelSourceServiceInfo, UpdateServiceLabel)
		labels.DELETE("/:id", GetLabelSourceServiceInfo, DeleteServiceLabel)
		labels.GET("", ListServiceLabels)
	}

	// host env and service api
	workload := router.Group("workloads")
	{
		workload.POST("", CreateK8sWorkloads)
		workload.GET("", ListWorkloadTemplate)
		workload.PUT("", UpdateWorkloads)
	}

	loader := router.Group("loader")
	{
		loader.POST("/preload/:codehostId", PreloadServiceTemplate)
		loader.POST("/load/:codehostId", LoadServiceTemplate)
		loader.PUT("/load/:codehostId", SyncServiceTemplate)
	}

	pm := router.Group("pm")
	{
		pm.PUT("/healthCheckUpdate", UpdateServiceHealthCheckStatus)
		pm.PUT("/healthCheckEnvUpdate", UpdateEnvVMServiceHealthCheck)
		pm.POST("/:productName", CreatePMService)
		pm.PUT("/:productName", UpdatePmServiceTemplate)
	}

	template := router.Group("template")
	{
		template.POST("/load", LoadServiceFromYamlTemplate)
		template.POST("/reload", ReloadServiceFromYamlTemplate)
		template.POST("/preview", PreviewServiceYamlFromYamlTemplate)
	}

	version := router.Group("version")
	{
		version.GET("/:serviceName", ListServiceVersions)
		version.GET("/:serviceName/diff", DiffServiceVersions)
		version.GET("/:serviceName/revision/:revision", GetServiceVersionYaml)
		version.POST("/:serviceName/rollback", RollbackServiceVersion)
	}
}

type OpenAPIRouter struct{}

func (*OpenAPIRouter) Inject(router *gin.RouterGroup) {
	template := router.Group("template")
	{
		template.POST("/load/yaml", LoadServiceFromYamlTemplateOpenAPI)
		template.POST("/production/load/yaml", LoadProductionServiceFromYamlTemplateOpenAPI)
	}

	yaml := router.Group("yaml")
	{
		yaml.POST("/raw", CreateRawYamlServicesOpenAPI)
		yaml.POST("/production/raw", CreateRawProductionYamlServicesOpenAPI)
		yaml.DELETE("/:name", DeleteYamlServicesOpenAPI)
		yaml.DELETE("production/:name", DeleteProductionServicesOpenAPI)
		yaml.GET("/services", ListYamlServicesOpenAPI)
		yaml.GET("/production/services", ListProductionYamlServicesOpenAPI)
		yaml.GET("/:name", GetYamlServiceOpenAPI)
		yaml.GET("/:name/labels", GetYamlServiceLabelOpenAPI)
		yaml.GET("/production/:name", GetProductionYamlServiceOpenAPI)
		yaml.PUT("/:name", UpdateServiceConfigOpenAPI)
		yaml.PUT("/production/:name", UpdateProductionServiceConfigOpenAPI)
		yaml.PUT("/:name/variable", UpdateServiceVariableOpenAPI)
		yaml.PUT("/production/:name/variable", UpdateProductionServiceVariableOpenAPI)
	}
}
