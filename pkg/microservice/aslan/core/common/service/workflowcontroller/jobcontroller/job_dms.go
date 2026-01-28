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
	"time"

	dms "github.com/alibabacloud-go/dms-enterprise-20181101/v3/client"
	"github.com/alibabacloud-go/tea/tea"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/cloudservice"
	"github.com/koderover/zadig/v2/pkg/tool/aliyun"
)

type DMSJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskDMSSpec
	ack         func()
}

func NewDMSJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *DMSJobCtl {
	jobTaskSpec := &commonmodels.JobTaskDMSSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &DMSJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *DMSJobCtl) Clean(ctx context.Context) {}

func (c *DMSJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	dmsInfo, err := mongodb.NewCloudServiceColl().Find(ctx, &mongodb.CloudServiceCollFindOption{Id: c.jobTaskSpec.ID})
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}

	client, err := cloudservice.NewDMSClient(dmsInfo)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}

	failed := false
	for _, order := range c.jobTaskSpec.Orders {
		err = execDMSDataCorrectOrder(ctx, client, order.ID)
		if err != nil {
			failed = true
			order.Error = err.Error()
			logError(c.job, err.Error(), c.logger)
			continue
		}
	}

	for {
		c.ack()

		select {
		case <-ctx.Done():
			c.job.Status = config.StatusCancelled
			logError(c.job, "job cancelled", c.logger)
			return
		default:
		}

		allDone := true
		for _, order := range c.jobTaskSpec.Orders {
			if order.Error != "" {
				continue
			}

			if order.JobStatus == "FAIL" || order.JobStatus == "SUCCESS" || order.JobStatus == "DELETE" {
				if order.JobStatus == "FAIL" {
					failed = true
				}
				continue
			}

			allDone = false

			taskDetail, err := getDMSDataCorrectTaskDetail(ctx, client, order.ID)
			if err != nil {
				failed = true
				order.Error = err.Error()
				logError(c.job, err.Error(), c.logger)
				continue
			}

			order.JobStatus = tea.StringValue(taskDetail.GetJobStatus())
		}

		if allDone {
			if failed {
				c.job.Status = config.StatusFailed
			} else {
				c.job.Status = config.StatusPassed
			}
			return
		}

		time.Sleep(time.Second * 3)
	}
}

func (c *DMSJobCtl) SaveInfo(ctx context.Context) error {
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

func execDMSDataCorrectOrder(ctx context.Context, client *dms.Client, orderID int64) error {
	execOrderRequest := &dms.ExecuteDataCorrectRequest{
		OrderId: tea.Int64(orderID),
	}
	_, err := client.ExecuteDataCorrect(execOrderRequest)
	if err != nil {
		err = aliyun.HandleError(err)
		return err
	}

	return nil
}

func getDMSDataCorrectTaskDetail(ctx context.Context, client *dms.Client, orderID int64) (*dms.GetDataCorrectTaskDetailResponseBodyDataCorrectTaskDetail, error) {
	getTaskDetailRequest := &dms.GetDataCorrectTaskDetailRequest{
		OrderId: tea.Int64(orderID),
	}
	taskDetail, err := client.GetDataCorrectTaskDetail(getTaskDetailRequest)
	if err != nil {
		err = aliyun.HandleError(err)
		return nil, err
	}
	return taskDetail.GetBody().GetDataCorrectTaskDetail(), nil
}
