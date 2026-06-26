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

package controller

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	jobctrl "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller/job"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/plutusenterprise"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

type Workflow struct {
	*commonmodels.WorkflowV4
}

func CreateWorkflowController(wf *commonmodels.WorkflowV4) *Workflow {
	return &Workflow{wf}
}

type notificationDynamicRecipients struct {
	LarkHook   commonmodels.DynamicRecipients
	LarkGroup  commonmodels.DynamicRecipients
	LarkPerson commonmodels.DynamicRecipients
	Wechat     commonmodels.DynamicRecipients
	DingDing   commonmodels.DynamicRecipients
	MSTeams    commonmodels.DynamicRecipients
	Mail       commonmodels.DynamicRecipients
}

type workflowNotificationSpecBackup struct {
	StageIndex int
	JobIndex   int
	Recipients *notificationDynamicRecipients
}

func cloneDynamicRecipients(items commonmodels.DynamicRecipients) commonmodels.DynamicRecipients {
	if items == nil {
		return nil
	}
	resp := make(commonmodels.DynamicRecipients, len(items))
	copy(resp, items)
	return resp
}

func backupNotificationDynamicRecipients(
	larkHook *commonmodels.LarkHookNotificationConfig,
	larkGroup *commonmodels.LarkGroupNotificationConfig,
	larkPerson *commonmodels.LarkPersonNotificationConfig,
	wechat *commonmodels.WechatNotificationConfig,
	dingDing *commonmodels.DingDingNotificationConfig,
	msTeams *commonmodels.MSTeamsNotificationConfig,
	mail *commonmodels.MailNotificationConfig,
) *notificationDynamicRecipients {
	resp := &notificationDynamicRecipients{}
	if larkHook != nil {
		resp.LarkHook = cloneDynamicRecipients(larkHook.DynamicRecipients)
	}
	if larkGroup != nil {
		resp.LarkGroup = cloneDynamicRecipients(larkGroup.DynamicRecipients)
	}
	if larkPerson != nil {
		resp.LarkPerson = cloneDynamicRecipients(larkPerson.DynamicRecipients)
	}
	if wechat != nil {
		resp.Wechat = cloneDynamicRecipients(wechat.DynamicRecipients)
	}
	if dingDing != nil {
		resp.DingDing = cloneDynamicRecipients(dingDing.DynamicRecipients)
	}
	if msTeams != nil {
		resp.MSTeams = cloneDynamicRecipients(msTeams.DynamicRecipients)
	}
	if mail != nil {
		resp.Mail = cloneDynamicRecipients(mail.DynamicRecipients)
	}
	return resp
}

func restoreNotificationDynamicRecipients(
	recipients *notificationDynamicRecipients,
	larkHook *commonmodels.LarkHookNotificationConfig,
	larkGroup *commonmodels.LarkGroupNotificationConfig,
	larkPerson *commonmodels.LarkPersonNotificationConfig,
	wechat *commonmodels.WechatNotificationConfig,
	dingDing *commonmodels.DingDingNotificationConfig,
	msTeams *commonmodels.MSTeamsNotificationConfig,
	mail *commonmodels.MailNotificationConfig,
) {
	if recipients == nil {
		return
	}
	if larkHook != nil {
		larkHook.DynamicRecipients = cloneDynamicRecipients(recipients.LarkHook)
	}
	if larkGroup != nil {
		larkGroup.DynamicRecipients = cloneDynamicRecipients(recipients.LarkGroup)
	}
	if larkPerson != nil {
		larkPerson.DynamicRecipients = cloneDynamicRecipients(recipients.LarkPerson)
	}
	if wechat != nil {
		wechat.DynamicRecipients = cloneDynamicRecipients(recipients.Wechat)
	}
	if dingDing != nil {
		dingDing.DynamicRecipients = cloneDynamicRecipients(recipients.DingDing)
	}
	if msTeams != nil {
		msTeams.DynamicRecipients = cloneDynamicRecipients(recipients.MSTeams)
	}
	if mail != nil {
		mail.DynamicRecipients = cloneDynamicRecipients(recipients.Mail)
	}
}

func backupNotificationDynamicRecipientsFromWorkflowSpec(spec *commonmodels.NotificationJobSpec) *notificationDynamicRecipients {
	if spec == nil {
		return nil
	}
	return backupNotificationDynamicRecipients(
		spec.LarkHookNotificationConfig,
		spec.LarkGroupNotificationConfig,
		spec.LarkPersonNotificationConfig,
		spec.WechatNotificationConfig,
		spec.DingDingNotificationConfig,
		spec.MSTeamsNotificationConfig,
		spec.MailNotificationConfig,
	)
}

func restoreNotificationDynamicRecipientsToWorkflowSpec(spec *commonmodels.NotificationJobSpec, recipients *notificationDynamicRecipients) {
	if spec == nil || recipients == nil {
		return
	}
	restoreNotificationDynamicRecipients(
		recipients,
		spec.LarkHookNotificationConfig,
		spec.LarkGroupNotificationConfig,
		spec.LarkPersonNotificationConfig,
		spec.WechatNotificationConfig,
		spec.DingDingNotificationConfig,
		spec.MSTeamsNotificationConfig,
		spec.MailNotificationConfig,
	)
}

func backupWorkflowNotificationRuntimeRenderFields(workflow *commonmodels.WorkflowV4) ([]*workflowNotificationSpecBackup, error) {
	if workflow == nil {
		return nil, nil
	}

	resp := make([]*workflowNotificationSpecBackup, 0)
	for stageIndex, stage := range workflow.Stages {
		if stage == nil {
			continue
		}
		for jobIndex, job := range stage.Jobs {
			if job == nil || job.JobType != config.JobNotification {
				continue
			}
			spec := &commonmodels.NotificationJobSpec{}
			if err := commonmodels.IToi(job.Spec, spec); err != nil {
				return nil, fmt.Errorf("failed to decode notification job spec for job %s, error: %w", job.Name, err)
			}
			resp = append(resp, &workflowNotificationSpecBackup{
				StageIndex: stageIndex,
				JobIndex:   jobIndex,
				Recipients: backupNotificationDynamicRecipientsFromWorkflowSpec(spec),
			})
		}
	}
	return resp, nil
}

func restoreWorkflowNotificationRuntimeRenderFields(workflow *commonmodels.WorkflowV4, backups []*workflowNotificationSpecBackup) error {
	if workflow == nil {
		return nil
	}

	for _, backup := range backups {
		if backup == nil || backup.Recipients == nil {
			continue
		}
		if backup.StageIndex >= len(workflow.Stages) || workflow.Stages[backup.StageIndex] == nil {
			continue
		}
		stage := workflow.Stages[backup.StageIndex]
		if backup.JobIndex >= len(stage.Jobs) || stage.Jobs[backup.JobIndex] == nil {
			continue
		}
		job := stage.Jobs[backup.JobIndex]
		spec := &commonmodels.NotificationJobSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			return fmt.Errorf("failed to restore notification job spec for job %s, error: %w", job.Name, err)
		}
		restoreNotificationDynamicRecipientsToWorkflowSpec(spec, backup.Recipients)
		job.Spec = spec
	}
	return nil
}

func backupNotificationDynamicRecipientsFromTaskSpec(spec *commonmodels.JobTaskNotificationSpec) *notificationDynamicRecipients {
	if spec == nil {
		return nil
	}
	return backupNotificationDynamicRecipients(
		spec.LarkHookNotificationConfig,
		spec.LarkGroupNotificationConfig,
		spec.LarkPersonNotificationConfig,
		spec.WechatNotificationConfig,
		spec.DingDingNotificationConfig,
		spec.MSTeamsNotificationConfig,
		spec.MailNotificationConfig,
	)
}

func restoreNotificationDynamicRecipientsToTaskSpec(spec *commonmodels.JobTaskNotificationSpec, recipients *notificationDynamicRecipients) {
	if spec == nil || recipients == nil {
		return
	}
	restoreNotificationDynamicRecipients(
		recipients,
		spec.LarkHookNotificationConfig,
		spec.LarkGroupNotificationConfig,
		spec.LarkPersonNotificationConfig,
		spec.WechatNotificationConfig,
		spec.DingDingNotificationConfig,
		spec.MSTeamsNotificationConfig,
		spec.MailNotificationConfig,
	)
}

// RenderJobTaskWithGlobalVariables replays a task spec with persisted GlobalContext during retry/manual execution.
func RenderJobTaskWithGlobalVariables(task *commonmodels.JobTask, globalKeyMap map[string]string) error {
	if task == nil {
		return nil
	}

	var notificationRecipients *notificationDynamicRecipients
	if task.JobType == string(config.JobNotification) {
		// DynamicRecipients must stay as templates until NotificationJobCtl resolves them with payload/user mapping.
		// Rendering them here would turn {{.payload.user.email}} into a raw value and lose the identity suffix.
		spec := &commonmodels.JobTaskNotificationSpec{}
		if err := commonmodels.IToi(task.Spec, spec); err != nil {
			return fmt.Errorf("failed to decode notification task spec for task %s, error: %w", task.Name, err)
		}
		notificationRecipients = backupNotificationDynamicRecipientsFromTaskSpec(spec)
	}

	taskBytes, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task %s, error: %w", task.Name, err)
	}
	taskString := string(taskBytes)
	for k, v := range globalKeyMap {
		// Use json.Marshal to properly escape the value as it would appear in JSON.
		escapedValueBytes, _ := json.Marshal(v)
		escapedValue := string(escapedValueBytes)
		// Remove the surrounding quotes since we're replacing within a JSON string.
		escapedValue = strings.Trim(escapedValue, `"`)

		taskString = strings.ReplaceAll(taskString, fmt.Sprintf("{{.%s}}", k), escapedValue)
	}

	if err := json.Unmarshal([]byte(taskString), task); err != nil {
		return fmt.Errorf("failed to replace input variable for task: %s, error: %w", task.Name, err)
	}

	if notificationRecipients == nil {
		return nil
	}

	spec := &commonmodels.JobTaskNotificationSpec{}
	if err := commonmodels.IToi(task.Spec, spec); err != nil {
		return fmt.Errorf("failed to restore notification task spec for task %s, error: %w", task.Name, err)
	}
	restoreNotificationDynamicRecipientsToTaskSpec(spec, notificationRecipients)
	task.Spec = spec
	return nil
}

func (w *Workflow) SetPreset(ticket *commonmodels.ApprovalTicket) error {
	for _, stage := range w.Stages {
		for _, job := range stage.Jobs {
			if job.RunPolicy == config.DefaultNotRun {
				job.Skipped = true
			}

			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return err
			}

			err = ctrl.Update(true, ticket)
			if err != nil {
				return err
			}

			err = ctrl.SetOptions(ticket)
			if err != nil {
				return err
			}

			if job.JobType == config.JobZadigBuild ||
				job.JobType == config.JobIstioRelease ||
				job.JobType == config.JobIstioRollback ||
				job.JobType == config.JobZadigHelmChartDeploy ||
				job.JobType == config.JobK8sBlueGreenDeploy ||
				job.JobType == config.JobApollo ||
				job.JobType == config.JobK8sCanaryDeploy ||
				job.JobType == config.JobK8sGrayRelease {
				ctrl.ClearSelection()
			}

			job.Spec = ctrl.GetSpec()
		}
	}

	return nil
}

func (w *Workflow) ToJobTasks(taskID int64, creator, account, uid string, releasePlan *commonmodels.ReleasePlanRef) ([]*commonmodels.StageTask, error) {
	resp := make([]*commonmodels.StageTask, 0)

	globalKeyMap := make(map[string]string)

	// first we need to set the commit info to jobs so the built-in parameters can be rendered
	for _, stage := range w.Stages {
		for _, job := range stage.Jobs {
			if job.Skipped {
				continue
			}

			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return nil, err
			}

			err = ctrl.SetRepoCommitInfo()
			if err != nil {
				return nil, err
			}

			ctrl.ClearOptions()

			job.Spec = ctrl.GetSpec()

			finalKVs := make([]*commonmodels.KeyVal, 0)

			kvs, err := ctrl.GetVariableList(job.Name,
				true,
				false,
				false,
				true,
				true,
			)
			if err != nil {
				return nil, err
			}

			for _, kv := range kvs {
				finalKVs = append(finalKVs, kv)
			}

			keyvaultKV, err := commonservice.ListAvailableKeyVaultItemsForProject(w.Project, true)
			if err != nil {
				return nil, err
			}
			for _, kv := range keyvaultKV.Groups {
				for _, item := range kv.KVs {
					key := strings.Join([]string{"parameter", item.Group, item.Key}, ".")
					finalKVs = append(finalKVs, &commonmodels.KeyVal{Key: key, Value: item.Value, IsCredential: item.IsSensitive})
				}
			}

			for _, kv := range finalKVs {
				if kv.GetValue() != "" && !strings.HasPrefix(kv.GetValue(), "{{.") {
					globalKeyMap[kv.Key] = kv.GetValue()
					log.Debugf("insert key %s with value %s", kv.Key, kv.GetValue())
				} else {
					log.Warnf("key %s skipped due to no value or reference value: [%s]", kv.Key, kv.GetValue())
				}
			}
		}
	}

	workflowDefaultParams, err := w.getWorkflowDefaultParams(taskID, creator, account, uid, releasePlan)
	for _, param := range workflowDefaultParams {
		if param.GetValue() != "" && !strings.HasPrefix(param.GetValue(), "{{.") {
			globalKeyMap[param.Name] = param.GetValue()
			log.Debugf("insert key %s with value %s", param.Name, param.GetValue())
		} else {
			log.Warnf("key %s skipped due to no value or reference value: [%s]", param.Name, param.GetValue())
		}
	}

	// then we render the workflow with the built-in & user-defined parameter
	// TODO: this is probably deprecated due because we already render the workflow default params in the previous loop
	err = w.RenderWorkflowDefaultParams(taskID, creator, account, uid, releasePlan)
	if err != nil {
		return nil, err
	}

	for _, stage := range w.Stages {
		stageTask := &commonmodels.StageTask{
			Name:       stage.Name,
			Parallel:   stage.Parallel,
			ManualExec: stage.ManualExec,
		}

		jobTasks := make([]*commonmodels.JobTask, 0)
		for _, job := range stage.Jobs {
			if job.Skipped {
				if job.RunPolicy == config.ForceRun {
					return nil, fmt.Errorf("job %s skipped, but the run policy is set to force run", job.Name)
				} else {
					continue
				}
			}
			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return nil, err
			}

			tasks, err := ctrl.ToTask(taskID)
			if err != nil {
				return nil, err
			}
			for _, task := range tasks {
				task.NotifyCtls = job.NotifyCtls
			}

			// Update the spec, since sometimes we update the calculated field
			job.Spec = ctrl.GetSpec()

			switch job.JobType {
			case config.JobFreestyle, config.JobZadigTesting, config.JobZadigBuild, config.JobZadigScanning:
				if w.Debug {
					for _, task := range tasks {
						task.BreakpointBefore = true
					}
				}
			}

			for _, task := range tasks {
				if err := RenderJobTaskWithGlobalVariables(task, globalKeyMap); err != nil {
					return nil, err
				}
			}

			jobTasks = append(jobTasks, tasks...)
		}

		if len(jobTasks) > 0 {
			stageTask.Jobs = jobTasks
			resp = append(resp, stageTask)
		}
	}

	return resp, nil
}

func (w *Workflow) SetParameterRepoCommitInfo() {
	for _, param := range w.Params {
		if param.ParamsType != "repo" {
			continue
		}
		err := commonservice.FillRepositoryInfo(param.Repo)
		// TODO: possibly fix this logic. This is a compatibility code for old version. we should not skip it.
		if err != nil {
			log.Errorf("failed to fill repository info for workflow: %s, param key: %s, error: %s", w.Name, param.Name, err)
		}
	}
}

// UpdateWithLatestWorkflow use the current workflow as input and update the fields to the latest workflow's setting
func (w *Workflow) UpdateWithLatestWorkflow(ticket *commonmodels.ApprovalTicket) error {
	latestWorkflowSettings, err := commonrepo.NewWorkflowV4Coll().Find(w.Name)
	if err != nil {
		return e.ErrFindWorkflow.AddDesc(fmt.Sprintf("cannot find workflow [%s]'s latest setting, error: %s", w.Name, err))
	}

	w.Params = renderParams(latestWorkflowSettings.Params, w.Params)

	newStage := make([]*commonmodels.WorkflowStage, 0)
	err = util.DeepCopy(&newStage, &latestWorkflowSettings.Stages)
	if err != nil {
		return err
	}

	originJobMap := make(map[string]*commonmodels.Job)
	for _, stage := range w.Stages {
		for _, job := range stage.Jobs {
			originJobMap[job.Name] = job
		}
	}

	for _, stage := range newStage {
		jobList := make([]*commonmodels.Job, 0)
		for _, job := range stage.Jobs {
			if originJob, ok := originJobMap[job.Name]; !ok || originJob.JobType != job.JobType {
				// if we didn't find the job in the workflow to be merged, simply merge the new job with the latest workflow
				ctrl, err := jobctrl.CreateJobController(job, latestWorkflowSettings)
				if err != nil {
					return err
				}

				err = ctrl.SetOptions(ticket)
				if err != nil {
					return err
				}

				job.Spec = ctrl.GetSpec()
				jobList = append(jobList, job)

				continue
			}

			// otherwise we do a merge of the origin job with the latest workflow
			ctrl, err := jobctrl.CreateJobController(originJobMap[job.Name], latestWorkflowSettings)
			if err != nil {
				return err
			}

			err = ctrl.Update(true, ticket)
			if err != nil {
				return err
			}

			err = ctrl.SetOptions(ticket)
			if err != nil {
				return err
			}

			originJobMap[job.Name].Spec = ctrl.GetSpec()
			originJobMap[job.Name].RunPolicy = job.RunPolicy
			originJobMap[job.Name].ErrorPolicy = job.ErrorPolicy
			originJobMap[job.Name].ExecutePolicy = job.ExecutePolicy
			jobList = append(jobList, originJobMap[job.Name])
		}
		stage.Jobs = jobList
	}

	w.Stages = newStage
	return nil
}

func (w *Workflow) ClearOptions() error {
	for _, stage := range w.Stages {
		for _, job := range stage.Jobs {
			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return err
			}

			ctrl.ClearOptions()

			job.Spec = ctrl.GetSpec()
		}
	}
	return nil
}

func (w *Workflow) RenderWorkflowDefaultParams(taskID int64, creator, account, uid string, releasePlan *commonmodels.ReleasePlanRef) error {
	b, err := json.Marshal(w.WorkflowV4)
	if err != nil {
		return fmt.Errorf("marshal workflow error: %v", err)
	}
	notificationBackups, err := backupWorkflowNotificationRuntimeRenderFields(w.WorkflowV4)
	if err != nil {
		return err
	}
	globalParams, err := w.getWorkflowDefaultParams(taskID, creator, account, uid, releasePlan)
	if err != nil {
		return fmt.Errorf("get workflow default params error: %v", err)
	}
	replacedString := renderMultiLineString(string(b), globalParams)
	if err := json.Unmarshal([]byte(replacedString), &w.WorkflowV4); err != nil {
		return err
	}
	return restoreWorkflowNotificationRuntimeRenderFields(w.WorkflowV4, notificationBackups)
}

func (w *Workflow) getWorkflowDefaultParams(taskID int64, creator, account, uid string, releasePlan *commonmodels.ReleasePlanRef) ([]*commonmodels.Param, error) {
	resp := []*commonmodels.Param{}

	projectName := ""
	if w.Project != "" {
		projectInfo, err := templaterepo.NewProductColl().Find(w.Project)
		if err != nil {
			return nil, fmt.Errorf("failed to find project info for project %s, error: %s", w.Project, err)
		}
		projectName = projectInfo.ProjectName
	}
	resp = append(resp, &commonmodels.Param{Name: "project", Value: w.Project, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "project.id", Value: w.Project, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "project.name", Value: projectName, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.id", Value: w.workflowID(), ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.name", Value: w.workflowName(), ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.id", Value: fmt.Sprintf("%d", taskID), ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.creator", Value: creator, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.creator.id", Value: account, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.creator.userId", Value: uid, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.is_release_plan_trigger", Value: fmt.Sprintf("%t", releasePlan != nil), ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.timestamp", Value: fmt.Sprintf("%d", time.Now().Unix()), ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.datetime", Value: time.Now().Format(time.DateTime), ParamsType: "string", IsCredential: false})
	detailURL := ""
	if w.Project != "" {
		detailURL = fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
			configbase.SystemAddress(),
			w.Project,
			w.workflowID(),
			taskID,
			url.QueryEscape(w.workflowName()),
		)
	}
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.url", Value: detailURL, ParamsType: "string", IsCredential: false})

	for _, param := range w.Params {
		paramsKey := strings.Join([]string{"workflow", "params", param.Name}, ".")
		newParam := &commonmodels.Param{Name: paramsKey, Value: param.Value, ParamsType: "string", IsCredential: false}
		if param.ParamsType == string(commonmodels.MultiSelectType) {
			newParam.Value = strings.Join(param.ChoiceValue, ",")
		} else if param.ParamsType == string(commonmodels.FileType) {
			continue
		}
		resp = append(resp, newParam)
	}
	if w.HookPayload != nil {
		for _, kv := range commonutil.BuildWorkflowTriggerVariableKVs(w.HookPayload) {
			resp = append(resp, &commonmodels.Param{
				Name:         kv.Key,
				Value:        kv.Value,
				ParamsType:   "string",
				IsCredential: kv.IsCredential,
			})
		}
		for _, kv := range commonutil.BuildPayloadVariables(w.HookPayload.RawPayload) {
			resp = append(resp, &commonmodels.Param{
				Name:         kv.Key,
				Value:        kv.Value,
				ParamsType:   "string",
				IsCredential: kv.IsCredential,
			})
		}
	}
	return resp, nil
}

func (w *Workflow) workflowID() string {
	if w.Name != "" {
		return w.Name
	}
	if w.TemplateName != "" {
		return w.TemplateName
	}
	return w.DisplayName
}

func (w *Workflow) workflowName() string {
	if w.DisplayName != "" {
		return w.DisplayName
	}
	if w.TemplateName != "" {
		return w.TemplateName
	}
	return w.Name
}

// sanitizeDynamicVariableKey sanitizes the dynamic variable key by replacing special characters with underscores.
func sanitizeDynamicVariableKey(key string) string {
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, ".", "_")
	return key
}

// GetWorkflowParamReferableVariables returns the workflow param referable variables.
func (w *Workflow) GetWorkflowParamReferableVariables(taskID int64, creator, account, uid string, releasePlan *commonmodels.ReleasePlanRef) ([]*commonmodels.KeyVal, error) {
	globalParams, err := w.getWorkflowDefaultParams(taskID, creator, account, uid, releasePlan)
	if err != nil {
		return nil, fmt.Errorf("get workflow default params error: %v", err)
	}

	resp := make([]*commonmodels.KeyVal, 0, len(globalParams))
	for _, param := range globalParams {
		if param.ParamsType == "repo" || param.ParamsType == "file" {
			continue
		}

		resp = append(resp, &commonmodels.KeyVal{
			Key:          param.Name,
			Value:        param.GetValue(),
			Type:         "string",
			IsCredential: param.IsCredential,
		})
	}

	return resp, nil
}

func (w *Workflow) getWorkflowParamDynamicValueMap(taskID int64, creator, account, uid string, releasePlan *commonmodels.ReleasePlanRef) (map[string]string, error) {
	globalParams, err := w.GetWorkflowParamReferableVariables(taskID, creator, account, uid, releasePlan)
	if err != nil {
		return nil, err
	}

	resp := make(map[string]string, len(globalParams))
	for _, param := range globalParams {
		if param.Value == "" {
			continue
		}
		resp[sanitizeDynamicVariableKey(param.Key)] = param.Value
	}

	return resp, nil
}

// RenderWorkflowDynamicParams renders the workflow dynamic params.
func (w *Workflow) RenderWorkflowDynamicParams(taskID int64, creator, account, uid string, releasePlan *commonmodels.ReleasePlanRef) error {
	valueMap, err := w.getWorkflowParamDynamicValueMap(taskID, creator, account, uid, releasePlan)
	if err != nil {
		return fmt.Errorf("get workflow param dynamic value map error: %v", err)
	}

	for _, param := range w.Params {
		if param.ParamsType == "repo" || param.ParamsType == "file" || param.Script == "" || param.CallFunction == "" {
			continue
		}

		resp, err := jobctrl.RenderScriptedVariableOptions("", "", param.Script, param.CallFunction, valueMap)
		if err != nil {
			return fmt.Errorf("render workflow param %s dynamic options error: %v", param.Name, err)
		}

		param.ChoiceOption = resp
		if param.GetValue() != "" {
			valueMap[sanitizeDynamicVariableKey(strings.Join([]string{"workflow", "params", param.Name}, "."))] = param.GetValue()
		}
	}

	return nil
}

func (w *Workflow) GetWorkflowParamDynamicValues(taskID int64, creator, account, uid string, key string, releasePlan *commonmodels.ReleasePlanRef) ([]string, error) {
	valueMap, err := w.getWorkflowParamDynamicValueMap(taskID, creator, account, uid, releasePlan)
	if err != nil {
		return nil, fmt.Errorf("get workflow param dynamic value map error: %v", err)
	}

	for _, param := range w.Params {
		if param.Name != key && strings.Join([]string{"workflow", "params", param.Name}, ".") != key {
			continue
		}

		resp, err := jobctrl.RenderScriptedVariableOptions("", "", param.Script, param.CallFunction, valueMap)
		if err != nil {
			return nil, fmt.Errorf("render workflow param %s dynamic options error: %v", param.Name, err)
		}
		return resp, nil
	}

	return nil, fmt.Errorf("workflow param %s not found", key)
}

func buildRuntimeReferableVariables(workflow *commonmodels.WorkflowV4) []*commonmodels.KeyVal {
	resp := make([]*commonmodels.KeyVal, 0)
	resp = append(resp, &commonmodels.KeyVal{
		Key:          "workflow.task.creator",
		Value:        "",
		Type:         "string",
		IsCredential: false,
	})
	resp = append(resp, &commonmodels.KeyVal{
		Key:          "workflow.task.creator.id",
		Value:        "",
		Type:         "string",
		IsCredential: false,
	})
	resp = append(resp, &commonmodels.KeyVal{
		Key:          "workflow.task.creator.userId",
		Value:        "",
		Type:         "string",
		IsCredential: false,
	})
	resp = append(resp, &commonmodels.KeyVal{
		Key:          "workflow.task.is_release_plan_trigger",
		Value:        "",
		Type:         "string",
		IsCredential: false,
	})
	resp = append(resp, &commonmodels.KeyVal{
		Key:          "workflow.task.timestamp",
		Value:        "",
		Type:         "string",
		IsCredential: false,
	})
	resp = append(resp, &commonmodels.KeyVal{
		Key:          "workflow.task.datetime",
		Value:        "",
		Type:         "string",
		IsCredential: false,
	})
	resp = append(resp, &commonmodels.KeyVal{
		Key:          "workflow.task.id",
		Value:        "",
		Type:         "string",
		IsCredential: false,
	})
	resp = append(resp, &commonmodels.KeyVal{
		Key:          "workflow.task.url",
		Value:        workflow.Name,
		Type:         "string",
		IsCredential: false,
	})
	resp = append(resp, &commonmodels.KeyVal{Key: "workflow.trigger.branch", Type: "string", IsCredential: false})
	resp = append(resp, &commonmodels.KeyVal{Key: "workflow.trigger.target_branch", Type: "string", IsCredential: false})
	resp = append(resp, &commonmodels.KeyVal{Key: "workflow.trigger.pr", Type: "string", IsCredential: false})
	resp = append(resp, &commonmodels.KeyVal{Key: "workflow.trigger.commit_id", Type: "string", IsCredential: false})
	resp = append(resp, &commonmodels.KeyVal{Key: "workflow.trigger.commit_sha", Type: "string", IsCredential: false})
	resp = append(resp, &commonmodels.KeyVal{Key: "workflow.trigger.commit_message", Type: "string", IsCredential: false})
	resp = append(resp, &commonmodels.KeyVal{Key: "workflow.trigger.committer", Type: "string", IsCredential: false})
	resp = append(resp, &commonmodels.KeyVal{Key: "workflow.trigger.event", Type: "string", IsCredential: false})
	return resp
}

func (w *Workflow) Validate(isExecution bool) error {
	if w.Project == "" {
		err := fmt.Errorf("project should not be empty")
		return e.ErrLintWorkflow.AddErr(err)
	}

	match, err := regexp.MatchString(setting.WorkflowRegx, w.Name)
	if err != nil {
		return e.ErrLintWorkflow.AddErr(err)
	}
	if !match {
		errMsg := "工作流标识支持大小写字母、数字和中划线"
		return e.ErrLintWorkflow.AddDesc(errMsg)
	}

	project, err := templaterepo.NewProductColl().Find(w.Project)
	if err != nil {
		return e.ErrLintWorkflow.AddErr(err)
	}

	licenseStatus, err := plutusenterprise.New().CheckZadigXLicenseStatus()
	if err != nil {
		return fmt.Errorf("failed to validate zadig license status, error: %s", err)
	}
	if !commonutil.ValidateZadigProfessionalLicense(licenseStatus) {
		if w.ConcurrencyLimit != -1 && w.ConcurrencyLimit != 1 {
			return e.ErrLicenseInvalid.AddDesc("基础版工作流并发只支持开关，不支持数量")
		}
	}

	if project.ProductFeature != nil {
		if project.ProductFeature.DeployType != setting.K8SDeployType && project.ProductFeature.DeployType != setting.HelmDeployType {
			return e.ErrLintWorkflow.AddDesc("common workflow only support k8s and helm project")
		}
	}
	stageNameMap := make(map[string]bool)
	jobNameMap := make(map[string]string)

	reg, err := regexp.Compile(setting.JobNameRegx)
	if err != nil {
		return e.ErrLintWorkflow.AddErr(err)
	}

	var latestWorkflowSettings *commonmodels.WorkflowV4
	if isExecution {
		latestWorkflowSettings, err = commonrepo.NewWorkflowV4Coll().Find(w.Name)
		if err != nil {
			return e.ErrFindWorkflow.AddDesc(fmt.Sprintf("cannot find workflow [%s]'s latest setting, error: %s", w.Name, err))
		}
		w.RemarkRequired = latestWorkflowSettings.RemarkRequired
		if latestWorkflowSettings.RemarkRequired && strings.TrimSpace(w.Remark) == "" {
			return e.ErrLintWorkflow.AddDesc("workflow remark is required.")
		}
		if err := validateRequiredWorkflowParams(w.Params); err != nil {
			return e.ErrLintWorkflow.AddErr(err)
		}
	}

	for _, stage := range w.Stages {
		if !commonutil.ValidateZadigProfessionalLicense(licenseStatus) {
			if stage.ManualExec != nil && stage.ManualExec.Enabled {
				return e.ErrLicenseInvalid.AddDesc("基础版不支持工作流手动执行")
			}
		}
		if stage.ManualExec != nil && stage.ManualExec.LarkPersonNotificationConfig != nil {
			if stage.ManualExec.LarkPersonNotificationConfig.AppID == "" {
				return e.ErrLintWorkflow.AddDesc(fmt.Sprintf("manual execution notification app id cannot be empty for stage %s", stage.Name))
			}
		}
		if _, ok := stageNameMap[stage.Name]; !ok {
			stageNameMap[stage.Name] = true
		} else {
			return e.ErrLintWorkflow.AddDesc(fmt.Sprintf("duplicated stage name: %s", stage.Name))
		}
		for _, job := range stage.Jobs {
			if match := reg.MatchString(job.Name); !match {
				return e.ErrLintWorkflow.AddDesc(fmt.Sprintf("job name [%s] did not match %s", job.Name, setting.JobNameRegx))
			}
			if _, ok := jobNameMap[job.Name]; !ok {
				jobNameMap[job.Name] = string(job.JobType)
			} else {
				return e.ErrLintWorkflow.AddDesc(fmt.Sprintf("duplicated job name: %s", job.Name))
			}
			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return e.ErrLintWorkflow.AddErr(err)
			}

			if isExecution {
				if job.Skipped {
					// skip validation if a job is skipped when executing
					continue
				}
				ctrl.SetWorkflow(latestWorkflowSettings)
			}

			if err := ctrl.Validate(isExecution); err != nil {
				return e.ErrLintWorkflow.AddErr(err)
			}
		}
	}
	return nil
}

func validateRequiredWorkflowParams(params []*commonmodels.Param) error {
	for _, param := range params {
		if !param.Required {
			continue
		}
		if !workflowParamValueProvided(param) {
			return fmt.Errorf("workflow param %s is required", param.Name)
		}
	}
	return nil
}

func workflowParamValueProvided(param *commonmodels.Param) bool {
	if param == nil {
		return false
	}

	switch param.ParamsType {
	case string(commonmodels.MultiSelectType):
		return len(param.ChoiceValue) > 0
	case "repo":
		return repositoryValueProvided(param.Repo)
	case string(commonmodels.FileType):
		return strings.TrimSpace(param.FileID) != "" || strings.TrimSpace(param.FilePath) != ""
	default:
		return strings.TrimSpace(param.Value) != ""
	}
}

func repositoryValueProvided(repo *types.Repository) bool {
	if repo == nil {
		return false
	}
	if strings.TrimSpace(repo.RepoName) == "" || strings.TrimSpace(repo.GetRepoNamespace()) == "" {
		return false
	}
	return strings.TrimSpace(repo.Ref()) != ""
}

func (w *Workflow) SetRepo(repo *types.Repository) error {
	for _, stage := range w.Stages {
		for _, job := range stage.Jobs {
			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return err
			}

			err = ctrl.SetRepo(repo)
			if err != nil {
				return err
			}

			job.Spec = ctrl.GetSpec()
		}
	}
	return nil
}

func (w *Workflow) GetDynamicVariableValues(jobName, serviceName, moduleName, key string, buildInVarMap map[string]string) ([]string, error) {
	latestWorkflowSettings, err := commonrepo.NewWorkflowV4Coll().Find(w.Name)
	if err != nil {
		return nil, e.ErrFindWorkflow.AddDesc(fmt.Sprintf("cannot find workflow [%s]'s latest setting, error: %s", w.Name, err))
	}

	job, err := w.FindJob(jobName, "")
	if err != nil {
		return nil, err
	}

	ctrl, err := jobctrl.CreateJobController(job, latestWorkflowSettings)
	if err != nil {
		return nil, err
	}

	resp, err := ctrl.RenderDynamicVariableOptions(key, &jobctrl.RenderDynamicVariableValue{
		ServiceName:   serviceName,
		ServiceModule: moduleName,
		Values:        buildInVarMap,
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

type GetWorkflowVariablesOption struct {
	// GetAggregatedVariables gets the job's aggregated information, such as job.<name>.SERVICES in build job/ job.<name>.IMAGES in deploy job
	GetAggregatedVariables bool
	// GetRuntimeVariables gets the variables that can only be rendered in the workflow runtime. There are several examples:
	// 1. workflow level variables: workflow.task.creator
	// 2. outputs defined in each separate job
	GetRuntimeVariables bool
	// GetPlaceHolderVariables gets the variables with service/module placeholders, such as
	// job.jobName.<service>.<module>.xxxx
	GetPlaceHolderVariables bool
	// GetServiceSpecificVariables gets the variables with service/module placeholders, such as
	// job.jobName.service1.module1.xxxx
	// NOTE that there is a special case to this flag: job.jobName.service1.module1.BRANCH/COMMITID/GITURL in the build job.
	// these 3 variable is controlled by GetRuntimeVariables
	GetServiceSpecificVariables bool
	// GetReferredKeyValVariables gets the referred build/scan/testing job's key value as variables
	UseUserInput bool
}

// CurrentJobVariableMode defines how to handle the current job's variables
type CurrentJobVariableMode string

const (
	// CurrentJobModeSkip - don't want any parameters from the current job
	CurrentJobModeSkip CurrentJobVariableMode = "skip"
	// CurrentJobModeInputOnly - want the current job's input, but not aggregated variables and runtime variables
	CurrentJobModeInputOnly CurrentJobVariableMode = "input_only"
	// CurrentJobModeAll - want all parameters from the current job
	CurrentJobModeAll CurrentJobVariableMode = "all"
)

// GetReferableVariables gets all the variable that can be used by dynamic variables/other job to refer.
// 1. the key in the response is returned in the a.b.c format, there will be no {{.}} format or replacing . with _ logic
// caller will need to process that by themselves.
// 2. Note that runtime variables will not have values in the response, use the value in the response with care.
// 3. the rendered KV will only have type string since it is mainly used for dynamic variable rendering, change this if required
// 4. currentJobMode controls how the current job's variables are handled:
//   - CurrentJobModeSkip: don't include any parameters from the current job
//   - CurrentJobModeInputOnly: include current job's input, but not aggregated variables and runtime variables
//   - CurrentJobModeAll: include all parameters from the current job
func (w *Workflow) GetReferableVariables(currentJobName string, option GetWorkflowVariablesOption, currentJobMode CurrentJobVariableMode) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)

	resp = append(resp, &commonmodels.KeyVal{
		Key:          "project",
		Value:        w.Project,
		Type:         "string",
		IsCredential: false,
	})

	resp = append(resp, &commonmodels.KeyVal{
		Key:          "project.id",
		Value:        w.Project,
		Type:         "string",
		IsCredential: false,
	})

	// compatible with workflow template: project is empty when this is a workflow template
	if w.Project != "" {
		projectInfo, err := templaterepo.NewProductColl().Find(w.Project)
		if err != nil {
			return nil, fmt.Errorf("failed to find project info for project %s, error: %s", w.Project, err)
		}

		resp = append(resp, &commonmodels.KeyVal{
			Key:          "project.name",
			Value:        projectInfo.ProjectName,
			Type:         "string",
			IsCredential: false,
		})
	} else {
		resp = append(resp, &commonmodels.KeyVal{
			Key:          "project.name",
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})
	}

	resp = append(resp, &commonmodels.KeyVal{
		Key:          "workflow.id",
		Value:        w.workflowID(),
		Type:         "string",
		IsCredential: false,
	})

	resp = append(resp, &commonmodels.KeyVal{
		Key:          "workflow.name",
		Value:        w.workflowName(),
		Type:         "string",
		IsCredential: false,
	})

	if option.GetRuntimeVariables {
		resp = append(resp, buildRuntimeReferableVariables(w.WorkflowV4)...)
	}

	for _, param := range w.Params {
		if param.ParamsType == "repo" || param.ParamsType == "file" {
			continue
		}

		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"workflow", "params", param.Name}, "."),
			Value:        param.GetValue(),
			Type:         "string",
			IsCredential: false,
		})
	}

	currJob, err := w.FindJob(currentJobName, "")
	if err != nil {
		return nil, fmt.Errorf("failed to find job: %s, error: %s", currentJobName, err)
	}

	currJobCtrl, err := jobctrl.CreateJobController(currJob, w.WorkflowV4)
	if err != nil {
		return nil, err
	}

	jobRankMap := jobctrl.GetJobRankMap(w.Stages)

	for _, stage := range w.Stages {
		for _, j := range stage.Jobs {
			isCurrentJob := j.Name == currentJobName

			// Handle current job based on currentJobMode
			if isCurrentJob && currentJobMode == CurrentJobModeSkip {
				continue
			}

			getServiceSpecificVariablesFlag := option.GetServiceSpecificVariables
			getPlaceHolderVariablesFlag := option.GetPlaceHolderVariables
			getRuntimeVariableFlag := option.GetRuntimeVariables
			getAggregatedVariableFlag := option.GetAggregatedVariables

			// For current job with InputOnly mode, disable aggregated and runtime variables
			if isCurrentJob && currentJobMode == CurrentJobModeInputOnly {
				getRuntimeVariableFlag = false
				getAggregatedVariableFlag = false
			}

			if currentJobName != "" && jobRankMap[currentJobName] < jobRankMap[j.Name] {
				// you cant get a job's output if the current job is runs before given job
				getRuntimeVariableFlag = false
			}

			// service_module cannot be determined in
			if currJob.JobType == config.JobZadigDeploy {
				getPlaceHolderVariablesFlag = false
			}

			ctrl, err := jobctrl.CreateJobController(j, w.WorkflowV4)
			if err != nil {
				return nil, err
			}

			if !currJobCtrl.IsServiceTypeJob() {
				getServiceSpecificVariablesFlag = true
				getPlaceHolderVariablesFlag = false
			}

			kv, err := ctrl.GetVariableList(j.Name,
				getAggregatedVariableFlag,
				getRuntimeVariableFlag,
				getPlaceHolderVariablesFlag,
				getServiceSpecificVariablesFlag,
				option.UseUserInput,
			)

			if err != nil {
				return nil, err
			}

			resp = append(resp, kv...)
		}
	}

	keyvaultKV, err := commonservice.ListAvailableKeyVaultItemsForProject(w.Project, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get kv pair from keyvault for project %s, error: %s", w.Project, err)
	}

	for _, group := range keyvaultKV.Groups {
		for _, item := range group.KVs {
			key := strings.Join([]string{"parameter", item.Group, item.Key}, ".")
			resp = append(resp, &commonmodels.KeyVal{Key: key, Value: item.Value, IsCredential: item.IsSensitive})
		}
	}

	return resp, nil
}

func (w *Workflow) GetUsedRepos() ([]*types.Repository, error) {
	resp := make([]*types.Repository, 0)
	for _, stage := range w.Stages {
		for _, j := range stage.Jobs {
			ctrl, err := jobctrl.CreateJobController(j, w.WorkflowV4)
			if err != nil {
				return nil, err
			}

			usedRepos, err := ctrl.GetUsedRepos()
			if err != nil {
				return nil, err
			}
			resp = append(resp, usedRepos...)
		}
	}
	return resp, nil
}

func renderParams(origin, input []*commonmodels.Param) []*commonmodels.Param {
	resp := make([]*commonmodels.Param, 0)
	for _, originParam := range origin {
		found := false
		for _, inputParam := range input {
			if originParam.Name == inputParam.Name {
				// always use origin credential config.
				newParam := &commonmodels.Param{
					Name:         originParam.Name,
					Description:  originParam.Description,
					ParamsType:   originParam.ParamsType,
					Value:        originParam.Value,
					Repo:         originParam.Repo,
					ChoiceOption: originParam.ChoiceOption,
					ChoiceValue:  originParam.ChoiceValue,
					Script:       originParam.Script,
					CallFunction: originParam.CallFunction,
					Default:      originParam.Default,
					IsCredential: originParam.IsCredential,
					Source:       originParam.Source,
					Required:     originParam.Required,
				}
				if originParam.Source != config.ParamSourceFixed && originParam.Source != config.ParamSourceReference {
					newParam.Value = inputParam.Value
					newParam.Repo = inputParam.Repo
					newParam.ChoiceValue = inputParam.ChoiceValue
				}
				resp = append(resp, newParam)
				found = true
				break
			}
		}
		if !found {
			resp = append(resp, originParam)
		}
	}

	return resp
}
