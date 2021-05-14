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

package permission

const (
	SuperUserUUID = "00000" //超级用户权限
	//产品工作流相关
	WorkflowTaskUUID     = "30001" //产品工作流(执行任务、重启任务、取消任务)
	WorkflowUpdateUUID   = "30002" //产品工作流(更新)
	WorkflowCreateUUID   = "30003" //产品工作流(创建)
	WorkflowDeleteUUID   = "30004" //产品工作流(删除)
	WorkflowListUUID     = "30005" //产品工作流(查看)
	WorkflowDeliveryUUID = "30006" //产品工作流(添加交付)

	//集成环境(测试环境)
	TestEnvCreateUUID = "40001" //创建环境
	TestEnvDeleteUUID = "40002" //删除环境
	TestEnvManageUUID = "40003" //环境管理(环境更新、单服务更新、服务伸缩、服务重启、configmap更新、configmap回滚、重启实例、yaml导出)
	TestEnvListUUID   = "40004" //环境查看
	TestEnvShareUUID  = "40005" //环境授权
	TestUpdateEnvUUID = "40010" //更新环境

	//集成环境(生产环境)
	ProdEnvListUUID   = "40006" //环境查看
	ProdEnvCreateUUID = "40007" //创建环境
	ProdEnvManageUUID = "40008" //环境管理(环境更新、单服务更新、服务伸缩、服务重启、configMap更新、configMap回滚、重启实例、yaml导出)
	ProdEnvDeleteUUID = "40009" //删除环境

	//原工程管理相关
	//构建管理
	BuildDeleteUUID = "50007" //删除构建配置
	BuildManageUUID = "50008" //构建配置管理(创建构建、修改构建、修改服务组件)
	BuildListUUID   = "50019" //构建配置查看

	//服务管理
	ServiceTemplateEditUUID   = "50022" //修改服务编排
	ServiceTemplateManageUUID = "50023" //服务模板管理(创建服务模板、修改服务模板)
	ServiceTemplateDeleteUUID = "50024" //删除服务模板
	ServiceTemplateListUUID   = "50025" //查看服务模板

	//测试管理
	TestDeleteUUID = "50010" //删除测试
	TestManageUUID = "50011" //测试管理(创建测试、修改测试)
	TestListUUID   = "50020" //查看测试

	//交付中心
	ReleaseDeleteUUID = "70001" //交付中心删除版本

	ParamType      = 1
	QueryType      = 2
	ContextKeyType = 3
)
