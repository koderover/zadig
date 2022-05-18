package jobcontroller

import (
	"context"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/stepcontroller"
	"go.uber.org/zap"
)

type JobCtl struct {
	job           *commonmodels.JobTask
	jobContext    *sync.Map
	globalContext *sync.Map
	logger        *zap.SugaredLogger
	Ctx           context.Context
	ack           func() error
}

func NewJobCtl(ctx context.Context, job *commonmodels.JobTask, globalContext *sync.Map, ack func() error, logger *zap.SugaredLogger) *JobCtl {
	return &JobCtl{
		job:           job,
		globalContext: globalContext,
		logger:        logger,
		ack:           ack,
		Ctx:           ctx,
	}
}

func (c *JobCtl) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.job.Status = config.StatusRunning
	c.job.StartTime = time.Now().Unix()
	c.logger.Infof("start job: %s,status: %s", c.job.Name, c.job.Status)
	defer func() {
		c.job.EndTime = time.Now().Unix()
		c.logger.Infof("finish job: %s,status: %s", c.job.Name, c.job.Status)
		c.stop(ctx)
		c.ack()
		cancel()
	}()
	done := make(chan struct{}, 1)
	go func() {
		c.run(ctx)
		done <- struct{}{}
	}()
	select {
	case <-done:
		return
	case <-ctx.Done():
		c.job.Status = config.StatusCancelled
		return
	case <-time.After(time.Minute * time.Duration(c.job.Properties.Timeout)):
		c.job.Status = config.StatusTimeout
		return
	}
}

func (c *JobCtl) run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()
	// TODO create pods
	for _, step := range c.job.Steps {
		(*stepcontroller.StepTask)(step).Run(ctx, c.globalContext, c.jobContext, c.ack, c.logger)
		if step.Status != config.StatusPassed {
			c.job.Status = step.Status
			return
		}

		// done := make(chan struct{}, 1)
		// go func(stepTask *stepcontroller.StepTask) {
		// 	stepTask.Run(ctx, c.globalContext, c.jobContext, c.ack, c.logger)
		// 	done <- struct{}{}
		// }((*stepcontroller.StepTask)(step))
		// select {
		// case <-done:
		// 	return
		// case <-ctx.Done():
		// 	step.Status = config.StatusCancelled
		// 	return
		// }
	}
	c.job.Status = config.StatusPassed
}

func (c *JobCtl) stop(ctx context.Context) {
	// TODO delete pods
}
