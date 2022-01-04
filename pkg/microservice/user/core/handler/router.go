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
	users := router.Group("")
	{
		users.GET("/callback", login.Callback)

		users.POST("/users", user.CreateUser)

		users.PUT("/users/:uid/password", user.UpdatePassword)

		users.PUT("/users/:uid", user.UpdateUser)

		users.PUT("/users/:uid/personal", user.UpdatePersonalUser)

		users.GET("/users/:uid", user.GetUser)

		users.DELETE("/users/:uid", user.DeleteUser)

		users.GET("/users/:uid/personal", user.GetPersonalUser)

		users.POST("/users/search", user.ListUsers)

		users.POST("/users/ldap/:ldapId", user.SyncLdapUser)

		router.GET("login", login.Login)

		router.GET("login-enabled", login.ThirdPartyLoginEnabled)

		router.POST("login", login.LocalLogin)

		router.POST("signup", user.SignUp)

		router.GET("retrieve", user.Retrieve)

		router.POST("reset", user.Reset)

		router.GET("/healthz", Healthz)
	}
}
