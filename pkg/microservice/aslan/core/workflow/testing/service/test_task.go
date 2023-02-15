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
	"fmt"
	"sort"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/scmnotify"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type CreateTaskResp struct {
	PipelineName string `json:"pipeline_name"`
	TaskID       int64  `json:"task_id"`
}

func CreateTestTask(args *commonmodels.TestTaskArgs, log *zap.SugaredLogger) (*CreateTaskResp, error) {
	if args == nil {
		return nil, fmt.Errorf("args should not be nil")
	}
	_, err := commonrepo.NewTestingColl().Find(args.TestName, args.ProductName)
	if err != nil {
		log.Errorf("find test[%s] error: %v", args.TestName, err)
		return nil, fmt.Errorf("find test[%s] error: %v", args.TestName, err)
	}

	pipelineName := fmt.Sprintf("%s-%s", args.TestName, "job")

	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.TestTaskFmt, pipelineName))
	if err != nil {
		log.Errorf("CreateTestTask Counter.GetNextSeq error: %v", err)
		return nil, e.ErrGetCounter.AddDesc(err.Error())
	}

	configPayload := commonservice.GetConfigPayload(0)

	defaultS3Store, err := s3.FindDefaultS3()
	if err != nil {
		err = e.ErrFindDefaultS3Storage.AddDesc("default storage is required by distribute task")
		return nil, err
	}

	defaultURL, err := defaultS3Store.GetEncryptedURL()
	if err != nil {
		err = e.ErrS3Storage.AddErr(err)
		return nil, err
	}

	pipelineTask := &task.Task{
		TaskID:       nextTaskID,
		PipelineName: pipelineName,
		ProductName:  args.ProductName,
	}

	stages := make([]*commonmodels.Stage, 0)
	testTask, err := workflowservice.TestArgsToTestSubtask(args, pipelineTask, log)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCreateTask.AddDesc(err.Error())
	}

	workflowservice.FmtBuilds(testTask.JobCtx.Builds, log)
	testSubTask, err := testTask.ToSubTask()
	if err != nil {
		log.Error(err)
		return nil, e.ErrCreateTask.AddDesc(err.Error())
	}

	err = workflowservice.SetCandidateRegistry(configPayload, log)
	if err != nil {
		return nil, err
	}

	workflowservice.AddSubtaskToStage(&stages, testSubTask, testTask.TestModuleName)

	sort.Sort(workflowservice.ByStageKind(stages))

	triggerBy := &commonmodels.TriggerBy{
		CodehostID:     args.CodehostID,
		RepoOwner:      args.RepoOwner,
		RepoName:       args.RepoName,
		Source:         args.Source,
		MergeRequestID: args.MergeRequestID,
		CommitID:       args.CommitID,
		Ref:            args.Ref,
		EventType:      args.EventType,
	}
	task := &task.Task{
		TaskID:        nextTaskID,
		Type:          config.TestType,
		ProductName:   args.ProductName,
		PipelineName:  pipelineName,
		TaskCreator:   args.TestTaskCreator,
		Status:        config.StatusCreated,
		Stages:        stages,
		TestArgs:      args,
		ConfigPayload: configPayload,
		StorageURI:    defaultURL,
		TriggerBy:     triggerBy,
	}

	if len(task.Stages) <= 0 {
		return nil, e.ErrCreateTask.AddDesc(e.PipelineSubTaskNotFoundErrMsg)
	}

	if err := workflowservice.CreateTask(task); err != nil {
		log.Error(err)
		return nil, e.ErrCreateTask
	}

	_ = scmnotify.NewService().UpdateWebhookCommentForTest(task, log)
	resp := &CreateTaskResp{PipelineName: pipelineName, TaskID: nextTaskID}
	return resp, nil
}
