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
	JobNameRegx  = "^[a-z][a-z0-9-]{0,31}$"
	WorkflowRegx = "^[a-z0-9-]{1,32}$"
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
	if err := commonrepo.NewworkflowTaskv4Coll().DeleteByWorkflowName(name); err != nil {
		logger.Errorf("Failed to delete WorkflowV4 task: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}
	if err := commonrepo.NewCounterColl().Delete("WorkflowTaskV4:" + name); err != nil {
		log.Errorf("Counter.Delete error: %s", err)
	}
	return nil
}

func ListWorkflowV4(projectName, userID string, logger *zap.SugaredLogger) ([]*Workflow, error) {
	resp := make([]*Workflow, 0)
	workflowV4List, _, err := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{
		ProjectName: projectName,
	}, 0, 0)
	if err != nil {
		logger.Errorf("Failed to list workflow v4, the error is: %s", err)
		return resp, err
	}

	workflow, err := ListWorkflows([]string{projectName}, userID, []string{}, logger)
	if err != nil {
		return resp, err
	}
	workflowList := []string{}
	for _, wV4 := range workflowV4List {
		workflowList = append(workflowList, wV4.Name)
	}
	resp = append(resp, workflow...)
	tasks, _, err := commonrepo.NewworkflowTaskv4Coll().List(&commonrepo.ListWorkflowTaskV4Option{WorkflowNames: workflowList})
	if err != nil {
		return resp, err
	}

	for _, workflowModel := range workflowV4List {
		stages := []string{}
		for _, stage := range workflowModel.Stages {
			stages = append(stages, stage.Name)
		}
		workflow := &Workflow{
			Name:          workflowModel.Name,
			ProjectName:   workflowModel.Project,
			EnabledStages: stages,
			CreateTime:    workflowModel.CreateTime,
			UpdateTime:    workflowModel.UpdateTime,
			UpdateBy:      workflowModel.UpdatedBy,
			WorkflowType:  "common_workflow",
			Description:   workflowModel.Description,
		}
		getRecentTaskV4Info(workflow, tasks)

		resp = append(resp, workflow)
	}
	return resp, nil
}

func getRecentTaskV4Info(workflow *Workflow, tasks []*commonmodels.WorkflowTask) {
	recentTask := &commonmodels.WorkflowTask{}
	recentFailedTask := &commonmodels.WorkflowTask{}
	recentSucceedTask := &commonmodels.WorkflowTask{}
	for _, task := range tasks {
		if task.WorkflowName != workflow.Name {
			continue
		}
		if task.TaskID > recentTask.TaskID {
			recentTask = task
		}
		if task.Status == config.StatusPassed && task.TaskID > recentSucceedTask.TaskID {
			recentSucceedTask = task
		}
		if task.Status == config.StatusFailed && task.TaskID > recentFailedTask.TaskID {
			recentFailedTask = task
		}
	}
	if recentTask.TaskID > 0 {
		workflow.RecentTask = &TaskInfo{
			TaskID:       recentTask.TaskID,
			PipelineName: recentTask.WorkflowName,
			Status:       string(recentTask.Status),
		}
	}
	if recentSucceedTask.TaskID > 0 {
		workflow.RecentSuccessfulTask = &TaskInfo{
			TaskID:       recentSucceedTask.TaskID,
			PipelineName: recentSucceedTask.WorkflowName,
			Status:       string(recentSucceedTask.Status),
		}
	}
	if recentFailedTask.TaskID > 0 {
		workflow.RecentFailedTask = &TaskInfo{
			TaskID:       recentFailedTask.TaskID,
			PipelineName: recentFailedTask.WorkflowName,
			Status:       string(recentFailedTask.Status),
		}
	}
}

func LintWorkflowV4(workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	if workflow.Project == "" {
		err := fmt.Errorf("project should not be empty")
		logger.Errorf(err.Error())
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	match, err := regexp.MatchString(WorkflowRegx, workflow.Name)
	if err != nil {
		logger.Errorf("reg compile failed: %v", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	if !match {
		err := fmt.Errorf("workflow name should match %s", WorkflowRegx)
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
	stageNameMap := make(map[string]bool)
	jobNameMap := make(map[string]string)
	// deploy job can not qoute a build job which runs after it.
	buildJobNameMap := make(map[string]string)

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
		stageBuildJobNameMap := make(map[string]string)
		for _, job := range stage.Jobs {
			if !stage.Parallel {
				buildJobNameMap[job.Name] = string(job.JobType)
			} else {
				stageBuildJobNameMap[job.Name] = string(job.JobType)
			}
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
				jobType, ok := buildJobNameMap[spec.JobName]
				if !ok || jobType != string(config.JobZadigBuild) {
					errMsg := fmt.Sprintf("can not quote job %s in job %s", spec.JobName, job.Name)
					logger.Error(errMsg)
					return e.ErrUpsertWorkflow.AddDesc(errMsg)
				}
			}
		}
		for k, v := range stageBuildJobNameMap {
			buildJobNameMap[k] = v
		}
	}
	return nil
}
