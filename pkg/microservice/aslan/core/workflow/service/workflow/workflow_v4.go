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

package workflow

import (
	"fmt"
	"regexp"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	jobctl "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow/job"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

const (
	JobNameRegx = "^[a-z][a-z0-9-]{0,31}$"
)

func CreateWorkflowV4(user string, workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	_, err := commonrepo.NewWorkflowV4Coll().Find(workflow.Name)
	if err == nil {
		errStr := fmt.Sprintf("workflow v4 [%s] 在项目 [%s] 中已经存在!", workflow.Name, workflow.Project)
		return e.ErrUpsertWorkflow.AddDesc(errStr)
	}
	if err := LintWorkflowV4(workflow, logger); err != nil {
		return err
	}

	workflow.CreatedBy = user
	workflow.UpdatedBy = user
	workflow.CreateTime = time.Now().Unix()
	workflow.UpdateTime = time.Now().Unix()

	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if err := jobctl.Instantiate(job, workflow); err != nil {
				logger.Errorf("Failed to instantiate workflow v4,error: %v", err)
				return e.ErrUpsertWorkflow.AddErr(err)
			}
		}
	}

	if _, err := commonrepo.NewWorkflowV4Coll().Create(workflow); err != nil {
		logger.Errorf("Failed to create workflow v4, the error is: %s", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}

	return nil
}

func UpdateWorkflowV4(name, user string, inputWorkflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(name)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", name, err)
		return e.ErrFindWorkflow.AddErr(err)
	}
	if err := LintWorkflowV4(inputWorkflow, logger); err != nil {
		return err
	}

	inputWorkflow.UpdatedBy = user
	inputWorkflow.UpdateTime = time.Now().Unix()
	inputWorkflow.ID = workflow.ID

	for _, stage := range inputWorkflow.Stages {
		for _, job := range stage.Jobs {
			if err := jobctl.Instantiate(job, workflow); err != nil {
				logger.Errorf("Failed to instantiate workflow v4,error: %v", err)
				return e.ErrUpsertWorkflow.AddErr(err)
			}
		}
	}

	if err := commonrepo.NewWorkflowV4Coll().Update(
		workflow.ID.Hex(),
		inputWorkflow,
	); err != nil {
		logger.Errorf("update workflowV4 error: %s", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	return nil
}

func FindWorkflowV4(name string, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(name)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", name, err)
		return workflow, e.ErrFindWorkflow.AddErr(err)
	}
	return workflow, err
}

func DeleteWorkflowV4(name string, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(name)
	if err != nil {
		logger.Errorf("Failed to delete WorkflowV4: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}
	if err := commonrepo.NewWorkflowV4Coll().DeleteByID(workflow.ID.Hex()); err != nil {
		logger.Errorf("Failed to delete WorkflowV4: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}

	if err := commonrepo.NewCounterColl().Delete("WorkflowTaskV4:" + name); err != nil {
		log.Errorf("Counter.Delete error: %s", err)
	}
	return nil
}

func ListWorkflowV4(projectName, userID string, pageNum, pageSize int64, logger *zap.SugaredLogger) ([]*Workflow, int64, error) {
	resp := make([]*Workflow, 0)
	workflowV4List, total, err := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{
		ProjectName: projectName,
	}, pageNum, pageSize)
	if err != nil {
		logger.Errorf("Failed to list workflow v4, the error is: %s", err)
		return resp, 0, err
	}

	workflow, err := ListWorkflows([]string{projectName}, userID, []string{}, logger)
	if err != nil {
		return resp, 0, err
	}
	resp = append(resp, workflow...)

	for _, workflow := range workflowV4List {
		stages := []string{}
		for _, stage := range workflow.Stages {
			stages = append(stages, stage.Name)
		}
		resp = append(resp, &Workflow{
			Name:          workflow.Name,
			ProjectName:   workflow.Project,
			EnabledStages: stages,
			CreateTime:    workflow.CreateTime,
			UpdateTime:    workflow.UpdateTime,
			UpdateBy:      workflow.UpdatedBy,
			WorkflowType:  "common_workflow",
			Description:   workflow.Description,
		})
	}
	return resp, total, nil
}

func LintWorkflowV4(workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	if workflow.Project == "" {
		err := fmt.Errorf("project should not be empty")
		logger.Errorf(err.Error())
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	project, err := templaterepo.NewProductColl().Find(workflow.Project)
	if err != nil {
		logger.Errorf("Failed to get project %s, error: %v", workflow.Project, err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}

	if project.ProductFeature != nil {
		if project.ProductFeature.DeployType != setting.K8SDeployType && project.ProductFeature.DeployType != setting.HelmDeployType {
			logger.Error("common workflow only support k8s and helm project")
			return e.ErrUpsertWorkflow.AddDesc("common workflow only support k8s and helm project")
		}
	}
	stageNameMap := make(map[string]bool, 0)
	jobNameMap := make(map[string]string, 0)
	reg, err := regexp.Compile(JobNameRegx)
	if err != nil {
		logger.Errorf("reg compile failed: %v", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	for _, stage := range workflow.Stages {
		if _, ok := stageNameMap[stage.Name]; !ok {
			stageNameMap[stage.Name] = true
		} else {
			logger.Errorf("duplicated stage name: %s", stage.Name)
			return e.ErrUpsertWorkflow.AddDesc(fmt.Sprintf("duplicated job name: %s", stage.Name))
		}

		for _, job := range stage.Jobs {
			if match := reg.MatchString(job.Name); !match {
				logger.Errorf("job name [%s] did not match %s", job.Name, JobNameRegx)
				return e.ErrUpsertWorkflow.AddDesc(fmt.Sprintf("job name [%s] did not match %s", job.Name, JobNameRegx))
			}
			if _, ok := jobNameMap[job.Name]; !ok {
				jobNameMap[job.Name] = string(job.JobType)
			} else {
				logger.Errorf("duplicated job name: %s", job.Name)
				return e.ErrUpsertWorkflow.AddDesc(fmt.Sprintf("duplicated job name: %s", job.Name))
			}

			if job.JobType == config.JobZadigDeploy {
				spec := &commonmodels.ZadigDeployJobSpec{}
				if err := commonmodels.IToiYaml(job.Spec, spec); err != nil {
					logger.Errorf("decode job spec error: %v", err)
					return e.ErrUpsertWorkflow.AddErr(err)
				}
				if spec.Source != config.SourceFromJob {
					continue
				}
				jobType, ok := jobNameMap[spec.JobName]
				if !ok || jobType != string(config.JobZadigBuild) {
					errMsg := fmt.Sprintf("build job %s not found", spec.JobName)
					logger.Error(errMsg)
					return e.ErrCreateTask.AddDesc(errMsg)
				}
			}
		}
	}
	return nil
}
