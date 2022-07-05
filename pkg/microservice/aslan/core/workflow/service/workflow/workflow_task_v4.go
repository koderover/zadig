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
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	jobctl "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow/job"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflowcontroller"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/types"
	stepspec "github.com/koderover/zadig/pkg/types/step"
	"go.uber.org/zap"
)

type CreateTaskV4Resp struct {
	ProjectName  string `json:"project_name"`
	WorkflowName string `json:"workflow_name"`
	TaskID       int64  `json:"task_id"`
}

type WorkflowTaskPreview struct {
	TaskID       int64               `bson:"task_id"                   json:"task_id"`
	WorkflowName string              `bson:"workflow_name"             json:"workflow_name"`
	Status       config.Status       `bson:"status"                    json:"status,omitempty"`
	TaskCreator  string              `bson:"task_creator"              json:"task_creator,omitempty"`
	TaskRevoker  string              `bson:"task_revoker,omitempty"    json:"task_revoker,omitempty"`
	CreateTime   int64               `bson:"create_time"               json:"create_time,omitempty"`
	StartTime    int64               `bson:"start_time"                json:"start_time,omitempty"`
	EndTime      int64               `bson:"end_time"                  json:"end_time,omitempty"`
	Stages       []*StageTaskPreview `bson:"stages"                    json:"stages"`
	ProjectName  string              `bson:"project_name"              json:"project_name"`
	Error        string              `bson:"error,omitempty"           json:"error,omitempty"`
	IsRestart    bool                `bson:"is_restart"                json:"is_restart"`
}

type StageTaskPreview struct {
	Name      string                 `bson:"name"          json:"name"`
	Status    config.Status          `bson:"status"        json:"status"`
	StartTime int64                  `bson:"start_time"    json:"start_time,omitempty"`
	EndTime   int64                  `bson:"end_time"      json:"end_time,omitempty"`
	Parallel  bool                   `bson:"parallel"      json:"parallel"`
	Approval  *commonmodels.Approval `bson:"approval"      json:"approval"`
	Jobs      []*JobTaskPreview      `bson:"jobs"          json:"jobs"`
}

type JobTaskPreview struct {
	Name      string        `bson:"name"           json:"name"`
	JobType   string        `bson:"type"           json:"type"`
	Status    config.Status `bson:"status"         json:"status"`
	StartTime int64         `bson:"start_time"     json:"start_time,omitempty"`
	EndTime   int64         `bson:"end_time"       json:"end_time,omitempty"`
	Error     string        `bson:"error"          json:"error"`
	Spec      interface{}   `bson:"spec"           json:"spec"`
}

type ZadigBuildJobSpec struct {
	Repos         []*types.Repository `bson:"repos"           json:"repos"`
	Image         string              `bson:"image"           json:"image"`
	ServiceName   string              `bson:"service_name"    json:"service_name"`
	ServiceModule string              `bson:"service_module"  json:"service_module"`
}

type ZadigDeployJobSpec struct {
	Env              string             `bson:"env"                          json:"env"`
	ServiceAndImages []*ServiceAndImage `bson:"service_and_images"           json:"service_and_images"`
}

type ServiceAndImage struct {
	ServiceName   string `bson:"service_name"           json:"service_name"`
	ServiceModule string `bson:"service_module"         json:"service_module"`
	Image         string `bson:"image"                  json:"image"`
}

func GetWorkflowv4Preset(workflowName string, log *zap.SugaredLogger) (*commonmodels.WorkflowV4, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		log.Errorf("cannot find workflow %s, the error is: %v", workflowName, err)
		return nil, e.ErrFindWorkflow.AddDesc(err.Error())
	}
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if err := jobctl.SetPreset(job, workflow); err != nil {
				log.Errorf("cannot get workflow %s preset, the error is: %v", workflowName, err)
				return nil, e.ErrFindWorkflow.AddDesc(err.Error())
			}
		}
	}
	return workflow, nil
}

func CreateWorkflowTaskV4(user string, workflow *commonmodels.WorkflowV4, log *zap.SugaredLogger) (*CreateTaskV4Resp, error) {
	resp := &CreateTaskV4Resp{
		ProjectName:  workflow.Project,
		WorkflowName: workflow.Name,
	}
	if err := LintWorkflowV4(workflow, log); err != nil {
		return resp, err
	}
	workflowTask := &commonmodels.WorkflowTask{}
	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.WorkflowTaskV4Fmt, workflow.Name))
	if err != nil {
		log.Errorf("Counter.GetNextSeq error: %v", err)
		return resp, e.ErrGetCounter.AddDesc(err.Error())
	}
	resp.TaskID = nextTaskID

	workflowTask.TaskID = nextTaskID
	workflowTask.TaskCreator = user
	workflowTask.TaskRevoker = user
	workflowTask.CreateTime = time.Now().Unix()
	workflowTask.WorkflowName = workflow.Name
	workflowTask.ProjectName = workflow.Project

	for _, stage := range workflow.Stages {
		stageTask := &commonmodels.StageTask{
			Name:     stage.Name,
			Parallel: stage.Parallel,
			Approval: stage.Approval,
		}
		workflowTask.Stages = append(workflowTask.Stages, stageTask)
		for _, job := range stage.Jobs {
			if job.JobType == config.JobZadigBuild {
				if err := setZadigBuildRepos(job, log); err != nil {
					log.Errorf("set build info error: %v", err)
					return resp, e.ErrCreateTask.AddDesc(err.Error())
				}
			}
			jobs, err := jobctl.ToJobs(job, workflow, nextTaskID)
			if err != nil {
				log.Errorf("cannot create workflow %s, the error is: %v", workflow.Name, err)
				return resp, e.ErrCreateTask.AddDesc(err.Error())
			}
			stageTask.Jobs = append(stageTask.Jobs, jobs...)
		}
	}

	if err := workflowTaskLint(workflowTask, log); err != nil {
		return resp, err
	}

	workflowTask.WorkflowArgs = workflow

	if err := workflowcontroller.CreateTask(workflowTask); err != nil {
		log.Errorf("create workflow task error: %v", err)
		return resp, e.ErrCreateTask.AddDesc(err.Error())
	}
	return resp, nil
}

func CloneWorkflowTaskV4(workflowName string, taskID int64, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4, error) {
	task, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("find workflowTaskV4 error: %s", err)
		return nil, e.ErrGetTask.AddErr(err)
	}
	return task.WorkflowArgs, nil
}

func UpdateWorkflowTaskV4(id string, workflowTask *commonmodels.WorkflowTask, logger *zap.SugaredLogger) error {
	err := commonrepo.NewworkflowTaskv4Coll().Update(
		id,
		workflowTask,
	)
	if err != nil {
		logger.Errorf("update workflowTaskV4 error: %s", err)
		return e.ErrCreateTask.AddErr(err)
	}
	return nil
}

func ListWorkflowTaskV4(workflowName string, pageNum, pageSize int64, logger *zap.SugaredLogger) ([]*commonmodels.WorkflowTask, int64, error) {
	resp, total, err := commonrepo.NewworkflowTaskv4Coll().List(&commonrepo.ListWorkflowTaskV4Option{WorkflowName: workflowName, Limit: int(pageSize), Skip: int((pageNum - 1) * pageSize)})
	if err != nil {
		logger.Errorf("list workflowTaskV4 error: %s", err)
		return resp, total, err
	}
	return resp, total, nil
}

func CancelWorkflowTaskV4(userName, workflowName string, taskID int64, logger *zap.SugaredLogger) error {
	if err := workflowcontroller.CancelWorkflowTask(userName, workflowName, taskID, logger); err != nil {
		logger.Errorf("cancel workflowTaskV4 error: %s", err)
		return e.ErrCancelTask.AddErr(err)
	}
	return nil
}

func GetWorkflowTaskV4(workflowName string, taskID int64, logger *zap.SugaredLogger) (*WorkflowTaskPreview, error) {
	task, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("find workflowTaskV4 error: %s", err)
		return nil, err
	}
	resp := &WorkflowTaskPreview{
		TaskID:       task.TaskID,
		WorkflowName: task.WorkflowName,
		ProjectName:  task.ProjectName,
		Status:       task.Status,
		TaskCreator:  task.TaskCreator,
		TaskRevoker:  task.TaskRevoker,
		CreateTime:   task.CreateTime,
		StartTime:    task.StartTime,
		EndTime:      task.EndTime,
		Error:        task.Error,
		IsRestart:    task.IsRestart,
	}
	for _, stage := range task.Stages {
		resp.Stages = append(resp.Stages, &StageTaskPreview{
			Name:      stage.Name,
			Status:    stage.Status,
			StartTime: stage.StartTime,
			EndTime:   stage.EndTime,
			Parallel:  stage.Parallel,
			Approval:  stage.Approval,
			Jobs:      jobsToJobPreviews(stage.Jobs),
		})
	}
	return resp, nil
}

func ApproveStage(workflowName, stageName, userName, userID, comment string, taskID int64, approve bool, logger *zap.SugaredLogger) error {
	if workflowName == "" || stageName == "" || taskID == 0 {
		errMsg := fmt.Sprintf("can not find approved workflow: %s, taskID: %d,stage: %s", workflowName, taskID, stageName)
		logger.Error(errMsg)
		return e.ErrApproveTask.AddDesc(errMsg)
	}
	if err := workflowcontroller.ApproveStage(workflowName, stageName, userName, userID, comment, taskID, approve); err != nil {
		logger.Error(err)
		return e.ErrApproveTask.AddErr(err)
	}
	return nil
}

func jobsToJobPreviews(jobs []*commonmodels.JobTask) []*JobTaskPreview {
	resp := []*JobTaskPreview{}
	for _, job := range jobs {
		jobPreview := &JobTaskPreview{
			Name:      job.Name,
			Status:    job.Status,
			StartTime: job.StartTime,
			EndTime:   job.EndTime,
			Error:     job.Error,
			JobType:   job.JobType,
		}
		switch job.JobType {
		case string(config.JobZadigBuild):
			spec := ZadigBuildJobSpec{}
			for _, arg := range job.Properties.Args {
				if arg.Key == "IMAGE" {
					spec.Image = arg.Value
					continue
				}
				if arg.Key == "SERVICE" {
					spec.ServiceName = arg.Value
					continue
				}
				if arg.Key == "SERVICE_MODULE" {
					spec.ServiceModule = arg.Value
					continue
				}
			}
			for _, step := range job.Steps {
				if step.StepType == config.StepGit {
					stepSpec := &stepspec.StepGitSpec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					spec.Repos = stepSpec.Repos
					continue
				}
			}
			jobPreview.Spec = spec
		case string(config.JobZadigDeploy):
			spec := ZadigDeployJobSpec{}
			for _, step := range job.Steps {
				if step.StepType == config.StepDeploy {
					stepSpec := &stepspec.StepDeploySpec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					spec.Env = stepSpec.Env
					spec.ServiceAndImages = append(spec.ServiceAndImages, &ServiceAndImage{
						ServiceName:   stepSpec.ServiceName,
						ServiceModule: stepSpec.ServiceModule,
						Image:         stepSpec.Image,
					})
					continue
				}
				if step.StepType == config.StepHelmDeploy {
					stepSpec := &stepspec.StepHelmDeploySpec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					spec.Env = stepSpec.Env
					for _, imageAndmodule := range stepSpec.ImageAndModules {
						spec.ServiceAndImages = append(spec.ServiceAndImages, &ServiceAndImage{
							ServiceName:   stepSpec.ServiceName,
							ServiceModule: imageAndmodule.ServiceModule,
							Image:         imageAndmodule.Image,
						})
					}
					continue
				}
			}
			jobPreview.Spec = spec
		default:
			jobPreview.Spec = job.Steps
		}
		resp = append(resp, jobPreview)
	}
	return resp
}

func setZadigBuildRepos(job *commonmodels.Job, logger *zap.SugaredLogger) error {
	spec := &commonmodels.ZadigBuildJobSpec{}
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return err
	}
	for _, build := range spec.ServiceAndBuilds {
		if err := setManunalBuilds(build.Repos, build.Repos, logger); err != nil {
			return err
		}
	}
	job.Spec = spec
	return nil
}

func workflowTaskLint(workflowTask *commonmodels.WorkflowTask, logger *zap.SugaredLogger) error {
	if len(workflowTask.Stages) <= 0 {
		errMsg := fmt.Sprintf("no stage found in workflow task: %s,taskID: %d", workflowTask.WorkflowName, workflowTask.TaskID)
		logger.Error(errMsg)
		return e.ErrCreateTask.AddDesc(errMsg)
	}
	for _, stage := range workflowTask.Stages {
		if len(stage.Jobs) <= 0 {
			errMsg := fmt.Sprintf("no job found in workflow task: %s,taskID: %d,stage: %s", workflowTask.WorkflowName, workflowTask.TaskID, stage.Name)
			logger.Error(errMsg)
			return e.ErrCreateTask.AddDesc(errMsg)
		}
	}
	return nil
}
