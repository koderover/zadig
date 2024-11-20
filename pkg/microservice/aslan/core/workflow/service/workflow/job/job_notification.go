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
	"github.com/koderover/zadig/v2/pkg/setting"
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
	j.spec.LarkGroupNotificationConfig = latestSpec.LarkGroupNotificationConfig
	j.spec.LarkPersonNotificationConfig = latestSpec.LarkPersonNotificationConfig
	j.spec.WechatNotificationConfig = latestSpec.WechatNotificationConfig
	j.spec.DingDingNotificationConfig = latestSpec.DingDingNotificationConfig
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

	switch spec.WebHookType {
	case setting.NotifyWebHookTypeDingDing:
		if spec.DingDingNotificationConfig != nil {
			resp.DingDingNotificationConfig = spec.DingDingNotificationConfig
		} else {
			if len(spec.DingDingWebHook) == 0 {
				return nil, fmt.Errorf("failed to parse old notification data: dingding_webhook field is empty")
			}
			resp.DingDingNotificationConfig = &commonmodels.DingDingNotificationConfig{
				HookAddress: spec.DingDingWebHook,
				AtMobiles:   spec.AtMobiles,
				IsAtAll:     spec.IsAtAll,
			}
		}
	case setting.NotifyWebHookTypeWechatWork:
		if spec.WechatNotificationConfig != nil {
			resp.WechatNotificationConfig = spec.WechatNotificationConfig
		} else {
			if len(spec.WeChatWebHook) == 0 {
				return nil, fmt.Errorf("failed to parse old notification data: weChat_webHook field is empty")
			}
			resp.WechatNotificationConfig = &commonmodels.WechatNotificationConfig{
				HookAddress: spec.WeChatWebHook,
				AtUsers:     spec.WechatUserIDs,
				IsAtAll:     spec.IsAtAll,
			}
		}
	case setting.NotifyWebHookTypeMail:
		if spec.MailNotificationConfig != nil {
			resp.MailNotificationConfig = spec.MailNotificationConfig
		} else {
			if len(spec.MailUsers) == 0 {
				return nil, fmt.Errorf("failed to parse old notification data: mail_users field is empty")
			}
			resp.MailNotificationConfig = &commonmodels.MailNotificationConfig{TargetUsers: spec.MailUsers}
		}
	case setting.NotifyWebhookTypeFeishuApp:
		if spec.LarkGroupNotificationConfig != nil {
			resp.LarkGroupNotificationConfig = spec.LarkGroupNotificationConfig
		} else {
			if len(spec.FeiShuAppID) == 0 || spec.FeishuChat == nil {
				return nil, fmt.Errorf("failed to parse old notification data: either feishu_app field is empty or feishu_chat field is empty")
			}
			resp.LarkGroupNotificationConfig = &commonmodels.LarkGroupNotificationConfig{
				AppID:   spec.FeiShuAppID,
				Chat:    spec.FeishuChat,
				AtUsers: spec.LarkAtUsers,
				IsAtAll: spec.IsAtAll,
			}
		}
	case setting.NotifyWebHookTypeWebook:
		if spec.WebhookNotificationConfig != nil {
			resp.WebhookNotificationConfig = spec.WebhookNotificationConfig
		} else {
			if len(spec.WebHookNotify.Address) == 0 && len(spec.WebHookNotify.Token) == 0 {
				return nil, fmt.Errorf("failed to parse old notification data: webhook_notify field is nil")
			}
			resp.WebhookNotificationConfig = &spec.WebHookNotify
		}
	case setting.NotifyWebHookTypeFeishuPerson:
		if spec.LarkPersonNotificationConfig == nil {
			return nil, fmt.Errorf("lark_person_notification_config cannot be empty for type feishu_person notification")
		}

		resp.LarkPersonNotificationConfig = spec.LarkPersonNotificationConfig
	default:
		return nil, fmt.Errorf("unsupported notification type: %s", spec.WebHookType)
	}

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
