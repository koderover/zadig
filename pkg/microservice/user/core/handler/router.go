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

		users.PUT("/users/:uid/setting", user.UpdateUserSetting)

		users.GET("/users/:uid", user.GetUser)

		users.DELETE("/users/:uid", user.DeleteUser)

		users.GET("/users/:uid/personal", user.GetPersonalUser)

		users.GET("/users/:uid/setting", user.GetUserSetting)

		users.POST("/users/search", user.ListUsers)

		users.GET("/user/count", user.CountSystemUsers)

		router.GET("login", login.Login)

		router.GET("login-enabled", login.ThirdPartyLoginEnabled)

		router.POST("login", login.LocalLogin)

		router.GET("captcha", login.GetCaptcha)

		router.GET("logout", login.LocalLogout)

		router.POST("signup", user.SignUp)

		router.GET("retrieve", user.Retrieve)

		router.POST("reset", user.Reset)

		router.GET("/healthz", Healthz)

		router.GET("/auth-info", user.GetUserAuthInfo)

		router.GET("/collaboration-permission", user.CheckCollaborationModePermission)

		router.GET("/collaboration-action", user.CheckPermissionGivenByCollaborationMode)

		router.GET("/authorized-projects", user.ListAuthorizedProject)

		router.GET("/authorized-projects/verb", user.ListAuthorizedProjectByVerb)

		router.GET("/authorized-workflows", user.ListAuthorizedWorkflows)

		router.GET("/authorized-envs", user.ListAuthorizedEnvs)
	}

	{
		users.GET("/user-group", user.ListUserGroups)
		users.POST("/user-group", user.CreateUserGroup)
		users.GET("/user-group/:id", user.GetUserGroup)
		users.PUT("/user-group/:id", user.UpdateUserGroupInfo)
		users.DELETE("/user-group/:id", user.DeleteUserGroup)

		users.PUT("/user-group/:id/bulk-users", user.BulkAddUserToUserGroup)
		users.DELETE("/user-group/:id/bulk-users", user.BulkRemoveUserFromUserGroup)
	}
}
