/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package jobcontroller

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	systemconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/aslan"
	"github.com/koderover/zadig/pkg/tool/log"
)

type WorkflowTriggerJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskWorkflowTriggerSpec
	ack         func()
}

func NewWorkflowTriggerJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *WorkflowTriggerJobCtl {
	jobTaskSpec := &commonmodels.JobTaskWorkflowTriggerSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &WorkflowTriggerJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *WorkflowTriggerJobCtl) Clean(ctx context.Context) {}

func (c *WorkflowTriggerJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	client := aslan.New(systemconfig.AslanServiceAddress())

	type runningTask struct {
		WorkflowName string
		TaskID       int64
	}
	var runningTasks = make(map[runningTask]*commonmodels.WorkflowTriggerEvent)
	cancelAllRunningTasks := func() {
		if c.jobTaskSpec.IsEnableCheck {
			for task, event := range runningTasks {
				err := client.CancelWorkflowTaskV4(setting.WorkflowTriggerTaskCreator,
					task.WorkflowName, task.TaskID)
				if err != nil {
					log.Errorf("WorkflowTriggerJobCtl: CancelWorkflowTaskV4 %s-%d err: %v", task.WorkflowName, task.TaskID, err)
				} else {
					log.Debugf("WorkflowTriggerJobCtl: CancelWorkflowTaskV4 %s-%d success", task.WorkflowName, task.TaskID)
					event.Status = config.StatusCancelled
				}
			}
			c.ack()
		}
	}
	defer cancelAllRunningTasks()

	for _, e := range c.jobTaskSpec.WorkflowTriggerEvents {
		list, _, err := mongodb.NewWorkflowV4Coll().List(&mongodb.ListWorkflowV4Option{
			ProjectName: e.ProjectName,
			Names:       []string{e.WorkflowName},
		}, 0, 0)
		if err != nil {
			logError(c.job, fmt.Sprintf("find workflow %s err: %v", e.WorkflowName, err), c.logger)
			return
		}
		if len(list) == 0 {
			logError(c.job, fmt.Sprintf("project %s workflow %s not found", e.ProjectName, e.WorkflowName), c.logger)
			return
		}
		w := list[0]
		w.Params = e.Params
		e.WorkflowDisplayName = w.DisplayName

		resp, err := client.CreateWorkflowTaskV4(&aslan.CreateWorkflowTaskV4Req{
			Workflow: w,
			UserName: setting.WorkflowTriggerTaskCreator,
		})
		if err != nil {
			logError(c.job, fmt.Sprintf("create workflow task %s err: %v", w.Name, err), c.logger)
			return
		}
		runningTasks[runningTask{
			WorkflowName: w.Name,
			TaskID:       resp.TaskID,
		}] = e
		e.TaskID = resp.TaskID
	}

	if c.jobTaskSpec.IsEnableCheck {
		var jobFailed bool
	LOOP:
		for {
			time.Sleep(time.Second)
			select {
			case <-ctx.Done():
				return
			default:
				for task, event := range runningTasks {
					t, err := mongodb.NewworkflowTaskv4Coll().Find(task.WorkflowName, task.TaskID)
					if err != nil {
						logError(c.job, fmt.Sprintf("get workflow task %s-%d err: %v", task.WorkflowName, task.TaskID, err), c.logger)
						return
					}
					switch t.Status {
					case config.StatusPassed, config.StatusFailed, config.StatusCancelled, config.StatusReject, config.StatusTimeout:
						event.Status = t.Status
						delete(runningTasks, task)
						log.Debugf("WorkflowTriggerJobCtl: %s-%d final status: %s", task.WorkflowName, task.TaskID, t.Status)
						if t.Status != config.StatusPassed {
							jobFailed = true
						}
						c.ack()
					case config.StatusRunning:
						if event.Status != config.StatusRunning {
							event.Status = t.Status
							c.ack()
						}
					}
				}
				if len(runningTasks) == 0 {
					break LOOP
				}
			}
		}
		if jobFailed {
			c.job.Status = config.StatusFailed
			return
		}
	}
	c.job.Status = config.StatusPassed
	return
}

func (c *WorkflowTriggerJobCtl) SaveInfo(ctx context.Context) error {
	return mongodb.NewJobInfoColl().Create(context.TODO(), &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),
	})
}
