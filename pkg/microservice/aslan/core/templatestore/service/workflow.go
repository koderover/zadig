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

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	jobctl "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow/job"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func InitWorkflowTemplateInfos() []*commonmodels.WorkflowV4Template {
	buildInWorkflowTemplateInfos := []*commonmodels.WorkflowV4Template{
		{
			TemplateName: "自定义工作流模板1",
			BuildIn:      true,
			Stages: []*commonmodels.WorkflowStage{
				{
					Name: "构建",
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec:    commonmodels.ZadigBuildJobSpec{},
						},
					},
				},
				{
					Name: "部署",
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								Source:  config.SourceFromJob,
								JobName: "build",
							},
						},
					},
				},
				{
					Name: "测试",
					Jobs: []*commonmodels.Job{
						{
							Name:    "test",
							JobType: config.JobZadigTesting,
							Spec:    commonmodels.ZadigTestingJobSpec{},
						},
					},
				},
			},
		},
		{
			TemplateName: "自定义工作流模板2",
			BuildIn:      true,
			Stages: []*commonmodels.WorkflowStage{
				{
					Name: "MySQL 变更",
					Jobs: []*commonmodels.Job{
						{
							Name:    "Mysql-update",
							JobType: config.JobPlugin,
							Spec: commonmodels.PluginJobSpec{
								Properties: &commonmodels.JobProperties{
									Timeout:         10,
									ResourceRequest: setting.MinRequest,
								},
								Plugin: &commonmodels.PluginTemplate{
									Name:        "MySQL 数据库变更",
									IsOffical:   true,
									Description: "针对 MySQL 数据库执行 SQL 变量",
									Version:     "v0.0.1",
									Image:       "koderover.tencentcloudcr.com/koderover-public/mysql-runner:v0.0.1",
									Envs: []*commonmodels.Env{
										{
											Name:  "MYSQL_HOST",
											Value: "$(inputs.mysql_host)",
										},
										{
											Name:  "MYSQL_PORT",
											Value: "$(inputs.mysql_port)",
										},
										{
											Name:  "USERNAME",
											Value: "$(inputs.username)",
										},
										{
											Name:  "PASSWORD",
											Value: "$(inputs.password)",
										},
										{
											Name:  "QUERY",
											Value: "$(inputs.query)",
										},
									},
									Inputs: []*commonmodels.Param{
										{
											Name:        "mysql_host",
											Description: "mysql host",
											ParamsType:  "string",
										},
										{
											Name:        "mysql_port",
											Description: "mysql port",
											ParamsType:  "string",
										},
										{
											Name:        "username",
											Description: "mysql username",
											ParamsType:  "string",
										},
										{
											Name:        "password",
											Description: "mysql password",
											ParamsType:  "string",
										},
										{
											Name:        "query",
											Description: "query to be used",
											ParamsType:  "string",
										},
									},
								},
							},
						},
					},
				},
				{
					Name: "构建",
					Jobs: []*commonmodels.Job{
						{
							Name:    "build",
							JobType: config.JobZadigBuild,
							Spec:    commonmodels.ZadigBuildJobSpec{},
						},
					},
				},
				{
					Name: "部署",
					Jobs: []*commonmodels.Job{
						{
							Name:    "deploy",
							JobType: config.JobZadigDeploy,
							Spec: commonmodels.ZadigDeployJobSpec{
								Source:  config.SourceFromJob,
								JobName: "build",
							},
						},
					},
				},
			},
		},
	}
	return buildInWorkflowTemplateInfos
}

type WorkflowtemplatePreView struct {
	ID           primitive.ObjectID       `json:"id"`
	TemplateName string                   `json:"template_name"`
	UpdateTime   int64                    `json:"update_time"`
	CreateTime   int64                    `json:"create_time"`
	UpdateBy     string                   `json:"update_by,omitempty"`
	Stages       []string                 `json:"stages"`
	Description  string                   `json:"description"`
	Category     setting.WorkflowCategory `json:"category"`
	BuildIn      bool                     `json:"build_in"`
}

func CreateWorkflowTemplate(userName string, template *commonmodels.WorkflowV4Template, logger *zap.SugaredLogger) error {
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
			if err := jobctl.Instantiate(job, workflow); err != nil {
				logger.Errorf("Failed to instantiate workflow v4 template, error: %v", err)
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
			if err := jobctl.Instantiate(job, workflow); err != nil {
				logger.Errorf("Failed to instantiate workflow v4 template, error: %v", err)
				return e.ErrUpdateWorkflowTemplate.AddErr(err)
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

func ListWorkflowTemplate(category string, excludeBuildIn bool, logger *zap.SugaredLogger) ([]*WorkflowtemplatePreView, error) {
	resp := []*WorkflowtemplatePreView{}
	templates, err := commonrepo.NewWorkflowV4TemplateColl().List(&commonrepo.WorkflowTemplateListOption{Category: category, ExcludeBuildIn: excludeBuildIn})
	if err != nil {
		errMsg := fmt.Sprintf("Failed to list workflow template err: %v", err)
		logger.Error(errMsg)
		return resp, e.ErrListWorkflowTemplate.AddDesc(errMsg)
	}
	for _, template := range templates {
		stages := []string{}
		for _, stage := range template.Stages {
			if stage.Approval != nil && stage.Approval.Enabled {
				stages = append(stages, "人工审核")
			}
			stages = append(stages, stage.Name)
		}
		resp = append(resp, &WorkflowtemplatePreView{
			ID:           template.ID,
			TemplateName: template.TemplateName,
			UpdateTime:   template.UpdateTime,
			CreateTime:   template.CreateTime,
			UpdateBy:     template.UpdatedBy,
			Stages:       stages,
			Description:  template.Description,
			Category:     template.Category,
			BuildIn:      template.BuildIn,
		})
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
		if err := CreateWorkflowTemplate("system", template, logger); err != nil {
			logger.Error(err)
		}
	}
}
