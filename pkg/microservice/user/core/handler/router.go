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
	"github.com/koderover/zadig/pkg/microservice/user/core/handler/permission"
	"github.com/koderover/zadig/pkg/microservice/user/core/handler/user"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	users := router.Group("/users")
	{
		users.POST("", user.CreateUser)
		users.POST("/:uid/password", user.UpdatePassword)
		users.PUT("/:uid", user.UpdateUser)
		users.PUT("/:uid/personal", user.UpdatePersonalUser)
		users.PUT("/:uid/setting", user.UpdateUserSetting)
		users.GET("/:uid", user.GetUser)
		users.DELETE("/:uid", user.DeleteUser)
		users.GET("/:uid/personal", user.GetPersonalUser)
		users.GET("/:uid/setting", user.GetUserSetting)
		users.POST("/search", user.ListUsers)
		users.GET("/count", user.CountSystemUsers)
	}

	usergroups := router.Group("user-group")
	{
		// user group related apis
		usergroups.GET("", user.ListUserGroups)
		usergroups.POST("", user.CreateUserGroup)
		usergroups.GET("/:id", user.GetUserGroup)
		usergroups.PUT("/:id", user.UpdateUserGroupInfo)
		usergroups.DELETE("/:id", user.DeleteUserGroup)

		usergroups.PUT("/:id/bulk-users", user.BulkAddUserToUserGroup)
		usergroups.DELETE("/:id/bulk-users", user.BulkRemoveUserFromUserGroup)
	}

	// =======================================================
	// User Authorization APIs, internal use ONLY
	// =======================================================
	authz := router.Group("/authorization")
	{
		authz.GET("/auth-info", user.GetUserAuthInfo)
		authz.GET("/collaboration-permission", user.CheckCollaborationModePermission)
		authz.GET("/collaboration-action", user.CheckPermissionGivenByCollaborationMode)
		authz.GET("/authorized-projects", user.ListAuthorizedProject)
		authz.GET("/authorized-projects/verb", user.ListAuthorizedProjectByVerb)
		authz.GET("/authorized-workflows", user.ListAuthorizedWorkflows)
		authz.GET("/authorized-envs", user.ListAuthorizedEnvs)
	}

	// general login related actions
	general := router.Group("")
	{
		general.GET("/callback", login.Callback)
		general.GET("/login", login.Login)
		general.POST("/login", login.LocalLogin)
		general.GET("/login-enabled", login.ThirdPartyLoginEnabled)
		general.GET("/captcha", login.GetCaptcha)
		general.GET("/logout", login.LocalLogout)
		general.POST("/signup", user.SignUp)
		general.GET("/retrieve", user.Retrieve)
		general.POST("/reset", user.Reset)
		general.GET("/healthz", Healthz)
	}

	policy := router.Group("/policy")
	{
		roles := policy.Group("/roles")
		{
			roles.POST("", permission.CreateRole)
			roles.PUT("/:name", permission.UpdateRole)
			roles.GET("", permission.ListRoles)
			roles.GET("/:name", permission.GetRole)
			roles.DELETE("/:name", permission.DeleteRole)
		}

		roleBindings := policy.Group("/role-bindings")
		{
			roleBindings.GET("", permission.ListRoleBindings)
			roleBindings.POST("", permission.CreateRoleBinding)
			roleBindings.POST("/user/:uid", permission.UpdateRoleBindingForUser)
			roleBindings.DELETE("/user/:uid", permission.DeleteRoleBindingForUser)
			roleBindings.POST("/group/:gid", permission.UpdateRoleBindingForGroup)
			roleBindings.DELETE("/group/:gid", permission.DeleteRoleBindingForGroup)
		}

		resourceAction := policy.Group("resource-actions")
		{
			resourceAction.GET("", permission.GetResourceActionDefinitions)
		}

		policyUserPermission := policy.Group("permission")
		{
			policyUserPermission.GET("project/:name", permission.GetUserRulesByProject)
			policyUserPermission.GET("", permission.GetUserRules)
		}

		internalPolicyApis := policy.Group("internal")
		{
			internalPolicyApis.POST("deleteProjectRole", permission.DeleteProjectRoles)
		}
	}

	bundles := router.Group("bundles")
	{
		bundles.GET("/:name", permission.DownloadBundle)
	}
}
