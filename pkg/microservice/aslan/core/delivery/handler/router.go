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

	deliveryArtifact := router.Group("artifacts")
	{
		deliveryArtifact.GET("", ListDeliveryArtifacts)
		deliveryArtifact.GET("/:id", GetDeliveryArtifact)
		deliveryArtifact.GET("/image", GetDeliveryArtifactIDByImage)
		deliveryArtifact.POST("", CreateDeliveryArtifacts)
		deliveryArtifact.POST("/:id", UpdateDeliveryArtifact)
		deliveryArtifact.POST("/:id/activities", CreateDeliveryActivities)
	}

	deliveryProduct := router.Group("products")
	{
		deliveryProduct.GET("/:releaseId", GetProductByDeliveryInfo)
	}

	deliveryRelease := router.Group("releases")
	{
		deliveryRelease.GET("/:id", GetDeliveryVersion)
		deliveryRelease.GET("", ListDeliveryVersion)
		deliveryRelease.DELETE("/:id", GetProductNameByDelivery, DeleteDeliveryVersion)
		deliveryRelease.POST("/helm", CreateHelmDeliveryVersion)
		deliveryRelease.POST("/helm/global-variables", ApplyDeliveryGlobalVariables)
		deliveryRelease.GET("/helm/charts", DownloadDeliveryChart)
		deliveryRelease.GET("/helm/charts/version", GetChartVersionFromRepo)
		deliveryRelease.GET("/helm/charts/preview", PreviewGetDeliveryChart)
		deliveryRelease.GET("/helm/charts/filePath", GetDeliveryChartFilePath)
		deliveryRelease.GET("/helm/charts/fileContent", GetDeliveryChartFileContent)
	}

	deliveryPackage := router.Group("packages")
	{
		deliveryPackage.GET("", ListPackagesVersion)
	}

	deliveryService := router.Group("servicenames")
	{
		deliveryService.GET("", ListDeliveryServiceNames)
	}

	deliverySecurity := router.Group("security")
	{
		deliverySecurity.GET("/stats", ListDeliverySecurityStatistics)
		deliverySecurity.GET("", ListDeliverySecurity)
		deliverySecurity.POST("", CreateDeliverySecurity)
	}
}
