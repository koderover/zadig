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

package workflow

import (
	"fmt"
	"sort"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type TaskResp struct {
	Name   string `json:"name"`
	TaskID int64  `json:"task_id"`
}

func CreateWorkflowTaskV3(args *commonmodels.WorkflowV3Args, username, reqID string, log *zap.SugaredLogger) (*TaskResp, error) {
	workflowV3, err := commonrepo.NewWorkflowV3Coll().GetByID(args.ID)
	if err != nil {
		log.Errorf("workflowV3.Find %s error: %s", args.Name, err)
		return nil, e.ErrCreateTask.AddDesc(e.FindPipelineErrMsg)
	}

	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.WorkflowTaskV3Fmt, args.Name))
	if err != nil {
		log.Errorf("Counter.GetNextSeq error: %s", err)
		return nil, e.ErrGetCounter.AddDesc(err.Error())
	}

	// 自定义基础镜像的镜像名称可能会被更新，需要使用ID获取最新的镜像名称
	for i, subTask := range workflowV3.SubTasks {
		pre, err := base.ToPreview(subTask)
		if err != nil {
			log.Errorf("subTask.ToPreview error: %s", err)
			continue
		}
		switch pre.TaskType {
		case config.TaskBuildV3:
			build, err := base.ToBuildTask(subTask)
			if err != nil || build == nil {
				log.Errorf("subTask.ToBuildTask error: %s", err)
				continue
			}
			if build.ImageID == "" {
				continue
			}
			basicImage, err := commonrepo.NewBasicImageColl().Find(build.ImageID)
			if err != nil {
				log.Errorf("BasicImage.Find failed, id:%s, err:%s", build.ImageID, err)
				continue
			}
			build.BuildOS = basicImage.Value

			// 创建任务时可以临时编辑环境变量，需要将pipeline中的环境变量更新成新的值。
			if args.BuildArgs != nil {
				build.JobCtx.EnvVars = args.BuildArgs
			}
			build.ServiceName = fmt.Sprintf("%s-job", args.Name)
			workflowV3.SubTasks[i], err = build.ToSubTask()
			if err != nil {
				log.Errorf("build.ToSubTask error: %s", err)
				continue
			}
		case config.TaskTrigger:
			trigger, err := base.ToTriggerTask(subTask)
			if err != nil || trigger == nil {
				log.Errorf("subTask.ToTriggerTask error: %s", err)
				continue
			}
			workflowV3.SubTasks[i], err = trigger.ToSubTask()
		}
	}

	var defaultStorageURI string
	if defaultS3, err := s3.FindDefaultS3(); err == nil {
		defaultStorageURI, err = defaultS3.GetEncryptedURL()
		if err != nil {
			return nil, e.ErrS3Storage.AddErr(err)
		}
	}

	pt := &task.Task{
		TaskID:        nextTaskID,
		ProductName:   args.ProjectName,
		PipelineName:  args.Name,
		Type:          config.WorkflowTypeV3,
		TaskCreator:   username,
		ReqID:         reqID,
		TaskArgs:      &commonmodels.TaskArgs{Builds: args.Builds, BuildArgs: args.BuildArgs},
		Status:        config.StatusCreated,
		SubTasks:      workflowV3.SubTasks,
		ConfigPayload: commonservice.GetConfigPayload(0),
		StorageURI:    defaultStorageURI,
	}

	sort.Sort(ByTaskKind(pt.SubTasks))

	if err := ensurePipelineTask(&task.TaskOpt{
		Task: pt,
	}, log); err != nil {
		log.Errorf("Service.ensurePipelineTask failed %v %s", args, err)
		if err, ok := err.(*ContainerNotFound); ok {
			return nil, e.NewWithExtras(
				e.ErrCreateTaskFailed,
				"container doesn't exists", map[string]interface{}{
					"productName":   err.ProductName,
					"envName":       err.EnvName,
					"serviceName":   err.ServiceName,
					"containerName": err.Container,
				})
		}

		return nil, e.ErrCreateTask.AddDesc(err.Error())
	}

	if len(pt.SubTasks) <= 0 {
		return nil, e.ErrCreateTask.AddDesc(e.PipelineSubTaskNotFoundErrMsg)
	}

	// send to queue to execute task
	if err := CreateTask(pt); err != nil {
		log.Error(err)
		return nil, e.ErrCreateTask
	}

	resp := &TaskResp{
		Name:   args.Name,
		TaskID: nextTaskID,
	}
	return resp, nil
}

func RestartWorkflowTaskV3(userName string, taskID int64, workflowName string, typeString config.PipelineType, log *zap.SugaredLogger) error {
	t, err := commonrepo.NewTaskColl().Find(taskID, workflowName, typeString)
	if err != nil {
		log.Errorf("[%d:%s] find workflow task v3 error: %s", taskID, workflowName, err)
		return e.ErrRestartTask.AddDesc(e.FindPipelineTaskErrMsg)
	}

	// 不重试已经成功的workflow task
	if t.Status == config.StatusRunning || t.Status == config.StatusPassed {
		log.Errorf("cannot restart running or passed task. Status: %v", t.Status)
		return e.ErrRestartTask.AddDesc(e.RestartPassedTaskErrMsg)
	}

	t.IsRestart = true
	t.Status = config.StatusCreated
	t.TaskCreator = userName
	if err := UpdateTask(t); err != nil {
		log.Errorf("update workflow task v3 error: %s", err)
		return e.ErrRestartTask.AddDesc(e.UpdatePipelineTaskErrMsg)
	}
	return nil
}

// ListWorkflowTasksV3Result 工作流任务分页信息
func ListWorkflowTasksV3Result(name string, typeString config.PipelineType, maxResult, startAt int, log *zap.SugaredLogger) (*TaskResult, error) {
	ret := &TaskResult{MaxResult: maxResult, StartAt: startAt}
	var err error
	ret.Data, err = commonrepo.NewTaskColl().List(&commonrepo.ListTaskOption{PipelineName: name, Limit: maxResult, Skip: startAt, Detail: true, Type: typeString})
	if err != nil {
		log.Errorf("PipelineTaskV2.List error: %s", err)
		return ret, e.ErrListTasks
	}

	pipelineList := []string{name}
	ret.Total, err = commonrepo.NewTaskColl().Count(&commonrepo.CountTaskOption{PipelineNames: pipelineList, Type: typeString})
	if err != nil {
		log.Errorf("workflowTaskV3.List error: %s", err)
		return ret, e.ErrCountTasks
	}
	return ret, nil
}

func GetWorkflowTaskV3(taskID int64, pipelineName string, typeString config.PipelineType, log *zap.SugaredLogger) (*task.Task, error) {
	resp, err := commonrepo.NewTaskColl().Find(taskID, pipelineName, typeString)
	if err != nil {
		log.Errorf("[%d:%s] PipelineTaskV2.Find error: %s", taskID, pipelineName, err)
		return resp, e.ErrGetTask
	}

	CleanWorkflow3(resp)
	return resp, nil
}

func GetWorkflowTaskV3Callback(taskID int64, pipelineName string, logger *zap.SugaredLogger) (*commonmodels.CallbackRequest, error) {
	return commonrepo.NewCallbackRequestColl().Find(&commonrepo.CallbackFindOption{
		TaskID:       taskID,
		PipelineName: pipelineName,
	})
}

func CleanWorkflow3(task *task.Task) {
	task.ConfigPayload = nil
	task.TaskArgs = nil

	EnsureSubTasksV3Resp(task.SubTasks)
	for _, stage := range task.Stages {
		ensureSubStageV3Resp(stage)
	}
}

// EnsureSubTasksResp 确保SubTask中敏感信息和其他不必要信息不返回给前端
func EnsureSubTasksV3Resp(subTasks []map[string]interface{}) {
	for i, subTask := range subTasks {
		pre, err := base.ToPreview(subTask)
		if err != nil {
			continue
		}

		switch pre.TaskType {
		case config.TaskBuildV3:
			if newTask, err := ensureBuildTask(subTask); err == nil {
				subTasks[i] = newTask
			}
		}

	}
}

func ensureSubStageV3Resp(stage *commonmodels.Stage) {
	subTasks := stage.SubTasks
	for key, subTask := range subTasks {
		pre, err := base.ToPreview(subTask)
		if err != nil {
			continue
		}

		switch pre.TaskType {
		case config.TaskBuildV3:
			if newTask, err := ensureBuildTask(subTask); err == nil {
				subTasks[key] = newTask
			}
		}
	}
}
