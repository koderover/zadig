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

	"github.com/hashicorp/go-multierror"
	"github.com/samber/lo"
	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/instantmessage"
	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/mail"
	util2 "github.com/koderover/zadig/v2/pkg/util"
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

		for _, user := range c.jobTaskSpec.LarkGroupNotificationConfig.AtUsers {
			larkAtUserIDs = append(larkAtUserIDs, user.ID)
		}

		client, err := larkservice.GetLarkClientByIMAppID(c.jobTaskSpec.LarkGroupNotificationConfig.AppID)
		if err != nil {
			c.logger.Error(err)
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.ack()
			return
		}

		// TODO: distinct the receiver type
		err = sendLarkMessage(client, c.workflowCtx.ProjectName, c.workflowCtx.WorkflowName, c.workflowCtx.WorkflowDisplayName, c.workflowCtx.TaskID, instantmessage.LarkReceiverTypeChat, c.jobTaskSpec.LarkGroupNotificationConfig.Chat.ChatID, c.jobTaskSpec.Title, c.jobTaskSpec.Content, larkAtUserIDs, c.jobTaskSpec.LarkGroupNotificationConfig.IsAtAll)
		if err != nil {
			c.logger.Error(err)
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.ack()
			return
		}
	} else if c.jobTaskSpec.WebHookType == setting.NotifyWebHookTypeDingDing {
		err := sendDingDingMessage(c.workflowCtx.ProjectName, c.workflowCtx.WorkflowName, c.workflowCtx.WorkflowDisplayName, c.workflowCtx.TaskID, c.jobTaskSpec.DingDingNotificationConfig.HookAddress, c.jobTaskSpec.Title, c.jobTaskSpec.Content, c.jobTaskSpec.DingDingNotificationConfig.AtMobiles, c.jobTaskSpec.DingDingNotificationConfig.IsAtAll)
		if err != nil {
			c.logger.Error(err)
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.ack()
			return
		}
	} else if c.jobTaskSpec.WebHookType == setting.NotifyWebHookTypeWechatWork {
		err := sendWorkWxMessage(c.workflowCtx.ProjectName, c.workflowCtx.WorkflowName, c.workflowCtx.WorkflowDisplayName, c.workflowCtx.TaskID, c.jobTaskSpec.WechatNotificationConfig.HookAddress, c.jobTaskSpec.Title, c.jobTaskSpec.Content, c.jobTaskSpec.WechatNotificationConfig.AtUsers, c.jobTaskSpec.WechatNotificationConfig.IsAtAll)
		if err != nil {
			c.logger.Error(err)
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.ack()
			return
		}
	} else if c.jobTaskSpec.WebHookType == setting.NotifyWebHookTypeMail {
		err := sendMailMessage(c.jobTaskSpec.Title, c.jobTaskSpec.Content, c.jobTaskSpec.MailNotificationConfig.TargetUsers, c.workflowCtx.WorkflowTaskCreatorUserID)
		if err != nil {
			c.logger.Error(err)
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.ack()
			return
		}
	} else if c.jobTaskSpec.WebHookType == setting.NotifyWebHookTypeFeishuPerson {
		client, err := larkservice.GetLarkClientByIMAppID(c.jobTaskSpec.LarkPersonNotificationConfig.AppID)
		if err != nil {
			c.logger.Error(err)
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.ack()
			return
		}

		// the logic to change the executor in the list to the real user.
		respErr := new(multierror.Error)

		for _, target := range c.jobTaskSpec.LarkPersonNotificationConfig.TargetUsers {
			if target.IsExecutor {
				userInfo, err := user.New().GetUserByID(c.workflowCtx.WorkflowTaskCreatorUserID)
				if err != nil || userInfo == nil {
					respErr = multierror.Append(respErr, fmt.Errorf("cannot find task invoker's information, error: %s", err))
					continue
				}

				if len(userInfo.Phone) == 0 {
					respErr = multierror.Append(respErr, fmt.Errorf("phone not configured"))
					continue
				}

				larkUser, err := client.GetUserIDByEmailOrMobile(lark.QueryTypeMobile, userInfo.Phone, setting.LarkUserID)
				if err != nil {
					respErr = multierror.Append(fmt.Errorf("find lark user with phone %s error: %v", userInfo.Phone, err))
					continue
				}

				userDetailedInfo, err := client.GetUserInfoByID(util2.GetStringFromPointer(larkUser.UserId), setting.LarkUserID)
				if err != nil {
					respErr = multierror.Append(fmt.Errorf("find lark user info for userID %s error: %v", util2.GetStringFromPointer(larkUser.UserId), err))
					continue
				}

				target.ID = util2.GetStringFromPointer(larkUser.UserId)
				target.Name = userDetailedInfo.Name
				target.Avatar = userDetailedInfo.Avatar
				target.IDType = setting.LarkUserID
			}

			err = sendLarkMessage(client, c.workflowCtx.ProjectName, c.workflowCtx.WorkflowName, c.workflowCtx.WorkflowDisplayName, c.workflowCtx.TaskID, target.IDType, target.ID, c.jobTaskSpec.Title, c.jobTaskSpec.Content, make([]string, 0), false)
			if err != nil {
				respErr = multierror.Append(respErr, err)
			}
		}

		if respErr.ErrorOrNil() != nil {
			c.logger.Error(respErr.Error())
			c.job.Status = config.StatusFailed
			c.job.Error = respErr.Error()
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

	// reference: https://open.dingtalk.com/document/orgapp/message-link-description
	dingtalkRedirectURL := fmt.Sprintf("dingtalk://dingtalkclient/page/link?url=%s&pc_slide=false",
		url.QueryEscape(actionURL),
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
					ActionURL: dingtalkRedirectURL,
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

	moreInformation := fmt.Sprintf("[%s](%s)", "点击查看更多信息", actionURL)

	content := fmt.Sprintf("### %s\n%s\n%s", title, message, moreInformation)

	markDownMessage := &instantmessage.WeChatWorkCard{
		MsgType:  string(instantmessage.WeChatTextTypeMarkdown),
		Markdown: instantmessage.Markdown{Content: content},
	}

	// TODO: if required, add proxy to it
	c := httpclient.New()

	_, err := c.Post(uri, httpclient.SetBody(markDownMessage))
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

	emailSvc, err := systemconfig.New().GetEmailService()
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
			From:     emailSvc.Address,
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
