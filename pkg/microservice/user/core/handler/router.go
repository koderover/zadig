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
	"github.com/koderover/zadig/pkg/microservice/user/core/handler/login"
	"github.com/koderover/zadig/pkg/microservice/user/core/handler/user"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	users := router.Group("/")
	{
		users.GET("login", login.Login)

		users.GET("api/v1/callback", login.Callback)

		users.POST("login", login.InternalLogin)

		users.POST("api/v1/users", user.CreateUser)

		users.PUT("api/v1/users/:uid/password", user.UpdatePassword)

		users.GET("api/v1/users/:uid", user.GetUser)

		users.GET("api/v1/users", user.GetUsers)
	}
}
