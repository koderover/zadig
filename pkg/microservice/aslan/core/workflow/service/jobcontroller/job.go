package jobcontroller

import (
	"context"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"go.uber.org/zap"
)

type JobCtl interface {
	Run(ctx context.Context)
}

func runJob(ctx context.Context, job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, globalContext *sync.Map, logger *zap.SugaredLogger, ack func()) {
	ctx, cancel := context.WithCancel(ctx)
	job.Status = config.StatusRunning
	job.StartTime = time.Now().Unix()
	logger.Infof("start job: %s,status: %s", job.Name, job.Status)
	defer func() {
		job.EndTime = time.Now().Unix()
		logger.Infof("finish job: %s,status: %s", job.Name, job.Status)
		ack()
		cancel()
	}()
	var jobCtl JobCtl
	switch job.JobType {
	case "deploy":
		// TODO
	default:
		jobCtl = NewFreestyleJobCtl(job, workflowCtx, globalContext, ack, logger)
	}
	done := make(chan struct{}, 1)
	go func() {
		jobCtl.Run(ctx)
		done <- struct{}{}
	}()
	select {
	case <-done:
		return
	case <-ctx.Done():
		job.Status = config.StatusCancelled
		return
	case <-time.After(time.Second * time.Duration(job.Properties.Timeout)):
		job.Status = config.StatusTimeout
		return
	}
}

func RunJobs(ctx context.Context, jobs []*commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, globalContext *sync.Map, logger *zap.SugaredLogger, ack func()) {
	jobPool := NewPool(ctx, jobs, workflowCtx, concurrency, globalContext, logger, ack)
	jobPool.Run()
}

// Pool is a worker group that runs a number of tasks at a
// configured concurrency.
type Pool struct {
	Jobs          []*commonmodels.JobTask
	workflowCtx   *commonmodels.WorkflowTaskCtx
	concurrency   int
	jobsChan      chan *commonmodels.JobTask
	globalContext *sync.Map
	logger        *zap.SugaredLogger
	ack           func()
	ctx           context.Context
	wg            sync.WaitGroup
}

// NewPool initializes a new pool with the given tasks and
// at the given concurrency.
func NewPool(ctx context.Context, jobs []*commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, globalContext *sync.Map, logger *zap.SugaredLogger, ack func()) *Pool {
	return &Pool{
		Jobs:          jobs,
		concurrency:   concurrency,
		workflowCtx:   workflowCtx,
		jobsChan:      make(chan *commonmodels.JobTask),
		globalContext: globalContext,
		logger:        logger,
		ack:           ack,
		ctx:           ctx,
	}
}

// Run runs all job within the pool and blocks until it's
// finished.
func (p *Pool) Run() {
	for i := 0; i < p.concurrency; i++ {
		go p.work()
	}

	p.wg.Add(len(p.Jobs))
	for _, task := range p.Jobs {
		p.jobsChan <- task
	}

	// all workers return
	close(p.jobsChan)

	p.wg.Wait()
}

// The work loop for any single goroutine.
func (p *Pool) work() {
	for job := range p.jobsChan {
		runJob(p.ctx, job, p.workflowCtx, p.globalContext, p.logger, p.ack)
		p.wg.Done()
	}
}
