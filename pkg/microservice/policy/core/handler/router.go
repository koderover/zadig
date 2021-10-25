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
		roles.PUT("/:name", UpdateRole)
		roles.GET("", ListRoles)
		roles.DELETE("/:name", DeleteRole)
	}

	publicRoles := router.Group("public-roles")
	{
		publicRoles.POST("", CreatePublicRole)
		publicRoles.GET("", ListPublicRoles)
		publicRoles.PUT("/:name", UpdatePublicRole)
		publicRoles.DELETE("/:name", DeletePublicRole)
	}

	systemRoles := router.Group("system-roles")
	{
		systemRoles.POST("", CreateSystemRole)
		systemRoles.GET("", ListSystemRoles)
		systemRoles.DELETE("/:name", DeleteSystemRole)
	}

	roleBindings := router.Group("rolebindings")
	{
		roleBindings.POST("", CreateRoleBinding)
		roleBindings.GET("", ListRoleBindings)
		roleBindings.DELETE("/:name", DeleteRoleBinding)
	}

	systemRoleBindings := router.Group("system-rolebindings")
	{
		systemRoleBindings.POST("", CreateSystemRoleBinding)
		systemRoleBindings.GET("", ListSystemRoleBindings)
		systemRoleBindings.DELETE("/:name", DeleteSystemRoleBinding)
	}

	bundles := router.Group("bundles")
	{
		bundles.GET("/:name", DownloadBundle)
	}

	policyRegistrations := router.Group("policies")
	{
		policyRegistrations.PUT("/:resourceName", CreateOrUpdatePolicyRegistration)
	}

	policyDefinitions := router.Group("policy-definitions")
	{
		policyDefinitions.GET("", GetPolicyRegistrationDefinitions)
	}

}
