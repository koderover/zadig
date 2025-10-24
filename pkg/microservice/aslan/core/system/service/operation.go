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

package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type OperationLogArgs struct {
	Username     string `json:"username"`
	ProductName  string `json:"product_name"`
	ExactProduct string `json:"exact_product"`
	Function     string `json:"function"`
	Status       int    `json:"status"`
	PerPage      int    `json:"per_page"`
	Page         int    `json:"page"`
	Scene        string `json:"scene"`
	TargetID     string `json:"target_id"`
	Detail       string `json:"detail"`
}

var OperationLogI18NMethodMap = map[string]string{
	"新建":                 "Create",
	"创建":                 "Create",
	"修改":                 "Modify",
	"删除":                 "Delete",
	"更新":                 "Update",
	"回滚":                 "Rollback",
	"配置":                 "Config",
	"初始化":                "Initialize",
	"新增":                 "Add",
	"批量新增":               "Batch Add",
	"复制":                 "Copy",
	"清理":                 "Clean",
	"同步":                 "Sync",
	"取消":                 "Cancel",
	"克隆":                 "Clone",
	"重试":                 "Retry",
	"手动执行":               "Manually Execute",
	"重启":                 "Restart",
	"关联":                 "Associate",
	"扩缩容":                "Scale",
	"接入主机":               "Join Host",
	"下线主机":               "Offline Host",
	"恢复主机":               "Restore Host",
	"组件升级":               "Upgrade Agent",
	"执行终止":               "Abort",
	"版本回滚":               "Rollback Version",
	"部署回滚":               "Rollback Deployment",
	"灰度执行":               "Gray Release",
	"启动调试容器":             "Start Debug Container",
	"开启Istio灰度":          "Enable Istio Gray Release",
	"关闭Istio灰度":          "Disable Istio Gray Release",
	"设置Istio灰度配置":        "Set Istio Gray Release Config",
	"开启自测模式":             "Enable Self-Test Mode",
	"关闭自测模式":             "Disable Self-Test Mode",
	"OpenAPI初始化":         "(OpenAPI) Initialize",
	"(OpenAPI)初始化":       "(OpenAPI) Initialize",
	"(openAPI)更新":        "(OpenAPI) Update",
	"(OpenAPI)更新":        "(OpenAPI) Update",
	"(OpenAPI)新建":        "(OpenAPI) Create",
	"(OpenAPI)创建":        "(OpenAPI) Create",
	"OpenAPI新增":          "(OpenAPI) Add",
	"OpenAPI删除":          "(OpenAPI) Delete",
	"OpenAPI重启":          "(OpenAPI) Restart",
	"(OpenAPI)新增":        "(OpenAPI) Add",
	"(OpenAPI)删除":        "(OpenAPI) Delete",
	"(OpenAPI)重启":        "(OpenAPI) Restart",
	"(OpenAPI)重试":        "(OpenAPI) Retry",
	"OpenAPI-开启自测模式":     "(OpenAPI) Enable Self-Test Mode",
	"OpenAPI-关闭自测模式":     "(OpenAPI) Disable Self-Test Mode",
	"(OpenAPI)环境管理-更新服务": "(OpenAPI) Environment Management - Update Service",
	"(OpenAPI)更新测试服务配置":  "(OpenAPI) Update Test Service Config",
	"(OpenAPI)更新生产服务配置":  "(OpenAPI) Update Production Service Config",
	"(OpenAPI)更新测试服务变量":  "(OpenAPI) Update Test Service Variables",
	"(OpenAPI)更新生产服务变量":  "(OpenAPI) Update Production Service Variables",
}

var OperationLogI18NFunctionMap = map[string]string{
	"环境":              "Environment",
	"环境变量":            "Environment Variables",
	"环境配置":            "Environment Configuration",
	"环境的服务":           "Environment Service",
	"环境的helm release": "Environment Helm Release",
	"更新服务":            "Update Service",
	"更新环境配置":          "Update Environment Configuration",
	"更新服务变量":          "Update Service Variables",
	"更新全局变量":          "Update Global Variables",
	"环境-服务镜像":         "Environment - Service Image",
	"环境-服务实例":         "Environment - Service Instance",
	"OpenAPI-环境-服务镜像": "OpenAPI - Environment - Service Image",
	"环境-环境回收":         "Environment - Environment Recycle",
	"环境-服务-configMap": "Environment - Service - ConfigMap",
	"环境-服务-ConfigMap": "Environment - Service - ConfigMap",
	"环境巡检-cron":       "Environment Inspection - Cron",
	"环境定时睡眠与唤醒":       "Environment Timing Sleep and Wake Up",
	"SAE环境":           "SAE Environment",
	"SAE环境-应用实例":      "SAE Environment - Application Instance",
	"SAE环境-添加应用":      "SAE Environment - Add Application",
	"SAE环境-删除应用":      "SAE Environment - Delete Application",
	"测试环境":            "Test Environment",
	"生产环境":            "Production Environment",
	"测试环境配置":          "Test Environment Configuration",
	"生产环境配置":          "Production Environment Configuration",
	"测试环境的服务":         "Test Environment Service",
	"生产环境的服务":         "Production Environment Service",
	"环境-服务":           "Environment- Service",
	"环境-单服务":          "Environment- Single Service",
	"环境管理-更新服务":       "Environment Management - Update Service",
	"测试环境管理-更新全局变量":   "Test Environment Management - Update Global Variables",
	"生产环境管理-更新全局变量":   "Production Environment Management - Update Global Variables",

	"项目资源-主机管理":      "Project Resource - Host Management",
	"项目管理-项目":        "Project Management - Project",
	"项目管理-k8s项目":     "Project Management - K8S Project",
	"项目管理-helm项目":    "Project Management - Helm Project",
	"项目管理-项目环境模板或变量": "Project Management - Project Environment Template or Variables",
	"项目管理-项目服务编排":    "Project Management - Project Service Orchestration",
	"项目管理-测试服务编排":    "Project Management - Test Service Orchestration",
	"项目管理-生产服务编排":    "Project Management - Production Service Orchestration",
	"项目管理-服务":        "Project Management - Service",
	"项目管理-测试服务":      "Project Management - Test Service",
	"项目管理-生产服务":      "Project Management - Production Service",
	"项目管理-主机服务":      "Project Management - Host Service",
	"项目管理-服务变量":      "Project Management - Service Variables",
	"项目管理-测试服务变量":    "Project Management - Test Service Variables",
	"项目管理-生产服务变量":    "Project Management - Production Service Variables",
	"项目管理-服务标签":      "Project Management - Service Label",
	"项目管理-测试服务标签":    "Project Management - Test Service Label",
	"项目管理-生产服务标签":    "Project Management - Production Service Label",
	"项目管理-代码扫描":      "Project Management - Code Scan",
	"项目管理-测试":        "Project Management - Test",
	"工程管理-项目":        "Engineering Management - Project",
	"项目管理-构建":        "Project Management - Build",
	"项目管理-服务组件":      "Project Management - Service Component",

	"服务":             "Service",
	"测试服务":           "Test Service",
	"生产服务":           "Production Service",

	"迭代管理-流程":        "Iteration Management - Sprint",
	"迭代管理-流程添加阶段":    "Iteration Management - Sprint Template Add Stage",
	"迭代管理-流程删除阶段":    "Iteration Management - Sprint Template Delete Stage",
	"迭代管理-流程阶段名称":    "Iteration Management - Sprint Template Stage Name",
	"迭代管理-流程阶段工作流":   "Iteration Management - Sprint Template Stage Workflows",
	"迭代管理-工作项":       "Iteration Management - WorkItem",
	"迭代管理-迭代":        "Iteration Management - Sprint",

	"系统配置-自定义导航":     "System Configuration - Custom Navigation",
	"系统配置-快捷链接":      "System Configuration - Shortcut Link",
	"系统配置-外部系统":      "System Configuration - External System",
	"系统配置-应用设置":      "System Configuration - Application Settings",
	"系统配置-Sonar集成":   "System Configuration - Sonar Integration",
	"资源设置-插件管理":      "Resource Settings - Plugin Management",
	"系统设置-Registry":  "Resource Configuration - Image Repository",
	"系统设置-对象存储":      "System Configuration - Object Storage",
	"资源配置-镜像仓库":      "Resource Configuration - Image Repository",
	"资源配置-集群":        "Resource Configuration - Cluster",
	"资源管理-主机管理":      "Resource Management - Host Management",

	"模版-构建":          "Template - Build",
	"模版-YAML":        "Template - YAML",
	"模版-YAML-变量":     "Template - YAML - Variables",
	"模版-代码扫描":        "Template - Code Scan",
	"模版库-Chart":      "Template - Chart",
	"模版库-Dockerfile": "Template - Dockerfile",

	"收藏工作流": "Favorite Workflow",
	"工作流视图": "Workflow View",

	"工作流":             "Workflow",
	"工作流-webhook":     "Workflow - Webhook",
	"工作流-jirahook":    "Workflow - Jira Hook",
	"工作流-meegohook":   "Workflow - Meego Hook",
	"工作流-generalhook": "Workflow - General Hook",
	"工作流-cron":        "Workflow - Cron",

	"工作流任务":  "Workflow Task",
	"代码扫描任务": "Code Scanning Task",

	"OpenAPI测试task": "OpenAPI - Test Task",
	"OpenAPI测试任务":   "OpenAPI - Test Task",
	"测试-任务":         "Test Task",
	"测试任务":          "Test Task",

	"分组":             "Group",
	"发布计划":           "Release Plan",
	"基础镜像":  "Base Image",
	"任务配置":  "Task Configuration",
	"代理":    "Proxy",
	"安全与隐私": "Security & Privacy",
	"协作模式":  "Collaboration Mode",
	"版本交付":  "Version Delivery",
	"角色绑定":  "Role Binding",
	"全局角色":  "Global Role",
	"角色":    "Role",
}

type OperationLogI18N struct {
	MethodI18Map   map[string]string `json:"method_i18_map"`
	FunctionI18Map map[string]string `json:"function_i18_map"`
}

type FindOperationResponse struct {
	OperationLogs []*models.OperationLog `json:"operation_logs"`
	Total         int                    `json:"total"`
	I18N          *OperationLogI18N      `json:"i18n"`
}

func FindOperation(args *OperationLogArgs, log *zap.SugaredLogger) (*FindOperationResponse, int, error) {
	logs, count, err := mongodb.NewOperationLogColl().Find(&mongodb.OperationLogArgs{
		Username:     args.Username,
		ProductName:  args.ProductName,
		ExactProduct: args.ExactProduct,
		Function:     args.Function,
		Status:       args.Status,
		PerPage:      args.PerPage,
		Page:         args.Page,
		Scene:        args.Scene,
		TargetID:     args.TargetID,
		Detail:       args.Detail,
	})
	if err != nil {
		log.Errorf("find operation log error: %v", err)
		return nil, count, e.ErrFindOperationLog.AddErr(err)
	}

	resp := &FindOperationResponse{
		OperationLogs: logs,
		Total:         count,
		I18N: &OperationLogI18N{
			MethodI18Map:   OperationLogI18NMethodMap,
			FunctionI18Map: OperationLogI18NFunctionMap,
		},
	}

	return resp, count, err
}

type AddAuditLogResp struct {
	OperationLogID string `json:"id"`
}

func InsertOperation(args *models.OperationLog, log *zap.SugaredLogger) (*AddAuditLogResp, error) {
	err := mongodb.NewOperationLogColl().Insert(args)
	if err != nil {
		log.Errorf("insert operation log error: %v", err)
		return nil, e.ErrCreateOperationLog
	}

	return &AddAuditLogResp{
		OperationLogID: args.ID.Hex(),
	}, nil
}

func UpdateOperation(id string, status int, log *zap.SugaredLogger) error {
	err := mongodb.NewOperationLogColl().Update(id, status)
	if err != nil {
		log.Errorf("update operation log error: %v", err)
		return e.ErrUpdateOperationLog
	}
	return nil
}
