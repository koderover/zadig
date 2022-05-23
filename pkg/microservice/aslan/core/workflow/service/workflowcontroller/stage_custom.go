package workflowcontroller

import (
	"context"
	"sync"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/jobcontroller"
	"go.uber.org/zap"
)

type CustomStageCtl struct {
	stage         *commonmodels.StageTask
	globalContext *sync.Map
	logger        *zap.SugaredLogger
	ack           func()
}

func NewCustomStageCtl(stage *commonmodels.StageTask, globalContext *sync.Map, logger *zap.SugaredLogger, ack func()) *CustomStageCtl {
	return &CustomStageCtl{
		stage:         stage,
		globalContext: globalContext,
		logger:        logger,
		ack:           ack,
	}
}

func (c *CustomStageCtl) Run(ctx context.Context, concurrency int) {
	var workerConcurrency = 1
	if c.stage.Parallel {
		if len(c.stage.Jobs) > int(concurrency) {
			workerConcurrency = int(concurrency)
		} else {
			workerConcurrency = len(c.stage.Jobs)
		}

	}
	jobcontroller.RunJobs(ctx, c.stage.Jobs, workerConcurrency, c.globalContext, c.logger, c.ack)
}
