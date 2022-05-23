package workflowcontroller

import (
	"context"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"go.uber.org/zap"
)

type StageCtl interface {
	Run(ctx context.Context, concurrency int)
}

func runStage(ctx context.Context, stage *commonmodels.StageTask, concurrency int, globalContext *sync.Map, logger *zap.SugaredLogger, ack func()) {
	stage.Status = config.StatusRunning
	stage.StartTime = time.Now().Unix()
	ack()
	logger.Infof("start stage: %s,status: %s", stage.Name, stage.Status)
	defer func() {
		updateStageStatus(stage)
		stage.EndTime = time.Now().Unix()
		logger.Infof("finish stage: %s,status: %s", stage.Name, stage.Status)
		ack()
	}()
	var stageCtl StageCtl
	switch stage.StageType {
	case "approve":
		// TODO approval stage
	default:
		stageCtl = NewCustomStageCtl(stage, globalContext, logger, ack)
	}
	stageCtl.Run(ctx, concurrency)
}

func RunStages(ctx context.Context, stages []*commonmodels.StageTask, concurrency int, globalContext *sync.Map, logger *zap.SugaredLogger, ack func()) {
	for _, stage := range stages {
		runStage(ctx, stage, concurrency, globalContext, logger, ack)
	}
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
