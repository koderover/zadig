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

package workflowcontroller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

func RunningTasks() []*commonmodels.WorkflowQueue {
	tasks := make([]*commonmodels.WorkflowQueue, 0)
	queueTasks, err := commonrepo.NewWorkflowQueueColl().List(&commonrepo.ListWorfklowQueueOption{})
	if err != nil {
		log.Errorf("list queue worklfow task failed, err:%v", err)
		return tasks
	}
	for _, t := range queueTasks {
		// task状态为TaskQueued说明task已经被send到nsq,wd已经开始处理但是没有返回ack
		if t.Status == config.StatusRunning {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func PendingTasks() []*commonmodels.WorkflowQueue {
	tasks := make([]*commonmodels.WorkflowQueue, 0)
	queueTasks, err := commonrepo.NewWorkflowQueueColl().List(&commonrepo.ListWorfklowQueueOption{})
	if err != nil {
		log.Errorf("list queue workflow task failed, err:%v", err)
		return tasks
	}
	for _, t := range queueTasks {
		if t.Status == config.StatusWaiting || t.Status == config.StatusBlocked || t.Status == config.StatusQueued {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

type CancelMessage struct {
	Revoker      string `json:"revoker"`
	PipelineName string `json:"pipeline_name"`
	TaskID       int64  `json:"task_id"`
	ReqID        string `json:"req_id"`
}

// CreateTask 接受create task请求, 保存task到数据库, 发送task到queue
func CreateTask(t *commonmodels.WorkflowTask) error {
	t.Status = config.StatusWaiting
	if _, err := commonrepo.NewworkflowTaskv4Coll().Create(t); err != nil {
		log.Errorf("create workflow task v4 error: %v", err)
		return err
	}
	return Push(t)
}

func UpdateTask(t *commonmodels.WorkflowTask) error {
	t.Status = config.StatusWaiting
	if err := commonrepo.NewworkflowTaskv4Coll().Update(t.ID.Hex(), t); err != nil {
		log.Errorf("create workflow task v4 error: %v", err)
		return err
	}
	return Push(t)
}

func Push(t *commonmodels.WorkflowTask) error {
	if t == nil {
		return errors.New("nil task")
	}

	if !t.MultiRun {
		opt := &commonrepo.ListWorfklowQueueOption{
			WorkflowName: t.WorkflowName,
		}
		tasks, err := commonrepo.NewWorkflowQueueColl().List(opt)
		if err == nil && len(tasks) > 0 {
			log.Infof("blocked task recevied: %v %v %v", t.CreateTime, t.TaskID, t.WorkflowName)
			t.Status = config.StatusBlocked
		}
	}

	if err := commonrepo.NewWorkflowQueueColl().Create(ConvertTaskToQueue(t)); err != nil {
		log.Errorf("workflowTaskV4.Create error: %v", err)
		return err
	}
	return nil
}

func InitWorkflowController() {
	InitQueue()
	go WorfklowTaskSender()
}

func InitQueue() error {
	log := log.SugaredLogger()

	// 从数据库查找未完成的任务
	// status = created, running
	tasks, err := commonrepo.NewworkflowTaskv4Coll().InCompletedTasks()
	if err != nil {
		log.Errorf("find [InCompletedTasks] error: %v", err)
		return err
	}

	for _, task := range tasks {
		// 如果 Queue 重新初始化, 取消所有 running tasks
		if err := CancelWorkflowTask(setting.DefaultTaskRevoker, task.WorkflowName, task.TaskID, log); err != nil {
			log.Errorf("[CancelRunningTask] error: %v", err)
			continue
		}
	}
	return nil
}

// WorfklowTaskSender 监控warpdrive空闲情况, 如果有空闲, 则发现下一个waiting task给warpdrive
// 并将task状态设置为queued
func WorfklowTaskSender() {
	for {
		time.Sleep(time.Second * 3)

		sysSetting, err := commonrepo.NewSystemSettingColl().Get()
		if err != nil {
			log.Errorf("get system stettings error: %v", err)
		}
		//c.checkAgents()
		if !hasAgentAvaiable(int(sysSetting.WorkflowConcurrency)) {
			continue
		}
		t, err := NextWaitingTask()
		if err != nil {
			// no waiting task found
			blockTasks, err := BlockedTaskQueue()
			if err != nil {
				//no blocked task found
				continue
			}
			for _, blockTask := range blockTasks {
				if hasAgentAvaiable(int(sysSetting.WorkflowConcurrency)) {
					//判断相同的工作流是否正在运行
					if ParallelRunningAndQueuedTasks(blockTask) {
						continue
					}
					// update agent and queue
					if err := updateQueueAndRunTask(blockTask, int(sysSetting.BuildConcurrency)); err != nil {
						continue
					}
				} else {
					break
				}
			}
			continue
		}
		// update agent and queue
		if err := updateQueueAndRunTask(t, int(sysSetting.BuildConcurrency)); err != nil {
			continue
		}

	}
}

func hasAgentAvaiable(workflowConcurrency int) bool {
	return len(RunningAndQueuedTasks()) < int(workflowConcurrency)
}

func RunningAndQueuedTasks() []*commonmodels.WorkflowQueue {
	tasks := make([]*commonmodels.WorkflowQueue, 0)
	for _, t := range ListTasks() {
		// task状态为TaskQueued说明task已经被send到nsq,wd已经开始处理但是没有返回ack
		if t.Status == config.StatusRunning || t.Status == config.StatusQueued {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func ListTasks() []*commonmodels.WorkflowQueue {
	opt := new(commonrepo.ListWorfklowQueueOption)
	queues, err := commonrepo.NewWorkflowQueueColl().List(opt)
	if err != nil {
		log.Errorf("pqColl.List error: %v", err)
	}
	return queues
}

// NextWaitingTask 查询下一个等待的task
func NextWaitingTask() (*commonmodels.WorkflowQueue, error) {
	opt := &commonrepo.ListWorfklowQueueOption{
		Status: config.StatusWaiting,
	}

	tasks, err := commonrepo.NewWorkflowQueueColl().List(opt)
	if err != nil {
		return nil, err
	}

	for _, t := range tasks {
		return t, nil
	}

	return nil, errors.New("no waiting task found")
}

func BlockedTaskQueue() ([]*commonmodels.WorkflowQueue, error) {
	opt := &commonrepo.ListWorfklowQueueOption{
		Status: config.StatusBlocked,
	}

	queues, err := commonrepo.NewWorkflowQueueColl().List(opt)
	if err != nil || len(queues) == 0 {
		return nil, errors.New("no blocked task found")
	}

	return queues, nil
}

func ParallelRunningAndQueuedTasks(currentTask *commonmodels.WorkflowQueue) bool {
	for _, t := range ListTasks() {
		// task状态为TaskQueued说明task已经被send到nsq,wd已经开始处理但是没有返回ack
		if t.Status != config.StatusRunning && t.Status != config.StatusQueued {
			continue
		}
		if t.WorkflowName == currentTask.WorkflowName {
			return true
		}
	}
	return false
}

func updateQueueAndRunTask(t *commonmodels.WorkflowQueue, jobConcurrency int) error {
	logger := log.SugaredLogger()
	// 更新队列状态为TaskQueued
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(t.WorkflowName, t.TaskID)
	if err != nil {
		logger.Errorf("%s:%d get workflow task error: %v", t.WorkflowName, t.TaskID, err)
		return fmt.Errorf("%s:%d get workflow task error: %v", t.WorkflowName, t.TaskID, err)
	}
	workflowTask.Status = config.StatusQueued
	if success := UpdateQueue(workflowTask); !success {
		logger.Errorf("%s:%d update t status error", t.WorkflowName, t.TaskID)
		return fmt.Errorf("%s:%d update t status error", t.WorkflowName, t.TaskID)
	}
	ctx := context.Background()
	go NewWorkflowController(workflowTask, logger).Run(ctx, jobConcurrency)
	return nil
}

func UpdateQueue(task *commonmodels.WorkflowTask) bool {
	if err := commonrepo.NewWorkflowQueueColl().Update(ConvertTaskToQueue(task)); err != nil {
		return false
	}
	return true
}

func ConvertTaskToQueue(task *commonmodels.WorkflowTask) *commonmodels.WorkflowQueue {
	return &commonmodels.WorkflowQueue{
		TaskID:              task.TaskID,
		WorkflowName:        task.WorkflowName,
		WorkflowDisplayName: task.WorkflowDisplayName,
		ProjectName:         task.ProjectName,
		Status:              task.Status,
		Stages:              cleanStages(task.Stages),
		TaskCreator:         task.TaskCreator,
		TaskRevoker:         task.TaskRevoker,
		CreateTime:          task.CreateTime,
		MultiRun:            task.MultiRun,
	}
}

func cleanStages(stages []*commonmodels.StageTask) []*commonmodels.StageTask {
	resp := []*commonmodels.StageTask{}
	data, _ := json.Marshal(stages)
	json.Unmarshal(data, &resp)
	for _, stage := range resp {
		for _, job := range stage.Jobs {
			job.Spec = nil
		}
	}
	return resp
}

func Remove(taskQueue *commonmodels.WorkflowQueue) error {
	return commonrepo.NewWorkflowQueueColl().Delete(taskQueue)
}
