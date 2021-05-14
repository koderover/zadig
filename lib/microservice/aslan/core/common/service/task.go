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

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/task"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/nsq"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/scmnotify"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type cancelMessage struct {
	Revoker      string `json:"revoker"`
	PipelineName string `json:"pipeline_name"`
	TaskID       int64  `json:"task_id"`
	ReqID        string `json:"req_id"`
}

func CancelTaskV2(userName, pipelineName string, taskID int64, typeString config.PipelineType, log *xlog.Logger) error {

	if err := CancelTask(userName, pipelineName, taskID, typeString, log.ReqID(), log); err != nil {
		log.Errorf("[%s] cancel pipeline [%s:%d] error: %v", userName, pipelineName, taskID, err)
		return e.ErrCancelTask.AddDesc(err.Error())
	}
	return nil
}

func CancelTask(userName, pipelineName string, taskID int64, typeString config.PipelineType, reqID string, log *xlog.Logger) error {
	scmNotifyService := scmnotify.NewService()

	for _, t := range List(log) {
		if t.PipelineName == pipelineName &&
			t.TaskID == taskID &&
			t.Type == typeString &&
			(t.Status == config.StatusWaiting || t.Status == config.StatusBlocked || t.Status == config.StatusCreated) {
			Remove(t, log)

			err := repo.NewTaskColl().UpdateStatus(taskID, pipelineName, userName, config.StatusCancelled)
			if err != nil {
				log.Errorf("PipelineTaskV2.UpdateStatus [%s:%d] error: %v", pipelineName, taskID, err)
				return err
			}

			// 获取更新后的任务数据并发送通知
			t, err := repo.NewTaskColl().Find(taskID, pipelineName, typeString)
			if err != nil {
				log.Errorf("[%s] task: %s:%d not found", userName, pipelineName, taskID)
				return err
			}

			if typeString == config.WorkflowType {
				_ = scmNotifyService.UpdateWebhookComment(t, log)
				_ = scmNotifyService.UpdateDiffNote(t, log)
			} else if typeString == config.TestType {
				_ = scmNotifyService.UpdateWebhookCommentForTest(t, log)
			} else if typeString == config.SingleType {
				_ = scmNotifyService.UpdatePipelineWebhookComment(t, log)
			}

			return nil
		}
	}

	b, err := json.Marshal(cancelMessage{Revoker: userName, PipelineName: pipelineName, TaskID: taskID, ReqID: reqID})
	if err != nil {
		log.Errorf("marshal cancel message error: %v", err)
		return err
	}

	t, err := repo.NewTaskColl().Find(taskID, pipelineName, typeString)
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

	if err := repo.NewTaskColl().Update(t); err != nil {
		log.Errorf("[%s] update task: %s:%d error: %v", userName, pipelineName, taskID, err)
		return err
	}

	q := covertTaskToQueue(t)
	Remove(q, log)

	if err := nsq.Publish(config.TopicCancel, b); err != nil {
		log.Errorf("[%s] cancel %s %s:%d error", userName, t.AgentHost, pipelineName, taskID)
		return err
	}

	if typeString == config.WorkflowType {
		_ = scmNotifyService.UpdateWebhookComment(t, log)
		_ = scmNotifyService.UpdateDiffNote(t, log)
	} else if typeString == config.TestType {
		_ = scmNotifyService.UpdateWebhookCommentForTest(t, log)
	} else if typeString == config.SingleType {
		_ = scmNotifyService.UpdatePipelineWebhookComment(t, log)
	}

	return nil
}

func GetServiceTasks(log *xlog.Logger) (map[string][]string, error) {
	taskPreviewList, err := repo.NewTaskColl().List(&repo.ListTaskOption{Type: config.ServiceType})
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

type Preview struct {
	TaskType config.TaskType `json:"type"`
	Enabled  bool            `json:"enabled"`
}

func ToPreview(sb map[string]interface{}) (*Preview, error) {
	var pre *Preview
	if err := task.IToi(sb, &pre); err != nil {
		return nil, fmt.Errorf("convert interface to SubTaskPreview error: %v", err)
	}
	return pre, nil
}

func ToBuildTask(sb map[string]interface{}) (*task.Build, error) {
	var t *task.Build
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to BuildTaskV2 error: %v", err)
	}
	return t, nil
}

func ToArtifactTask(sb map[string]interface{}) (*task.Artifact, error) {
	var t *task.Artifact
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to ArtifactTask error: %v", err)
	}
	return t, nil
}

func ToDockerBuildTask(sb map[string]interface{}) (*task.DockerBuild, error) {
	var t *task.DockerBuild
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to DockerBuildTask error: %v", err)
	}
	return t, nil
}

func ToDeployTask(sb map[string]interface{}) (*task.Deploy, error) {
	var t *task.Deploy
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to DeployTask error: %v", err)
	}
	return t, nil
}

func ToTestingTask(sb map[string]interface{}) (*task.Testing, error) {
	var t *task.Testing
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to Testing error: %v", err)
	}
	return t, nil
}

func ToDistributeToS3Task(sb map[string]interface{}) (*task.DistributeToS3, error) {
	var t *task.DistributeToS3
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to DistributeToS3Task error: %v", err)
	}
	return t, nil
}

func ToReleaseImageTask(sb map[string]interface{}) (*task.ReleaseImage, error) {
	var t *task.ReleaseImage
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to ReleaseImageTask error: %v", err)
	}
	return t, nil
}

func ToJiraTask(sb map[string]interface{}) (*task.Jira, error) {
	var t *task.Jira
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to JiraTask error: %v", err)
	}
	return t, nil
}

func ToSecurityTask(sb map[string]interface{}) (*task.Security, error) {
	var t *task.Security
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to securityTask error: %v", err)
	}
	return t, nil
}

// ToJenkinsTask ...
func ToJenkinsBuildTask(sb map[string]interface{}) (*task.JenkinsBuild, error) {
	var jenkinsBuild *task.JenkinsBuild
	if err := task.IToi(sb, &jenkinsBuild); err != nil {
		return nil, fmt.Errorf("convert interface to JenkinsBuildTask error: %v", err)
	}
	return jenkinsBuild, nil
}
