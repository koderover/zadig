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
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {

	Agent := router.Group("agent")
	{
		Agent.GET("/:id/agent.yaml", GetClusterYaml("/api/hub"))
	}

	router.Use(gin2.Auth())

	Cluster := router.Group("clusters")
	{
		Cluster.GET("", ListClusters)
		Cluster.GET("/:id", GetCluster)

		Cluster.POST("", gin2.RequireSuperAdminAuth, CreateCluster)
		Cluster.PUT("/:id", gin2.RequireSuperAdminAuth, UpdateCluster)
		Cluster.DELETE("/:id", gin2.RequireSuperAdminAuth, DeleteCluster)
		Cluster.PUT("/:id/disconnect", gin2.RequireSuperAdminAuth, DisconnectCluster)
		Cluster.PUT("/:id/reconnect", gin2.RequireSuperAdminAuth, ReconnectCluster)
	}
}
