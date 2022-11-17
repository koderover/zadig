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

package jobcontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/util/rand"
)

type JobCtl interface {
	Run(ctx context.Context)
	// do some clean stuff when workflow finished, like collect reports or clean up resources.
	Clean(ctx context.Context)
}

func initJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) JobCtl {
	var jobCtl JobCtl
	switch job.JobType {
	case string(config.JobZadigDeploy):
		jobCtl = NewDeployJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobZadigHelmDeploy):
		jobCtl = NewHelmDeployJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobCustomDeploy):
		jobCtl = NewCustomDeployJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobPlugin):
		jobCtl = NewPluginsJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sCanaryDeploy):
		jobCtl = NewCanaryDeployJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sCanaryRelease):
		jobCtl = NewCanaryReleaseJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sBlueGreenDeploy):
		jobCtl = NewBlueGreenDeployJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sBlueGreenRelease):
		jobCtl = NewBlueGreenReleaseJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sGrayRelease):
		jobCtl = NewGrayReleaseJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sGrayRollback):
		jobCtl = NewGrayRollbackJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sPatch):
		jobCtl = NewK8sPatchJobCtl(job, workflowCtx, ack, logger)
	default:
		jobCtl = NewFreestyleJobCtl(job, workflowCtx, ack, logger)
	}
	return jobCtl
}

func runJob(ctx context.Context, job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) {
	// render global variables for every job.
	workflowCtx.GlobalContextEach(func(k, v string) bool {
		b, _ := json.Marshal(job)
		replacedString := strings.ReplaceAll(string(b), fmt.Sprintf(setting.RenderValueTemplate, k), v)
		json.Unmarshal([]byte(replacedString), &job)
		return true
	})
	job.Status = config.StatusRunning
	job.StartTime = time.Now().Unix()
	job.K8sJobName = getJobName(workflowCtx.WorkflowName, workflowCtx.TaskID)
	ack()

	logger.Infof("start job: %s,status: %s", job.Name, job.Status)
	defer func() {
		job.EndTime = time.Now().Unix()
		logger.Infof("finish job: %s,status: %s", job.Name, job.Status)
		ack()
	}()
	jobCtl := initJobCtl(job, workflowCtx, logger, ack)

	jobCtl.Run(ctx)
}

func RunJobs(ctx context.Context, jobs []*commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, logger *zap.SugaredLogger, ack func()) {
	if concurrency == 1 {
		for _, job := range jobs {
			runJob(ctx, job, workflowCtx, logger, ack)
			if jobStatusFailed(job.Status) {
				return
			}
		}
		return
	}
	jobPool := NewPool(ctx, jobs, workflowCtx, concurrency, logger, ack)
	jobPool.Run()
}

func CleanWorkflowJobs(ctx context.Context, workflowTask *commonmodels.WorkflowTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) {
	for _, stage := range workflowTask.Stages {
		for _, job := range stage.Jobs {
			jobCtl := initJobCtl(job, workflowCtx, logger, ack)
			jobCtl.Clean(ctx)
		}
	}
}

// Pool is a worker group that runs a number of tasks at a
// configured concurrency.
type Pool struct {
	Jobs        []*commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	concurrency int
	jobsChan    chan *commonmodels.JobTask
	logger      *zap.SugaredLogger
	ack         func()
	ctx         context.Context
	wg          sync.WaitGroup
}

// NewPool initializes a new pool with the given tasks and
// at the given concurrency.
func NewPool(ctx context.Context, jobs []*commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, logger *zap.SugaredLogger, ack func()) *Pool {
	return &Pool{
		Jobs:        jobs,
		concurrency: concurrency,
		workflowCtx: workflowCtx,
		jobsChan:    make(chan *commonmodels.JobTask),
		logger:      logger,
		ack:         ack,
		ctx:         ctx,
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
		runJob(p.ctx, job, p.workflowCtx, p.logger, p.ack)
		p.wg.Done()
	}
}

func saveFile(src io.Reader, localFile string) error {
	out, err := os.Create(localFile)
	if err != nil {
		return err
	}

	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

func getJobName(workflowName string, taskID int64) string {
	// max lenth of workflowName was 32, so job name was unique in one task.
	base := strings.Replace(
		strings.ToLower(
			fmt.Sprintf(
				"%s-%d-",
				workflowName,
				taskID,
			),
		),
		"_", "-", -1,
	)
	return rand.GenerateName(base)
}

func jobStatusFailed(status config.Status) bool {
	if status == config.StatusCancelled || status == config.StatusFailed || status == config.StatusTimeout || status == config.StatusReject {
		return true
	}
	return false
}

func logError(job *commonmodels.JobTask, msg string, logger *zap.SugaredLogger) {
	logger.Error(msg)
	job.Status = config.StatusFailed
	job.Error = msg
}
