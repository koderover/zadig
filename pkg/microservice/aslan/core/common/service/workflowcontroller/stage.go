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
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	approvalservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/approval"
)

type StageCtl interface {
	Run(ctx context.Context, concurrency int)
}

func runStage(ctx context.Context, stage *commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, logger *zap.SugaredLogger, ack func()) {
	stage.Status = config.StatusRunning
	ack()
	logger.Infof("start stage: %s,status: %s", stage.Name, stage.Status)
	wait, err := waitForManualExec(ctx, stage, workflowCtx, logger, ack)
	if err != nil {
		stage.Error = err.Error()
		logger.Errorf("finish stage: %s,status: %s error: %s", stage.Name, stage.Status, stage.Error)
		ack()
		return
	}
	if wait {
		logger.Infof("wait stage to manual execute: %s,status: %s", stage.Name, stage.Status)
		return
	}

	defer func() {
		updateStageStatus(ctx, stage)
		stage.EndTime = time.Now().Unix()
		logger.Infof("finish stage: %s,status: %s", stage.Name, stage.Status)
		ack()
	}()
	stage.StartTime = time.Now().Unix()
	ack()
	stageCtl := NewCustomStageCtl(stage, workflowCtx, logger, ack)

	stageCtl.Run(ctx, concurrency)
	stageCtl.AfterRun()
}

func RunStages(ctx context.Context, stages []*commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, logger *zap.SugaredLogger, ack func()) {
	for _, stage := range stages {
		// should skip passed stage when workflow task be restarted
		if stage.Status == config.StatusPassed {
			continue
		}
		runStage(ctx, stage, workflowCtx, concurrency, logger, ack)
		if statusStopped(stage.Status) {
			return
		}
	}
}

func ApproveStage(workflowName, stageName, userName, userID, comment string, taskID int64, approve bool) error {
	approveKey := fmt.Sprintf("%s-%d-%s", workflowName, taskID, stageName)
	_, err := approvalservice.GlobalApproveMap.DoApproval(approveKey, userName, userID, comment, approve)
	return err
}

func waitForManualExec(ctx context.Context, stage *commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) (wait bool, err error) {
	if stage.ManualExec == nil {
		return false, nil
	}
	if !stage.ManualExec.Enabled {
		return false, nil
	}
	if stage.ManualExec.Excuted {
		return false, nil
	}

	// workflowCtx.SetStatus contain ack() function, so we don't need to call ack() here
	stage.Status = config.StatusPause
	workflowCtx.SetStatus(config.StatusPause)

	return true, err
}

func statusStopped(status config.Status) bool {
	if status == config.StatusCancelled || status == config.StatusFailed ||
		status == config.StatusTimeout || status == config.StatusReject ||
		status == config.StatusPause {
		return true
	}
	return false
}

func updateStageStatus(ctx context.Context, stage *commonmodels.StageTask) {
	select {
	case <-ctx.Done():
		stage.Status = config.StatusCancelled
		return
	default:
	}
	statusMap := map[config.Status]int{
		config.StatusCancelled: 6,
		config.StatusTimeout:   5,
		config.StatusFailed:    4,
		config.StatusPause:     3,
		config.StatusReject:    2,
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
