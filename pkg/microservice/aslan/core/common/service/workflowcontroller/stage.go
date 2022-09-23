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

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/instantmessage"
)

type approveMap struct {
	m map[string]*approveWithLock
	sync.RWMutex
}

type approveWithLock struct {
	approval *commonmodels.Approval
	sync.RWMutex
}

var globalApproveMap approveMap

func init() {
	globalApproveMap.m = make(map[string]*approveWithLock, 0)
}

type StageCtl interface {
	Run(ctx context.Context, concurrency int)
}

func runStage(ctx context.Context, stage *commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, logger *zap.SugaredLogger, ack func()) {
	stage.Status = config.StatusRunning
	stage.StartTime = time.Now().Unix()
	ack()
	logger.Infof("start stage: %s,status: %s", stage.Name, stage.Status)
	if err := waitiForApprove(ctx, stage, workflowCtx, logger, ack); err != nil {
		stage.Error = err.Error()
		stage.EndTime = time.Now().Unix()
		logger.Errorf("finish stage: %s,status: %s", stage.Name, stage.Status)
		ack()
		return
	}
	defer func() {
		updateStageStatus(stage)
		stage.EndTime = time.Now().Unix()
		logger.Infof("finish stage: %s,status: %s", stage.Name, stage.Status)
		ack()
	}()

	stageCtl := NewCustomStageCtl(stage, workflowCtx, logger, ack)

	stageCtl.Run(ctx, concurrency)
}

func RunStages(ctx context.Context, stages []*commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, logger *zap.SugaredLogger, ack func()) {
	for _, stage := range stages {
		runStage(ctx, stage, workflowCtx, concurrency, logger, ack)
		if statusFailed(stage.Status) {
			return
		}
	}
}

func ApproveStage(workflowName, stageName, userName, userID, comment string, taskID int64, approve bool) error {
	approveKey := fmt.Sprintf("%s-%d-%s", workflowName, taskID, stageName)
	approveWithL, ok := globalApproveMap.getApproval(approveKey)
	if !ok {
		return fmt.Errorf("workflow %s ID %d stage %s do not need approve", workflowName, taskID, stageName)
	}
	return approveWithL.doApproval(userName, userID, comment, approve)
}

func waitiForApprove(ctx context.Context, stage *commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) error {
	if stage.Approval == nil {
		return nil
	}
	if !stage.Approval.Enabled {
		return nil
	}
	if stage.Approval.Timeout == 0 {
		stage.Approval.Timeout = 60
	}
	approveKey := fmt.Sprintf("%s-%d-%s", workflowCtx.WorkflowName, workflowCtx.TaskID, stage.Name)
	approveWithL := &approveWithLock{approval: stage.Approval}
	globalApproveMap.setApproval(approveKey, approveWithL)
	defer func() {
		globalApproveMap.deleteApproval(approveKey)
		ack()
	}()
	if err := instantmessage.NewWeChatClient().SendWorkflowTaskAproveNotifications(workflowCtx.WorkflowName, workflowCtx.TaskID); err != nil {
		logger.Errorf("send approve notification failed, error: %v", err)
	}

	timeout := time.After(time.Duration(stage.Approval.Timeout) * time.Minute)
	latestApproveCount := 0
	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			stage.Status = config.StatusCancelled
			return fmt.Errorf("workflow was canceled")

		case <-timeout:
			stage.Status = config.StatusCancelled
			return fmt.Errorf("workflow timeout")
		default:
			approved, approveCount, err := approveWithL.isApproval()
			if err != nil {
				stage.Status = config.StatusReject
				return err
			}
			if approved {
				return nil
			}
			if approveCount > latestApproveCount {
				ack()
				latestApproveCount = approveCount
			}
		}
	}
}

func statusFailed(status config.Status) bool {
	if status == config.StatusCancelled || status == config.StatusFailed || status == config.StatusTimeout || status == config.StatusReject {
		return true
	}
	return false
}

func updateStageStatus(stage *commonmodels.StageTask) {
	statusMap := map[config.Status]int{
		config.StatusCancelled: 4,
		config.StatusTimeout:   3,
		config.StatusFailed:    2,
		config.StatusPassed:    1,
		config.StatusSkipped:   0,
	}

	// 初始化stageStatus为创建状态
	stageStatus := config.StatusRunning

	jobStatus := make([]int, len(stage.Jobs))

	for i, j := range stage.Jobs {
		statusCode, ok := statusMap[j.Status]
		if !ok {
			statusCode = -1
		}
		jobStatus[i] = statusCode
	}
	var stageStatusCode int
	for i, code := range jobStatus {
		if i == 0 || code > stageStatusCode {
			stageStatusCode = code
		}
	}

	for taskstatus, code := range statusMap {
		if stageStatusCode == code {
			stageStatus = taskstatus
			break
		}
	}
	stage.Status = stageStatus
}

func (c *approveMap) setApproval(key string, value *approveWithLock) {
	c.Lock()
	defer c.Unlock()
	c.m[key] = value
}

func (c *approveMap) getApproval(key string) (*approveWithLock, bool) {
	c.RLock()
	defer c.RUnlock()
	v, existed := c.m[key]
	return v, existed
}
func (c *approveMap) deleteApproval(key string) {
	c.Lock()
	defer c.Unlock()
	delete(c.m, key)
}

func (c *approveWithLock) isApproval() (bool, int, error) {
	c.Lock()
	defer c.Unlock()
	approveCount := 0
	for _, user := range c.approval.ApproveUsers {
		if user.RejectOrApprove == config.Reject {
			c.approval.RejectOrApprove = config.Reject
			return false, approveCount, fmt.Errorf("%s reject this task", user.UserName)
		}
		if user.RejectOrApprove == config.Approve {
			approveCount++
		}
	}
	if approveCount >= c.approval.NeededApprovers {
		c.approval.RejectOrApprove = config.Approve
		return true, approveCount, nil
	}
	return false, approveCount, nil
}

func (c *approveWithLock) doApproval(userName, userID, comment string, appvove bool) error {
	c.Lock()
	defer c.Unlock()
	for _, user := range c.approval.ApproveUsers {
		if user.UserID != userID {
			continue
		}
		if user.RejectOrApprove != "" {
			return fmt.Errorf("%s have %s already", userName, user.RejectOrApprove)
		}
		user.Comment = comment
		user.OperationTime = time.Now().Unix()
		if appvove {
			user.RejectOrApprove = config.Approve
			return nil
		} else {
			user.RejectOrApprove = config.Reject
			return nil
		}
	}
	return fmt.Errorf("user %s has no authority to approve", userName)
}
