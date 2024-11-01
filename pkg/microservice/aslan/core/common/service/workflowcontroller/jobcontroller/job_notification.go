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
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/mail"
	"github.com/samber/lo"
	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/instantmessage"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
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

	if c.jobTaskSpec.WebHookType == setting.NotifyWebhookTypeFeishuApp {
		larkAtUserIDs := make([]string, 0)

		for _, user := range c.jobTaskSpec.LarkAtUsers {
			larkAtUserIDs = append(larkAtUserIDs, user.ID)
		}

		client, err := larkservice.GetLarkClientByIMAppID(c.jobTaskSpec.FeiShuAppID)
		if err != nil {
			c.logger.Error(err)
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.ack()
			return
		}

		// TODO: distinct the receiver type
		err = sendLarkMessage(client, c.workflowCtx.ProjectName, c.workflowCtx.WorkflowName, c.workflowCtx.WorkflowDisplayName, c.workflowCtx.TaskID, instantmessage.LarkReceiverTypeChat, c.jobTaskSpec.FeishuChat.ChatID, c.jobTaskSpec.Title, c.jobTaskSpec.Content, larkAtUserIDs, c.jobTaskSpec.IsAtAll)
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
	} else if c.jobTaskSpec.WebHookType == setting.NotifyWebHookTypeWechatWork {
		err := sendWorkWxMessage(c.workflowCtx.ProjectName, c.workflowCtx.WorkflowName, c.workflowCtx.WorkflowDisplayName, c.workflowCtx.TaskID, c.jobTaskSpec.WeChatWebHook, c.jobTaskSpec.Title, c.jobTaskSpec.Content, c.jobTaskSpec.WechatUserIDs, c.jobTaskSpec.IsAtAll)
		if err != nil {
			c.logger.Error(err)
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.ack()
			return
		}
	} else if c.jobTaskSpec.WebHookType == setting.NotifyWebHookTypeMail {
		err := sendMailMessage(c.jobTaskSpec.Title, c.jobTaskSpec.Content, c.jobTaskSpec.MailUsers, c.workflowCtx.WorkflowTaskCreatorUserID)
		if err != nil {
			c.logger.Error(err)
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.ack()
			return
		}
	} else {
		c.logger.Error("unsupported notification type")
		c.job.Status = config.StatusFailed
		c.job.Error = "unsupported notification type"
		c.ack()
		return
	}

	//time.Sleep(10 * time.Second)
	c.job.Status = config.StatusPassed

	return
}

func sendLarkMessage(client *lark.Client, productName, workflowName, workflowDisplayName string, taskID int64, receiverType, receiverID, title, message string, idList []string, isAtAll bool) error {
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

	messageContent, err := json.Marshal(card)
	if err != nil {
		return err
	}

	err = client.SendMessage(receiverType, instantmessage.LarkMessageTypeCard, receiverID, string(messageContent))

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

		larkAtMessage := &instantmessage.FeiShuMessage{
			Text: atMessage,
		}

		atMessageContent, err := json.Marshal(larkAtMessage)
		if err != nil {
			return err
		}

		err = client.SendMessage(receiverType, instantmessage.LarkMessageTypeText, receiverID, string(atMessageContent))

		if err != nil {
			return err
		}
	}
	return nil
}

func sendDingDingMessage(productName, workflowName, workflowDisplayName string, taskID int64, uri, title, message string, idList []string, isAtAll bool) error {
	processedMessage := generateDingDingNotificationMessage(title, message, idList)

	actionURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		configbase.SystemAddress(),
		productName,
		workflowName,
		taskID,
		url.PathEscape(workflowDisplayName),
	)

	messageReq := instantmessage.DingDingMessage{
		MsgType: instantmessage.DingDingMsgType,
		ActionCard: &instantmessage.DingDingActionCard{
			HideAvatar:        "0",
			ButtonOrientation: "0",
			Text:              processedMessage,
			Title:             title,
			Buttons: []*instantmessage.DingDingButton{
				{
					Title:     "点击查看更多信息",
					ActionURL: actionURL,
				},
			},
		},
	}

	messageReq.At = &instantmessage.DingDingAt{
		AtMobiles: idList,
		IsAtAll:   isAtAll,
	}

	// TODO: if required, add proxy to it
	c := httpclient.New()

	resp, err := c.Post(uri, httpclient.SetBody(messageReq))
	if err != nil {
		return err
	} else {
		fmt.Println(string(resp.Body()))
	}

	return nil
}

func sendWorkWxMessage(productName, workflowName, workflowDisplayName string, taskID int64, uri, title, message string, idList []string, isAtAll bool) error {
	actionURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		configbase.SystemAddress(),
		productName,
		workflowName,
		taskID,
		url.PathEscape(workflowDisplayName),
	)

	idList = lo.Filter(idList, func(s string, _ int) bool { return s != "All" })
	atList := make([]string, 0)

	for _, id := range idList {
		atList = append(atList, fmt.Sprintf("<@%s>", id))
	}

	msgCard := &instantmessage.WeChatWorkCard{
		MsgType: string(instantmessage.WeChatTextTypeTemplateCard),
		TemplateCard: instantmessage.TemplateCard{
			CardType: "text_notice",
			MainTitle: &instantmessage.TemplateCardTitle{
				Title: title,
			},
			SubTitleText: message,
			JumpList: []*instantmessage.WechatWorkLink{
				{
					Type:  1,
					URL:   actionURL,
					Title: "点击查看更多信息",
				},
			},
			CardAction: &instantmessage.WechatWorkCardAction{
				Type: 1,
				URL:  actionURL,
			},
		},
	}

	// TODO: if required, add proxy to it
	c := httpclient.New()

	_, err := c.Post(uri, httpclient.SetBody(msgCard))
	if err != nil {
		return err
	}

	if len(atList) > 0 {
		atMessage := fmt.Sprintf("##### **相关人员**: %s \n", strings.Join(atList, " "))

		atMessageBody := &instantmessage.WeChatWorkCard{
			MsgType:  string(instantmessage.WeChatTextTypeMarkdown),
			Markdown: instantmessage.Markdown{Content: atMessage},
		}

		_, err := c.Post(uri, httpclient.SetBody(atMessageBody))

		if err != nil {
			return err
		}
	}

	return nil
}

func sendMailMessage(title, message string, users []*commonmodels.User, callerID string) error {
	if len(users) == 0 {
		return nil
	}

	email, err := systemconfig.New().GetEmailHost()
	if err != nil {
		return err
	}

	users, userMap := util.GeneFlatUsersWithCaller(users, callerID)
	for _, u := range users {
		log.Infof("Sending Mail to user: %s", u.UserName)
		info, ok := userMap[u.UserID]
		if !ok {
			info, err = user.New().GetUserByID(u.UserID)
			if err != nil {
				log.Warnf("sendMailMessage GetUserByUid error, error msg:%s", err)
				continue
			}
		}

		if info.Email == "" {
			log.Warnf("sendMailMessage user %s email is empty", info.Name)
			continue
		}
		err = mail.SendEmail(&mail.EmailParams{
			From:     email.UserName,
			To:       info.Email,
			Subject:  title,
			Host:     email.Name,
			UserName: email.UserName,
			Password: email.Password,
			Port:     email.Port,
			Body:     message,
		})
		if err != nil {
			log.Errorf("sendMailMessage SendEmail error, error msg:%s", err)
			continue
		}
	}

	return err
}

func generateDingDingNotificationMessage(title, content string, idList []string) string {
	titleStr := fmt.Sprintf("### <font color=#3270e3>**%s**</font>", title)

	atMessage := ""
	if len(idList) > 0 {
		atMessage = fmt.Sprintf("##### **相关人员**: @%s \n", strings.Join(idList, "@"))
	}

	resp := fmt.Sprintf("%s\n%s\n%s", titleStr, content, atMessage)
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
