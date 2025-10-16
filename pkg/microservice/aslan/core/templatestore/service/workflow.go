/*
Copyright 2022 The KodeRover Authors.

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
	"fmt"
	"regexp"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	systemservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	jobcontroller "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller/job"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func CreateWorkflowTemplate(userName string, template *commonmodels.WorkflowV4Template, logger *zap.SugaredLogger) error {
	if _, err := commonrepo.NewWorkflowV4TemplateColl().Find(&commonrepo.WorkflowTemplateQueryOption{Name: template.TemplateName}); err == nil {
		errMsg := fmt.Sprintf("工作流模板名称: %s 已存在", template.TemplateName)
		logger.Error(errMsg)
		return e.ErrCreateWorkflowTemplate.AddDesc(errMsg)
	}
	if err := lintWorkflowTemplate(template, logger); err != nil {
		return err
	}
	template.CreatedBy = userName
	template.UpdatedBy = userName

	workflow := &commonmodels.WorkflowV4{
		Stages: template.Stages,
	}

	for _, stage := range template.Stages {
		for _, job := range stage.Jobs {
			jobCtrl, err := jobcontroller.CreateJobController(job, workflow)
			if err != nil {
				logger.Errorf("Failed to create job controller for job [%s], error: %v", job.Name, err)
				return e.ErrCreateWorkflowTemplate.AddErr(err)
			}
			if err := jobCtrl.Validate(false); err != nil {
				logger.Errorf("Failed to validate job [%s], error: %v", job.Name, err)
				return e.ErrCreateWorkflowTemplate.AddErr(err)
			}
		}
	}
	if err := commonrepo.NewWorkflowV4TemplateColl().Create(template); err != nil {
		errMsg := fmt.Sprintf("Failed to create workflow template %s, err: %v", template.TemplateName, err)
		logger.Error(errMsg)
		return e.ErrCreateWorkflowTemplate.AddDesc(errMsg)
	}
	return nil
}

func UpdateWorkflowTemplate(userName string, template *commonmodels.WorkflowV4Template, logger *zap.SugaredLogger) error {
	if _, err := commonrepo.NewWorkflowV4TemplateColl().Find(&commonrepo.WorkflowTemplateQueryOption{ID: template.ID.Hex()}); err != nil {
		errMsg := fmt.Sprintf("workflow template %s not found: %v", template.TemplateName, err)
		logger.Error(errMsg)
		return e.ErrUpdateWorkflowTemplate.AddDesc(errMsg)
	}
	if err := lintWorkflowTemplate(template, logger); err != nil {
		return err
	}
	template.UpdatedBy = userName

	workflow := &commonmodels.WorkflowV4{
		Stages: template.Stages,
	}

	for _, stage := range template.Stages {
		for _, job := range stage.Jobs {
			jobCtrl, err := jobcontroller.CreateJobController(job, workflow)
			if err != nil {
				logger.Errorf("Failed to create job controller for job [%s], error: %v", job.Name, err)
				return e.ErrCreateWorkflowTemplate.AddErr(err)
			}
			if err := jobCtrl.Validate(false); err != nil {
				logger.Errorf("Failed to validate job [%s], error: %v", job.Name, err)
				return e.ErrCreateWorkflowTemplate.AddErr(err)
			}
		}
	}
	if err := commonrepo.NewWorkflowV4TemplateColl().Update(template); err != nil {
		errMsg := fmt.Sprintf("Failed to update workflow template %s, err: %v", template.TemplateName, err)
		logger.Error(errMsg)
		return e.ErrUpdateWorkflowTemplate.AddDesc(errMsg)
	}
	return nil
}

type ListWorkflowTemplateResp struct {
	WorkflowTemplates            []*WorkflowtemplatePreView `json:"workflow_templates"`
	BuildInWorkflowTemplatei18ns []*WorkflowTemplatei18n    `json:"build_in_workflow_template_i18ns"`
}

func ListWorkflowTemplate(category string, excludeBuildIn bool, logger *zap.SugaredLogger) (*ListWorkflowTemplateResp, error) {
	workflowTemplates := []*WorkflowtemplatePreView{}
	templates, err := commonrepo.NewWorkflowV4TemplateColl().List(&commonrepo.WorkflowTemplateListOption{Category: category, ExcludeBuildIn: excludeBuildIn})
	if err != nil {
		errMsg := fmt.Sprintf("Failed to list workflow template err: %v", err)
		logger.Error(errMsg)
		return nil, e.ErrListWorkflowTemplate.AddDesc(errMsg)
	}
	for _, template := range templates {
		stages := []string{}
		for _, stage := range template.Stages {
			stages = append(stages, stage.Name)
		}

		stageDetails := make([]*WorkflowTemplateStage, 0)
		for _, stage := range template.Stages {
			stageDetail := &WorkflowTemplateStage{
				Name: stage.Name,
				Jobs: make([]*WorkflowTemplateJob, 0),
			}
			for _, job := range stage.Jobs {
				stageDetail.Jobs = append(stageDetail.Jobs, &WorkflowTemplateJob{
					Name:    job.Name,
					JobType: string(job.JobType),
				})
			}
			stageDetails = append(stageDetails, stageDetail)
		}

		workflowTemplates = append(workflowTemplates, &WorkflowtemplatePreView{
			ID:           template.ID,
			TemplateName: template.TemplateName,
			UpdateTime:   template.UpdateTime,
			CreateTime:   template.CreateTime,
			CreateBy:     template.CreatedBy,
			UpdateBy:     template.UpdatedBy,
			Stages:       stages,
			StageDetails: stageDetails,
			Description:  template.Description,
			Category:     template.Category,
			BuildIn:      template.BuildIn,
		})
	}

	resp := &ListWorkflowTemplateResp{
		WorkflowTemplates:            workflowTemplates,
		BuildInWorkflowTemplatei18ns: workflowTemplateNamei18ns,
	}

	return resp, nil
}

func GetWorkflowTemplateByID(idStr string, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4Template, error) {
	template, err := commonrepo.NewWorkflowV4TemplateColl().Find(&commonrepo.WorkflowTemplateQueryOption{ID: idStr})
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get workflow template err: %v", err)
		logger.Error(errMsg)
		return template, e.ErrGetWorkflowTemplate.AddDesc(errMsg)
	}
	// do data compatibility updates
	for _, stage := range template.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == config.JobNotification {
				spec := &commonmodels.NotificationJobSpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					logger.Errorf(err.Error())
					return nil, err
				}

				err := spec.GenerateNewNotifyConfigWithOldData()
				if err != nil {
					logger.Errorf(err.Error())
					return nil, err
				}

				job.Spec = spec
			}
		}
	}
	return template, nil
}

func DeleteWorkflowTemplateByID(idStr string, logger *zap.SugaredLogger) error {
	if err := commonrepo.NewWorkflowV4TemplateColl().DeleteByID(idStr); err != nil {
		errMsg := fmt.Sprintf("Failed to delete workflow template err: %v", err)
		logger.Error(errMsg)
		return e.ErrDeleteWorkflowTemplate.AddDesc(errMsg)
	}
	return nil
}

func lintWorkflowTemplate(template *commonmodels.WorkflowV4Template, logger *zap.SugaredLogger) error {
	stageNameMap := make(map[string]bool)
	jobNameMap := make(map[string]string)

	reg, err := regexp.Compile(setting.JobNameRegx)
	if err != nil {
		logger.Errorf("reg compile failed: %v", err)
		return e.ErrLintWorkflowTemplate.AddErr(err)
	}
	for _, stage := range template.Stages {
		if _, ok := stageNameMap[stage.Name]; !ok {
			stageNameMap[stage.Name] = true
		} else {
			logger.Errorf("duplicated stage name: %s", stage.Name)
			return e.ErrLintWorkflowTemplate.AddDesc(fmt.Sprintf("duplicated stage name: %s", stage.Name))
		}
		for _, job := range stage.Jobs {
			if match := reg.MatchString(job.Name); !match {
				logger.Errorf("job name [%s] did not match %s", job.Name, setting.JobNameRegx)
				return e.ErrLintWorkflowTemplate.AddDesc(fmt.Sprintf("job name [%s] did not match %s", job.Name, setting.JobNameRegx))
			}
			if _, ok := jobNameMap[job.Name]; !ok {
				jobNameMap[job.Name] = string(job.JobType)
			} else {
				logger.Errorf("duplicated job name: %s", job.Name)
				return e.ErrLintWorkflowTemplate.AddDesc(fmt.Sprintf("duplicated job name: %s", job.Name))
			}
		}
	}
	return nil
}

func InitWorkflowTemplate() {
	logger := log.SugaredLogger()
	for _, template := range InitWorkflowTemplateInfos() {
		template.CreateTime = time.Now().Unix()
		template.UpdateTime = time.Now().Unix()
		template.CreatedBy = "system"
		template.UpdatedBy = "system"
		template.Category = setting.CustomWorkflow
		template.ConcurrencyLimit = 1
		if err := commonrepo.NewWorkflowV4TemplateColl().UpsertByName(template); err != nil {
			logger.Errorf("update build-in workflow template error: %v", err)
		}
	}
	for _, name := range DeprecatedWorkflowTemplateName() {
		if err := commonrepo.NewWorkflowV4TemplateColl().DeleteInternalByName(name); err != nil {
			logger.Errorf("delete deprecated workflow template error: %v", err)
		}
	}
}

func DeprecatedWorkflowTemplateName() []string {
	return []string{
		"单环境多服务更新",
		"多环境多服务更新",
		"上线服务",
		"下线服务",
		"API 测试",
		"E2E 测试",
		"静态代码检测",
		"动态安全检测",
		"Chart 实例化部署",
		"多阶段灰度发布",
		"蓝绿发布",
		"金丝雀发布",
		"Isito 发布",
		"MSE 发布",
		"JIRA 问题状态及业务变更",
		"飞书工作项状态及业务变更",
		"Nacos 配置及业务变更",
		"Apollo 配置及业务变更",
		"SQL 数据及业务变更",
		"DMS 数据变更及服务升级",
		"MySQL 数据库及业务变更",
	}
}

type WorkflowTemplatei18n struct {
	NameKey       string                       `json:"name_key"`
	NameZh        string                       `json:"name_zh"`
	NameEn        string                       `json:"name_en"`
	DescriptionZh string                       `json:"description_zh"`
	DescriptionEn string                       `json:"description_en"`
	Stages        []*WorkflowTemplateStagei18n `json:"stages"`
}

type WorkflowTemplateStagei18n struct {
	Key    string                     `json:"key"`
	NameZh string                     `json:"name_zh"`
	NameEn string                     `json:"name_en"`
	Jobs   []*WorkflowTemplateJobi18n `json:"jobs"`
}

type WorkflowTemplateJobi18n struct {
	Key           string
	NameZh        string
	NameEn        string
	DescriptionZh string
	DescriptionEn string
}

var workflowTemplateNamei18ns = []*WorkflowTemplatei18n{
	{
		NameKey:       "single-environment-multi-service-update",
		NameZh:        "单环境多服务更新",
		NameEn:        "Single Environment Multi Service Update",
		DescriptionZh: "支持多个服务并行构建、部署过程",
		DescriptionEn: "Support multiple services to build and deploy in parallel",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-build",
				NameZh: "构建",
				NameEn: "Build",
			},
			{
				Key:    "stage-2-deploy",
				NameZh: "部署",
				NameEn: "Deploy",
			},
		},
	},
	{
		NameKey:       "multi-environment-multi-service-update",
		NameZh:        "多环境多服务更新",
		NameEn:        "Multi Environment Multi Service Update",
		DescriptionZh: "支持一次构建部署多个环境",
		DescriptionEn: "Support building and deploying multiple environments at once",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-build",
				NameZh: "构建",
				NameEn: "Build",
			},
			{
				Key:    "stage-2-deploy-env-dev",
				NameZh: "部署环境 dev",
				NameEn: "Deploy Environment Dev",
			},
			{
				Key:    "stage-3-image-distribute",
				NameZh: "镜像分发",
				NameEn: "Image Distribute",
			},
			{
				Key:    "stage-4-deploy-env-qa",
				NameZh: "部署环境 qa",
				NameEn: "Deploy Environment Qa",
			},
		},
	},
	{
		NameKey:       "online-service",
		NameZh:        "上线服务",
		NameEn:        "Online Service",
		DescriptionZh: "支持通过工作流上线服务",
		DescriptionEn: "Support onlineing services through workflows",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-deploy",
				NameZh: "部署",
				NameEn: "Deploy",
			},
			{
				Key:    "stage-2-test",
				NameZh: "测试",
				NameEn: "Test",
			},
		},
	},
	{
		NameKey:       "offline-service",
		NameZh:        "下线服务",
		NameEn:        "Offline Service",
		DescriptionZh: "支持通过工作流下线服务",
		DescriptionEn: "Support offlineing services through workflows",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-offline-service",
				NameZh: "下线服务",
				NameEn: "Offline Service",
			},
		},
	},
	{
		NameKey:       "api-test",
		NameZh:        "API 测试",
		NameEn:        "API Test",
		DescriptionZh: "支持自动化执行服务级别 API 测试",
		DescriptionEn: "Support automatically executing service-level API tests",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-build",
				NameZh: "构建",
				NameEn: "Build",
			},
			{
				Key:    "stage-2-deploy",
				NameZh: "部署",
				NameEn: "Deploy",
			},
			{
				Key:    "stage-3-api-test",
				NameZh: "API 测试",
				NameEn: "API Test",
			},
		},
	},
	{
		NameKey:       "e2e-test",
		NameZh:        "E2E 测试",
		NameEn:        "E2E Test",
		DescriptionZh: "支持自动化执行产品级别 E2E 测试",
		DescriptionEn: "Support automatically executing product-level E2E tests",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-build",
				NameZh: "构建",
				NameEn: "Build",
			},
			{
				Key:    "stage-2-deploy",
				NameZh: "部署",
				NameEn: "Deploy",
			},
			{
				Key:    "stage-3-e2e-test",
				NameZh: "E2E 测试",
				NameEn: "E2E Test",
			},
		},
	},
	{
		NameKey:       "static-code-scanning",
		NameZh:        "静态代码检测",
		NameEn:        "Static Code Scanning",
		DescriptionZh: "支持自动化执行代码扫描和代码成分分析",
		DescriptionEn: "Support automatically executing code scanning and code analysis",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-code-scanning",
				NameZh: "代码扫描",
				NameEn: "Code Scanning",
			},
			{
				Key:    "stage-2-code-analysis",
				NameZh: "代码成分分析",
				NameEn: "Code Analysis",
			},
			{
				Key:    "stage-3-build",
				NameZh: "构建",
				NameEn: "Build",
			},
			{
				Key:    "stage-4-deploy",
				NameZh: "部署",
				NameEn: "Deploy",
			},
		},
	},
	{
		NameKey:       "dynamic-security-scanning",
		NameZh:        "动态安全检测",
		NameEn:        "Dynamic Security Scanning",
		DescriptionZh: "支持自动化执行服务动态安全检测",
		DescriptionEn: "Support automatically executing service-level dynamic security testing",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-build",
				NameZh: "构建",
				NameEn: "Build",
			},
			{
				Key:    "stage-2-deploy",
				NameZh: "部署",
				NameEn: "Deploy",
			},
			{
				Key:    "stage-3-dynamic-security-scanning",
				NameZh: "动态安全检测",
				NameEn: "Dynamic Security Scanning",
			},
		},
	},
	{
		NameKey:       "chart-deployment",
		NameZh:        "Chart 实例化部署",
		NameEn:        "Chart Deployment",
		DescriptionZh: "支持自动化部署 Chart 仓库中已有的 Chart 到环境中（仅限 K8s Helm Chart 项目）",
		DescriptionEn: "Support automatically deploying Chart repositories to environments (only supported for K8s Helm Chart projects)",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-chart-deploy",
				NameZh: "Chart 部署",
				NameEn: "Chart Deploy",
			},
			{
				Key:    "stage-2-test",
				NameZh: "测试",
				NameEn: "Test",
			},
		},
	},
	{
		NameKey:       "multi-stage-gray-release",
		NameZh:        "多阶段灰度发布",
		NameEn:        "Multi-Stage Gray Release",
		DescriptionZh: "支持通过工作流进行多阶段灰度发布，结合人工审批，确保灰度发布过程可控",
		DescriptionEn: "Support multi-stage gray release through workflows, combined with manual approval, to ensure the gray release process is controllable",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-gray-20",
				NameZh: "灰度20%",
				NameEn: "Gray 20%",
			},
			{
				Key:    "stage-2-approval",
				NameZh: "审批",
				NameEn: "Approval",
			},
			{
				Key:    "stage-3-gray-50",
				NameZh: "灰度50%",
				NameEn: "Gray 50%",
			},
			{
				Key:    "stage-4-approval-2",
				NameZh: "审批2",
				NameEn: "Approval 2",
			},
			{
				Key:    "stage-5-gray-100",
				NameZh: "灰度100%",
				NameEn: "Gray 100%",
			},
		},
	},
	{
		NameKey:       "blue-green-release",
		NameZh:        "蓝绿发布",
		NameEn:        "Blue Green Release",
		DescriptionZh: "支持自动化执行蓝绿发布，结合人工审批，确保蓝绿过程可控",
		DescriptionEn: "Support automatically executing blue green release, combined with manual approval, to ensure the blue green process is controllable",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-blue-green-deploy",
				NameZh: "蓝绿部署",
				NameEn: "Blue Green Deploy",
			},
			{
				Key:    "stage-2-check",
				NameZh: "检查",
				NameEn: "Check",
			},
			{
				Key:    "stage-3-approval",
				NameZh: "审批",
				NameEn: "Approval",
			},
			{
				Key:    "stage-4-blue-green-release",
				NameZh: "蓝绿发布",
				NameEn: "Blue Green Release",
			},
		},
	},
	{
		NameKey:       "canary-release",
		NameZh:        "金丝雀发布",
		NameEn:        "Canary Release",
		DescriptionZh: "支持自动化执行金丝雀发布，结合人工审批，确保金丝雀发布过程可控",
		DescriptionEn: "Support automatically executing canary release, combined with manual approval, to ensure the canary process is controllable",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-canary-deploy",
				NameZh: "金丝雀部署",
				NameEn: "Canary Deploy",
			},
			{
				Key:    "stage-2-check",
				NameZh: "检查",
				NameEn: "Check",
			},
			{
				Key:    "stage-3-approval",
				NameZh: "审批",
				NameEn: "Approval",
			},
			{
				Key:    "stage-4-canary-release",
				NameZh: "金丝雀发布",
				NameEn: "Canary Release",
			},
		},
	},
	{
		NameKey:       "istio-release",
		NameZh:        "Istio 发布",
		NameEn:        "Istio Release",
		DescriptionZh: "支持  istio 灰度发布，结合人工审批，确保发布过程可控",
		DescriptionEn: "Support automatically executing istio release, combined with manual approval, to ensure the istio process is controllable",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-istio-20",
				NameZh: "Istio 流量 20%",
				NameEn: "Istio 20% Traffic",
			},
			{
				Key:    "stage-2-approval",
				NameZh: "审批",
				NameEn: "Approval",
			},
			{
				Key:    "stage-3-istio-60",
				NameZh: "Istio 流量 60%",
				NameEn: "Istio 60% Traffic",
			},
			{
				Key:    "stage-4-approval-2",
				NameZh: "审批2",
				NameEn: "Approval 2",
			},
			{
				Key:    "stage-5-istio-100",
				NameZh: "Istio 流量 100%",
				NameEn: "Istio 100% Traffic",
			},
		},
	},
	{
		NameKey:       "mse-release",
		NameZh:        "MSE 发布",
		NameEn:        "MSE Release",
		DescriptionZh: "支持 MSE 发布，结合人工审批，确保发布过程可控",
		DescriptionEn: "Support mse release, combined with manual approval, to ensure the release process is controllable",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-approval",
				NameZh: "审批",
				NameEn: "Approval",
			},
			{
				Key:    "stage-2-mse-release",
				NameZh: "MSE 发布",
				NameEn: "MSE Release",
			},
			{
				Key:    "stage-3-check",
				NameZh: "检查",
				NameEn: "Check",
			},
		},
	},
	{
		NameKey:       "jira-issue-status-change",
		NameZh:        "JIRA 问题状态及业务变更",
		NameEn:        "JIRA Issue Status and Business Change",
		DescriptionZh: "支持自动化执行 JIRA 问题状态变更",
		DescriptionEn: "Support automatically executing JIRA issue status change",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-build",
				NameZh: "构建",
				NameEn: "Build",
			},
			{
				Key:    "stage-2-deploy",
				NameZh: "部署",
				NameEn: "Deploy",
			},
			{
				Key:    "stage-3-test",
				NameZh: "测试",
				NameEn: "Test",
			},
			{
				Key:    "stage-4-jira-issue-status-change",
				NameZh: "JIRA 问题状态变更",
				NameEn: "JIRA Issue Status Change",
			},
		},
	},
	{
		NameKey:       "lark-issue-status-change",
		NameZh:        "飞书工作项状态及业务变更",
		NameEn:        "Lark Issue Status and Business Change",
		DescriptionZh: "支持自动化执行飞书工作项状态变更",
		DescriptionEn: "Support automatically executing lark issue status change",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-build",
				NameZh: "构建",
				NameEn: "Build",
			},
			{
				Key:    "stage-2-deploy",
				NameZh: "部署",
				NameEn: "Deploy",
			},
			{
				Key:    "stage-3-test",
				NameZh: "测试",
				NameEn: "Test",
			},
			{
				Key:    "stage-4-lark-issue-status-change",
				NameZh: "飞书工作项状态变更",
				NameEn: "Lark Issue Status Change",
			},
		},
	},
	{
		NameKey:       "nacos-configuration-change",
		NameZh:        "Nacos 配置及业务变更",
		NameEn:        "Nacos Configurations and Business Changes",
		DescriptionZh: "支持自动化执行 Nacos 配置变更",
		DescriptionEn: "Support automatically executing Nacos configuration change",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-build",
				NameZh: "构建",
				NameEn: "Build",
			},
			{
				Key:    "stage-2-nacos-configuration-change",
				NameZh: "Nacos 配置变更",
				NameEn: "Nacos Configuration Change",
			},
			{
				Key:    "stage-3-deploy",
				NameZh: "部署",
				NameEn: "Deploy",
			},
		},
	},
	{
		NameKey:       "apollo-configuration-change",
		NameZh:        "Apollo 配置及业务变更",
		NameEn:        "Apollo Configurations and Business Changes",
		DescriptionZh: "支持自动化执行 Apollo 配置变更",
		DescriptionEn: "Support automatically executing Apollo configuration change",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-build",
				NameZh: "构建",
				NameEn: "Build",
			},
			{
				Key:    "stage-2-apollo-configuration-change",
				NameZh: "Apollo 配置变更",
				NameEn: "Apollo Configuration Change",
			},
			{
				Key:    "stage-3-deploy",
				NameZh: "部署",
				NameEn: "Deploy",
			},
		},
	},
	{
		NameKey:       "sql-data-and-business-change",
		NameZh:        "SQL 数据及业务变更",
		NameEn:        "SQL Data and Business Changes",
		DescriptionZh: "支持自动化执行 SQL 数据变更以及多服务并行构建、部署过程",
		DescriptionEn: "Support automatically executing SQL data change and multi-service parallel build and deployment process",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-sql-data-change",
				NameZh: "SQL 数据变更",
				NameEn: "SQL Data Change",
			},
			{
				Key:    "stage-2-build",
				NameZh: "构建",
				NameEn: "Build",
			},
			{
				Key:    "stage-3-deploy",
				NameZh: "部署",
				NameEn: "Deploy",
			},
		},
	},
	{
		NameKey:       "dms-data-change-and-service-upgrade",
		NameZh:        "DMS 数据变更及服务升级",
		NameEn:        "DMS Data Change and Service Upgrade",
		DescriptionZh: "支持自动化创建并跟踪 DMS 数据变更工单以及多服务并行构建、部署过程",
		DescriptionEn: "Support automatically creating and tracking DMS data change work orders and multi-service parallel build and deployment process",
		Stages: []*WorkflowTemplateStagei18n{
			{
				Key:    "stage-1-dms-data-change",
				NameZh: "DMS 数据变更",
				NameEn: "DMS Data Change",
				Jobs: []*WorkflowTemplateJobi18n{
					{
						Key:           "plugin-job-dms-update",
						NameZh:        "DMS 数据变更工单",
						NameEn:        "DMS Data Change Ticket",
						DescriptionZh: "创建并跟踪 DMS 数据变更工单",
						DescriptionEn: "Create and track DMS data change ticket",
					},
				},
			},
			{
				Key:    "stage-2-build",
				NameZh: "构建",
				NameEn: "Build",
			},
			{
				Key:    "stage-3-deploy",
				NameZh: "部署",
				NameEn: "Deploy",
			},
		},
	},
}

func InitWorkflowTemplateInfos() []*commonmodels.WorkflowV4Template {
	imageID := ""
	buildOS := "focal"
	imageFrom := "koderover"
	basicImages, err := systemservice.ListBasicImages("koderover", "", log.SugaredLogger())
	if err != nil {
		log.Errorf("Failed to list basic images, err: %v", err)
	}
	for _, basicImage := range basicImages {
		if basicImage.Value == "focal" {
			imageID = basicImage.ID.Hex()
			buildOS = basicImage.Value
			imageFrom = basicImage.ImageFrom
			break
		}
	}
	if imageID == "" && len(basicImages) > 0 {
		imageID = basicImages[0].ID.Hex()
		buildOS = basicImages[0].Value
		imageFrom = basicImages[0].ImageFrom
	}

	buildInWorkflowTemplateInfos := []*commonmodels.WorkflowV4Template{
		// deploy service
		{
			TemplateName: "single-environment-multi-service-update",
			BuildIn:      true,
			// Description:  "支持多个服务并行构建、部署过程",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-build",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec: commonmodels.ZadigBuildJobSpec{
								Source: config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-2-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "build",
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "multi-environment-multi-service-update",
			BuildIn:      true,
			// Description:  "支持一次构建部署多个环境",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-build",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec: commonmodels.ZadigBuildJobSpec{
								Source: config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-2-deploy-env-dev",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy-env-dev",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "build",
							},
						},
					},
				},
				{
					Name:     "stage-3-image-distribute",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "image-distribute",
							JobType: config.JobZadigDistributeImage,
							Spec: commonmodels.ZadigDistributeImageJobSpec{
								Source:  config.SourceFromJob,
								JobName: "build",
							},
						},
					},
				},
				{
					Name:     "stage-4-deploy-env-qa",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy-env-qa",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "image-distribute",
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "online-service",
			BuildIn:      true,
			// Description:  "支持通过工作流上线服务",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage, config.DeployVars, config.DeployConfig},
								Source:         config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-2-test",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "test",
							JobType: config.JobZadigTesting,
							Spec: commonmodels.ZadigTestingJobSpec{
								TestType: " ",
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "offline-service",
			BuildIn:      true,
			// Description:  "支持通过工作流下线服务",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-offline-service",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "offline-service",
							JobType: config.JobOfflineService,
							Spec:    commonmodels.JobTaskOfflineServiceSpec{},
						},
					},
				},
			},
		},

		// test and security
		{
			TemplateName: "api-test",
			BuildIn:      true,
			// Description:  "支持自动化执行服务级别 API 测试",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-build",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec: commonmodels.ZadigBuildJobSpec{
								Source: config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-2-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "build",
							},
						},
					},
				},
				{
					Name:     "stage-3-api-test",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "test",
							JobType: config.JobZadigTesting,
							Spec: commonmodels.ZadigTestingJobSpec{
								TestType: config.ServiceTestType,
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "e2e-test",
			BuildIn:      true,
			// Description:  "支持自动化执行产品级别 E2E 测试",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-build",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec: commonmodels.ZadigBuildJobSpec{
								Source: config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-2-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "build",
							},
						},
					},
				},
				{
					Name:     "stage-3-e2e-test",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "test",
							JobType: config.JobZadigTesting,
							Spec: commonmodels.ZadigTestingJobSpec{
								TestType: config.ProductTestType,
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "static-code-scanning",
			BuildIn:      true,
			// Description:  "支持自动化执行代码扫描和代码成分分析",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-code-scanning",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "code-scanning",
							JobType: config.JobZadigScanning,
							Spec:    commonmodels.ZadigScanningJobSpec{},
						},
					},
				},
				{
					Name:     "stage-2-code-analysis",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "code-analyse",
							JobType: config.JobFreestyle,
							Spec: commonmodels.FreestyleJobSpec{
								Repos:      make([]*types.Repository, 0),
								Script:     "#!/bin/bash\nset -e",
								ScriptType: types.ScriptTypeShell,
								Runtime: &commonmodels.RuntimeInfo{
									Infrastructure: setting.JobK8sInfrastructure,
									BuildOS:        buildOS,
									ImageFrom:      imageFrom,
									ImageID:        imageID,
								},
								AdvancedSetting: &commonmodels.FreestyleJobAdvancedSettings{
									JobAdvancedSettings: &commonmodels.JobAdvancedSettings{
										Timeout:         60,
										ResourceRequest: "low",
										ResReqSpec:      setting.LowRequestSpec,
									},
								},
								ObjectStorageUpload: &commonmodels.ObjectStorageUpload{
									Enabled: false,
								},
								Envs: make(commonmodels.RuntimeKeyValList, 0),
							},
						},
					},
				},
				{
					Name:     "stage-3-build",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec: commonmodels.ZadigBuildJobSpec{
								Source: config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-4-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "build",
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "dynamic-security-scanning",
			BuildIn:      true,
			// Description:  "支持自动化执行服务动态安全检测",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-build",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec: commonmodels.ZadigBuildJobSpec{
								Source: config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-2-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "build",
							},
						},
					},
				},
				{
					Name:     "stage-3-dynamic-security-scanning",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "test",
							JobType: config.JobZadigTesting,
							Spec: commonmodels.ZadigTestingJobSpec{
								TestType: " ",
							},
						},
					},
				},
			},
		},

		// release
		{
			TemplateName: "chart-deployment",
			BuildIn:      true,
			// Description:  "支持自动化部署 Chart 仓库中已有的 Chart 到环境中（仅限 K8s Helm Chart 项目）",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-chart-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigHelmChartDeploy,
							Spec:    commonmodels.ZadigHelmChartDeployJobSpec{},
						},
					},
				},
				{
					Name:     "stage-2-test",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "test",
							JobType: config.JobZadigTesting,
							Spec: commonmodels.ZadigTestingJobSpec{
								TestType: " ",
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "multi-stage-gray-release",
			// Description:  "支持自动化执行多阶段的灰度发布，结合人工审批，确保灰度过程可控",
			BuildIn: true,
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-gray-20",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "gray-20",
							JobType: config.JobK8sGrayRelease,
							Spec: commonmodels.GrayReleaseJobSpec{
								DeployTimeout: 10,
								GrayScale:     20,
							},
						},
					},
				},
				{
					Name:     "stage-2-approval",
					Parallel: false,
					Jobs: []*commonmodels.Job{
						{
							Name:    "approval",
							JobType: config.JobApproval,
							Spec: commonmodels.ApprovalJobSpec{
								Timeout:     60,
								Type:        config.NativeApproval,
								Description: "Confirm release to 50%",
								NativeApproval: &commonmodels.NativeApproval{
									Timeout:         60,
									NeededApprovers: 1,
								},
							},
						},
					},
				},
				{
					Name:     "stage-3-gray-50",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "gray-50",
							JobType: config.JobK8sGrayRelease,
							Spec: commonmodels.GrayReleaseJobSpec{
								FromJob:       "gray-20",
								DeployTimeout: 10,
								GrayScale:     50,
							},
						},
					},
				},
				{
					Name:     "stage-4-approval-2",
					Parallel: false,
					Jobs: []*commonmodels.Job{
						{
							Name:      "approval-2",
							JobType:   config.JobApproval,
							RunPolicy: config.ForceRun,
							Spec: commonmodels.ApprovalJobSpec{
								Timeout:     60,
								Type:        config.NativeApproval,
								Description: "Confirm release to 100%",
								NativeApproval: &commonmodels.NativeApproval{
									Timeout:         60,
									NeededApprovers: 1,
								},
							},
						},
					},
				},
				{
					Name:     "stage-5-gray-100",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "gray-100",
							JobType: config.JobK8sGrayRelease,
							Spec: commonmodels.GrayReleaseJobSpec{
								FromJob:       "gray-20",
								DeployTimeout: 10,
								GrayScale:     100,
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "blue-green-release",
			// Description:  "支持自动化执行蓝绿发布，结合人工审批，确保蓝绿过程可控",
			BuildIn: true,
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-blue-green-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "blue-green-deploy",
							JobType: config.JobK8sBlueGreenDeploy,
							Spec: commonmodels.BlueGreenDeployV2JobSpec{
								Version:    "v2",
								Production: true,
							},
						},
					},
				},
				{
					Name:     "stage-2-check",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "check",
							JobType: config.JobFreestyle,
							Spec: commonmodels.FreestyleJobSpec{
								Repos:      make([]*types.Repository, 0),
								Script:     "#!/bin/bash\nset -e",
								ScriptType: types.ScriptTypeShell,
								Runtime: &commonmodels.RuntimeInfo{
									Infrastructure: setting.JobK8sInfrastructure,
									BuildOS:        buildOS,
									ImageFrom:      imageFrom,
									ImageID:        imageID,
								},
								AdvancedSetting: &commonmodels.FreestyleJobAdvancedSettings{
									JobAdvancedSettings: &commonmodels.JobAdvancedSettings{
										Timeout:         60,
										ResourceRequest: "low",
										ResReqSpec:      setting.LowRequestSpec,
									},
								},
								ObjectStorageUpload: &commonmodels.ObjectStorageUpload{
									Enabled: false,
								},
								Envs: make(commonmodels.RuntimeKeyValList, 0),
							},
						},
					},
				},
				{
					Name:     "stage-3-approval",
					Parallel: false,
					Jobs: []*commonmodels.Job{
						{
							Name:      "approval",
							JobType:   config.JobApproval,
							RunPolicy: config.ForceRun,
							Spec: commonmodels.ApprovalJobSpec{
								Timeout:     60,
								Type:        config.NativeApproval,
								Description: "Confirm to release",
								NativeApproval: &commonmodels.NativeApproval{
									Timeout:         60,
									NeededApprovers: 1,
								},
							},
						},
					},
				},
				{
					Name:     "stage-4-blue-green-release",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "blue-green-release",
							JobType: config.JobK8sBlueGreenRelease,
							Spec: commonmodels.BlueGreenReleaseV2JobSpec{
								FromJob: "blue-green-deploy",
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "canary-release",
			// Description:  "支持自动化执行金丝雀发布，结合人工审批，确保金丝雀发布过程可控",
			BuildIn: true,
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-canary-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "canary-deploy",
							JobType: config.JobK8sCanaryDeploy,
							Spec:    commonmodels.CanaryDeployJobSpec{},
						},
					},
				},
				{
					Name:     "stage-2-check",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "check",
							JobType: config.JobFreestyle,
							Spec: commonmodels.FreestyleJobSpec{
								Repos:      make([]*types.Repository, 0),
								Script:     "#!/bin/bash\nset -e",
								ScriptType: types.ScriptTypeShell,
								Runtime: &commonmodels.RuntimeInfo{
									Infrastructure: setting.JobK8sInfrastructure,
									BuildOS:        buildOS,
									ImageFrom:      imageFrom,
									ImageID:        imageID,
								},
								AdvancedSetting: &commonmodels.FreestyleJobAdvancedSettings{
									JobAdvancedSettings: &commonmodels.JobAdvancedSettings{
										Timeout:         60,
										ResourceRequest: "low",
										ResReqSpec:      setting.LowRequestSpec,
									},
								},
								ObjectStorageUpload: &commonmodels.ObjectStorageUpload{
									Enabled: false,
								},
								Envs: make(commonmodels.RuntimeKeyValList, 0),
							},
						},
					},
				},
				{
					Name:     "stage-3-approval",
					Parallel: false,
					Jobs: []*commonmodels.Job{
						{
							Name:      "approval",
							JobType:   config.JobApproval,
							RunPolicy: config.ForceRun,
							Spec: commonmodels.ApprovalJobSpec{
								Timeout:     60,
								Type:        config.NativeApproval,
								Description: "Confirm to release",
								NativeApproval: &commonmodels.NativeApproval{
									Timeout:         60,
									NeededApprovers: 1,
								},
							},
						},
					},
				},
				{
					Name:     "stage-4-canary-release",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "canary-release",
							JobType: config.JobK8sCanaryRelease,
							Spec: commonmodels.CanaryReleaseJobSpec{
								FromJob:        "canary-deploy",
								ReleaseTimeout: 10,
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "istio-release",
			// Description:  "支持  istio 灰度发布，结合人工审批，确保发布过程可控",
			BuildIn: true,
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-istio-20",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "istio-20",
							JobType: config.JobIstioRelease,
							Spec: commonmodels.IstioJobSpec{
								First:             true,
								ClusterID:         setting.LocalClusterID,
								Timeout:           10,
								ReplicaPercentage: 100,
								Weight:            20,
							},
						},
					},
				},
				{
					Name:     "stage-2-approval",
					Parallel: false,
					Jobs: []*commonmodels.Job{
						{
							Name:      "approval",
							JobType:   config.JobApproval,
							RunPolicy: config.ForceRun,
							Spec: commonmodels.ApprovalJobSpec{
								Timeout:     60,
								Type:        config.NativeApproval,
								Description: "Confirm to release 60%",
								NativeApproval: &commonmodels.NativeApproval{
									Timeout:         60,
									NeededApprovers: 1,
								},
							},
						},
					},
				},
				{
					Name:     "stage-3-istio-60",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "istio-60",
							JobType: config.JobIstioRelease,
							Spec: commonmodels.IstioJobSpec{
								First:             false,
								FromJob:           "istio-20",
								Timeout:           10,
								ReplicaPercentage: 100,
								Weight:            60,
							},
						},
					},
				},
				{
					Name:     "stage-4-approval-2",
					Parallel: false,
					Jobs: []*commonmodels.Job{
						{
							Name:      "approval-2",
							JobType:   config.JobApproval,
							RunPolicy: config.ForceRun,
							Spec: commonmodels.ApprovalJobSpec{
								Timeout:     60,
								Type:        config.NativeApproval,
								Description: "Confirm to release 100%",
								NativeApproval: &commonmodels.NativeApproval{
									Timeout:         60,
									NeededApprovers: 1,
								},
							},
						},
					},
				},
				{
					Name:     "stage-5-istio-100",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "istio-100",
							JobType: config.JobIstioRelease,
							Spec: commonmodels.IstioJobSpec{
								First:             false,
								FromJob:           "istio-20",
								Timeout:           10,
								ReplicaPercentage: 100,
								Weight:            100,
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "mse-release",
			// Description:  "支持 MSE 发布，结合人工审批，确保发布过程可控",
			BuildIn: true,
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-approval",
					Parallel: false,
					Jobs: []*commonmodels.Job{
						{
							Name:      "approval",
							JobType:   config.JobApproval,
							RunPolicy: config.ForceRun,
							Spec: commonmodels.ApprovalJobSpec{
								Timeout:     60,
								Type:        config.NativeApproval,
								Description: "Confirm to mse release",
								NativeApproval: &commonmodels.NativeApproval{
									Timeout:         60,
									NeededApprovers: 1,
								},
							},
						},
					},
				},
				{
					Name:     "stage-2-mse-release",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "mse-release",
							JobType: config.JobMseGrayRelease,
							Spec:    commonmodels.MseGrayReleaseJobSpec{},
						},
					},
				},
				{
					Name:     "stage-3-check",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "check",
							JobType: config.JobFreestyle,
							Spec: commonmodels.FreestyleJobSpec{
								Repos:      make([]*types.Repository, 0),
								Script:     "#!/bin/bash\nset -e",
								ScriptType: types.ScriptTypeShell,
								Runtime: &commonmodels.RuntimeInfo{
									Infrastructure: setting.JobK8sInfrastructure,
									BuildOS:        buildOS,
									ImageFrom:      imageFrom,
									ImageID:        imageID,
								},
								AdvancedSetting: &commonmodels.FreestyleJobAdvancedSettings{
									JobAdvancedSettings: &commonmodels.JobAdvancedSettings{
										Timeout:         60,
										ResourceRequest: "low",
										ResReqSpec:      setting.LowRequestSpec,
									},
								},
								ObjectStorageUpload: &commonmodels.ObjectStorageUpload{
									Enabled: false,
								},
								Envs: make(commonmodels.RuntimeKeyValList, 0),
							},
						},
					},
				},
			},
		},

		// project cooperation
		{
			TemplateName: "jira-issue-status-change",
			BuildIn:      true,
			Description:  "支持自动化执行 JIRA 问题状态变更",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-build",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec: commonmodels.ZadigBuildJobSpec{
								Source: config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-2-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "build",
							},
						},
					},
				},
				{
					Name:     "stage-3-test",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "test",
							JobType: config.JobZadigTesting,
							Spec: commonmodels.ZadigTestingJobSpec{
								TestType: " ",
							},
						},
					},
				},
				{
					Name:     "stage-4-jira-issue-status-change",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "jira-update",
							JobType: config.JobJira,
							Spec:    commonmodels.JiraJobSpec{},
						},
					},
				},
			},
		},
		{
			TemplateName: "lark-issue-status-change",
			BuildIn:      true,
			// Description:  "支持自动化执行飞书工作项状态变更",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-build",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec: commonmodels.ZadigBuildJobSpec{
								Source: config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-2-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "build",
							},
						},
					},
				},
				{
					Name:     "stage-3-test",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "test",
							JobType: config.JobZadigTesting,
							Spec: commonmodels.ZadigTestingJobSpec{
								TestType: " ",
							},
						},
					},
				},
				{
					Name:     "stage-4-lark-issue-status-change",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "lark-update",
							JobType: config.JobMeegoTransition,
							Spec:    commonmodels.MeegoTransitionJobSpec{},
						},
					},
				},
			},
		},

		// config update
		{
			TemplateName: "nacos-configuration-change",
			BuildIn:      true,
			// Description:  "支持自动化执行 Nacos 配置变更",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-build",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec: commonmodels.ZadigBuildJobSpec{
								Source: config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-2-nacos-configuration-change",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "nacos-update",
							JobType: config.JobNacos,
							Spec:    commonmodels.NacosJobSpec{},
						},
					},
				},
				{
					Name:     "stage-3-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "build",
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "apollo-configuration-change",
			BuildIn:      true,
			// Description:  "支持自动化执行 Apollo 配置变更",
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-build",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec: commonmodels.ZadigBuildJobSpec{
								Source: config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-2-apollo-configuration-change",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "apollo-update",
							JobType: config.JobApollo,
							Spec:    commonmodels.ApolloJobSpec{},
						},
					},
				},
				{
					Name:     "stage-3-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "build",
							},
						},
					},
				},
			},
		},

		// data update
		{
			TemplateName: "sql-data-and-business-change",
			// Description:  "支持自动化执行 SQL 数据变更以及多服务并行构建、部署过程",
			BuildIn: true,
			Stages: []*commonmodels.WorkflowStage{
				{
					Name: "stage-1-sql-data-change",
					Jobs: []*commonmodels.Job{
						{
							Name:    "sql-update",
							JobType: config.JobSQL,
							Spec:    commonmodels.SQLJobSpec{},
						},
					},
				},
				{
					Name:     "stage-2-build",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec: commonmodels.ZadigBuildJobSpec{
								Source: config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-3-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "build",
							},
						},
					},
				},
			},
		},
		{
			TemplateName: "dms-data-change-and-service-upgrade",
			// Description:  "支持自动化创建并跟踪 DMS 数据变更工单以及多服务并行构建、部署过程",
			BuildIn:  true,
			Category: setting.ReleaseWorkflow,
			Stages: []*commonmodels.WorkflowStage{
				{
					Name:     "stage-1-dms-data-change",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "dms-update",
							JobType: config.JobPlugin,
							Spec: commonmodels.PluginJobSpec{
								Properties: &commonmodels.JobProperties{
									Timeout:         60,
									ResourceRequest: "low",
									ResReqSpec:      setting.LowRequestSpec,
								},
								Plugin: &commonmodels.PluginTemplate{
									Name:      "plugin-job-dms-update",
									IsOffical: true,
									Category:  "",
									// Description: "创建并跟踪 DMS 数据变更工单",
									Version: "v0.0.1",
									Image:   "koderover.tencentcloudcr.com/koderover-public/dms-approval:v0.0.1",
									Envs: []*commonmodels.Env{
										{
											Name:  "AK",
											Value: "$(inputs.AK)",
										},
										{
											Name:  "SK",
											Value: "$(inputs.SK)",
										},
										{
											Name:  "DBS",
											Value: "$(inputs.DBS)",
										},
										{
											Name:  "AFFECT_ROWS",
											Value: "$(inputs.AFFECT_ROWS)",
										},
										{
											Name:  "EXEC_SQL",
											Value: "$(inputs.EXEC_SQL)",
										},
										{
											Name:  "COMMENT",
											Value: "$(inputs.COMMENT)",
										},
									},
									Inputs: []*commonmodels.Param{
										{
											Name:        "AK",
											Description: "aliyun account AK",
											ParamsType:  "string",
										},
										{
											Name:        "SK",
											Description: "aliyun account AK",
											ParamsType:  "string",
										},
										{
											Name:        "DBS",
											Description: "databases to operate, separated by commas, eg: test@127.0.0.1:3306,test@127.0.0.1:3306",
											ParamsType:  "string",
										},
										{
											Name:        "AFFECT_ROWS",
											Description: "affect rows",
											ParamsType:  "string",
										},
										{
											Name:        "EXEC_SQL",
											Description: "sql to exexute",
											ParamsType:  "text",
										},
										{
											Name:        "COMMENT",
											Description: "comment message",
											ParamsType:  "string",
										},
									},
								},
							},
						},
					},
				},
				{
					Name:     "stage-2-build",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec: commonmodels.ZadigBuildJobSpec{
								Source: config.SourceRuntime,
							},
						},
					},
				},
				{
					Name:     "stage-3-deploy",
					Parallel: true,
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								DeployContents: []config.DeployContent{config.DeployImage},
								Source:         config.SourceFromJob,
								JobName:        "build",
							},
						},
					},
				},
			},
		},
	}
	return buildInWorkflowTemplateInfos
}
