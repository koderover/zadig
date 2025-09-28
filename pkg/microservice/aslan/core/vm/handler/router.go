/*
Copyright 2023 The KodeRover Authors.

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

import "github.com/gin-gonic/gin"

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	vm := router.Group("")
	{
		vm.GET("/:vmid/agent/access", GetAgentAccessCmd)
		vm.PUT("/:vmid/agent/offline", OfflineVM)
		vm.PUT("/:vmid/agent/recovery", RecoveryVM)
		vm.PUT("/:vmid/agent/upgrade", UpgradeAgent)
		vm.GET("/vms", ListVMs)
		vm.GET("/labels", ListVMLabels)
	}

	vmAgent := router.Group("agents")
	{
		vmAgent.POST("/register", RegisterAgent)
		vmAgent.POST("/verify", VerifyAgent)
		vmAgent.POST("/heartbeat", HeartbeatAgent)
		vmAgent.GET("/job/request", PollingAgentJob)
		vmAgent.POST("/job/report", ReportAgentJob)
		vmAgent.GET("/tempFile/download/:fileId", DownloadTemporaryFile)
	}
}
