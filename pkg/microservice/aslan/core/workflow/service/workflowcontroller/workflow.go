package workflowcontroller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/jobcontroller"
	"go.uber.org/zap"
)

var approveChannelMap sync.Map
var cancelChannelMap sync.Map

type workflowCtl struct {
	workflowTask  *commonmodels.WorkflowTask
	globalContext *sync.Map
	logger        *zap.SugaredLogger
	ack           func() error
}

func NewWorkflowController(workflowTask *commonmodels.WorkflowTask, logger *zap.SugaredLogger) *workflowCtl {
	ctl := &workflowCtl{
		workflowTask: workflowTask,
		logger:       logger,
	}
	ctl.ack = ctl.updateWorkflowTask
	return ctl
}

func CancelWorkflowTask(workflowName string, id int64) error {
	value, ok := cancelChannelMap.Load(fmt.Sprintf("%s-%d", workflowName, id))
	if !ok {
		return fmt.Errorf("no mactched task found, id: %d, workflow name: %s", id, workflowName)
	}
	if f, ok := value.(context.CancelFunc); ok {
		f()
		return nil
	}
	return fmt.Errorf("cancel func type mismatched, id: %d, workflow name: %s", id, workflowName)
}

func (c *workflowCtl) Run(ctx context.Context, concurrency int) {
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
	cancelChannelMap.Store(fmt.Sprintf("%s-%d", c.workflowTask.WorkflowName, c.workflowTask.TaskID), cancel)
	for _, stage := range c.workflowTask.Stages {
		c.runStage(ctx, stage, concurrency)
		if stage.Status != config.StatusPassed {
			c.workflowTask.Status = stage.Status
			return
		}
	}
	c.workflowTask.Status = config.StatusPassed
}

func (c *workflowCtl) runStage(ctx context.Context, stage *commonmodels.StageTask, concurrency int) {
	stage.Status = config.StatusRunning
	stage.StartTime = time.Now().Unix()
	c.ack()
	c.logger.Infof("start stage: %s,status: %s", stage.Name, stage.Status)
	defer func() {
		updateStageStatus(stage)
		stage.EndTime = time.Now().Unix()
		c.logger.Infof("finish stage: %s,status: %s", stage.Name, stage.Status)
		c.ack()
	}()
	switch stage.StageType {
	case "approve":
		//TODO wait for approve
	default:
		// TODO run job in parallel
		var workerConcurrency = 1
		if stage.Parallel {
			if len(stage.Jobs) > int(concurrency) {
				workerConcurrency = int(concurrency)
			} else {
				workerConcurrency = len(stage.Jobs)
			}

		}
		jobCtls := []*jobcontroller.JobCtl{}
		for _, job := range stage.Jobs {
			jobCtl := jobcontroller.NewJobCtl(ctx, job, c.globalContext, c.ack, c.logger)
			jobCtls = append(jobCtls, jobCtl)
		}
		NewPool(jobCtls, workerConcurrency).Run()
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

func updateworkflowStatus(workflow *commonmodels.WorkflowTask) {
}

func (c *workflowCtl) updateWorkflowTask() error {
	// TODO update workflow task
	return nil
}

// Pool is a worker group that runs a number of tasks at a
// configured concurrency.
type Pool struct {
	Jobs        []*jobcontroller.JobCtl
	concurrency int
	jobsChan    chan *jobcontroller.JobCtl
	wg          sync.WaitGroup
}

// NewPool initializes a new pool with the given tasks and
// at the given concurrency.
func NewPool(jobs []*jobcontroller.JobCtl, concurrency int) *Pool {
	return &Pool{
		Jobs:        jobs,
		concurrency: concurrency,
		jobsChan:    make(chan *jobcontroller.JobCtl),
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
		job.Run(job.Ctx)
		p.wg.Done()
	}
}
