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
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
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

func (j *NotificationJob) SetOptions(approvalTicket *commonmodels.ApprovalTicket) error {
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
	j.spec.LarkGroupNotificationConfig = latestSpec.LarkGroupNotificationConfig
	j.spec.LarkPersonNotificationConfig = latestSpec.LarkPersonNotificationConfig
	j.spec.WechatNotificationConfig = latestSpec.WechatNotificationConfig
	j.spec.DingDingNotificationConfig = latestSpec.DingDingNotificationConfig
	j.spec.MSTeamsNotificationConfig = latestSpec.MSTeamsNotificationConfig
	j.spec.MailNotificationConfig = latestSpec.MailNotificationConfig
	j.spec.WebhookNotificationConfig = latestSpec.WebhookNotificationConfig

	// ========= compatibility code below, these field will only be used to generate new configuration ===============
	j.spec.WeChatWebHook = latestSpec.WeChatWebHook
	j.spec.DingDingWebHook = latestSpec.DingDingWebHook
	j.spec.FeiShuAppID = latestSpec.FeiShuAppID
	j.spec.FeishuChat = latestSpec.FeishuChat
	j.spec.MailUsers = latestSpec.MailUsers
	j.spec.WebHookNotify = latestSpec.WebHookNotify
	j.spec.AtMobiles = latestSpec.AtMobiles
	j.spec.WechatUserIDs = latestSpec.WechatUserIDs
	j.spec.LarkAtUsers = latestSpec.LarkAtUsers
	j.spec.IsAtAll = latestSpec.IsAtAll

	j.job.Spec = j.spec
	return nil
}

func (j *NotificationJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	j.spec = &commonmodels.NotificationJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return nil, err
	}
	j.job.Spec = j.spec

	taskSpec, err := generateNotificationJobSpec(j.spec)
	if err != nil {
		return nil, err
	}

	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.job.Name, 0),
		Key:         genJobKey(j.job.Name),
		DisplayName: genJobDisplayName(j.job.Name),
		OriginName:  j.job.Name,
		JobInfo: map[string]string{
			JobNameKey: j.job.Name,
		},
		JobType:     string(config.JobNotification),
		Spec:        taskSpec,
		Timeout:     0,
		ErrorPolicy: j.job.ErrorPolicy,
	}
	return []*commonmodels.JobTask{jobTask}, nil
}

func generateNotificationJobSpec(spec *commonmodels.NotificationJobSpec) (*commonmodels.JobTaskNotificationSpec, error) {
	resp := &commonmodels.JobTaskNotificationSpec{
		WebHookType: spec.WebHookType,
		Content:     spec.Content,
		Title:       spec.Title,
	}

	err := spec.GenerateNewNotifyConfigWithOldData()
	if err != nil {
		return nil, err
	}

	resp.MailNotificationConfig = spec.MailNotificationConfig
	resp.WechatNotificationConfig = spec.WechatNotificationConfig
	resp.LarkPersonNotificationConfig = spec.LarkPersonNotificationConfig
	resp.LarkGroupNotificationConfig = spec.LarkGroupNotificationConfig
	resp.DingDingNotificationConfig = spec.DingDingNotificationConfig
	resp.MSTeamsNotificationConfig = spec.MSTeamsNotificationConfig
	resp.WebhookNotificationConfig = spec.WebhookNotificationConfig

	return resp, nil
}

func (j *NotificationJob) LintJob() error {
	j.spec = &commonmodels.NotificationJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	if j.spec.Title == "" {
		return fmt.Errorf("job title is empty")
	}

	err := commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}
	return nil
}
