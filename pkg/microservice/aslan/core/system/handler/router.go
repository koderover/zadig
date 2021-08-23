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

	gin2 "github.com/koderover/zadig/pkg/middleware/gin"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	proxy := router.Group("proxy")
	{
		proxy.GET("/config", GetProxyConfig)
	}

	router.Use(gin2.Auth())

	// ---------------------------------------------------------------------------------------
	// 安装脚本管理接口
	// ---------------------------------------------------------------------------------------
	install := router.Group("install")
	{
		install.POST("", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, CreateInstall)
		install.PUT("", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, UpdateInstall)
		install.GET("/:name/:version", GetInstall)
		install.GET("", ListInstalls)
		install.PUT("/delete", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, DeleteInstall)
	}

	// ---------------------------------------------------------------------------------------
	// 代理管理接口
	// ---------------------------------------------------------------------------------------
	proxyManage := router.Group("proxyManage")
	{
		proxyManage.GET("", ListProxies)
		proxyManage.GET("/:id", GetProxy)
		proxyManage.POST("", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, CreateProxy)
		proxyManage.PUT("/:id", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, UpdateProxy)
		proxyManage.DELETE("/:id", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, DeleteProxy)

		proxyManage.POST("/connectionTest", TestConnection)
	}

	registry := router.Group("registry")
	{
		registry.GET("", ListRegistries)
		// 获取默认的镜像仓库配置，用于kodespace CLI调用
		registry.GET("/namespaces/default", GetDefaultRegistryNamespace)
		registry.GET("/namespaces", gin2.RequireSuperAdminAuth, ListRegistryNamespaces)
		registry.POST("/namespaces", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, CreateRegistryNamespace)
		registry.PUT("/namespaces/:id", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, UpdateRegistryNamespace)

		registry.DELETE("/namespaces/:id", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, DeleteRegistryNamespace)
		registry.GET("/release/repos", ListAllRepos)
		registry.POST("/images", ListImages)
		registry.GET("/images/repos/:name", ListRepoImages)
	}

	s3storage := router.Group("s3storage")
	{
		s3storage.GET("", ListS3Storage)
		s3storage.POST("", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, CreateS3Storage)
		s3storage.GET("/:id", GetS3Storage)
		s3storage.PUT("/:id", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, UpdateS3Storage)
		s3storage.DELETE("/:id", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, DeleteS3Storage)
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
		github.POST("", gin2.RequireSuperAdminAuth, CreateGithubApp)
		github.DELETE("/:id", gin2.RequireSuperAdminAuth, DeleteGithubApp)
	}

	// ---------------------------------------------------------------------------------------
	// jenkins集成接口以及jobs和buildWithParameters接口
	// ---------------------------------------------------------------------------------------
	jenkins := router.Group("jenkins")
	{
		jenkins.POST("/integration", gin2.RequireSuperAdminAuth, CreateJenkinsIntegration)
		jenkins.GET("/integration", ListJenkinsIntegration)
		jenkins.PUT("/integration/:id", gin2.RequireSuperAdminAuth, UpdateJenkinsIntegration)
		jenkins.DELETE("/integration/:id", gin2.RequireSuperAdminAuth, DeleteJenkinsIntegration)
		jenkins.POST("/user/connection", gin2.RequireSuperAdminAuth, TestJenkinsConnection)
		jenkins.GET("/jobNames", gin2.RequireSuperAdminAuth, ListJobNames)
		jenkins.GET("/buildArgs/:jobName", gin2.RequireSuperAdminAuth, ListJobBuildArgs)
	}

	//系统配额
	capacity := router.Group("capacity")
	{
		capacity.POST("", UpdateStrategy)
		capacity.GET("/target/:target", GetStrategy)
		capacity.POST("/gc", GarbageCollection)
		// 清理已被删除的工作流的所有缓存，暂时用于手动调用
		capacity.POST("/clean", CleanCache)
	}

	// ---------------------------------------------------------------------------------------
	// 自定义镜像管理接口
	// ---------------------------------------------------------------------------------------
	basicImages := router.Group("basicImages")
	{
		basicImages.GET("", ListBasicImages)
		basicImages.GET("/:id", GetBasicImage)
		basicImages.POST("", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, CreateBasicImage)
		basicImages.PUT("/:id", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, UpdateBasicImage)
		basicImages.DELETE("/:id", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, DeleteBasicImage)
	}

	// ---------------------------------------------------------------------------------------
	// helm chart 集成
	// ---------------------------------------------------------------------------------------
	integration := router.Group("helm")
	{
		integration.GET("", ListHelmRepos)
		integration.POST("", gin2.RequireSuperAdminAuth, CreateHelmRepo)
		integration.PUT("/:id", gin2.RequireSuperAdminAuth, UpdateHelmRepo)
		integration.DELETE("/:id", gin2.RequireSuperAdminAuth, DeleteHelmRepo)
	}

	// ---------------------------------------------------------------------------------------
	// ssh私钥管理接口
	// ---------------------------------------------------------------------------------------
	privateKey := router.Group("privateKey")
	{
		privateKey.GET("", ListPrivateKeys)
		privateKey.GET("/:id", GetPrivateKey)
		privateKey.POST("", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, CreatePrivateKey)
		privateKey.PUT("/:id", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, UpdatePrivateKey)
		privateKey.DELETE("/:id", gin2.RequireSuperAdminAuth, gin2.UpdateOperationLogStatus, DeletePrivateKey)
	}

	notification := router.Group("notification")
	{
		notification.GET("", PullNotify)
		notification.PUT("/read", ReadNotify)
		notification.POST("/delete", DeleteNotifies)
		notification.POST("/subscribe", UpsertSubscription)
		notification.PUT("/subscribe/:type", UpdateSubscribe)
		notification.DELETE("/unsubscribe/notifytype/:type", Unsubscribe)
		notification.GET("/subscribe", ListSubscriptions)
	}

	announcement := router.Group("announcement")
	{
		announcement.POST("", gin2.RequireSuperAdminAuth, CreateAnnouncement)
		announcement.PUT("/update", gin2.RequireSuperAdminAuth, UpdateAnnouncement)
		announcement.GET("/all", gin2.RequireSuperAdminAuth, PullAllAnnouncement)
		announcement.GET("", PullNotifyAnnouncement)
		announcement.DELETE("/:id", gin2.RequireSuperAdminAuth, DeleteAnnouncement)
	}

	operation := router.Group("operation")
	{
		operation.GET("", gin2.RequireSuperAdminAuth, GetOperationLogs)
		operation.POST("", gin2.RequireSuperAdminAuth, AddSystemOperationLog)
		operation.PUT("/:id", gin2.RequireSuperAdminAuth, UpdateOperationLog)
	}

}
