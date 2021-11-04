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

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/handler"
	handler2 "github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/handler"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	roles := router.Group("connectors")
	{
		roles.POST("", CreateConnector)
		roles.GET("", ListConnectors)
		roles.GET("/:id", GetConnector)
		roles.PUT("/:id", UpdateConnector)
		roles.DELETE("/:id", DeleteConnector)
	}
	features := router.Group("features")
	{
		features.GET("/:name", GetFeature)
	}
	emails := router.Group("emails")
	{
		emails.GET("/host", handler.GetEmailHost)
		emails.POST("/host", handler.CreateEmailHost)
		emails.PATCH("/host", handler.UpdateEmailHost)
		emails.DELETE("/host", handler.DeleteEmailHost)

		emails.GET("/service", handler.GetEmailService)
		emails.POST("/service", handler.CreateEmailService)
		emails.PATCH("/service", handler.UpdateEmailService)
		emails.DELETE("/service", handler.DeleteEmailService)
	}

	jira := router.Group("jira")
	{
		jira.GET("", handler2.GetJira)
		jira.POST("", handler2.CreateJira)
		jira.PATCH("", handler2.UpdateJira)
		jira.DELETE("", handler2.DeleteJira)
	}
	codehost := router.Group("codehost")
	{
		codehost.POST("", ListCodehost)
	}
}
