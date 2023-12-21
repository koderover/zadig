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
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	jobexecutor "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/job"
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
		CurrentJobNum:        0,
	}
}

type AgentController struct {
	Client               *network.ZadigClient
	JobChan              chan *types.ZadigJobTask
	StopPollingJobChan   chan struct{}
	StopRunJobChan       chan struct{}
	Concurrency          int
	ConcurrencyBlockTime int
	CurrentJobNum        int
	WorkingDirectory     string
}

func (c *AgentController) Start(ctx context.Context) {
	c.JobChan = make(chan *types.ZadigJobTask, c.Concurrency)

	go c.PollingJob(ctx)

	go c.RunJob(ctx)
}

func (c *AgentController) StopPollingJob() {
	defer close(c.StopPollingJobChan)

	c.StopPollingJobChan <- struct{}{}
}

func (c *AgentController) StopRunJob() {
	defer close(c.StopRunJobChan)

	c.StopRunJobChan <- struct{}{}
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
			if config.GetAgentStatus() == common.AGENT_STATUS_RUNNING && config.GetScheduleWorkflow() && c.CurrentJobNum < config.GetConcurrency() {
				job, err := c.Client.RequestJob()
				if err != nil {
					log.Errorf("failed to request job from zadig server, error: %s", err)
					time.Sleep(common.DefaultAgentPollingInterval * time.Second)
					continue
				}

				if job != nil && job.ID != "" {
					c.JobChan <- job
					c.CurrentJobNum++
					log.Infof("PollingJob: received job workflow name: %v, task id: %v, project name: %v, job name: %v",
						job.WorkflowName, job.TaskID, job.ProjectName, job.JobName)
					if config.GetEnableDebug() {
						log.Debugf("received job detail: %+v", job)
					}
				}

				time.Sleep(common.DefaultAgentPollingInterval * time.Second)
			} else {
				if c.CurrentJobNum >= config.GetConcurrency() {
					log.Infof("current job num %d is equal to concurrency %d, will block %d seconds to request job again.", c.CurrentJobNum, config.GetConcurrency(), c.ConcurrencyBlockTime)
				}
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

			go func() {
				defer func() {
					c.CurrentJobNum--
				}()
				if err := c.RunSingleJob(ctx, job); err != nil {
					log.Errorf("failed to run job, error: %s", err)
				}
			}()
		}
	}
}

func (c *AgentController) RunSingleJob(ctx context.Context, job *types.ZadigJobTask) error {
	var err error
	jobCtx, cancel := context.WithCancel(ctx)
	executor := jobexecutor.NewJobExecutor(ctx, job, c.Client, cancel)

	// execute some init job before execute zadig job
	err = executor.BeforeExecute()
	if err != nil {
		log.Errorf("failed to execute BeforeExecute, error: %s", err)

		err = executor.Reporter.FinishedJobReport(common.StatusFailed, fmt.Errorf("failed to init work directory for job, error: %s", err))
		if err != nil {
			return fmt.Errorf("failed to report job status when BeforeExecute failed, error: %s", err)
		}
		return fmt.Errorf("failed to execute workflow %s job %s, error: %s", job.WorkflowName, job.JobName, err)
	}
	if executor.CheckZadigCancel() {
		return nil
	}

	defer func() {
		if executor.Logger != nil {
			executor.Logger.Close()
		}
	}()

	util.Go(func() {
		func() {
			defer func() {
				executor.FinishedChan <- struct{}{}
				close(executor.FinishedChan)
			}()

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
			return nil
		// TODO: how to deal with job cancel by better way, if restart the same job after cancel immediately?
		case <-executor.FinishedChan:
			log.Infof("workflow %s job %s finished.", job.WorkflowName, job.JobName)

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
				return fmt.Errorf("job %s id %s is canceled by user", job.JobName, job.ID)
			}
			time.Sleep(time.Second)
		}
	}
}
