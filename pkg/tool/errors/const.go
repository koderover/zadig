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

package errors

// Common
const (
	InvalidFormatErrMsg = "格式定义错误"

	TimeoutErrMsg = "等待超时错误"

	SetNamespaceErrMsg = "无法获取用户容器环境"

	CreateDefaultRegistryErrMsg = "无法创建默认的镜像仓库"

	SetRegistrySecretErrMsg = "无法获取镜像仓库权限"

	SetCephSecretErrMsg = "无法获取Ceph存储权限"
)

// Product
const (
	DuplicateProductErrMsg = "产品已存在"

	DuplicateEnvErrMsg = "环境已存在"

	ProductNotFoundErrMsg = "产品不存在"

	EnvNotFoundErrMsg = "环境不存在"

	EnvCantUpdatedMsg = "环境正处于不能更新的状态"

	ProductAccessDeniedErrMsg = "无权访问产品"

	NothingUpdateErrMsg = "产品无需更新"

	NothingEnvUpdateErrMsg = "环境无需更新"

	ListProductErrMsg = "列出产品信息失败"

	GetProductRevErrMsg = "获取产品版本失败"

	GetEnvRevErrMsg = "获取环境版本失败"

	UpdateProductStatusErrMsg = "保存产品状态信息失败"

	UpdateEnvStatusErrMsg = "保存环境状态信息失败"

	PatchProductErrMsg = "产品适配集成测试覆盖率失败"

	UpdateProductErrorsErrMsg = "保存产品错误信息失败"

	UpdateEnvErrorsErrMsg = "保存环境错误信息失败"
)

// Product Template
const (
	FindProductTmplErrMsg = "获取产品失败"

	FindProductServiceErrMsg = "产品不包含任何服务"
)

// Service & Containers
const (
	DeleteNamespaceErrMsg = "删除命名空间失败"

	DeleteServiceContainerErrMsg = "删除容器资源失败"

	UpsertServiceErrMsg = "新增或更新服务失败"

	DeleteServiceErrMsg = "删除服务容器失败"

	ServiceRunUnstableErrMsg = "部分服务运行不稳定"

	RestartServiceErrMsg = "重启服务失败"
)

// Pipelines & Pipeline Task
const (
	FindPipelineErrMsg = "获取工作流失败"

	FindPipelineTaskErrMsg = "获取工作流任务失败"

	UpdatePipelineTaskErrMsg = "更新工作流任务失败"

	RestartPassedTaskErrMsg = "不能重试已经通过任务"

	PipelineDisabledErrMsg = "工作流未开启"

	PipelineSubTaskNotFoundErrMsg = "未配置工作流子任务"

	DeletePipelineTaskErrMsg = "删除工作流关联任务失败"

	ListKeystoreErrMsg = "列出工作流敏感信息配置失败"

	DeleteKeystoreErrMsg = "删除工作流关联敏感信息失败"

	InterfaceToTaskErrMsg = "获取工作流子任务基本信息失败"

	TaskStillQueuedErrMsg = "工作流调度中，请稍后再试"
)

const (
	DuplicateClusterNameFound = "已经有相同名称的集群存在"
	StartPodTimeout           = "这些服务中可能存在未正常运行的实例"
)
