/*
Copyright 2023 The KodeRover Authors.

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

import "github.com/gin-gonic/gin"

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	agent := router.Group("")
	{
		agent.POST("", CreateAgent)
		agent.PUT("/agent/:agentID", UpdateAgent)
		agent.DELETE("/agent/:agentID", DeleteAgent)
		agent.GET("", ListAgents)
		agent.GET("/agent/:agentID", GetAgent)
	}

	hostAgent := router.Group("agents")
	{
		hostAgent.POST("/register", RegisterAgent)
		hostAgent.POST("/verify", VerifyAgent)
		hostAgent.GET("/ping", PingAgent)
	}
}
