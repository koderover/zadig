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

	Agent := router.Group("agent")
	{
		Agent.GET("/:id/agent.yaml", GetClusterYaml("/api/hub"))
		Agent.GET("/:id/upgrade", UpgradeAgent)
	}

	Cluster := router.Group("clusters")
	{
		Cluster.GET("", ListClusters)
		Cluster.GET("/:id", GetCluster)

		Cluster.POST("", CreateCluster)
		Cluster.PUT("/:id", UpdateCluster)
		Cluster.PUT("/:id/strategy", AddOrUpdateClusterStrategy)
		Cluster.DELETE("/:id/strategy", DeleteClusterStrategy)
		Cluster.PUT("/:id/cache", UpdateClusterCache)
		Cluster.PUT("/:id/storage", UpdateClusterStorage)
		Cluster.PUT("/:id/dind", UpdateClusterDind)
		Cluster.GET("/:id/deletion", GetDeletionInfo)
		Cluster.DELETE("/:id", DeleteCluster)
		Cluster.GET("/:id/strategy/references", GetClusterStrategyReferences)
		Cluster.PUT("/:id/disconnect", DisconnectCluster)
		Cluster.PUT("/:id/reconnect", ReconnectCluster)
		Cluster.POST("/validate", ValidateCluster)

		Cluster.GET("/irsa", GetIRSAInfo)
	}

	istio := router.Group("istio")
	{
		istio.GET("/check/:id", CheckIstiod)
	}

	router.GET("/:id/storageclasses", ListStorageClasses)
	router.GET("/:id/:namespace/pvcs", ListPVCs)
	router.GET("/:id/:namespace/deployments", ListDeployments)
	router.GET("/:id/:namespace/istio/virtualservices", ListIstioVirtualServices)

	router.GET("/check/ephemeralcontainers", CheckEphemeralContainers)
}

type OpenAPIRouter struct{}

func (*OpenAPIRouter) Inject(router *gin.RouterGroup) {
	istio := router.Group("istio")
	{
		istio.GET("/check/:id", OpenAPICheckIstiod)
	}
}
