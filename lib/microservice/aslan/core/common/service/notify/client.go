/*
Copyright 2021 The KodeRover Authors.

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

package notify

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/scmnotify"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/wechat"
	"github.com/koderover/zadig/lib/tool/xlog"
)

const sevendays int64 = 60 * 60 * 24 * 7

type client struct {
	notifyColl       *repo.NotifyColl
	pipelineColl     *repo.PipelineColl
	subscriptionColl *repo.SubscriptionColl
	taskColl         *repo.TaskColl
	scmNotifyService *scmnotify.Service
	WeChatService    *wechat.WeChatService
}

func NewNotifyClient() *client {
	return &client{
		notifyColl:       repo.NewNotifyColl(),
		pipelineColl:     repo.NewPipelineColl(),
		subscriptionColl: repo.NewSubscriptionColl(),
		taskColl:         repo.NewTaskColl(),
		scmNotifyService: scmnotify.NewService(),
		WeChatService:    wechat.NewWeChatClient(),
	}
}

func (c *client) CreateNotify(sender string, nf *models.Notify) error {
	b, err := json.Marshal(nf.Content)
	if err != nil {
		return fmt.Errorf("marshal content error: %v", err)
	}
	switch nf.Type {
	case config.Announcement:
		var content *models.AnnouncementCtx
		if err = json.Unmarshal(b, &content); err != nil {
			return fmt.Errorf("[%s] convert announcement error: %v", sender, err)
		}
		nf.Content = content
	case config.PipelineStatus:
		var content *models.PipelineStatusCtx
		if err = json.Unmarshal(b, &content); err != nil {
			return fmt.Errorf("[%s] convert pipelinestatus error: %v", sender, err)
		}
		nf.Content = content
	case config.Message:
		var content *models.MessageCtx
		if err = json.Unmarshal(b, &content); err != nil {
			return fmt.Errorf("[%s] convert message error: %v", sender, err)
		}
		nf.Content = content
	default:
		return fmt.Errorf("notify type not found")
	}
	err = c.notifyColl.Create(nf)
	if err != nil {
		return fmt.Errorf("[%s] create Notify error: %v", sender, err)
	}
	return nil
}

func (c *client) PullNotifyAnnouncement(user string) ([]*models.Notify, error) {
	resp := make([]*models.Notify, 0)
	systemNotifyList, err := c.notifyColl.List("*")
	if err != nil {
		return resp, fmt.Errorf("[*] pull notify error: %v", err)
	}
	//取出所有当前生效的系统通告
	for _, systemNotify := range systemNotifyList {
		b, err := json.Marshal(systemNotify.Content)
		if err != nil {
			return resp, fmt.Errorf("marshal content error: %v", err)
		}
		var content *models.AnnouncementCtx
		if err = json.Unmarshal(b, &content); err != nil {
			return resp, fmt.Errorf("[%s] convert announcement error: %v", user, err)
		}
		if content.StartTime < time.Now().Unix() && time.Now().Unix() < content.EndTime {
			resp = append(resp, systemNotify)
		}
	}
	return resp, nil
}

func (c *client) PullAllAnnouncement(user string) ([]*models.Notify, error) {
	systemNotifyList, err := c.notifyColl.List("*")
	if err != nil {
		return nil, fmt.Errorf("[*] pull notify error: %v", err)
	}
	return systemNotifyList, nil
}

func (c *client) PullNotify(user string) ([]*models.Notify, error) {
	resp := make([]*models.Notify, 0)
	notifyList, err := c.notifyColl.List(user)
	if err != nil {
		return resp, fmt.Errorf("[*] pull notify error: %v", err)
	}
	var timeRange int64
	if len(notifyList) == 0 {
		return resp, nil
	}
	timeRange = notifyList[0].CreateTime - sevendays

	//取出所有当前生效的系统通告
	for _, notify := range notifyList {
		if notify.CreateTime > timeRange {
			resp = append(resp, notify)
		}
	}
	return resp, nil
}

func (c *client) UpsertSubscription(user string, subscription *models.Subscription) error {
	subscription.Subscriber = user
	err := c.subscriptionColl.Upsert(subscription)
	if err != nil {
		return fmt.Errorf("[%s] create subscription error: %v", user, err)
	}
	return nil
}

func (c *client) UpdateSubscribe(user string, notifyType int, subscription *models.Subscription) error {
	err := c.subscriptionColl.Update(user, notifyType, subscription)
	if err != nil {
		return fmt.Errorf("[%s] update subscription error: %v", user, err)
	}
	return nil
}

func (c *client) Unsubscribe(user string, notifyType int) error {
	err := c.subscriptionColl.Delete(user, notifyType)
	if err != nil {
		return fmt.Errorf("[%s] delete Subscription error: %v", user, err)
	}
	return nil
}

func (c *client) Read(user string, notifyIDs []string) error {
	for _, id := range notifyIDs {
		err := c.notifyColl.Read(id)
		if err != nil {
			return fmt.Errorf("[%s] update status error: %v", user, err)
		}
	}
	return nil
}

func (c *client) Update(user string, notifyID string, ctx *models.Notify) error {
	b, err := json.Marshal(ctx.Content)
	if err != nil {
		return fmt.Errorf("marshal content error: %v", err)
	}
	switch ctx.Type {
	case config.Announcement:
		var content *models.AnnouncementCtx
		if err = json.Unmarshal(b, &content); err != nil {
			return fmt.Errorf("[%s] convert announcement error: %v", user, err)
		}
		ctx.Content = content
	default:
		return fmt.Errorf("notify type not found")
	}
	err = c.notifyColl.Update(notifyID, ctx)
	if err != nil {
		return fmt.Errorf("[%s] update Notify error: %v", user, err)
	}
	return nil
}

func (c *client) DeleteNotifies(user string, notifyIDs []string) error {
	for _, id := range notifyIDs {
		err := c.notifyColl.DeleteByID(id)
		if err != nil {
			return fmt.Errorf("[%s] delete notify error: %v", user, err)
		}
	}
	return nil
}

func (c *client) DeleteByTime(user string, time int64) error {
	err := c.notifyColl.DeleteByTime(time)
	if err != nil {
		return fmt.Errorf("[%s] delete notify error:%v", user, err)
	}
	return nil
}

func (c *client) ListSubscriptions(user string) ([]*models.Subscription, error) {
	subs, err := c.subscriptionColl.List(user)
	if err != nil {
		return nil, fmt.Errorf("[%s] list subscription error:%v", user, err)
	}
	return subs, nil
}

func (c *client) ProccessNotify(notify *models.Notify) error {
	switch notify.Type {
	case config.PipelineStatus:
		b, err := json.Marshal(notify.Content)
		if err != nil {
			return fmt.Errorf("[%s] marshal content error: %v", notify.Receiver, err)
		}
		var ctx *models.PipelineStatusCtx
		if err = json.Unmarshal(b, &ctx); err != nil {
			return fmt.Errorf("[%s] convert content error: %v", notify.Receiver, err)
		}

		task, err := c.taskColl.Find(ctx.TaskID, ctx.PipelineName, ctx.Type)
		if err != nil {
			return fmt.Errorf("get task #%d notify, status: %s", ctx.TaskID, ctx.Status)
		}
		ctx.ProductName = task.ProductName
		notify.Content = ctx

		receivers := []string{notify.Receiver}
		logger := xlog.NewDummy()
		task.Status = ctx.Status
		if ctx.Type == config.SingleType {
			pipline, err := c.pipelineColl.Find(&repo.PipelineFindOption{Name: ctx.PipelineName})
			if err != nil {
				return fmt.Errorf("[%s] find pipline error: %v", notify.Receiver, err)
			}
			//notify通知接受者
			receivers = append(receivers, pipline.Notifiers...)

			logger.Infof("pipeline get task #%d notify, status: %s", ctx.TaskID, ctx.Status)
			_ = c.scmNotifyService.UpdatePipelineWebhookComment(task, logger)
		} else if ctx.Type == config.WorkflowType {
			logger.Infof("workflow get task #%d notify, status: %s", ctx.TaskID, ctx.Status)
			_ = c.scmNotifyService.UpdateWebhookComment(task, logger)
			_ = c.scmNotifyService.UpdateDiffNote(task, logger)
		} else if ctx.Type == config.TestType {
			logger.Infof("test get task #%d notify, status: %s", ctx.TaskID, ctx.Status)
			_ = c.scmNotifyService.UpdateWebhookCommentForTest(task, logger)
		}

		//发送微信通知
		err = c.WeChatService.SendWechatMessage(task)
		if err != nil {
			return fmt.Errorf("SendWechatMessage err : %v", err)
		}

		for _, receiver := range receivers {
			subs, err := c.subscriptionColl.List(notify.Receiver)
			if err != nil {
				return fmt.Errorf("list subscribers error: %v", err)
			}
			newNotify := &models.Notify{
				Type:     config.PipelineStatus,
				Receiver: receiver,
				Content:  ctx,
			}
			for _, sub := range subs {
				if sub.Type == newNotify.Type && (sub.PipelineStatus == ctx.Status || sub.PipelineStatus == "*") {
					if err := c.notifyColl.Create(newNotify); err != nil {
						return fmt.Errorf("create notify error: %v", err)
					}
				}
			}
		}
	case config.Message:
		b, err := json.Marshal(notify.Content)
		if err != nil {
			return fmt.Errorf("[%s] marshal content error: %v", notify.Receiver, err)
		}
		var ctx *models.MessageCtx
		if err = json.Unmarshal(b, &ctx); err != nil {
			return fmt.Errorf("[%s] convert content error: %v", notify.Receiver, err)
		}

		notify.Content = ctx
		if err := c.sendSubscribedNotify(notify); err != nil {
			return fmt.Errorf("[%s] send notify error: %v", notify.Receiver, err)
		}

	default:
		return fmt.Errorf("notifytype match error")
	}

	return nil
}

func (c *client) sendSubscribedNotify(notify *models.Notify) error {
	subs, err := c.subscriptionColl.List(notify.Receiver)
	if err != nil {
		return fmt.Errorf("list subscribers error: %v", err)
	}
	for _, sub := range subs {
		if sub.Type == notify.Type {
			if err := c.notifyColl.Create(notify); err != nil {
				return fmt.Errorf("create notify error: %v", err)
			}
		}
	}
	return nil
}
