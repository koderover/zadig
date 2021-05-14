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
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	router.Use(middleware.Auth())

	deliveryArtifact := router.Group("artifacts")
	{
		deliveryArtifact.GET("", ListDeliveryArtifacts)
		deliveryArtifact.GET("/:id", GetDeliveryArtifact)
		deliveryArtifact.POST("", CreateDeliveryArtifacts)
		deliveryArtifact.POST("/:id", UpdateDeliveryArtifact)
		deliveryArtifact.POST("/:id/activities", CreateDeliveryActivities)
	}

	deliveryProduct := router.Group("products")
	{
		deliveryProduct.GET("", ListDeliveryProduct)
	}

}
