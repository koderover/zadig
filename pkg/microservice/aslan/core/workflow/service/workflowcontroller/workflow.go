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
	"fmt"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	uuid "github.com/satori/go.uuid"
	"go.uber.org/zap"
)

var cancelChannelMap sync.Map

type workflowCtl struct {
	workflowTask       *commonmodels.WorkflowTask
	globalContextMutex sync.RWMutex
	logger             *zap.SugaredLogger
	ack                func()
}

func NewWorkflowController(workflowTask *commonmodels.WorkflowTask, logger *zap.SugaredLogger) *workflowCtl {
	ctl := &workflowCtl{
		workflowTask: workflowTask,
		logger:       logger,
	}
	ctl.ack = ctl.updateWorkflowTask
	return ctl
}

func CancelWorkflowTask(userName, workflowName string, taskID int64, logger *zap.SugaredLogger) error {
	t, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("[%s] task: %s:%d not found", userName, workflowName, taskID)
		return err
	}

	if t.Status == config.StatusPassed {
		logger.Errorf("[%s] task: %s:%d is passed, cannot cancel", userName, workflowName, taskID)
		return fmt.Errorf("task: %s:%d is passed, cannot cancel", workflowName, taskID)
	}

	t.Status = config.StatusCancelled
	t.TaskRevoker = userName

	logger.Infof("[%s] CancelRunningTask %s:%d", userName, taskID, taskID)

	if err := commonrepo.NewworkflowTaskv4Coll().Update(t.ID.Hex(), t); err != nil {
		logger.Errorf("[%s] update task: %s:%d error: %v", userName, workflowName, taskID, err)
		return err
	}

	q := ConvertTaskToQueue(t)
	if err := Remove(q); err != nil {
		logger.Errorf("[%s] remove queue task: %s:%d error: %v", userName, workflowName, taskID, err)
		return err
	}

	value, ok := cancelChannelMap.Load(fmt.Sprintf("%s-%d", workflowName, taskID))
	if !ok {
		logger.Errorf("no mactched task found, id: %d, workflow name: %s", taskID, workflowName)
		return nil
	}
	if f, ok := value.(context.CancelFunc); ok {
		f()
		return nil
	}
	return fmt.Errorf("cancel func type mismatched, id: %d, workflow name: %s", taskID, workflowName)
}

func (c *workflowCtl) Run(ctx context.Context, concurrency int) {
	if c.workflowTask.GlobalContext == nil {
		c.workflowTask.GlobalContext = make(map[string]string)
	}
	c.workflowTask.Status = config.StatusRunning
	c.workflowTask.StartTime = time.Now().Unix()
	c.ack()
	c.logger.Infof("start workflow: %s,status: %s", c.workflowTask.WorkflowName, c.workflowTask.Status)
	defer func() {
		c.workflowTask.EndTime = time.Now().Unix()
		c.logger.Infof("finish workflow: %s,status: %s", c.workflowTask.WorkflowName, c.workflowTask.Status)
		c.ack()
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cancelKey := fmt.Sprintf("%s-%d", c.workflowTask.WorkflowName, c.workflowTask.TaskID)
	cancelChannelMap.Store(cancelKey, cancel)
	defer cancelChannelMap.Delete(cancelKey)

	workflowCtx := &commonmodels.WorkflowTaskCtx{
		WorkflowName:      c.workflowTask.WorkflowName,
		ProjectName:       c.workflowTask.ProjectName,
		TaskID:            c.workflowTask.TaskID,
		DistDir:           fmt.Sprintf("%s/%s/dist/%d", config.S3StoragePath(), c.workflowTask.WorkflowName, c.workflowTask.TaskID),
		DockerMountDir:    fmt.Sprintf("/tmp/%s/docker/%d", uuid.NewV4(), time.Now().Unix()),
		ConfigMapMountDir: fmt.Sprintf("/tmp/%s/cm/%d", uuid.NewV4(), time.Now().Unix()),
		WorkflowKeyVals:   c.workflowTask.KeyVals,
		GlobalContextGet:  c.getGlobalContext,
		GlobalContextSet:  c.setGlobalContext,
		GlobalContextEach: c.globalContextEach,
	}

	RunStages(ctx, c.workflowTask.Stages, workflowCtx, concurrency, c.logger, c.ack)
	updateworkflowStatus(c.workflowTask)
}

func updateworkflowStatus(workflow *commonmodels.WorkflowTask) {
	statusMap := map[config.Status]int{
		config.StatusReject:    5,
		config.StatusCancelled: 4,
		config.StatusTimeout:   3,
		config.StatusFailed:    2,
		config.StatusPassed:    1,
		config.StatusSkipped:   0,
	}

	// 初始化workflowStatus为创建状态
	workflowStatus := config.StatusRunning

	stageStatus := make([]int, len(workflow.Stages))

	for i, j := range workflow.Stages {
		statusCode, ok := statusMap[j.Status]
		if !ok {
			statusCode = -1
		}
		stageStatus[i] = statusCode
	}
	var workflowStatusCode int
	for i, code := range stageStatus {
		if i == 0 || code > workflowStatusCode {
			workflowStatusCode = code
		}
	}

	for taskstatus, code := range statusMap {
		if workflowStatusCode == code {
			workflowStatus = taskstatus
			break
		}
	}
	workflow.Status = workflowStatus
}

func (c *workflowCtl) updateWorkflowTask() {
	taskInColl, err := commonrepo.NewworkflowTaskv4Coll().Find(c.workflowTask.WorkflowName, c.workflowTask.TaskID)
	if err != nil {
		c.logger.Errorf("find workflow task v4 %s failed,error: %v", c.workflowTask.WorkflowName, err)
		return
	}
	// 如果当前状态已经通过或者失败, 不处理新接受到的ACK
	if taskInColl.Status == config.StatusPassed || taskInColl.Status == config.StatusFailed || taskInColl.Status == config.StatusTimeout {
		c.logger.Infof("%s:%d:%s task already done", c.workflowTask.WorkflowName, c.workflowTask.TaskID, taskInColl.Status)
		return
	}
	// 如果当前状态已经是取消状态, 一般为用户取消了任务, 此时任务在短暂时间内会继续运行一段时间,
	if taskInColl.Status == config.StatusCancelled {
		// Task终止状态可能为Pass, Fail, Cancel, Timeout
		// backend 会继续接受到ACK, 在这种情况下, 终止状态之外的ACK都无需处理，避免出现取消之后又被重置成运行态
		if c.workflowTask.Status != config.StatusFailed && c.workflowTask.Status != config.StatusPassed && c.workflowTask.Status != config.StatusCancelled && c.workflowTask.Status != config.StatusTimeout {
			c.logger.Infof("%s:%d task has been cancelled, ACK dropped", c.workflowTask.WorkflowName, c.workflowTask.TaskID)
			return
		}
	}
	if success := UpdateQueue(c.workflowTask); !success {
		c.logger.Errorf("%s:%d update t status error", c.workflowTask.WorkflowName, c.workflowTask.TaskID)
	}
	// TODO update workflow task
	if err := commonrepo.NewworkflowTaskv4Coll().Update(c.workflowTask.ID.Hex(), c.workflowTask); err != nil {
		c.logger.Errorf("update workflow task v4 failed,error: %v", err)
	}

	if c.workflowTask.Status == config.StatusPassed || c.workflowTask.Status == config.StatusFailed || c.workflowTask.Status == config.StatusTimeout || c.workflowTask.Status == config.StatusCancelled {
		c.logger.Infof("%s:%d:%v task done", c.workflowTask.WorkflowName, c.workflowTask.TaskID, c.workflowTask.Status)
		q := ConvertTaskToQueue(c.workflowTask)
		if err := Remove(q); err != nil {
			c.logger.Errorf("remove queue task: %s:%d error: %v", c.workflowTask.WorkflowName, c.workflowTask.TaskID, err)
		}
		result, err := commonrepo.NewStrategyColl().GetByTarget(commonmodels.WorkflowTaskRetention)
		if err != nil {
			c.logger.Errorf("get workflow task retention strategy error: %s", err)
			result = commonmodels.DefaultWorkflowTaskRetention
		}
		if err = commonrepo.NewworkflowTaskv4Coll().ArchiveHistoryWorkflowTask(c.workflowTask.WorkflowName, result.Retention.MaxItems, result.Retention.MaxDays); err != nil {
			c.logger.Errorf("ArchiveHistoryWorkflowTask error: %v", err)
		}
	}

}

func (c *workflowCtl) getGlobalContext(key string) (string, bool) {
	c.globalContextMutex.RLock()
	defer c.globalContextMutex.RUnlock()
	v, existed := c.workflowTask.GlobalContext[key]
	return v, existed
}

func (c *workflowCtl) setGlobalContext(key, value string) {
	c.globalContextMutex.Lock()
	defer c.globalContextMutex.Unlock()
	c.workflowTask.GlobalContext[key] = value
}

func (c *workflowCtl) globalContextEach(f func(k, v string) bool) {
	c.globalContextMutex.RLock()
	defer c.globalContextMutex.RUnlock()
	for k, v := range c.workflowTask.GlobalContext {
		if !f(k, v) {
			return
		}
	}
}
