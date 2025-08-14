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

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/tapd"
)

type TapdJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskTapdSpec
	ack         func()
}

func NewTapdJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *TapdJobCtl {
	jobTaskSpec := &commonmodels.JobTaskTapdSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &TapdJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *TapdJobCtl) Clean(ctx context.Context) {}

func (c *TapdJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	client, err := tapd.NewClient(c.jobTaskSpec.TapdAddress, c.jobTaskSpec.TapdClientID, c.jobTaskSpec.TapdClientSecret, c.jobTaskSpec.TapdCompanyID)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}

	for _, data := range c.jobTaskSpec.Iterations {
		if err := client.UpdateIterationStatus(c.jobTaskSpec.ProjectID, data.IterationID, string(c.jobTaskSpec.Status), c.workflowCtx.WorkflowTaskCreatorUsername); err != nil {
			data.Error = err.Error()
			logError(c.job, err.Error(), c.logger)
			return
		}
	}
	c.job.Status = config.StatusPassed
}

func (c *TapdJobCtl) SaveInfo(ctx context.Context) error {
	return commonrepo.NewJobInfoColl().Create(context.TODO(), &commonmodels.JobInfo{
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
