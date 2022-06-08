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

package service

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/nsq"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/scmnotify"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type cancelMessage struct {
	Revoker      string `json:"revoker"`
	PipelineName string `json:"pipeline_name"`
	TaskID       int64  `json:"task_id"`
	ReqID        string `json:"req_id"`
}

func CancelWorkflowTaskV3(userName, pipelineName string, taskID int64, typeString config.PipelineType, requestID string, log *zap.SugaredLogger) error {
	if err := CancelTask(userName, pipelineName, taskID, typeString, requestID, log); err != nil {
		log.Errorf("[%s] cancel pipeline [%s:%d] error: %s", userName, pipelineName, taskID, err)
		return e.ErrCancelTask.AddDesc(err.Error())
	}
	return nil
}

func CancelTaskV2(userName, pipelineName string, taskID int64, typeString config.PipelineType, requestID string, log *zap.SugaredLogger) error {

	if err := CancelTask(userName, pipelineName, taskID, typeString, requestID, log); err != nil {
		log.Errorf("[%s] cancel pipeline [%s:%d] error: %v", userName, pipelineName, taskID, err)
		return e.ErrCancelTask.AddDesc(err.Error())
	}
	return nil
}

func CancelTask(userName, pipelineName string, taskID int64, typeString config.PipelineType, reqID string, log *zap.SugaredLogger) error {
	scmNotifyService := scmnotify.NewService()

	for _, t := range List(log) {
		if t.PipelineName == pipelineName &&
			t.TaskID == taskID &&
			t.Type == typeString &&
			(t.Status == config.StatusWaiting || t.Status == config.StatusBlocked || t.Status == config.StatusCreated) {
			Remove(t, log)

			err := mongodb.NewTaskColl().UpdateStatus(taskID, pipelineName, userName, config.StatusCancelled)
			if err != nil {
				log.Errorf("PipelineTaskV2.UpdateStatus [%s:%d] error: %v", pipelineName, taskID, err)
				return err
			}

			// 获取更新后的任务数据并发送通知
			t, err := mongodb.NewTaskColl().Find(taskID, pipelineName, typeString)
			if err != nil {
				log.Errorf("[%s] task: %s:%d not found", userName, pipelineName, taskID)
				return err
			}

			if typeString == config.WorkflowType {
				_ = scmNotifyService.UpdateWebhookComment(t, log)
			} else if typeString == config.TestType {
				_ = scmNotifyService.UpdateWebhookCommentForTest(t, log)
			} else if typeString == config.SingleType {
				_ = scmNotifyService.UpdatePipelineWebhookComment(t, log)
			} else if typeString == config.ScanningType {
				err = scmNotifyService.UpdateWebhookCommentForScanning(t, log)
				// this logic will not affect the result of cancellation, so we only print the error in logs.
				if err != nil {
					log.Errorf("Failed to update webhook comment for scanning in upon cancel, the error is: %s", err)
				}
			}

			return nil
		}
	}

	b, err := json.Marshal(cancelMessage{Revoker: userName, PipelineName: pipelineName, TaskID: taskID, ReqID: reqID})
	if err != nil {
		log.Errorf("marshal cancel message error: %v", err)
		return err
	}

	t, err := mongodb.NewTaskColl().Find(taskID, pipelineName, typeString)
	if err != nil {
		log.Errorf("[%s] task: %s:%d not found", userName, pipelineName, taskID)
		return err
	}

	if t.Status == config.StatusPassed {
		log.Errorf("[%s] task: %s:%d is passed, cannot cancel", userName, pipelineName, taskID)
		return fmt.Errorf("task: %s:%d is passed, cannot cancel", pipelineName, taskID)
	}

	t.Status = config.StatusCancelled
	t.TaskRevoker = userName

	log.Infof("[%s] CancelRunningTask %s:%d on %s", userName, pipelineName, taskID, t.AgentHost)

	if err := mongodb.NewTaskColl().Update(t); err != nil {
		log.Errorf("[%s] update task: %s:%d error: %v", userName, pipelineName, taskID, err)
		return err
	}

	q := covertTaskToQueue(t)
	Remove(q, log)

	if err := nsq.Publish(setting.TopicCancel, b); err != nil {
		log.Errorf("[%s] cancel %s %s:%d error", userName, t.AgentHost, pipelineName, taskID)
		return err
	}

	if typeString == config.WorkflowType {
		_ = scmNotifyService.UpdateWebhookComment(t, log)
	} else if typeString == config.TestType {
		_ = scmNotifyService.UpdateWebhookCommentForTest(t, log)
	} else if typeString == config.SingleType {
		_ = scmNotifyService.UpdatePipelineWebhookComment(t, log)
	} else if typeString == config.ScanningType {
		err = scmNotifyService.UpdateWebhookCommentForScanning(t, log)
		// this logic will not affect the result of cancellation, so we only print the error in logs.
		if err != nil {
			log.Errorf("Failed to update webhook comment for scanning in upon cancel, the error is: %s", err)
		}
	}

	return nil
}

func GetServiceTasks(log *zap.SugaredLogger) (map[string][]string, error) {
	taskPreviewList, err := mongodb.NewTaskColl().List(&mongodb.ListTaskOption{Type: config.ServiceType})
	if err != nil {
		log.Errorf("getJobNamespaceAndSelector PipelineTaskV2.List error: %v", err)
		return nil, e.ErrListTasks
	}

	respTaskNamespaceMap := make(map[string][]string)
	for _, taskPreview := range taskPreviewList {
		if _, isExist := respTaskNamespaceMap[taskPreview.Namespace]; isExist {
			list := sets.NewString(respTaskNamespaceMap[taskPreview.Namespace]...)
			if !list.Has(taskPreview.ServiceName) {
				respTaskNamespaceMap[taskPreview.Namespace] = append(respTaskNamespaceMap[taskPreview.Namespace], taskPreview.ServiceName)
			}
		} else {
			respTaskNamespaceMap[taskPreview.Namespace] = []string{taskPreview.ServiceName}
		}
	}

	return respTaskNamespaceMap, nil
}

// workaround. only used to delete a queue item
func covertTaskToQueue(t *task.Task) *models.Queue {
	return &models.Queue{
		TaskID:       t.TaskID,
		PipelineName: t.PipelineName,
		Type:         t.Type,
		CreateTime:   t.CreateTime,
	}
}

func GetWorkflowTaskCallback(taskID int64, pipelineName string) (*commonmodels.CallbackRequest, error) {
	return mongodb.NewCallbackRequestColl().Find(&mongodb.CallbackFindOption{
		TaskID:       taskID,
		PipelineName: pipelineName,
	})
}
