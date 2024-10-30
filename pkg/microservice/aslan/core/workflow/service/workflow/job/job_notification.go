/*
Copyright 2024 The KodeRover Authors.

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

package job

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type NotificationJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.NotificationJobSpec
}

func (j *NotificationJob) Instantiate() error {
	j.spec = &commonmodels.NotificationJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *NotificationJob) SetPreset() error {
	j.spec = &commonmodels.NotificationJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	j.job.Spec = j.spec
	return nil
}

func (j *NotificationJob) ClearSelectionField() error {
	return nil
}

func (j *NotificationJob) SetOptions() error {
	return nil
}

func (j *NotificationJob) ClearOptions() error {
	return nil
}

func (j *NotificationJob) MergeArgs(args *commonmodels.Job) error {
	j.spec = &commonmodels.NotificationJobSpec{}
	if err := commonmodels.IToi(args.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *NotificationJob) UpdateWithLatestSetting() error {
	j.spec = &commonmodels.NotificationJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	latestWorkflow, err := mongodb.NewWorkflowV4Coll().Find(j.workflow.Name)
	if err != nil {
		log.Errorf("Failed to find original workflow to set options, error: %s", err)
	}

	latestSpec := new(commonmodels.NotificationJobSpec)
	found := false
	for _, stage := range latestWorkflow.Stages {
		if !found {
			for _, job := range stage.Jobs {
				if job.Name == j.job.Name && job.JobType == j.job.JobType {
					if err := commonmodels.IToi(job.Spec, latestSpec); err != nil {
						return err
					}
					found = true
					break
				}
			}
		} else {
			break
		}
	}

	if !found {
		return fmt.Errorf("failed to find the original workflow: %s", j.workflow.Name)
	}

	// use the latest webhook settings, except for title and content
	j.spec.WebHookType = latestSpec.WebHookType
	j.spec.WeChatWebHook = latestSpec.WeChatWebHook
	j.spec.FeiShuWebHook = latestSpec.FeiShuWebHook
	j.spec.DingDingWebHook = latestSpec.DingDingWebHook
	j.spec.MailUsers = latestSpec.MailUsers
	j.spec.WebHookNotify = latestSpec.WebHookNotify
	j.spec.AtMobiles = latestSpec.AtMobiles
	j.spec.WechatUserIDs = latestSpec.WechatUserIDs
	j.spec.LarkAtUsers = latestSpec.LarkAtUsers

	j.job.Spec = j.spec
	return nil
}

func (j *NotificationJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	j.spec = &commonmodels.NotificationJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return nil, err
	}
	j.job.Spec = j.spec
	jobTask := &commonmodels.JobTask{
		Name: j.job.Name,
		JobInfo: map[string]string{
			JobNameKey: j.job.Name,
		},
		Key:     j.job.Name,
		JobType: string(config.JobNotification),
		Spec: &commonmodels.JobTaskNotificationSpec{
			WebHookType:     j.spec.WebHookType,
			WeChatWebHook:   j.spec.WeChatWebHook,
			DingDingWebHook: j.spec.DingDingWebHook,
			FeiShuAppID:     j.spec.FeiShuAppID,
			FeishuChat:      j.spec.FeishuChat,
			MailUsers:       j.spec.MailUsers,
			WebHookNotify:   j.spec.WebHookNotify,
			AtMobiles:       j.spec.AtMobiles,
			WechatUserIDs:   j.spec.WechatUserIDs,
			LarkAtUsers:     j.spec.LarkAtUsers,
			Content:         j.spec.Content,
			Title:           j.spec.Title,
			IsAtAll:         j.spec.IsAtAll,
		},
		Timeout:     0,
		ErrorPolicy: j.job.ErrorPolicy,
	}
	return []*commonmodels.JobTask{jobTask}, nil
}

func (j *NotificationJob) LintJob() error {
	j.spec = &commonmodels.NotificationJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	if j.spec.Title == "" {
		return fmt.Errorf("job title is empty")
	}
	return nil
}
