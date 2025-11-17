/*
Copyright 2025 The KodeRover Authors.

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
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type NotificationJobController struct {
	*BasicInfo

	jobSpec *commonmodels.NotificationJobSpec
}

func CreateNotificationJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.NotificationJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:          job.Name,
		jobType:       job.JobType,
		errorPolicy:   job.ErrorPolicy,
		executePolicy: job.ExecutePolicy,
		workflow:      workflow,
	}

	return NotificationJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j NotificationJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j NotificationJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j NotificationJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	return nil
}

func (j NotificationJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.NotificationJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.WebHookType = currJobSpec.WebHookType
	j.jobSpec.Source = currJobSpec.Source

	if currJobSpec.Source == "runtime" {
		if currJobSpec.LarkGroupNotificationConfig != nil && j.jobSpec.LarkGroupNotificationConfig != nil {
			currJobSpec.LarkGroupNotificationConfig.AtUsers = j.jobSpec.LarkGroupNotificationConfig.AtUsers
			currJobSpec.LarkGroupNotificationConfig.IsAtAll = j.jobSpec.LarkGroupNotificationConfig.IsAtAll
		}
		if currJobSpec.LarkPersonNotificationConfig != nil && j.jobSpec.LarkPersonNotificationConfig != nil {
			currJobSpec.LarkPersonNotificationConfig.TargetUsers = j.jobSpec.LarkPersonNotificationConfig.TargetUsers
		}
		if currJobSpec.WechatNotificationConfig != nil && j.jobSpec.WechatNotificationConfig != nil {
			currJobSpec.WechatNotificationConfig.AtUsers = j.jobSpec.WechatNotificationConfig.AtUsers
			currJobSpec.WechatNotificationConfig.IsAtAll = j.jobSpec.WechatNotificationConfig.IsAtAll
		}
		if currJobSpec.DingDingNotificationConfig != nil && j.jobSpec.DingDingNotificationConfig != nil {
			currJobSpec.DingDingNotificationConfig.AtMobiles = j.jobSpec.DingDingNotificationConfig.AtMobiles
			currJobSpec.DingDingNotificationConfig.IsAtAll = j.jobSpec.DingDingNotificationConfig.IsAtAll
		}
		if currJobSpec.MSTeamsNotificationConfig != nil && j.jobSpec.MSTeamsNotificationConfig != nil {
			currJobSpec.MSTeamsNotificationConfig.AtEmails = j.jobSpec.MSTeamsNotificationConfig.AtEmails
		}
		if currJobSpec.MailNotificationConfig != nil && j.jobSpec.MailNotificationConfig != nil {
			currJobSpec.MailNotificationConfig.TargetUsers = j.jobSpec.MailNotificationConfig.TargetUsers
		}
	}

	// use the latest webhook settings, except for title and content
	j.jobSpec.LarkGroupNotificationConfig = currJobSpec.LarkGroupNotificationConfig
	j.jobSpec.LarkPersonNotificationConfig = currJobSpec.LarkPersonNotificationConfig
	j.jobSpec.WechatNotificationConfig = currJobSpec.WechatNotificationConfig
	j.jobSpec.DingDingNotificationConfig = currJobSpec.DingDingNotificationConfig
	j.jobSpec.MSTeamsNotificationConfig = currJobSpec.MSTeamsNotificationConfig
	j.jobSpec.MailNotificationConfig = currJobSpec.MailNotificationConfig
	j.jobSpec.WebhookNotificationConfig = currJobSpec.WebhookNotificationConfig

	// ========= compatibility code below, these field will only be used to generate new configuration ===============
	j.jobSpec.WeChatWebHook = currJobSpec.WeChatWebHook
	j.jobSpec.DingDingWebHook = currJobSpec.DingDingWebHook
	j.jobSpec.FeiShuAppID = currJobSpec.FeiShuAppID
	j.jobSpec.FeishuChat = currJobSpec.FeishuChat
	j.jobSpec.MailUsers = currJobSpec.MailUsers
	j.jobSpec.WebHookNotify = currJobSpec.WebHookNotify
	j.jobSpec.AtMobiles = currJobSpec.AtMobiles
	j.jobSpec.WechatUserIDs = currJobSpec.WechatUserIDs
	j.jobSpec.LarkAtUsers = currJobSpec.LarkAtUsers
	j.jobSpec.IsAtAll = currJobSpec.IsAtAll

	return nil
}

func (j NotificationJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j NotificationJobController) ClearOptions() {
	return
}

func (j NotificationJobController) ClearSelection() {
	return
}

func (j NotificationJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	taskSpec, err := generateNotificationJobSpec(j.jobSpec)
	if err != nil {
		return nil, err
	}

	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType:       string(config.JobNotification),
		Spec:          taskSpec,
		Timeout:       0,
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
	}

	resp = append(resp, jobTask)

	return resp, nil
}

func (j NotificationJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j NotificationJobController) SetRepoCommitInfo() error {
	return nil
}

func (j NotificationJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)
	if getRuntimeVariables {
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "status"}, "."),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})
	}
	return resp, nil
}

func (j NotificationJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j NotificationJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j NotificationJobController) IsServiceTypeJob() bool {
	return false
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
