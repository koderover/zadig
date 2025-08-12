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

	"github.com/koderover/zadig/v2/pkg/microservice/user/core/handler/login"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/handler/permission"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/handler/user"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	users := router.Group("/users")
	{
		users.POST("", user.CreateUser)
		users.PUT("/:uid/password", user.UpdatePassword)
		users.PUT("/:uid", user.UpdateUser)
		users.PUT("/:uid/personal", user.UpdatePersonalUser)
		users.PUT("/:uid/setting", user.UpdateUserSetting)
		users.GET("/:uid", user.GetUser)
		users.DELETE("/:uid", user.DeleteUser)
		users.GET("/:uid/personal", user.GetPersonalUser)
		users.GET("/:uid/setting", user.GetUserSetting)
		users.POST("/brief", user.ListUsersBrief)
		users.POST("/search", user.ListUsers)
		users.GET("/count", user.CountSystemUsers)
		users.GET("/check/duplicate", user.CheckDuplicateUser)
	}

	usergroups := router.Group("user-group")
	{
		// user group related apis
		usergroups.GET("", user.ListUserGroups)
		usergroups.POST("", user.CreateUserGroup)
		usergroups.GET("/:id", user.GetUserGroup)
		usergroups.PUT("/:id", user.UpdateUserGroupInfo)
		usergroups.DELETE("/:id", user.DeleteUserGroup)

		usergroups.POST("/:id/bulk-create-users", user.BulkAddUserToUserGroup)
		usergroups.POST("/:id/bulk-delete-users", user.BulkRemoveUserFromUserGroup)
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

		roleTemplates := policy.Group("/role-templates")
		{
			roleTemplates.POST("", permission.CreateRoleTemplate)
			roleTemplates.PUT("/:name", permission.UpdateRoleTemplate)
			roleTemplates.GET("", permission.ListRoleTemplates)
			roleTemplates.GET("/:name", permission.GetRoleTemplate)
			roleTemplates.DELETE("/:name", permission.DeleteRoleTemplate)
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
			internalPolicyApis.POST("initializeProject", permission.InitializeProject)
			internalPolicyApis.POST("deleteProjectRole", permission.DeleteProjectRoles)
			internalPolicyApis.POST("setProjectVisibility", permission.SetProjectVisibility)
		}
	}
}

type OpenAPIRouter struct{}

func (*OpenAPIRouter) Inject(router *gin.RouterGroup) {
	users := router.Group("users")
	{
		users.GET("", user.OpenAPIListUsersBrief)
		users.GET("/:uid", user.OpenAPIGetUser)
		users.DELETE("/:uid", user.OpenAPIDeleteUser)
	}

	usergroups := router.Group("user-groups")
	{
		usergroups.GET("", user.OpenApiListUserGroups)
	}

	policy := router.Group("policy")
	{
		roles := policy.Group("/roles")
		{
			roles.POST("", permission.OpenAPICreateRole)
			roles.PUT("/:name", permission.OpenAPIUpdateRole)
			roles.GET("", permission.OpenAPIListRoles)
			roles.GET("/:name", permission.OpenAPIGetRole)
			roles.DELETE("/:name", permission.OpenAPIDeleteRole)
		}

		roleBindings := policy.Group("/role-bindings")
		{
			roleBindings.GET("", permission.OpenAPIListRoleBindings)
			roleBindings.POST("", permission.OpenAPICreateRoleBinding)
			roleBindings.POST("/user/:uid", permission.OpenAPIUpdateRoleBindingForUser)
			roleBindings.DELETE("/user/:uid", permission.OpenAPIDeleteRoleBindingForUser)
			roleBindings.POST("/group/:gid", permission.OpenAPIUpdateRoleBindingForGroup)
			roleBindings.DELETE("/group/:gid", permission.OpenAPIDeleteRoleBindingForGroup)
		}

		resourceAction := policy.Group("resource-actions")
		{
			resourceAction.GET("", permission.OpenAPIGetResourceActionDefinitions)
		}
	}
}
