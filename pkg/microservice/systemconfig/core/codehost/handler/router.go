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
	codehost := router.Group("codehosts")
	{
		codehost.GET("/callback", Callback)
		codehost.GET("", ListSystemCodeHost)
		codehost.GET("/internal", ListCodeHostInternal)
		codehost.DELETE("/:id", DeleteSystemCodeHost)
		codehost.POST("", CreateSystemCodeHost)
		codehost.PATCH("/:id", UpdateSystemCodeHost)
		codehost.GET("/:id", GetSystemCodeHost)
		codehost.GET("/:id/auth", AuthCodeHost)
		codehost.POST("/validate", ValidateCodeHost)
	}
}
