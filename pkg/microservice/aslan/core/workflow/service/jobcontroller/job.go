package jobcontroller

import (
	"context"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/jobcontroller/stepcontroller"
	"go.uber.org/zap"
)

type jobCtl struct {
	job           *commonmodels.JobTask
	jobContext    *sync.Map
	globalContext *sync.Map
	logger        *zap.SugaredLogger
	ack           func() error
}

func NewJobCtl(job *commonmodels.JobTask, globalContext *sync.Map, ack func() error, logger *zap.SugaredLogger) *jobCtl {
	return &jobCtl{
		job:           job,
		globalContext: globalContext,
		logger:        logger,
		ack:           ack,
	}
}

func (c *jobCtl) Run(ctx context.Context) {
	defer c.stop(ctx)
	defer c.ack()
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

func (c *jobCtl) run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()
	// TODO create pods
	for _, step := range c.job.Steps {
		done := make(chan struct{}, 1)
		go func(stepTask *stepcontroller.StepTask) {
			stepTask.Run(ctx, c.globalContext, c.jobContext, c.ack, c.job.Namepace, c.job.PodName, c.logger)
			done <- struct{}{}
		}((*stepcontroller.StepTask)(step))
		select {
		case <-done:
			return
		case <-ctx.Done():
			step.Status = config.StatusCancelled
			return
		}
	}
}

func (c *jobCtl) stop(ctx context.Context) {
	// TODO delete pods
}
