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
	"context"
	"fmt"
	"time"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	jobctl "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow/job"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflowcontroller"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

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

func CreateWorkflowTaskV4(user string, workflow *commonmodels.WorkflowV4, log *zap.SugaredLogger) error {
	workflowTask := &commonmodels.WorkflowTask{}
	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.WorkflowTaskV4Fmt, workflow.Name))
	if err != nil {
		log.Errorf("Counter.GetNextSeq error: %v", err)
		return e.ErrGetCounter.AddDesc(err.Error())
	}
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
			if err := setZadigBuildRepos(job, log); err != nil {
				log.Errorf("set build info error: %v", err)
				return e.ErrCreateTask.AddDesc(err.Error())
			}
			jobs, err := jobctl.ToJobs(job, workflow, nextTaskID)
			if err != nil {
				log.Errorf("cannot create workflow %s, the error is: %v", workflow.Name, err)
				return e.ErrCreateTask.AddDesc(err.Error())
			}
			stageTask.Jobs = append(stageTask.Jobs, jobs...)
		}

	}
	idString, err := commonrepo.NewworkflowTaskv4Coll().Create(workflowTask)
	if err != nil {
		log.Errorf("cannot create workflow task %s, the error is: %v", workflow.Name, err)
		return e.ErrCreateTask.AddDesc(err.Error())
	}
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		log.Errorf("transform task id error: %v", err)
		return e.ErrCreateTask.AddDesc(err.Error())
	}
	ctx := context.Background()
	workflowTask.ID = id
	go workflowcontroller.NewWorkflowController(workflowTask, log).Run(ctx, 5)
	return nil
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
	resp, total, err := commonrepo.NewworkflowTaskv4Coll().List(&commonrepo.ListWorkflowTaskV4Option{WorkflowName: workflowName}, pageNum, pageSize)
	if err != nil {
		logger.Errorf("list workflowTaskV4 error: %s", err)
		return resp, total, err
	}
	return resp, total, nil
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
