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

package jobcontroller

import (
	"context"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/instantmessage"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"go.uber.org/zap"
)

type NotificationJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskNotificationSpec
	ack         func()
}

func NewNotificationJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *NotificationJobCtl {
	jobTaskSpec := &commonmodels.JobTaskNotificationSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &NotificationJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *NotificationJobCtl) Clean(ctx context.Context) {}

func (c *NotificationJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	if c.jobTaskSpec.WebHookType == setting.NotifyWebHookTypeFeishu {
		err := sendLarkMessage(c.jobTaskSpec.FeiShuWebHook, c.jobTaskSpec.Title, c.jobTaskSpec.Content)
		if err != nil {
			c.logger.Error(err)
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.ack()
			return
		}
	}

	//time.Sleep(10 * time.Second)
	c.job.Status = config.StatusPassed

	return
}

func sendLarkMessage(uri, title, message string) error {
	// first generate lark card
	card := instantmessage.NewLarkCard()
	card.SetConfig(true)
	card.SetHeader(
		"blue-50",
		title,
		"plain_text",
	)

	card.AddI18NElementsZhcnFeild(message, true)
	card.AddI18NElementsZhcnAction("点击查看更多信息", "https://www.baidu.com")

	reqBody := instantmessage.LarkCardReq{
		MsgType: "interactive",
		Card:    card,
	}

	// TODO: if required, add proxy to it
	c := httpclient.New()
	_, err := c.Post(uri, httpclient.SetBody(reqBody))
	return err
}

func (c *NotificationJobCtl) SaveInfo(ctx context.Context) error {
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
