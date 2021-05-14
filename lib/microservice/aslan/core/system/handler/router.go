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

	"github.com/koderover/zadig/lib/microservice/aslan/middleware"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	proxy := router.Group("proxy")
	{
		proxy.GET("/config", GetProxyConfig)
	}

	router.Use(middleware.Auth())

	// ---------------------------------------------------------------------------------------
	// 安装脚本管理接口
	// ---------------------------------------------------------------------------------------
	install := router.Group("install")
	{
		install.POST("", middleware.RequireSuperAdminAuth, middleware.UpdateOperationLogStatus, CreateInstall)
		install.PUT("", middleware.RequireSuperAdminAuth, middleware.UpdateOperationLogStatus, UpdateInstall)
		install.GET("/:name/:version", GetInstall)
		install.GET("", ListInstalls)
		install.PUT("/delete", middleware.RequireSuperAdminAuth, middleware.UpdateOperationLogStatus, DeleteInstall)
	}

	// ---------------------------------------------------------------------------------------
	// 代理管理接口
	// ---------------------------------------------------------------------------------------
	proxyManage := router.Group("proxyManage")
	{
		proxyManage.GET("", ListProxies)
		proxyManage.GET("/:id", GetProxy)
		proxyManage.POST("", middleware.RequireSuperAdminAuth, middleware.UpdateOperationLogStatus, CreateProxy)
		proxyManage.PUT("/:id", middleware.RequireSuperAdminAuth, middleware.UpdateOperationLogStatus, UpdateProxy)
		proxyManage.DELETE("/:id", middleware.RequireSuperAdminAuth, middleware.UpdateOperationLogStatus, DeleteProxy)

		proxyManage.POST("/connectionTest", TestConnection)
	}

	registry := router.Group("registry")
	{
		registry.GET("", ListRegistries)
		registry.GET("/namespaces", middleware.RequireSuperAdminAuth, ListRegistryNamespaces)
		registry.POST("/namespaces", middleware.RequireSuperAdminAuth, middleware.UpdateOperationLogStatus, CreateRegistryNamespace)
		registry.PUT("/namespaces/:id", middleware.RequireSuperAdminAuth, middleware.UpdateOperationLogStatus, UpdateRegistryNamespace)

		registry.DELETE("/namespaces/:id", middleware.RequireSuperAdminAuth, middleware.UpdateOperationLogStatus, DeleteRegistryNamespace)
		registry.GET("/release/repos", ListAllRepos)
		registry.POST("/images", ListImages)
		registry.GET("/images/repos/:name", ListRepoImages)
	}

	s3storage := router.Group("s3storage")
	{
		s3storage.GET("", ListS3Storage)
		s3storage.POST("", middleware.RequireSuperAdminAuth, middleware.UpdateOperationLogStatus, CreateS3Storage)
		s3storage.GET("/:id", GetS3Storage)
		s3storage.PUT("/:id", middleware.RequireSuperAdminAuth, middleware.UpdateOperationLogStatus, UpdateS3Storage)
		s3storage.DELETE("/:id", middleware.RequireSuperAdminAuth, middleware.UpdateOperationLogStatus, DeleteS3Storage)
	}

	//系统清理缓存
	cleanCache := router.Group("cleanCache")
	{
		cleanCache.POST("/oneClick", CleanImageCache)
		cleanCache.GET("/state", CleanCacheState)
	}

	// ---------------------------------------------------------------------------------------
	// Github管理接口
	// ---------------------------------------------------------------------------------------
	github := router.Group("githubApp")
	{
		github.GET("", GetGithubApp)
		github.POST("", middleware.RequireSuperAdminAuth, CreateGithubApp)
		github.DELETE("/:id", middleware.RequireSuperAdminAuth, DeleteGithubApp)
	}

	// ---------------------------------------------------------------------------------------
	// jenkins集成接口以及jobs和buildWithParameters接口
	// ---------------------------------------------------------------------------------------
	jenkins := router.Group("jenkins")
	{
		jenkins.POST("/integration", middleware.RequireSuperAdminAuth, CreateJenkinsIntegration)
		jenkins.GET("/integration", ListJenkinsIntegration)
		jenkins.PUT("/integration/:id", middleware.RequireSuperAdminAuth, UpdateJenkinsIntegration)
		jenkins.DELETE("/integration/:id", middleware.RequireSuperAdminAuth, DeleteJenkinsIntegration)
		jenkins.POST("/user/connection", middleware.RequireSuperAdminAuth, TestJenkinsConnection)
		jenkins.GET("/jobNames", middleware.RequireSuperAdminAuth, ListJobNames)
		jenkins.GET("/buildArgs/:jobName", middleware.RequireSuperAdminAuth, ListJobBuildArgs)
	}

	// ---------------------------------------------------------------------------------------
	// 自定义镜像管理接口
	// ---------------------------------------------------------------------------------------
	basicImages := router.Group("basicImages")
	{
		basicImages.GET("", ListBasicImages)
		basicImages.GET("/:id", GetBasicImage)
	}

}
