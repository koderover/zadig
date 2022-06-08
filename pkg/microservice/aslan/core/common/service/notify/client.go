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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/instantmessage"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/scmnotify"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

const sevendays int64 = 60 * 60 * 24 * 7

type WorkflowTaskImage struct {
	Image        string `json:"image"`
	ServiceName  string `json:"service_name"`
	RegistryRepo string `json:"registry_repo"`
}

type client struct {
	notifyColl            *mongodb.NotifyColl
	pipelineColl          *mongodb.PipelineColl
	subscriptionColl      *mongodb.SubscriptionColl
	taskColl              *mongodb.TaskColl
	scmNotifyService      *scmnotify.Service
	InstantmessageService *instantmessage.Service
}

func NewNotifyClient() *client {
	return &client{
		notifyColl:            mongodb.NewNotifyColl(),
		pipelineColl:          mongodb.NewPipelineColl(),
		subscriptionColl:      mongodb.NewSubscriptionColl(),
		taskColl:              mongodb.NewTaskColl(),
		scmNotifyService:      scmnotify.NewService(),
		InstantmessageService: instantmessage.NewWeChatClient(),
	}
}

func (c *client) CreateNotify(sender string, nf *models.Notify) error {
	b, err := json.Marshal(nf.Content)
	if err != nil {
		return fmt.Errorf("marshal content error: %v", err)
	}
	switch nf.Type {
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
		logger := log.SugaredLogger()
		task.Status = ctx.Status
		testTaskStatusChanged := false
		if ctx.Type == config.SingleType {
			pipline, err := c.pipelineColl.Find(&mongodb.PipelineFindOption{Name: ctx.PipelineName})
			if err != nil {
				return fmt.Errorf("[%s] find pipline error: %v", notify.Receiver, err)
			}
			//notify通知接受者
			receivers = append(receivers, pipline.Notifiers...)

			logger.Infof("pipeline get task #%d notify, status: %s", ctx.TaskID, ctx.Status)
			_ = c.scmNotifyService.UpdatePipelineWebhookComment(task, logger)
		} else if ctx.Type == config.WorkflowType {
			if task.TaskCreator == setting.RequestModeOpenAPI {
				// send callback requests
				err = c.sendCallbackRequest(task)
				if err != nil {
					logger.Errorf("failed to send callback request for workflow: %s, taskID: %d, err: %s", task.PipelineName, task.TaskID, err)
				}
				return nil
			}
			logger.Infof("workflow get task #%d notify, status: %s", ctx.TaskID, ctx.Status)
			_ = c.scmNotifyService.UpdateWebhookComment(task, logger)
			task.Stages = ctx.Stages
		} else if ctx.Type == config.TestType {
			logger.Infof("test get task #%d notify, status: %s", ctx.TaskID, ctx.Status)
			_ = c.scmNotifyService.UpdateWebhookCommentForTest(task, logger)
			if ctx.TaskID > 1 {
				testPreTask, err := c.taskColl.Find(ctx.TaskID-1, ctx.PipelineName, ctx.Type)
				if err != nil {
					return fmt.Errorf("get test previous task #%d notify, status: %s,err:%s", ctx.TaskID-1, ctx.Status, err)
				}
				if testPreTask.Status != task.Status && task.Status != config.StatusRunning {
					testTaskStatusChanged = true
				}
			}
			task.Stages = ctx.Stages
		} else if ctx.Type == config.ScanningType {
			logger.Infof("scanning get task #%d notify, status: %s", ctx.TaskID, ctx.Status)
			err = c.scmNotifyService.UpdateWebhookCommentForScanning(task, logger)
			if err != nil {
				// FIXME: This error will not be returned since the logic above didn't return. It is expected to return if we know what we are doing.
				log.Errorf("Failed to update webhook comment for scanning: %s, err: %s", task.ScanningArgs.ScanningID, err)
			}
			if ctx.TaskID > 1 {
				scanningPreTask, err := c.taskColl.Find(ctx.TaskID-1, ctx.PipelineName, ctx.Type)
				if err != nil {
					return fmt.Errorf("get test previous task #%d notify, status: %s,err:%s", ctx.TaskID-1, ctx.Status, err)
				}
				if scanningPreTask.Status != task.Status && task.Status != config.StatusRunning {
					testTaskStatusChanged = true
				}
			}
			task.Stages = ctx.Stages
		}

		err = c.InstantmessageService.SendInstantMessage(task, testTaskStatusChanged)
		if err != nil {
			return fmt.Errorf("SendInstantMessage err : %s", err)
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

func taskFinished(task *task.Task) bool {
	status := task.Status
	if status == config.StatusPassed || status == config.StatusFailed || status == config.StatusTimeout || status == config.StatusCancelled {
		return true
	}
	return false
}

func getImages(task *task.Task) ([]*WorkflowTaskImage, error) {
	ret := make([]*WorkflowTaskImage, 0)
	for _, stages := range task.Stages {
		if stages.TaskType != config.TaskBuild || stages.Status != config.StatusPassed {
			continue
		}
		for _, subTask := range stages.SubTasks {
			buildInfo, err := base.ToBuildTask(subTask)
			if err != nil {
				log.Errorf("get buildInfo ToBuildTask failed ! err:%s", err)
				return nil, err
			}

			WorkflowTaskImage := &WorkflowTaskImage{
				Image:       buildInfo.JobCtx.Image,
				ServiceName: buildInfo.ServiceName,
			}
			if buildInfo.DockerBuildStatus != nil {
				WorkflowTaskImage.RegistryRepo = buildInfo.DockerBuildStatus.RegistryRepo
			}
			ret = append(ret, WorkflowTaskImage)
		}
	}
	return ret, nil
}

// send callback request when workflow is finished
func (c *client) sendCallbackRequest(task *task.Task) error {
	if !taskFinished(task) {
		return nil
	}

	//TODO remove this log after openApi becomes stable
	log.Infof("sending callback request for workflow:%s, taskID:%d", task.PipelineName, task.TaskID)

	if task.WorkflowArgs == nil {
		return nil
	}

	callback := task.WorkflowArgs.Callback
	if callback == nil {
		return nil
	}

	callbackUrl, err := url.PathUnescape(callback.CallbackUrl)
	if err != nil {
		return errors.Wrapf(err, "failed to unescape callback url")
	}

	responseBody := make(map[string]interface{})

	// set task status data
	responseBody["status"] = task.Status
	responseBody["task_id"] = task.TaskID
	responseBody["workflow_name"] = task.PipelineName
	responseBody["error"] = task.Error

	// set custom kvs
	responseBody["vars"] = callback.CallbackVars

	// set images
	responseBody["images"], err = getImages(task)

	reqBody, err := json.Marshal(responseBody)
	if err != nil {
		log.Errorf("marshal json args error: %s", err)
		return err
	}

	req, err := http.NewRequest("POST", callbackUrl, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("callback response code error: %d", resp.StatusCode)
	}
	return nil
}
