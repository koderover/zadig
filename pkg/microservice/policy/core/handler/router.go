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
	roles := router.Group("roles")
	{
		roles.POST("", CreateRole)
		roles.POST("/bulk-delete", DeleteRoles)
		roles.PATCH("/:name", UpdateRole)
		roles.PUT("/:name", UpdateOrCreateRole)
		roles.GET("", ListRoles)
		roles.GET("/:name", GetRole)
		roles.DELETE("/:name", DeleteRole)
	}

	policies := router.Group("policies")
	{
		policies.POST("", CreatePolicies)
		policies.POST("/bulk-delete", DeletePolicies)
		policies.PATCH("/:name", UpdatePolicy)
		policies.PUT("/:name", UpdateOrCreatePolicy)
		policies.GET("", ListPolicies)
		policies.GET("/:name", GetPolicy)
		policies.GET("/bulk", GetPolicies)
		policies.DELETE("/:name", DeletePolicy)
	}

	presetRoles := router.Group("preset-roles")
	{
		presetRoles.POST("", CreatePresetRole)
		presetRoles.GET("", ListPresetRoles)
		presetRoles.GET("/:name", GetPresetRole)
		presetRoles.PATCH("/:name", UpdatePresetRole)
		presetRoles.PUT("/:name", UpdateOrCreatePresetRole)
		presetRoles.DELETE("/:name", DeletePresetRole)
	}

	systemRoles := router.Group("system-roles")
	{
		systemRoles.POST("", CreateSystemRole)
		systemRoles.PUT("/:name", UpdateOrCreateSystemRole)
		systemRoles.GET("", ListSystemRoles)
		systemRoles.DELETE("/:name", DeleteSystemRole)
	}

	roleBindings := router.Group("rolebindings")
	{
		roleBindings.POST("", CreateRoleBinding)
		roleBindings.PUT("/:name", UpdateRoleBinding)
		roleBindings.GET("", ListRoleBindings)
		roleBindings.DELETE("/:name", DeleteRoleBinding)
		roleBindings.POST("/bulk-delete", DeleteRoleBindings)
		roleBindings.POST("/update", UpdateRoleBindings)
	}

	policyBindings := router.Group("policybindings")
	{
		policyBindings.POST("", CreatePolicyBinding)
		policyBindings.PUT("/:name", UpdatePolicyBinding)
		policyBindings.GET("", ListPolicyBindings)
		policyBindings.DELETE("/:name", DeletePolicyBinding)
		policyBindings.POST("/bulk-delete", DeletePolicyBindings)
	}

	systemRoleBindings := router.Group("system-rolebindings")
	{
		systemRoleBindings.POST("", CreateSystemRoleBinding)
		systemRoleBindings.POST("/search", SearchSystemRoleBinding)
		systemRoleBindings.POST("/update", UpdateSystemRoleBindings)
		systemRoleBindings.GET("", ListSystemRoleBindings)
		systemRoleBindings.DELETE("/:name", DeleteSystemRoleBinding)
		systemRoleBindings.PUT("/:name", CreateOrUpdateSystemRoleBinding)
	}

	userBindings := router.Group("userbindings")
	{
		userBindings.GET("", ListUserBindings)
	}

	bundles := router.Group("bundles")
	{
		bundles.GET("/:name", DownloadBundle)
	}

	policyDefinitions := router.Group("policy-definitions")
	{
		policyDefinitions.GET("", GetPolicyRegistrationDefinitions)
	}

	policyUserPermission := router.Group("permission")
	{
		policyUserPermission.GET("project/:name", GetUserRulesByProject)
		policyUserPermission.GET("releaseworkflow", GetUserReleaseWorkflows)
		policyUserPermission.GET("testing", ListTesting)
		policyUserPermission.GET("", GetUserRules)

	}
}
