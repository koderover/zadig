/*
 * Copyright 2024 The KodeRover Authors.
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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/blueking"
)

type BlueKingJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskBlueKingSpec
	ack         func()
}

func NewBlueKingJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *BlueKingJobCtl {
	jobTaskSpec := &commonmodels.JobTaskBlueKingSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &BlueKingJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *BlueKingJobCtl) Clean(ctx context.Context) {}

func (c *BlueKingJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusPrepare
	c.ack()

	info, err := mongodb.NewCICDToolColl().Get(c.jobTaskSpec.ToolID)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}
	c.jobTaskSpec.Host = info.Host
	c.jobTaskSpec.ToolName = info.Name

	bkClient := blueking.NewClient(
		info.Host,
		info.AppCode,
		info.AppSecret,
		info.BKUserName,
	)

	fmt.Println(">>>>>>>>>>>>>>>>>>", c.jobTaskSpec.Parameters, "<<<<<<<<<<<<<")
	for i, param := range c.jobTaskSpec.Parameters {
		fmt.Println("+++++++++++++++++", i)
		fmt.Printf("+++++++++++++++++++ %+v", param)
	}

	instanceBriefInfo, err := bkClient.RunExecutionPlan(
		c.jobTaskSpec.BusinessID,
		c.jobTaskSpec.ExecutionPlanID,
		c.jobTaskSpec.Parameters,
	)

	if err != nil {
		errMsg := fmt.Sprintf("failed to run execution plan of id: %d in business: %d, err: %s", c.jobTaskSpec.ExecutionPlanID, c.jobTaskSpec.BusinessID, err)
		logError(c.job, errMsg, c.logger)
		return
	}

	c.jobTaskSpec.InstanceName = instanceBriefInfo.JobInstanceName
	c.jobTaskSpec.InstanceID = instanceBriefInfo.JobInstanceID
	c.ack()

	instanceInfo, err := bkClient.GetExecutionPlanInstance(
		c.jobTaskSpec.BusinessID,
		c.jobTaskSpec.InstanceID,
	)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get execution plan instance of id: %d in business: %d, err: %s", c.jobTaskSpec.InstanceID, c.jobTaskSpec.BusinessID, err)
		logError(c.job, errMsg, c.logger)
		return
	}

	for !instanceInfo.Finished {
		instanceInfo, err = bkClient.GetExecutionPlanInstance(
			c.jobTaskSpec.BusinessID,
			c.jobTaskSpec.InstanceID,
		)
		if err != nil {
			errMsg := fmt.Sprintf("failed to get execution plan instance of id: %d in business: %d, err: %s", c.jobTaskSpec.InstanceID, c.jobTaskSpec.BusinessID, err)
			logError(c.job, errMsg, c.logger)
			return
		}
		c.jobTaskSpec.BKJobStatus = instanceInfo.JobInstance.Status
		c.ack()
		time.Sleep(2 * time.Second)
	}

	c.jobTaskSpec.BKJobStatus = instanceInfo.JobInstance.Status
	if instanceInfo.JobInstance.Status != 3 {
		c.job.Status = config.StatusFailed
	} else {
		c.job.Status = config.StatusPassed
	}

	return
}

func (c *BlueKingJobCtl) SaveInfo(ctx context.Context) error {
	return mongodb.NewJobInfoColl().Create(ctx, &commonmodels.JobInfo{
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
