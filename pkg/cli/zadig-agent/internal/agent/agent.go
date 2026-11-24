/*
Copyright 2023 The KodeRover Authors.

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

package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	jobexecutor "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/job"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/reporter"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/network"
	"github.com/koderover/zadig/v2/pkg/util"
)

func NewAgentController() *AgentController {
	return &AgentController{
		Client:               network.NewZadigClient(),
		StopPollingJobChan:   make(chan struct{}, 1),
		StopRunJobChan:       make(chan struct{}, 1),
		ConcurrencyBlockTime: common.DefaultAgentConcurrencyBlockTime,
	}
}

type AgentController struct {
	Client               *network.ZadigClient
	JobChan              chan *types.ZadigJobTask
	StopPollingJobChan   chan struct{}
	StopRunJobChan       chan struct{}
	Concurrency          int
	ConcurrencyBlockTime int
	mu                   sync.Mutex
	currentJobNum        int
	paused               bool
	polling              bool
	jobError             error
	forceDrain           bool
	jobCancels           map[string]context.CancelCauseFunc
	stopPollingOnce      sync.Once
	stopRunOnce          sync.Once
	WorkingDirectory     string
}

func (c *AgentController) Start(ctx context.Context) {
	c.JobChan = make(chan *types.ZadigJobTask, c.Concurrency)

	go c.PollingJob(ctx)

	go c.RunJob(ctx)
}

func (c *AgentController) StopPollingJob() {
	c.stopPollingOnce.Do(func() { close(c.StopPollingJobChan) })
}

func (c *AgentController) StopRunJob() {
	c.stopRunOnce.Do(func() { close(c.StopRunJobChan) })
}

func (c *AgentController) SetPaused(paused bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paused = paused
	if !paused {
		c.forceDrain = false
	}
}

var errUpgradeCancelled = errors.New("job interrupted by forced agent upgrade")

func (c *AgentController) PauseAndDrain(ctx context.Context, force bool) error {
	c.mu.Lock()
	c.paused, c.forceDrain = true, force
	if force {
		for _, cancel := range c.jobCancels {
			cancel(errUpgradeCancelled)
		}
	}
	c.mu.Unlock()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		c.mu.Lock()
		done, err := !c.polling && c.currentJobNum == 0, c.jobError
		c.mu.Unlock()
		if done {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *AgentController) beginPoll() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused || c.currentJobNum >= config.GetConcurrency() {
		return false
	}
	c.polling = true
	return true
}

func (c *AgentController) PollingJob(ctx context.Context) {
	defer func() {
		close(c.JobChan)
	}()

	log.Infof("start polling job.")
	for {
		select {
		case <-ctx.Done():
			log.Infof("stop polling job, received context cancel signal.")
			return
		case <-c.StopPollingJobChan:
			log.Infof("stop polling job, received stop signal.")
			return
		default:
			if config.GetAgentStatus() == common.AGENT_STATUS_RUNNING && config.GetScheduleWorkflow() && c.beginPoll() {
				job, err := c.Client.RequestJob()
				c.mu.Lock()
				if job != nil && job.ID != "" {
					c.currentJobNum++
				}
				c.polling = false
				c.mu.Unlock()
				if err != nil {
					log.Errorf("failed to request job from zadig server, error: %s", err)
					time.Sleep(common.DefaultAgentPollingInterval * time.Second)
					continue
				}

				if job != nil && job.ID != "" {
					c.JobChan <- job
					log.Infof("PollingJob: received job workflow name: %v, task id: %v, project name: %v, job name: %v",
						job.WorkflowName, job.TaskID, job.ProjectName, job.JobName)
					if config.GetEnableDebug() {
						log.Debugf("received job detail: %+v", job)
					}
				}

				time.Sleep(common.DefaultAgentPollingInterval * time.Second)
			} else {
				time.Sleep(time.Duration(c.ConcurrencyBlockTime) * time.Second)
			}
		}
	}
}

func (c *AgentController) RunJob(ctx context.Context) {
	log.Infof("start running job.")
	for {
		select {
		case <-ctx.Done():
			log.Infof("stop running job, received context cancel signal.")
			return
		case <-c.StopRunJobChan:
			log.Infof("stop running job, received stop signal.")
			return
		case job, ok := <-c.JobChan:
			if !ok {
				log.Infof("job chan closed.")
				return
			}

			go func(job *types.ZadigJobTask) {
				err := c.RunSingleJob(ctx, job)
				c.mu.Lock()
				c.currentJobNum--
				if errors.Is(err, reporter.ErrFinalReport) {
					c.jobError = fmt.Errorf("cannot upgrade after an incomplete job: %w", err)
				}
				c.mu.Unlock()
				if err != nil {
					log.Errorf("failed to run job, error: %s", err)
				}
			}(job)
		}
	}
}

func (c *AgentController) RunSingleJob(ctx context.Context, job *types.ZadigJobTask) error {
	var err error
	ctx, stop := context.WithCancelCause(ctx)
	c.mu.Lock()
	if c.jobCancels == nil {
		c.jobCancels = make(map[string]context.CancelCauseFunc)
	}
	c.jobCancels[job.ID] = stop
	if c.forceDrain {
		stop(errUpgradeCancelled)
	}
	c.mu.Unlock()
	defer func() {
		stop(nil)
		c.mu.Lock()
		delete(c.jobCancels, job.ID)
		c.mu.Unlock()
	}()
	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	executor := jobexecutor.NewJobExecutor(jobCtx, job, c.Client, cancel)
	if errors.Is(context.Cause(ctx), errUpgradeCancelled) {
		return executor.Reporter.FinishedJobReport(common.StatusCancelled, errUpgradeCancelled)
	}

	// execute some init job before execute zadig job
	err = executor.BeforeExecute()
	if err != nil {
		log.Errorf("failed to execute BeforeExecute, error: %s", err)
		executor.CleanupTempFileAndDir()
		if errors.Is(context.Cause(ctx), errUpgradeCancelled) {
			return executor.Reporter.FinishedJobReport(common.StatusCancelled, errUpgradeCancelled)
		}

		err = executor.Reporter.FinishedJobReport(common.StatusFailed, fmt.Errorf("failed to init work directory for job, error: %s", err))
		if err != nil {
			return fmt.Errorf("failed to report job status when BeforeExecute failed: %w", err)
		}
		return fmt.Errorf("failed to execute workflow %s job %s, error: %s", job.WorkflowName, job.JobName, err)
	}
	if executor.CheckZadigCancel() {
		cancel()
		executor.CleanupTempFileAndDir()
		return nil
	}

	defer func() {
		cancel()
		if executor.Logger != nil {
			executor.Logger.Close()
		}
	}()

	util.Go(func() {
		func() {
			defer close(executor.FinishedChan)

			// execute zadig job
			executor.Execute()

			// execute some job after execute zadig job, such as upload the remaining unuploaded logs.
			err = executor.AfterExecute()
		}()
	})

	util.Go(func() {
		executor.Reporter.Start(jobCtx)
	})

	for {
		select {
		case <-ctx.Done():
			log.Infof("stop running job, received context cancel signal.")
			cancel()
			<-executor.FinishedChan
			if errors.Is(context.Cause(ctx), errUpgradeCancelled) {
				return executor.Reporter.FinishedJobReport(common.StatusCancelled, errUpgradeCancelled)
			}
			return nil
		// TODO: how to deal with job cancel by better way, if restart the same job after cancel immediately?
		case <-executor.FinishedChan:
			log.Infof("workflow %s job %s finished.", job.WorkflowName, job.JobName)
			if errors.Is(context.Cause(ctx), errUpgradeCancelled) {
				return executor.Reporter.FinishedJobReport(common.StatusCancelled, errUpgradeCancelled)
			}

			if e := executor.JobResult.GetError(); e != nil {
				log.Errorf("workflow %s job %s failed, error: %v", job.WorkflowName, job.JobName, e)
				return executor.Reporter.FinishedJobReport(common.StatusFailed, nil)
			}

			if err != nil {
				return executor.Reporter.FinishedJobReport(common.StatusFailed, err)
			}

			log.Infof("workflow %s job %s success.", job.WorkflowName, job.JobName)
			return executor.Reporter.FinishedJobReport(common.StatusPassed, nil)
		default:
			if executor.CheckZadigCancel() {
				cancel()
				// Wait for Execute and AfterExecute to finish. AfterExecute owns
				// the final cleanup, so returning before it completes leaves the
				// workspace and job temporary directories behind.
				<-executor.FinishedChan
				return fmt.Errorf("job %s id %s is canceled by user", job.JobName, job.ID)
			}
			time.Sleep(time.Second)
		}
	}
}
