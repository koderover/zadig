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
	"fmt"
	"strings"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/instantmessage"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/samber/lo"
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
		larkAtUserIds := []string{}
		for _, user := range c.jobTaskSpec.LarkAtUsers {
			larkAtUserIds = append(larkAtUserIds, user.ID)
		}
		err := sendLarkMessage(c.workflowCtx.ProjectName, c.workflowCtx.WorkflowName, c.workflowCtx.WorkflowDisplayName, c.workflowCtx.TaskID, c.jobTaskSpec.FeiShuWebHook, c.jobTaskSpec.Title, c.jobTaskSpec.Content, larkAtUserIds, c.jobTaskSpec.IsAtAll)
		if err != nil {
			c.logger.Error(err)
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.ack()
			return
		}
	} else if c.jobTaskSpec.WebHookType == setting.NotifyWebHookTypeDingDing {
		err := sendDingDingMessage(c.workflowCtx.ProjectName, c.workflowCtx.WorkflowName, c.workflowCtx.WorkflowDisplayName, c.workflowCtx.TaskID, c.jobTaskSpec.DingDingWebHook, c.jobTaskSpec.Title, c.jobTaskSpec.Content, c.jobTaskSpec.AtMobiles, c.jobTaskSpec.IsAtAll)
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

func sendLarkMessage(productName, workflowName, workflowDisplayName string, taskID int64, uri, title, message string, idList []string, isAtAll bool) error {
	// first generate lark card
	card := instantmessage.NewLarkCard()
	card.SetConfig(true)
	card.SetHeader(
		"blue",
		title,
		"plain_text",
	)

	card.AddI18NElementsZhcnFeild(message, true)
	url := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		configbase.SystemAddress(),
		productName,
		workflowName,
		taskID,
		workflowDisplayName,
	)
	card.AddI18NElementsZhcnAction("点击查看更多信息", url)

	reqBody := instantmessage.LarkCardReq{
		MsgType: "interactive",
		Card:    card,
	}

	// TODO: if required, add proxy to it
	c := httpclient.New()
	_, err := c.Post(uri, httpclient.SetBody(reqBody))

	if err != nil {
		return err
	}

	// then send @ message
	if len(idList) > 0 || isAtAll {
		atUserList := []string{}
		idList = lo.Filter(idList, func(s string, _ int) bool { return s != "All" })
		for _, userID := range idList {
			atUserList = append(atUserList, fmt.Sprintf("<at user_id=\"%s\"></at>", userID))
		}
		atMessage := strings.Join(atUserList, " ")
		if isAtAll {
			atMessage += "<at user_id=\"all\"></at>"
		}

		var larkAtMessage interface{}

		larkAtMessage = &instantmessage.FeiShuMessage{
			Text: atMessage,
		}

		if strings.Contains(uri, "bot/v2/hook") {
			larkAtMessage = &instantmessage.FeiShuMessageV2{
				MsgType: "text",
				Content: instantmessage.FeiShuContentV2{
					Text: atMessage,
				},
			}
		}

		_, err = c.Post(uri, httpclient.SetBody(larkAtMessage))
		if err != nil {
			return err
		}
	}
	return nil
}

func sendDingDingMessage(productName, workflowName, workflowDisplayName string, taskID int64, uri, title, message string, idList []string, isAtAll bool) error {
	processedMessage := generateGeneralNotificationMessage(productName, workflowName, workflowDisplayName, taskID, message)

	messageReq := instantmessage.DingDingMessage{
		MsgType: "markdown",
		MarkDown: &instantmessage.DingDingMarkDown{
			Title: title,
			Text:  processedMessage,
		},
	}

	messageReq.At = &instantmessage.DingDingAt{
		AtMobiles: idList,
		IsAtAll:   isAtAll,
	}

	// TODO: if required, add proxy to it
	c := httpclient.New()

	_, err := c.Post(uri, httpclient.SetBody(messageReq))
	if err != nil {
		return err
	}

	return nil
}

func generateGeneralNotificationMessage(productName, workflowName, workflowDisplayName string, taskID int64, content string) string {
	url := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		configbase.SystemAddress(),
		productName,
		workflowName,
		taskID,
		workflowDisplayName,
	)

	resp := content + "\n"
	resp += fmt.Sprintf("- 触发的工作流: %s\n", url)
	return resp
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
