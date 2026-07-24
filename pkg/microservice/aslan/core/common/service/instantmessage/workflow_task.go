/*
Copyright 2022 The KodeRover Authors.

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

package instantmessage

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/hashicorp/go-multierror"
	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/dynamicrecipient"
	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhooknotify"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	workflownotifyutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util/workflownotify"
	"github.com/koderover/zadig/v2/pkg/setting"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/sonar"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

var (
	zhTextMap = map[string]string{
		"taskTypeWorkflow": "工作流",
		"taskTypeScanning": "代码扫描",
		"taskTypeTesting":  "测试",

		"taskStatusSuccess":           "执行成功",
		"taskStatusFailed":            "执行失败",
		"taskStatusCancelled":         "执行取消",
		"taskStatusTimeout":           "执行超时",
		"taskStatusRejected":          "执行被拒绝",
		"taskStatusExecutionStarted":  "开始执行",
		"taskStatusManualApproval":    "待确认",
		"taskStatusPause":             "暂停",
		"taskStatusWaitingManualExec": "等待手动执行",
		"jobStatusUnstarted":          "未执行",

		"jobTypeBuild":            "构建",
		"jobTypeDeploy":           "容器服务部署",
		"jobTypeVmDeploy":         "主机服务部署",
		"jobTypeFreestyle":        "通用任务",
		"jobTypeNacos":            "Nacos 配置变更",
		"jobTypePlugin":           "插件",
		"jobTypeTest":             "测试",
		"jobTypeScan":             "代码扫描",
		"jobTypeApproval":         "人工审批",
		"jobTypeDistribute":       "镜像分发",
		"jobTypeCustomDeploy":     "Kubernetes 部署",
		"jobTypeCanaryDeploy":     "金丝雀部署",
		"jobTypeCanaryRelease":    "金丝雀发布",
		"jobTypeMseGrayRelease":   "MSE 灰度发布",
		"jobTypeMseGrayOffline":   "下线 MSE 灰度服务",
		"jobTypeBlueGreenDeploy":  "部署蓝绿环境",
		"jobTypeBlueGreenRelease": "蓝绿发布",
		"jobTypeK8sResourcePatch": "更新 K8s YAML 任务",
		"jobTypeK8sGrayRollback":  "灰度回滚",
		"jobTypeGrayDeploy":       "灰度发布",
		"jobTypeIstioRelease":     "Istio 发布",
		"jobTypeIstioRollback":    "Istio 回滚",
		"jobTypeIstioStrategy":    "更新 Istio 灰度策略",
		"jobTypeJira":             "JIRA 问题状态变更",
		"jobTypeApollo":           "Apollo 配置变更",
		"jobTypeLark":             "飞书工作项状态变更",
		"jobTypeWorkflowTrigger":  "触发 Zadig 工作流",
		"jobTypeOfflineService":   "下线服务",
		"jobTypeHelmChartDeploy":  "Helm Chart 部署",
		"jobTypeGrafana":          "Grafana 监测",
		"jobTypeJenkinsJob":       "执行 Jenkins job",
		"jobTypeBlueKingJob":      "执行蓝鲸作业",
		"jobTypeSql":              "SQL 数据变更",
		"jobTypeNotification":     "通知",
		"jobTypeSaeDeploy":        "SAE 应用部署",
		"jobTypePingCode":         "PingCode 工作项状态变更",
		"jobTypeTapd":             "Tapd 状态变更",

		"testStatusSuccess": "成功",
		"testStatusFailed":  "失败",
		"testTotal":         "总数",

		"sonarQualityGateStatus": "质量检查",
		"sonarNcloc":             "行数",
		"sonarBugs":              "Bugs",
		"sonarVulnerabilities":   "代码漏洞",
		"sonarCodeSmells":        "容易出错",
		"sonarCoverage":          "覆盖率",

		"notificationTextWorkflow":           "工作流",
		"notificationTextWaitingForApproval": "等待审批",
		"notificationTextExecutor":           "执行用户",
		"notificationTextProjectName":        "项目名称",
		"notificationTextPendingStage":       "待执行阶段",
		"notificationTextStartTime":          "开始时间",
		"notificationTextDuration":           "持续时间",
		"notificationTextRemark":             "备注",
		"notificationTextEnvironment":        "环境",
		"notificationTextClickForMore":       "点击查看更多信息",
		"notificationTextStatus":             "状态",
		"notificationTextCommitMessage":      "提交信息",
		"notificationTextRepositoryInfo":     "代码信息",
		"notificationTextImageInfo":          "镜像信息",
		"notificationTextTestResult":         "测试结果",
		"notificationTextSonarMetrics":       "扫描结果",
	}

	enTextMap = map[string]string{
		"taskTypeWorkflow": "workflow",
		"taskTypeScanning": "scanning",
		"taskTypeTesting":  "testing",

		"taskStatusSuccess":           "Passed",
		"taskStatusFailed":            "Failed",
		"taskStatusCancelled":         "Cancelled",
		"taskStatusTimeout":           "Timeout",
		"taskStatusRejected":          "Rejected",
		"taskStatusExecutionStarted":  "Created",
		"taskStatusManualApproval":    "Waiting for confirmation",
		"taskStatusPause":             "Pause",
		"taskStatusWaitingManualExec": "Waiting for Manual Execution",
		"jobStatusUnstarted":          "Unstarted",

		"jobTypeBuild":            "Build",
		"jobTypeDeploy":           "Deploy",
		"jobTypeVmDeploy":         "Deploy to Host",
		"jobTypeFreestyle":        "Common",
		"jobTypeNacos":            "Nacos Configuration Changes",
		"jobTypePlugin":           "Plugin",
		"jobTypeTest":             "Test",
		"jobTypeScan":             "Scan",
		"jobTypeApproval":         "Approval",
		"jobTypeDistribute":       "Image Distribute",
		"jobTypeCustomDeploy":     "Kubernetes Deploy",
		"jobTypeCanaryDeploy":     "Canary Deploy",
		"jobTypeCanaryRelease":    "Canary Release",
		"jobTypeMseGrayRelease":   "MSE Gray Deploy",
		"jobTypeMseGrayOffline":   "Offline MSE Gray Service",
		"jobTypeBlueGreenDeploy":  "Blue-Green Deploy",
		"jobTypeBlueGreenRelease": "Blue-Green Release",
		"jobTypeK8sResourcePatch": "Kubernetes Resource Patch",
		"jobTypeK8sGrayRollback":  "Gray Rollback",
		"jobTypeGrayDeploy":       "Gray Release",
		"jobTypeIstioRelease":     "Istio Release",
		"jobTypeIstioRollback":    "Istio Rollback",
		"jobTypeIstioStrategy":    "Istio Strategy",
		"jobTypeJira":             "JIRA Issue Status Change",
		"jobTypeApollo":           "Apollo Configs",
		"jobTypeLark":             "Status change of Lark work item",
		"jobTypeWorkflowTrigger":  "Trigger other workflows",
		"jobTypeOfflineService":   "Service Offline",
		"jobTypeHelmChartDeploy":  "Helm Chart Deploy",
		"jobTypeGrafana":          "Grafana Monitor",
		"jobTypeJenkinsJob":       "Execute Jenkins job",
		"jobTypeBlueKingJob":      "Execute BlueKing job",
		"jobTypeSql":              "SQL Changes",
		"jobTypeNotification":     "Notification",
		"jobTypeSaeDeploy":        "SAE Deploy",
		"jobTypePingCode":         "PingCode Work Item Status Change",
		"jobTypeTapd":             "Tapd Status Change",
		"testStatusSuccess":       "Success",
		"testStatusFailed":        "Failed",
		"testTotal":               "Total",

		"sonarQualityGateStatus": "Quality Gate Status",
		"sonarNcloc":             "Ncloc",
		"sonarBugs":              "Bugs",
		"sonarVulnerabilities":   "Vulnerabilities",
		"sonarCodeSmells":        "Code Smells",
		"sonarCoverage":          "Coverage",

		"notificationTextWorkflow":           "Workflow",
		"notificationTextWaitingForApproval": "waiting for approval",
		"notificationTextExecutor":           "Executor",
		"notificationTextProjectName":        "Project Name",
		"notificationTextPendingStage":       "Pending Stage",
		"notificationTextStartTime":          "Start Time",
		"notificationTextDuration":           "Duration",
		"notificationTextRemark":             "Remark",
		"notificationTextEnvironment":        "Environment",
		"notificationTextClickForMore":       "Click for More Information",
		"notificationTextStatus":             "Status",
		"notificationTextCommitMessage":      "Commit Message",
		"notificationTextRepositoryInfo":     "Repository Information",
		"notificationTextImageInfo":          "Image Information",
		"notificationTextTestResult":         "Test Result",
		"notificationTextSonarMetrics":       "Scanning Result",
	}
)

//go:embed notification.html
var notificationHTML []byte

//go:embed notification_en.html
var notificationENHTML []byte

func (w *Service) SendWorkflowTaskApproveNotifications(workflowName string, taskID int64, task *models.WorkflowTask) error {
	resp, err := w.workflowV4Coll.Find(workflowName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to find workflowv4, err: %s", err)
		log.Error(errMsg)
		return errors.New(errMsg)
	}

	if task == nil {
		task, err = w.workflowTaskV4Coll.Find(workflowName, taskID)
		if err != nil {
			errMsg := fmt.Sprintf("failed to find workflowv4 task, err: %s", err)
			log.Error(errMsg)
			return errors.New(errMsg)
		}
	}

	for _, sourceNotify := range resp.NotifyCtls {
		notify, err := models.CloneNotifyCtl(sourceNotify)
		if err != nil {
			return err
		}
		if notify == nil {
			continue
		}
		statusSets := sets.NewString(notify.NotifyTypes...)
		if !isTaskWaitingApproveNotifyType(statusSets) {
			continue
		}
		if !notify.Enabled {
			continue
		}

		err = notify.GenerateNewNotifyConfigWithOldData()
		if err != nil {
			return err
		}

		title, content, larkCard, webhookNotify, err := w.getApproveNotificationContent(notify, task)
		if err != nil {
			errMsg := fmt.Sprintf("failed to get notification content, err: %s", err)
			log.Error(errMsg)
			return errors.New(errMsg)
		}

		if err := resolveWorkflowNotifyDynamicRecipients(task, notify); err != nil {
			log.Errorf("failed to resolve workflow notification dynamic recipients, err: %s", err)
			continue
		}

		if notify.WebHookType == setting.NotifyWebHookTypeMail {
			if task.TaskCreatorID != "" {
				for _, user := range notify.MailUsers {
					if user.Type == setting.UserTypeTaskCreator {
						userInfo, err := userclient.New().GetUserByID(task.TaskCreatorID)
						if err != nil {
							log.Errorf("failed to find user %s, error: %s", task.TaskCreatorID, err)
							break
						}
						notify.MailUsers = append(notify.MailUsers, &models.User{
							Type:     setting.UserTypeUser,
							UserID:   userInfo.Uid,
							UserName: userInfo.Name,
						})
						break
					}
				}
			}
		}

		if notify.WebHookType == setting.NotifyWebHookTypeFeishuPerson {
			for _, target := range notify.LarkPersonNotificationConfig.TargetUsers {
				if target.IsExecutor {
					client, err := larkservice.GetLarkClientByIMAppID(notify.LarkPersonNotificationConfig.AppID)
					if err != nil {
						return fmt.Errorf("failed to get notify target info: create feishu client error: %s", err)
					}
					resolvedTarget, err := w.resolveWorkflowTaskExecutorLarkTarget(client, task)
					if err != nil {
						log.Errorf("failed to resolve workflow executor lark target: %v", err)
						return err
					}
					target.ID = resolvedTarget.ID
					target.Name = resolvedTarget.Name
					target.Avatar = resolvedTarget.Avatar
					target.IDType = resolvedTarget.IDType
				}
			}
		}

		if err := w.sendNotification(title, content, notify, larkCard, webhookNotify, task.Status); err != nil {
			log.Errorf("failed to send notification, err: %s", err)
		}
	}
	return nil
}

// TODO: manual error handling is not supported in the SendWorkflowTaskNotifications function, mainly because the error handling is done in the lifetime of a job, where the
// controller cannot access the task's full information. We need to implement a method where the job controller can send notification.
func (w *Service) SendWorkflowTaskNotifications(task *models.WorkflowTask) error {
	if len(task.OriginWorkflowArgs.NotifyCtls) == 0 {
		return nil
	}
	if task.TaskID <= 0 {
		return nil
	}
	statusChanged := false
	preTask, err := w.workflowTaskV4Coll.Find(task.WorkflowName, task.TaskID-1)
	if err != nil {
		errMsg := fmt.Sprintf("failed to find previous workflowv4, err: %s", err)
		log.Error(errMsg)
		statusChanged = true
	}
	if preTask != nil && task.Status != preTask.Status && task.Status != config.StatusRunning {
		statusChanged = true
	}
	if task.Status == config.StatusCreated {
		statusChanged = false
	}
	for _, sourceNotify := range task.OriginWorkflowArgs.NotifyCtls {
		notify, err := models.CloneNotifyCtl(sourceNotify)
		if err != nil {
			return err
		}
		if notify == nil {
			continue
		}
		if !notify.Enabled {
			continue
		}

		err = notify.GenerateNewNotifyConfigWithOldData()
		if err != nil {
			return err
		}

		statusSets := sets.NewString(notify.NotifyTypes...)
		if statusSets.Has(string(task.Status)) || (statusChanged && statusSets.Has(string(config.StatusChanged))) {
			if shouldSkipFeishuPersonPauseNotification(task, notify) {
				continue
			}

			title, content, larkCard, webhookNotify, err := w.getNotificationContent(notify, task)
			if err != nil {
				errMsg := fmt.Sprintf("failed to get notification content, err: %s", err)
				log.Error(errMsg)
				return errors.New(errMsg)
			}

			if err := resolveWorkflowNotifyDynamicRecipients(task, notify); err != nil {
				log.Errorf("failed to resolve workflow notification dynamic recipients, err: %s", err)
				continue
			}

			if notify.WebHookType == setting.NotifyWebHookTypeMail {
				if task.TaskCreatorID != "" {
					for _, user := range notify.MailNotificationConfig.TargetUsers {
						if user.Type == setting.UserTypeTaskCreator {
							userInfo, err := userclient.New().GetUserByID(task.TaskCreatorID)
							if err != nil {
								log.Errorf("failed to find user %s, error: %s", task.TaskCreatorID, err)
								break
							}
							user.Type = setting.UserTypeUser
							user.UserID = userInfo.Uid
							user.UserName = userInfo.Name
							break
						}
					}
				}
			}

			if notify.WebHookType == setting.NotifyWebHookTypeFeishuPerson {

				for _, target := range notify.LarkPersonNotificationConfig.TargetUsers {
					if target.IsExecutor {
						client, err := larkservice.GetLarkClientByIMAppID(notify.LarkPersonNotificationConfig.AppID)
						if err != nil {
							return fmt.Errorf("failed to get notify target info: create feishu client error: %s", err)
						}
						resolvedTarget, err := w.resolveWorkflowTaskExecutorLarkTarget(client, task)
						if err != nil {
							log.Errorf("failed to resolve workflow executor lark target: %v", err)
							return err
						}
						target.ID = resolvedTarget.ID
						target.Name = resolvedTarget.Name
						target.Avatar = resolvedTarget.Avatar
						target.IDType = resolvedTarget.IDType
					}
				}
			}

			if err := w.sendNotification(title, content, notify, larkCard, webhookNotify, task.Status); err != nil {
				log.Errorf("failed to send notification, err: %s", err)
			}
		}
	}
	return nil
}

func shouldSkipFeishuPersonPauseNotification(task *models.WorkflowTask, notify *models.NotifyCtl) bool {
	if task == nil || notify == nil || task.Status != config.StatusPause {
		return false
	}
	if notify.WebHookType != setting.NotifyWebHookTypeFeishuPerson {
		return false
	}

	for _, stage := range task.Stages {
		if stage == nil || stage.Status != config.StatusPause {
			continue
		}
		if stage.ManualExec == nil || !stage.ManualExec.Enabled || stage.ManualExec.Excuted {
			continue
		}
		return true
	}

	return false
}

func resolveWorkflowNotifyDynamicRecipients(task *models.WorkflowTask, notify *models.NotifyCtl) error {
	if task == nil || notify == nil {
		return nil
	}

	workflowArgs := task.WorkflowArgs
	if workflowArgs == nil {
		workflowArgs = task.OriginWorkflowArgs
	}
	if workflowArgs == nil {
		return nil
	}

	keyMap := commonutil.KeyValsToMap(commonutil.BuildWorkflowPayloadVariableKVs(workflowArgs))
	return dynamicrecipient.ResolveNotificationConfigs(keyMap, dynamicrecipient.NotificationConfigs{
		LarkHook:   notify.LarkHookNotificationConfig,
		LarkGroup:  notify.LarkGroupNotificationConfig,
		LarkPerson: notify.LarkPersonNotificationConfig,
		DingDing:   notify.DingDingNotificationConfig,
		MSTeams:    notify.MSTeamsNotificationConfig,
		Mail:       notify.MailNotificationConfig,
	})
}

func (w *Service) resolveWorkflowTaskExecutorLarkTarget(client *lark.Client, task *models.WorkflowTask) (*lark.UserInfo, error) {
	if task == nil || task.TaskCreatorID == "" {
		return nil, fmt.Errorf("executor id is empty, cannot send message")
	}

	resolvedTarget, _, err := w.resolveManualExecStageLarkTargetFromUser(client, task.TaskCreatorID, task.TaskCreator)
	if err != nil {
		return nil, err
	}
	if resolvedTarget == nil {
		return nil, fmt.Errorf("executor lark target cannot be empty")
	}

	return resolvedTarget, nil
}

func (w *Service) SendManualExecStageNotifications(workflowCtx *models.WorkflowTaskCtx, stage *models.StageTask) error {
	if workflowCtx == nil || stage == nil || stage.ManualExec == nil {
		return nil
	}

	taskForNotification := &models.WorkflowTask{
		WorkflowName:        workflowCtx.WorkflowName,
		WorkflowDisplayName: workflowCtx.WorkflowDisplayName,
		ProjectName:         workflowCtx.ProjectName,
		ProjectDisplayName:  workflowCtx.ProjectDisplayName,
		TaskID:              workflowCtx.TaskID,
		Remark:              workflowCtx.Remark,
		TaskCreator:         workflowCtx.WorkflowTaskCreatorUsername,
		TaskCreatorID:       workflowCtx.WorkflowTaskCreatorUserID,
		TaskCreatorPhone:    workflowCtx.WorkflowTaskCreatorMobile,
		TaskCreatorEmail:    workflowCtx.WorkflowTaskCreatorEmail,
		StartTime:           workflowCtx.StartTime.Unix(),
		Status:              config.StatusPause,
		Stages:              []*models.StageTask{stage},
		Type:                config.WorkflowTaskTypeWorkflow,
	}
	stageForNotification := stage
	if taskInColl, findErr := w.workflowTaskV4Coll.Find(workflowCtx.WorkflowName, workflowCtx.TaskID); findErr == nil && taskInColl != nil {
		taskForNotification = taskInColl
		if matchedStage := getStageTaskByName(taskInColl.Stages, stage.Name); matchedStage != nil {
			stageForNotification = matchedStage
		}
	}
	taskForNotification.Status = config.StatusPause

	notifyCtls := getManualExecStageNotifyCtls(taskForNotification)
	if len(notifyCtls) == 0 {
		return nil
	}

	return w.sendManualStageUserNotifications(taskForNotification, stageForNotification, notifyCtls, config.StatusPause, "taskStatusWaitingManualExec")
}

func (w *Service) sendManualStageUserNotifications(taskForNotification *models.WorkflowTask, stageForNotification *models.StageTask, notifyCtls []*models.NotifyCtl, status config.Status, statusTextKeyOverride string) error {
	respErr := new(multierror.Error)
	for _, sourceNotify := range notifyCtls {
		notify, err := models.CloneNotifyCtl(sourceNotify)
		if err != nil {
			respErr = multierror.Append(respErr, err)
			continue
		}
		if notify == nil {
			continue
		}
		if err := notify.GenerateNewNotifyConfigWithOldData(); err != nil {
			respErr = multierror.Append(respErr, err)
			continue
		}

		switch notify.WebHookType {
		case setting.NotifyWebHookTypeFeishuPerson:
			if err := resolveWorkflowNotifyDynamicRecipients(taskForNotification, notify); err != nil {
				respErr = multierror.Append(respErr, err)
				continue
			}
			resolvedTargets, err := w.resolveManualExecStageLarkTargets(taskForNotification, stageForNotification, notify)
			if err != nil {
				respErr = multierror.Append(respErr, err)
				continue
			}
			if len(resolvedTargets) == 0 {
				continue
			}

			notifyToSend := *notify
			notifyToSend.LarkPersonNotificationConfig = &models.LarkPersonNotificationConfig{
				AppID:       notify.LarkPersonNotificationConfig.AppID,
				TargetUsers: resolvedTargets,
			}

			title, content, card, webhookNotify, err := w.getNotificationContentWithOptions(&notifyToSend, taskForNotification, &workflowNotificationOptions{
				StatusTextKeyOverride: statusTextKeyOverride,
				PendingStageName:      stageForNotification.Name,
			})
			if err != nil {
				respErr = multierror.Append(respErr, err)
				continue
			}
			if err := w.sendNotification(title, content, &notifyToSend, card, webhookNotify, status); err != nil {
				respErr = multierror.Append(respErr, err)
			}

		case setting.NotifyWebHookTypeMail:
			if err := resolveWorkflowNotifyDynamicRecipients(taskForNotification, notify); err != nil {
				respErr = multierror.Append(respErr, err)
				continue
			}
			resolvedUsers, err := w.resolveManualExecStageMailUsers(taskForNotification, stageForNotification, notify)
			if err != nil {
				respErr = multierror.Append(respErr, err)
				continue
			}
			if len(resolvedUsers) == 0 {
				continue
			}

			notifyToSend := *notify
			notifyToSend.MailNotificationConfig = &models.MailNotificationConfig{TargetUsers: resolvedUsers}
			title, content, card, webhookNotify, err := w.getNotificationContentWithOptions(&notifyToSend, taskForNotification, &workflowNotificationOptions{
				StatusTextKeyOverride: statusTextKeyOverride,
				PendingStageName:      stageForNotification.Name,
			})
			if err != nil {
				respErr = multierror.Append(respErr, err)
				continue
			}
			if err := w.sendNotification(title, content, &notifyToSend, card, webhookNotify, status); err != nil {
				respErr = multierror.Append(respErr, err)
			}
		default:
			title, content, card, webhookNotify, err := w.getNotificationContentWithOptions(notify, taskForNotification, &workflowNotificationOptions{
				StatusTextKeyOverride: statusTextKeyOverride,
				PendingStageName:      stageForNotification.Name,
			})
			if err != nil {
				respErr = multierror.Append(respErr, err)
				continue
			}
			if err := w.sendNotification(title, content, notify, card, webhookNotify, status); err != nil {
				respErr = multierror.Append(respErr, err)
			}
		}
	}

	return respErr.ErrorOrNil()
}

func getManualExecStageNotifyCtls(task *models.WorkflowTask) []*models.NotifyCtl {
	if task == nil {
		return nil
	}

	var notifyCtls []*models.NotifyCtl
	switch {
	case task.OriginWorkflowArgs != nil:
		notifyCtls = task.OriginWorkflowArgs.NotifyCtls
	case task.WorkflowArgs != nil:
		notifyCtls = task.WorkflowArgs.NotifyCtls
	}

	ret := make([]*models.NotifyCtl, 0, len(notifyCtls))
	for _, notify := range notifyCtls {
		if notify == nil || !notify.Enabled {
			continue
		}
		if !sets.NewString(notify.NotifyTypes...).Has(string(config.StatusPause)) {
			continue
		}
		if notify.WebHookType != setting.NotifyWebHookTypeFeishuPerson && notify.WebHookType != setting.NotifyWebHookTypeMail {
			continue
		}
		ret = append(ret, notify)
	}

	return ret
}

func (w *Service) resolveManualExecStageLarkTargets(task *models.WorkflowTask, stage *models.StageTask, notify *models.NotifyCtl) ([]*lark.UserInfo, error) {
	if notify == nil || notify.LarkPersonNotificationConfig == nil || notify.LarkPersonNotificationConfig.AppID == "" {
		return nil, nil
	}

	client, err := larkservice.GetLarkClientByIMAppID(notify.LarkPersonNotificationConfig.AppID)
	if err != nil {
		return nil, fmt.Errorf("create feishu client error: %w", err)
	}

	respErr := new(multierror.Error)
	targets := make([]*lark.UserInfo, 0, len(notify.LarkPersonNotificationConfig.TargetUsers))
	targetSet := sets.NewString()
	stageUsers, stageUserInfoMap := resolveManualExecStageUsers(stage, task.TaskCreatorID)

	for _, target := range notify.LarkPersonNotificationConfig.TargetUsers {
		if target == nil {
			continue
		}

		if target.IsExecutor {
			if task.TaskCreatorID == "" {
				respErr = multierror.Append(respErr, fmt.Errorf("executor id is empty, cannot send message"))
				continue
			}
			userInfo, err := userclient.New().GetUserByID(task.TaskCreatorID)
			if err != nil {
				respErr = multierror.Append(respErr, fmt.Errorf("failed to find user %s, error: %w", task.TaskCreatorID, err))
				continue
			}
			resolvedTarget, _, resolveErr := w.resolveManualExecStageLarkTargetFromUser(client, userInfo.Uid, userInfo.Name)
			if resolveErr != nil {
				respErr = multierror.Append(respErr, resolveErr)
				continue
			}
			targets = appendManualExecStageLarkTarget(targets, targetSet, resolvedTarget)
			continue
		}

		if target.IsStageExecutor {
			for _, stageUser := range stageUsers {
				if stageUser == nil || stageUser.UserID == "" {
					continue
				}
				displayName := stageUser.UserName
				if info, ok := stageUserInfoMap[stageUser.UserID]; ok && info != nil && info.Name != "" {
					displayName = info.Name
				}
				resolvedTarget, _, resolveErr := w.resolveManualExecStageLarkTargetFromUser(client, stageUser.UserID, displayName)
				if resolveErr != nil {
					respErr = multierror.Append(respErr, fmt.Errorf("stage executor %s: %w", stageUser.UserID, resolveErr))
					continue
				}
				targets = appendManualExecStageLarkTarget(targets, targetSet, resolvedTarget)
			}
			continue
		}

		resolvedTarget := &lark.UserInfo{
			ID:              target.ID,
			IDType:          target.IDType,
			Name:            target.Name,
			Avatar:          target.Avatar,
			IsExecutor:      target.IsExecutor,
			IsStageExecutor: target.IsStageExecutor,
		}
		if resolvedTarget.ID == "" {
			continue
		}
		if resolvedTarget.IDType == "" {
			resolvedTarget.IDType = setting.LarkUserID
		}
		targets = appendManualExecStageLarkTarget(targets, targetSet, resolvedTarget)
	}

	return targets, respErr.ErrorOrNil()
}

func (w *Service) resolveManualExecStageMailUsers(task *models.WorkflowTask, stage *models.StageTask, notify *models.NotifyCtl) ([]*models.User, error) {
	if notify == nil || notify.MailNotificationConfig == nil {
		return nil, nil
	}

	usersToExpand := make([]*models.User, 0, len(notify.MailNotificationConfig.TargetUsers))
	for _, user := range notify.MailNotificationConfig.TargetUsers {
		if user == nil {
			continue
		}
		if user.Type == setting.UserTypeStageExecutor {
			if stage == nil || stage.ManualExec == nil {
				continue
			}
			usersToExpand = append(usersToExpand, stage.ManualExec.ManualExecUsers...)
			continue
		}
		usersToExpand = append(usersToExpand, user)
	}

	if task.TaskCreatorID != "" {
		users, _ := commonutil.GeneFlatUsersWithCaller(usersToExpand, task.TaskCreatorID)
		return users, nil
	}

	users, _ := commonutil.GeneFlatUsers(usersToExpand)
	return users, nil
}

func resolveManualExecStageUsers(stage *models.StageTask, taskCreatorID string) ([]*models.User, map[string]*types.UserInfo) {
	if stage == nil || stage.ManualExec == nil || len(stage.ManualExec.ManualExecUsers) == 0 {
		return nil, map[string]*types.UserInfo{}
	}

	if taskCreatorID != "" {
		return commonutil.GeneFlatUsersWithCaller(stage.ManualExec.ManualExecUsers, taskCreatorID)
	}

	return commonutil.GeneFlatUsers(stage.ManualExec.ManualExecUsers)
}

// ---------------------------------------------------------------------------
// Generic task-level notification
// ---------------------------------------------------------------------------

// TaskNotifyInput holds the context needed to send a task-level notification.
type TaskNotifyInput struct {
	// Task is the workflow task. If nil, the function will fetch it from the database
	// using WorkflowName and TaskID.
	Task *models.WorkflowTask
	// Job is the current job that triggered the notification.
	Job *models.JobTask
	// WorkflowName is the name of the workflow, used to fetch the task if Task is nil.
	WorkflowName string
	// TaskID is the ID of the workflow task, used to fetch the task if Task is nil.
	TaskID int64
	// NotifyCtls is the task-level notification configuration.
	NotifyCtls []*models.NotifyCtl
	// Status is the status that triggered the notification.
	Status config.Status
	// StatusTextKeyOverride allows the caller to customise the status text shown in the
	// notification. If empty, the normal status text is used.
	StatusTextKeyOverride string
}

// HasTaskNotifyCtls reports whether there is at least one enabled task-level
// notification config that applies to the given status.
func HasTaskNotifyCtls(notifyCtls []*models.NotifyCtl, status config.Status) bool {
	if !isTaskNotifyStatus(status) {
		return false
	}

	for _, notify := range notifyCtls {
		if notify == nil || !notify.Enabled {
			continue
		}

		notifyToCheck := *notify
		if err := notifyToCheck.GenerateNewNotifyConfigWithOldData(); err != nil {
			continue
		}

		statusSets := sets.NewString(notifyToCheck.NotifyTypes...)
		if status == config.StatusWaitingApprove && isTaskWaitingApproveNotifyType(statusSets) {
			return true
		}
		if statusSets.Has(string(status)) {
			return true
		}
	}

	return false
}

func isTaskWaitingApproveNotifyType(statusSets sets.String) bool {
	return statusSets.Has(string(config.StatusWaitingApprove))
}

func isTaskNotifyStatus(status config.Status) bool {
	switch status {
	case config.StatusPrepare,
		config.StatusPassed,
		config.StatusFailed,
		config.StatusTimeout,
		config.StatusCancelled,
		config.StatusReject,
		config.StatusWaitingApprove:
		return true
	default:
		return false
	}
}

// SendTaskNotifications sends task-level notifications when a task enters a configured
// status. It supports all existing notification channels (feishu group, feishu
// person, feishu webhook, dingding, wechat work, msteams, mail, webhook) and reuses
// the existing notification rendering and sending pipeline.
func (w *Service) SendTaskNotifications(input *TaskNotifyInput) error {
	if input == nil || len(input.NotifyCtls) == 0 {
		return nil
	}
	if !isTaskNotifyStatus(input.Status) {
		return nil
	}

	task := input.Task
	if task == nil {
		if input.WorkflowName == "" || input.TaskID <= 0 {
			return nil
		}
		var err error
		task, err = w.workflowTaskV4Coll.Find(input.WorkflowName, input.TaskID)
		if err != nil {
			return fmt.Errorf("failed to find workflow task %s/%d: %w", input.WorkflowName, input.TaskID, err)
		}
	}

	// Override the task status for notification rendering so the template engine
	// picks up the correct status text and colour.
	stageForNotification := findTaskNotifyStage(task.Stages, input.Job)
	taskCopy := *task
	taskCopy.Status = input.Status
	if input.Job != nil {
		taskCopy.Stages = []*models.StageTask{
			{
				Name:      input.Job.DisplayName,
				Status:    input.Job.Status,
				StartTime: input.Job.StartTime,
				EndTime:   input.Job.EndTime,
				Error:     input.Job.Error,
				Jobs:      []*models.JobTask{input.Job},
			},
		}
	}
	task = &taskCopy

	statusTextKey := input.StatusTextKeyOverride

	respErr := new(multierror.Error)
	for _, sourceNotify := range input.NotifyCtls {
		notify, err := models.CloneNotifyCtl(sourceNotify)
		if err != nil {
			respErr = multierror.Append(respErr, fmt.Errorf("failed to clone notify config: %w", err))
			continue
		}
		if notify == nil || !notify.Enabled {
			continue
		}

		if err := notify.GenerateNewNotifyConfigWithOldData(); err != nil {
			respErr = multierror.Append(respErr, fmt.Errorf("failed to generate notify config: %w", err))
			continue
		}

		// Only send notifications for configs that include the trigger status.
		statusSets := sets.NewString(notify.NotifyTypes...)
		if input.Status == config.StatusWaitingApprove {
			if !isTaskWaitingApproveNotifyType(statusSets) {
				continue
			}
		} else if !statusSets.Has(string(input.Status)) {
			continue
		}

		notifyToSend := *notify

		if err := resolveWorkflowNotifyDynamicRecipients(task, &notifyToSend); err != nil {
			respErr = multierror.Append(respErr, err)
			continue
		}

		// Resolve feishu_person targets (e.g. executor placeholders).
		if notifyToSend.WebHookType == setting.NotifyWebHookTypeFeishuPerson && notifyToSend.LarkPersonNotificationConfig != nil {
			resolvedTargets, err := w.resolveManualExecStageLarkTargets(task, stageForNotification, &notifyToSend)
			if err != nil {
				respErr = multierror.Append(respErr, err)
				continue
			}
			if len(resolvedTargets) == 0 {
				continue
			}
			appID := notifyToSend.LarkPersonNotificationConfig.AppID
			notifyToSend.LarkPersonNotificationConfig = &models.LarkPersonNotificationConfig{
				AppID:       appID,
				TargetUsers: resolvedTargets,
			}
		}

		// Resolve mail targets (e.g. executor placeholders and user groups).
		if notifyToSend.WebHookType == setting.NotifyWebHookTypeMail && notifyToSend.MailNotificationConfig != nil {
			resolvedUsers, err := w.resolveManualExecStageMailUsers(task, stageForNotification, &notifyToSend)
			if err != nil {
				respErr = multierror.Append(respErr, err)
				continue
			}
			if len(resolvedUsers) == 0 {
				continue
			}
			notifyToSend.MailNotificationConfig = &models.MailNotificationConfig{TargetUsers: resolvedUsers}
		}

		title, content, larkCard, webhookNotify, err := w.getNotificationContentWithOptions(&notifyToSend, task, &workflowNotificationOptions{
			StatusTextKeyOverride: statusTextKey,
		})
		if err != nil {
			respErr = multierror.Append(respErr, fmt.Errorf("failed to get notification content: %w", err))
			continue
		}

		if err := w.sendNotification(title, content, &notifyToSend, larkCard, webhookNotify, task.Status); err != nil {
			respErr = multierror.Append(respErr, fmt.Errorf("failed to send %s notification: %w", notifyToSend.WebHookType, err))
		}
	}

	return respErr.ErrorOrNil()
}

func findTaskNotifyStage(stages []*models.StageTask, job *models.JobTask) *models.StageTask {
	if job == nil {
		return nil
	}
	for _, stage := range stages {
		if stage == nil {
			continue
		}
		for _, stageJob := range stage.Jobs {
			if stageJob == nil {
				continue
			}
			if job.Name != "" && stageJob.Name == job.Name {
				return stage
			}
			if job.Key != "" && stageJob.Key == job.Key {
				return stage
			}
		}
	}
	return nil
}

func (w *Service) resolveManualExecStageLarkTargetFromUser(client *lark.Client, userID, userName string) (*lark.UserInfo, string, error) {
	userInfo, err := userclient.New().GetUserByID(userID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find user %s, error: %w", userID, err)
	}
	if len(userInfo.Phone) == 0 {
		return nil, "", fmt.Errorf("phone not configured")
	}

	larkUser, err := client.GetUserIDByEmailOrMobile(lark.QueryTypeMobile, userInfo.Phone, setting.LarkUserID)
	if err != nil {
		return nil, "", fmt.Errorf("find lark user with phone %s error: %w", userInfo.Phone, err)
	}

	userDetailedInfo, err := client.GetUserInfoByID(util.GetStringFromPointer(larkUser.UserId), setting.LarkUserID)
	if err != nil {
		return nil, "", fmt.Errorf("find lark user info for userID %s error: %w", util.GetStringFromPointer(larkUser.UserId), err)
	}

	displayName := userDetailedInfo.Name
	if displayName == "" {
		displayName = userName
	}

	return &lark.UserInfo{
		ID:     util.GetStringFromPointer(larkUser.UserId),
		IDType: setting.LarkUserID,
		Name:   userDetailedInfo.Name,
		Avatar: userDetailedInfo.Avatar,
	}, displayName, nil
}

func appendManualExecStageLarkTarget(targets []*lark.UserInfo, targetSet sets.String, target *lark.UserInfo) []*lark.UserInfo {
	if target == nil || target.ID == "" {
		return targets
	}
	if target.IDType == "" {
		target.IDType = setting.LarkUserID
	}

	targetKey := fmt.Sprintf("%s:%s", target.IDType, target.ID)
	if !targetSet.Has(targetKey) {
		targetSet.Insert(targetKey)
		targets = append(targets, target)
	}

	return targets
}

func getStageTaskByName(stages []*models.StageTask, stageName string) *models.StageTask {
	for _, stage := range stages {
		if stage != nil && stage.Name == stageName {
			return stage
		}
	}
	return nil
}

func buildWorkflowNotifyReleasePlan(releasePlan *models.ReleasePlanRef) *webhooknotify.WorkflowNotifyReleasePlan {
	if releasePlan == nil {
		return nil
	}

	return &webhooknotify.WorkflowNotifyReleasePlan{
		ID:    releasePlan.ID,
		Name:  releasePlan.Name,
		Index: releasePlan.Index,
	}
}

func (w *Service) getApproveNotificationContent(notify *models.NotifyCtl, task *models.WorkflowTask) (string, string, *LarkCard, *webhooknotify.WorkflowNotify, error) {
	project, err := templaterepo.NewProductColl().Find(task.ProjectName)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("failed to find project %s, error: %v", task.ProjectName, err)
	}

	systemSetting, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		log.Errorf("getSystemLanguage err:%s", err)
		return "", "", nil, nil, fmt.Errorf("getSystemLanguage err:%s", err)
	}
	language := systemSetting.Language

	workflowNotification := &workflowTaskNotification{
		Task:               task,
		ProjectDisplayName: project.ProjectName,
		EncodedDisplayName: url.PathEscape(task.WorkflowDisplayName),
		BaseURI:            configbase.SystemAddress(),
		WebHookType:        notify.WebHookType,
		TotalTime:          time.Now().Unix() - task.StartTime,
	}

	webhookNotify := &webhooknotify.WorkflowNotify{
		TaskID:              task.TaskID,
		WorkflowName:        task.WorkflowName,
		WorkflowDisplayName: task.WorkflowDisplayName,
		ProjectName:         task.ProjectName,
		ProjectDisplayName:  project.ProjectName,
		ReleasePlan:         buildWorkflowNotifyReleasePlan(task.ReleasePlan),
		Status:              task.Status,
		Remark:              task.Remark,
		Error:               task.Error,
		CreateTime:          task.CreateTime,
		StartTime:           task.StartTime,
		EndTime:             task.EndTime,
		TaskCreator:         task.TaskCreator,
		TaskCreatorID:       task.TaskCreatorID,
		TaskCreatorEmail:    task.TaskCreatorEmail,
	}

	tplTitle := "{{if and (ne .WebHookType \"feishu\") (ne .WebHookType \"feishu_app\") (ne .WebHookType \"feishu_person\")}}### {{end}}{{if eq .WebHookType \"dingding\"}}<font color=#3270e3>**{{end}}{{getIcon .Task.Status }}{{getText \"notificationTextWorkflow\"}} {{.Task.WorkflowDisplayName}} #{{.Task.TaskID}} {{getText \"notificationTextWaitingForApproval\"}}{{if eq .WebHookType \"dingding\"}}**</font>{{end}} \n"
	mailTplTitle := "{{getIcon .Task.Status }}{{getText \"notificationTextWorkflow\"}} {{.Task.WorkflowDisplayName}} #{{.Task.TaskID}} {{getText \"notificationTextWaitingForApproval\"}}\n"

	tplBaseInfo := []string{"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextExecutor\"}}**：{{.Task.TaskCreator}}  \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextProjectName\"}}**：{{.ProjectDisplayName}}  \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextStartTime\"}}**：{{ getStartTime .Task.StartTime}}  \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextDuration\"}}**：{{ getDuration .TotalTime}}  \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextRemark\"}}**：{{.Task.Remark}}  \n",
	}
	mailTplBaseInfo := []string{"{{getText \"notificationTextExecutor\"}}：{{.Task.TaskCreator}} \n",
		"{{getText \"notificationTextProjectName\"}}：{{.ProjectDisplayName}} \n",
		"{{getText \"notificationTextStartTime\"}}：{{ getStartTime .Task.StartTime}} \n",
		"{{getText \"notificationTextDuration\"}}：{{ getDuration .TotalTime}} \n",
		"{{getText \"notificationTextRemark\"}}：{{ .Task.Remark}} \n\n",
	}

	jobContents := []string{}
	for _, stage := range task.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == string(config.JobZadigDeploy) || job.JobType == string(config.JobZadigHelmDeploy) {
				jobTplcontent := "{{if and (ne .WebHookType \"feishu\") (ne .WebHookType \"feishu_app\") (ne .WebHookType \"feishu_person\")}}\n\n{{end}}{{if eq .WebHookType \"dingding\"}}---\n\n##### {{end}}**{{jobType .Job.JobType }}**: {{.Job.DisplayName}}  \n"
				mailJobTplcontent := "{{jobType .Job.JobType }}：{{.Job.DisplayName}}  \n"

				switch job.JobType {
				case string(config.JobZadigDeploy):
					jobSpec := &models.JobTaskDeploySpec{}
					models.IToi(job.Spec, jobSpec)
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextEnvironment\"}}**：%s  \n", jobSpec.Env)
					mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextEnvironment\"}}：%s \n", jobSpec.Env)
				case string(config.JobZadigHelmDeploy):
					jobSpec := &models.JobTaskHelmDeploySpec{}
					models.IToi(job.Spec, jobSpec)
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextEnvironment\"}}**：%s  \n", jobSpec.Env)
					mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextEnvironment\"}}：%s \n", jobSpec.Env)
				}

				jobNotifaication := &jobTaskNotification{
					Job:         job,
					WebHookType: notify.WebHookType,
				}

				if notify.WebHookType == setting.NotifyWebHookTypeMail {
					jobContent, err := getJobTaskTplExec(mailJobTplcontent, jobNotifaication, language)
					if err != nil {
						return "", "", nil, nil, err
					}
					jobContents = append(jobContents, jobContent)
				} else {
					jobContent, err := getJobTaskTplExec(jobTplcontent, jobNotifaication, language)
					if err != nil {
						return "", "", nil, nil, err
					}
					jobContents = append(jobContents, jobContent)
				}
			}
		}
	}

	title, err := getWorkflowTaskTplExec(tplTitle, workflowNotification)
	if err != nil {
		return "", "", nil, nil, err
	}

	buttonContent := getText("notificationTextClickForMore", language)
	workflowDetailURL := "{{.BaseURI}}/v1/projects/detail/{{.Task.ProjectName}}/pipelines/custom/{{.Task.WorkflowName}}/{{.Task.TaskID}}?display_name={{.EncodedDisplayName}}"
	moreInformation := fmt.Sprintf("[%s](%s)", buttonContent, workflowDetailURL)
	if notify.WebHookType == setting.NotifyWebHookTypeMail {
		title, err = getWorkflowTaskTplExec(mailTplTitle, workflowNotification)
		if err != nil {
			return "", "", nil, nil, err
		}

		tplcontent := strings.Join(mailTplBaseInfo, "")
		tplcontent += strings.Join(jobContents, "")
		content, err := getWorkflowTaskTplExec(tplcontent, workflowNotification)
		if err != nil {
			return "", "", nil, nil, err
		}
		content = strings.TrimSpace(content)

		t, err := template.New("workflow_notification").Parse(getMailTemplate(language))
		if err != nil {
			err = fmt.Errorf("workflow notification template parse error, error msg:%s", err)
			return "", "", nil, nil, err
		}

		var buf bytes.Buffer
		err = t.Execute(&buf, struct {
			WorkflowName   string
			WorkflowTaskID int64
			Content        string
			Url            string
		}{
			WorkflowName:   task.WorkflowDisplayName,
			WorkflowTaskID: task.TaskID,
			Content:        content,
			Url:            fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s?display_name=%s", configbase.SystemAddress(), task.ProjectName, task.WorkflowName, url.PathEscape(task.WorkflowDisplayName)),
		})
		if err != nil {
			err = fmt.Errorf("workflow notification template execute error, error msg:%s", err)
			return "", "", nil, nil, err
		}

		content = buf.String()
		return title, content, nil, nil, nil
	} else if notify.WebHookType == setting.NotifyWebHookTypeWebook {
		webhookNotify.DetailURL = fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s?display_name=%s", configbase.SystemAddress(), task.ProjectName, task.WorkflowName, url.PathEscape(task.WorkflowDisplayName))
		return "", "", nil, webhookNotify, nil
	} else if notify.WebHookType != setting.NotifyWebHookTypeFeishu && notify.WebHookType != setting.NotifyWebhookTypeFeishuApp && notify.WebHookType != setting.NotifyWebHookTypeFeishuPerson {
		tplcontent := strings.Join(tplBaseInfo, "")
		tplcontent += strings.Join(jobContents, "")
		tplcontent = appendInlineNotifyAtContent(tplcontent, notify)
		tplcontent = fmt.Sprintf("%s%s", title, tplcontent)
		if notify.WebHookType == setting.NotifyWebHookTypeWechatWork {
			tplcontent = fmt.Sprintf("%s%s", tplcontent, moreInformation)
		}
		content, err := getWorkflowTaskTplExec(tplcontent, workflowNotification)
		if err != nil {
			return "", "", nil, nil, err
		}
		return title, content, nil, webhookNotify, nil
	}

	lc := NewLarkCard()
	lc.SetConfig(true)
	lc.SetHeader(feishuHeaderTemplateGreen, title, feiShuTagText)
	for idx, feildContent := range tplBaseInfo {
		feildExecContent, _ := getWorkflowTaskTplExec(feildContent, workflowNotification)
		lc.AddI18NElementsZhcnFeild(feildExecContent, idx == 0)
	}

	for _, feildContent := range jobContents {
		feildExecContent, _ := getWorkflowTaskTplExec(feildContent, workflowNotification)
		lc.AddI18NElementsZhcnFeild(feildExecContent, true)
	}
	workflowDetailURL, _ = getWorkflowTaskTplExec(workflowDetailURL, workflowNotification)
	lc.AddI18NElementsZhcnAction(buttonContent, workflowDetailURL)
	return "", "", lc, nil, nil
}

// @note custom workflow task v4 notification
func (w *Service) getNotificationContent(notify *models.NotifyCtl, task *models.WorkflowTask) (string, string, *LarkCard, *webhooknotify.WorkflowNotify, error) {
	return w.getNotificationContentWithOptions(notify, task, nil)
}

func (w *Service) BuildWorkflowWebhookNotify(task *models.WorkflowTask) (*webhooknotify.WorkflowNotify, error) {
	_, _, _, webhookNotify, err := w.getNotificationContentWithOptions(&models.NotifyCtl{WebHookType: setting.NotifyWebHookTypeWebook}, task, nil)
	if err != nil {
		return nil, err
	}
	if webhookNotify == nil {
		return nil, fmt.Errorf("failed to build workflow webhook payload for workflow %s, taskID: %d", task.WorkflowName, task.TaskID)
	}

	return webhookNotify, nil
}

func (w *Service) SendSystemWorkflowHook(task *models.WorkflowTask, hookSetting *models.WorkflowHookSettings, hookEvent models.WorkflowHookEvent) error {
	if task == nil || !isWorkflowHookEventEnabled(hookSetting, hookEvent) {
		return nil
	}

	webhookNotify, err := w.BuildWorkflowWebhookNotify(task)
	if err != nil {
		return err
	}

	return webhooknotify.NewClient(hookSetting.HookAddress, hookSetting.HookSecret).SendWorkflowWebhook(webhookNotify, workflowHookEventToWebhookEvent(hookEvent))
}

func isWorkflowHookEventEnabled(hookSetting *models.WorkflowHookSettings, hookEvent models.WorkflowHookEvent) bool {
	if hookSetting == nil || !hookSetting.Enable {
		return false
	}

	for _, configuredEvent := range hookSetting.HookEvents {
		if configuredEvent == hookEvent {
			return true
		}
	}

	return false
}

func workflowHookEventToWebhookEvent(hookEvent models.WorkflowHookEvent) webhooknotify.WebHookNotifyEvent {
	switch hookEvent {
	case models.WorkflowHookEventStartExecute:
		return webhooknotify.WebHookNotifyEventWorkflowStartExecute
	case models.WorkflowHookEventCompleteExecute:
		return webhooknotify.WebHookNotifyEventWorkflowCompleteExecute
	default:
		return webhooknotify.WebHookNotifyEventWorkflow
	}
}

type workflowNotificationOptions struct {
	StatusTextKeyOverride string
	PendingStageName      string
}

func (w *Service) getNotificationContentWithOptions(notify *models.NotifyCtl, task *models.WorkflowTask, opts *workflowNotificationOptions) (string, string, *LarkCard, *webhooknotify.WorkflowNotify, error) {
	project, err := templaterepo.NewProductColl().Find(task.ProjectName)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("failed to find project %s, error: %v", task.ProjectName, err)
	}

	systemSetting, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		log.Errorf("getSystemLanguage err:%s", err)
		return "", "", nil, nil, fmt.Errorf("getSystemLanguage err:%s", err)
	}
	language := systemSetting.Language

	workflowNotification := &workflowTaskNotification{
		Task:               task,
		ProjectDisplayName: project.ProjectName,
		EncodedDisplayName: url.PathEscape(task.WorkflowDisplayName),
		BaseURI:            configbase.SystemAddress(),
		WebHookType:        notify.WebHookType,
		TotalTime:          time.Now().Unix() - task.StartTime,
	}
	if task.Status == config.StatusPause && isFeishuNotificationType(notify.WebHookType) {
		workflowNotification.StatusTextKeyOverride = "taskStatusWaitingManualExec"
	}
	if opts != nil {
		workflowNotification.StatusTextKeyOverride = opts.StatusTextKeyOverride
		workflowNotification.PendingStageName = opts.PendingStageName
	}

	if task.Type == config.WorkflowTaskTypeScanning {
		segs := strings.Split(task.WorkflowName, "-")
		workflowNotification.ScanningID = segs[len(segs)-1]
	}

	webhookNotify := &webhooknotify.WorkflowNotify{
		TaskID:              task.TaskID,
		WorkflowName:        task.WorkflowName,
		WorkflowDisplayName: task.WorkflowDisplayName,
		ProjectName:         task.ProjectName,
		ProjectDisplayName:  project.ProjectName,
		ReleasePlan:         buildWorkflowNotifyReleasePlan(task.ReleasePlan),
		Status:              task.Status,
		Remark:              task.Remark,
		Error:               task.Error,
		CreateTime:          task.CreateTime,
		StartTime:           task.StartTime,
		EndTime:             task.EndTime,
		TaskCreator:         task.TaskCreator,
		TaskCreatorID:       task.TaskCreatorID,
		TaskCreatorEmail:    task.TaskCreatorEmail,
		TaskType:            task.Type,
	}

	tplTitle := "{{if and (ne .WebHookType \"feishu\") (ne .WebHookType \"feishu_app\") (ne .WebHookType \"feishu_person\")}}### {{end}}{{if eq .WebHookType \"dingding\"}}<font color=\"{{ getColor .Task.Status }}\"><b>{{end}}{{getIcon .Task.Status }}{{getTaskType .Task.Type}} {{.Task.WorkflowDisplayName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }}{{if eq .WebHookType \"dingding\"}}</b></font>{{end}} \n"
	mailTplTitle := "{{getIcon .Task.Status }} {{getTaskType .Task.Type}} {{.Task.WorkflowDisplayName}}#{{.Task.TaskID}} {{ taskStatus .Task.Status }}"

	tplBaseInfo := []string{"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextExecutor\"}}**：{{.Task.TaskCreator}}  \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextProjectName\"}}**：{{.ProjectDisplayName}}  \n",
		"{{if .PendingStageName}}{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextPendingStage\"}}**：{{.PendingStageName}}  \n{{end}}",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextStartTime\"}}**：{{ getStartTime .Task.StartTime}}  \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextDuration\"}}**：{{ getDuration .TotalTime}}  \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextRemark\"}}**：{{.Task.Remark}} \n",
	}
	mailTplBaseInfo := []string{"{{getText \"notificationTextExecutor\"}}：{{.Task.TaskCreator}} \n",
		"{{getText \"notificationTextProjectName\"}}：{{.ProjectDisplayName}} \n",
		"{{if .PendingStageName}}{{getText \"notificationTextPendingStage\"}}：{{.PendingStageName}} \n{{end}}",
		"{{getText \"notificationTextStartTime\"}}：{{ getStartTime .Task.StartTime}} \n",
		"{{getText \"notificationTextDuration\"}}：{{ getDuration .TotalTime}} \n",
		"{{getText \"notificationTextRemark\"}}：{{ .Task.Remark}} \n",
	}

	jobContents, workflowNotifyStages, err := workflownotifyutil.BuildWorkflowJobContents(&workflownotifyutil.BuildJobContentsArgs{
		Task:        task,
		Stages:      task.Stages,
		WebHookType: notify.WebHookType,
		RenderTemplate: func(tpl string, job *models.JobTask) (string, error) {
			return getJobTaskTplExec(tpl, &jobTaskNotification{Job: job, WebHookType: notify.WebHookType}, language)
		},
		GetTestResult: func(jobName string) (string, error) {
			return genTestResultText(task.WorkflowName, jobName, task.TaskID, language)
		},
		GetSonarMetrics: func(jobSpec *models.JobTaskFreestyleSpec) (string, string, error) {
			return genSonartMetricsText(jobSpec, language)
		},
	})
	if err != nil {
		return "", "", nil, nil, err
	}
	webhookNotify.Stages = workflowNotifyStages

	title, err := getWorkflowTaskTplExec(tplTitle, workflowNotification)
	if err != nil {
		return "", "", nil, nil, err
	}
	buttonContent := getText("notificationTextClickForMore", language)
	workflowDetailURLTpl := ""
	workflowDetailURL := ""
	switch task.Type {
	case config.WorkflowTaskTypeWorkflow:
		workflowDetailURLTpl = "{{.BaseURI}}/v1/projects/detail/{{.Task.ProjectName}}/pipelines/custom/{{.Task.WorkflowName}}/{{.Task.TaskID}}?display_name={{.EncodedDisplayName}}"
		workflowDetailURL = fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s?display_name=%s", configbase.SystemAddress(), task.ProjectName, task.WorkflowName, url.PathEscape(task.WorkflowDisplayName))
	case config.WorkflowTaskTypeScanning:
		workflowDetailURLTpl = "{{.BaseURI}}/v1/projects/detail/{{.Task.ProjectName}}/scanner/detail/{{.Task.WorkflowDisplayName}}/task/{{.Task.TaskID}}?status={{.Task.Status}}&id={{.ScanningID}}"
		workflowDetailURL = fmt.Sprintf("%s/v1/projects/detail/%s/scanner/detail/%s/task/%d?id=%s", configbase.SystemAddress(), task.ProjectName, url.PathEscape(task.WorkflowDisplayName), task.TaskID, workflowNotification.ScanningID)
	case config.WorkflowTaskTypeTesting:
		workflowDetailURLTpl = "{{.BaseURI}}/v1/projects/detail/{{.Task.ProjectName}}/test/detail/function/{{.Task.WorkflowDisplayName}}/{{.Task.TaskID}}?status={{.Task.Status}}&id=&display_name={{.Task.WorkflowDisplayName}}"
		workflowDetailURL = fmt.Sprintf("%s/v1/projects/detail/%s/test/detail/function/%s/%d", configbase.SystemAddress(), task.ProjectName, url.PathEscape(task.WorkflowDisplayName), task.TaskID)
	default:
		workflowDetailURLTpl = "{{.BaseURI}}/v1/projects/detail/{{.Task.ProjectName}}/pipelines/custom/{{.Task.WorkflowName}}/{{.Task.TaskID}}?display_name={{.EncodedDisplayName}}"
		workflowDetailURL = fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s?display_name=%s", configbase.SystemAddress(), task.ProjectName, task.WorkflowName, url.PathEscape(task.WorkflowDisplayName))
	}
	moreInformation := fmt.Sprintf("\n\n{{if eq .WebHookType \"dingding\"}}---\n\n{{end}}[%s](%s)", buttonContent, workflowDetailURLTpl)

	if notify.WebHookType == setting.NotifyWebHookTypeMail {
		title, err := getWorkflowTaskTplExec(mailTplTitle, workflowNotification)
		if err != nil {
			return "", "", nil, nil, err
		}

		tplcontent := strings.Join(mailTplBaseInfo, "")
		tplcontent += strings.Join(jobContents, "")
		content, err := getWorkflowTaskTplExec(tplcontent, workflowNotification)
		if err != nil {
			return "", "", nil, nil, err
		}
		content = strings.TrimSpace(content)

		t, err := template.New("workflow_notification").Funcs(template.FuncMap{
			"getTaskType": func(taskType config.CustomWorkflowTaskType) string {
				if taskType == config.WorkflowTaskTypeWorkflow {
					return getText("taskTypeWorkflow", language)
				} else if taskType == config.WorkflowTaskTypeScanning {
					return getText("taskTypeScanning", language)
				} else if taskType == config.WorkflowTaskTypeTesting {
					return getText("taskTypeTesting", language)
				}
				return getText("taskTypeWorkflow", language)
			},
		}).Parse(getMailTemplate(language))
		if err != nil {
			err = fmt.Errorf("workflow notification template parse error, error msg:%s", err)
			return "", "", nil, nil, err
		}

		var buf bytes.Buffer
		err = t.Execute(&buf, struct {
			WorkflowName   string
			WorkflowTaskID int64
			TaskType       config.CustomWorkflowTaskType
			Content        string
			Url            string
		}{
			WorkflowName:   task.WorkflowDisplayName,
			WorkflowTaskID: task.TaskID,
			TaskType:       task.Type,
			Content:        content,
			Url:            workflowDetailURL,
		})
		if err != nil {
			err = fmt.Errorf("workflow notification template execute error, error msg:%s", err)
			return "", "", nil, nil, err
		}

		content = buf.String()
		return title, content, nil, nil, nil
	} else if notify.WebHookType == setting.NotifyWebHookTypeWebook {
		webhookNotify.DetailURL = workflowDetailURL
		return "", "", nil, webhookNotify, nil
	} else if notify.WebHookType != setting.NotifyWebHookTypeFeishu && notify.WebHookType != setting.NotifyWebhookTypeFeishuApp && notify.WebHookType != setting.NotifyWebHookTypeFeishuPerson {
		tplcontent := strings.Join(tplBaseInfo, "")
		tplcontent += strings.Join(jobContents, "")
		tplcontent = appendInlineNotifyAtContent(tplcontent, notify)
		tplcontent = fmt.Sprintf("%s%s", title, tplcontent)
		if notify.WebHookType == setting.NotifyWebHookTypeWechatWork {
			tplcontent = fmt.Sprintf("%s%s", tplcontent, moreInformation)
		}
		content, err := getWorkflowTaskTplExec(tplcontent, workflowNotification)
		if err != nil {
			return "", "", nil, nil, err
		}

		return title, content, nil, webhookNotify, nil
	}

	lc := NewLarkCard()
	lc.SetConfig(true)
	lc.SetHeader(getColorTemplateWithStatus(task.Status), title, feiShuTagText)
	for idx, feildContent := range tplBaseInfo {
		feildExecContent, _ := getWorkflowTaskTplExec(feildContent, workflowNotification)
		lc.AddI18NElementsZhcnFeild(feildExecContent, idx == 0)
	}
	for _, feildContent := range jobContents {
		feildExecContent, _ := getWorkflowTaskTplExec(feildContent, workflowNotification)
		lc.AddI18NElementsZhcnFeild(feildExecContent, true)
	}
	workflowDetailURLTpl, _ = getWorkflowTaskTplExec(workflowDetailURLTpl, workflowNotification)
	lc.AddI18NElementsZhcnAction(buttonContent, workflowDetailURLTpl)
	return "", "", lc, nil, nil
}

type workflowTaskNotification struct {
	Task                  *models.WorkflowTask      `json:"task"`
	ProjectDisplayName    string                    `json:"project_display_name"`
	EncodedDisplayName    string                    `json:"encoded_display_name"`
	BaseURI               string                    `json:"base_uri"`
	WebHookType           setting.NotifyWebHookType `json:"web_hook_type"`
	TotalTime             int64                     `json:"total_time"`
	ScanningID            string                    `json:"scanning_id"`
	StatusTextKeyOverride string                    `json:"status_text_key_override"`
	PendingStageName      string                    `json:"pending_stage_name"`
}

func isFeishuNotificationType(notifyType setting.NotifyWebHookType) bool {
	return notifyType == setting.NotifyWebHookTypeFeishu || notifyType == setting.NotifyWebhookTypeFeishuApp || notifyType == setting.NotifyWebHookTypeFeishuPerson
}

func getWorkflowTaskTplExec(tplcontent string, args *workflowTaskNotification) (string, error) {
	systemSetting, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		log.Errorf("getSystemLanguage err:%s", err)
		return "", fmt.Errorf("getSystemLanguage err:%s", err)
	}

	language := systemSetting.Language
	tmpl := template.Must(template.New("notify").Funcs(template.FuncMap{
		"getTaskType": func(taskType config.CustomWorkflowTaskType) string {
			if taskType == config.WorkflowTaskTypeWorkflow {
				return getText("taskTypeWorkflow", language)
			} else if taskType == config.WorkflowTaskTypeScanning {
				return getText("taskTypeScanning", language)
			} else if taskType == config.WorkflowTaskTypeTesting {
				return getText("taskTypeTesting", language)
			}
			return getText("taskTypeWorkflow", language)
		},
		"getColor": func(status config.Status) string {
			if status == config.StatusPassed || status == config.StatusCreated || status == config.StatusPrepare {
				return textColorGreen
			} else {
				return textColorRed
			}
		},
		"taskStatus": func(status config.Status) string {
			if args != nil && args.StatusTextKeyOverride != "" {
				return getText(args.StatusTextKeyOverride, language)
			}
			if status == config.StatusPassed {
				return getText("taskStatusSuccess", language)
			} else if status == config.StatusCancelled {
				return getText("taskStatusCancelled", language)
			} else if status == config.StatusTimeout {
				return getText("taskStatusTimeout", language)
			} else if status == config.StatusReject {
				return getText("taskStatusRejected", language)
			} else if status == config.StatusCreated || status == config.StatusPrepare {
				return getText("taskStatusExecutionStarted", language)
			} else if status == config.StatusManualApproval {
				return getText("taskStatusManualApproval", language)
			} else if status == config.StatusPause {
				return getText("taskStatusPause", language)
			} else {
				return getText("taskStatusFailed", language)
			}

		},
		"getIcon": func(status config.Status) string {
			if status == config.StatusPassed || status == config.StatusCreated || status == config.StatusPrepare {
				return "👍"
			} else if status == config.StatusFailed {
				return "❌"
			}
			return "⚠️"
		},
		"getStartTime": func(startTime int64) string {
			return time.Unix(startTime, 0).Format("2006-01-02 15:04:05")
		},
		"getDuration": func(startTime int64) string {
			duration, er := time.ParseDuration(strconv.FormatInt(startTime, 10) + "s")
			if er != nil {
				log.Errorf("getTplExec ParseDuration err:%s", er)
				return "0s"
			}
			return duration.String()
		},
		"getText": func(key string) string {
			return getText(key, language)
		},
	}).Parse(tplcontent))

	buffer := bytes.NewBufferString("")
	if err := tmpl.Execute(buffer, args); err != nil {
		log.Errorf("getTplExec Execute err:%s", err)
		return "", fmt.Errorf("getTplExec Execute err:%s", err)

	}
	return buffer.String(), nil
}

type jobTaskNotification struct {
	Job         *models.JobTask           `json:"task"`
	WebHookType setting.NotifyWebHookType `json:"web_hook_type"`
}

func getJobTaskTplExec(tplcontent string, args *jobTaskNotification, language string) (string, error) {
	tmpl := template.Must(template.New("notify").Funcs(template.FuncMap{
		"taskStatus": func(status config.Status) string {
			if status == config.StatusPassed {
				return getText("taskStatusSuccess", language)
			} else if status == config.StatusCancelled {
				return getText("taskStatusCancelled", language)
			} else if status == config.StatusTimeout {
				return getText("taskStatusTimeout", language)
			} else if status == config.StatusReject {
				return getText("taskStatusRejected", language)
			} else if status == config.StatusCreated || status == config.StatusPrepare {
				return getText("taskStatusExecutionStarted", language)
			} else if status == config.StatusManualApproval {
				return getText("taskStatusManualApproval", language)
			} else if status == config.StatusPause {
				return getText("taskStatusPause", language)
			} else if status == "" {
				return getText("jobStatusUnstarted", language)
			}
			return getText("taskStatusFailed", language)
		},
		"jobType": func(jobType string) string {
			switch jobType {
			case string(config.JobZadigBuild):
				return getText("jobTypeBuild", language)
			case string(config.JobZadigDeploy):
				return getText("jobTypeDeploy", language)
			case string(config.JobZadigVMDeploy):
				return getText("jobTypeVmDeploy", language)
			case string(config.JobFreestyle):
				return getText("jobTypeFreestyle", language)
			case string(config.JobNacos):
				return getText("jobTypeNacos", language)
			case string(config.JobPlugin):
				return getText("jobTypePlugin", language)
			case string(config.JobZadigTesting):
				return getText("jobTypeTest", language)
			case string(config.JobZadigScanning):
				return getText("jobTypeScan", language)
			case string(config.JobApproval):
				return getText("jobTypeApproval", language)
			case string(config.JobZadigDistributeImage):
				return getText("jobTypeDistribute", language)
			case string(config.JobCustomDeploy):
				return getText("jobTypeCustomDeploy", language)
			case string(config.JobK8sCanaryDeploy):
				return getText("jobTypeCanaryDeploy", language)
			case string(config.JobK8sCanaryRelease):
				return getText("jobTypeCanaryRelease", language)
			case string(config.JobMseGrayRelease):
				return getText("jobTypeMseGrayRelease", language)
			case string(config.JobMseGrayOffline):
				return getText("jobTypeMseGrayOffline", language)
			case string(config.JobK8sBlueGreenDeploy):
				return getText("jobTypeBlueGreenDeploy", language)
			case string(config.JobK8sBlueGreenRelease):
				return getText("jobTypeBlueGreenRelease", language)
			case string(config.JobK8sPatch):
				return getText("jobTypeK8sResourcePatch", language)
			case string(config.JobK8sGrayRollback):
				return getText("jobTypeK8sGrayRollback", language)
			case string(config.JobK8sGrayRelease):
				return getText("jobTypeGrayDeploy", language)
			case string(config.JobIstioRelease):
				return getText("jobTypeIstioRelease", language)
			case string(config.JobIstioRollback):
				return getText("jobTypeIstioRollback", language)
			case string(config.JobUpdateEnvIstioConfig):
				return getText("jobTypeIstioStrategy", language)
			case string(config.JobJira):
				return getText("jobTypeJira", language)
			case string(config.JobPingCode):
				return getText("jobTypePingCode", language)
			case string(config.JobTapd):
				return getText("jobTypeTapd", language)
			case string(config.JobApollo):
				return getText("jobTypeApollo", language)
			case string(config.JobMeegoTransition):
				return getText("jobTypeLark", language)
			case string(config.JobWorkflowTrigger):
				return getText("jobTypeWorkflowTrigger", language)
			case string(config.JobOfflineService):
				return getText("jobTypeOfflineService", language)
			case string(config.JobZadigHelmChartDeploy):
				return getText("jobTypeHelmChartDeploy", language)
			case string(config.JobGrafana):
				return getText("jobTypeGrafana", language)
			case string(config.JobJenkins):
				return getText("jobTypeJenkinsJob", language)
			case string(config.JobBlueKing):
				return getText("jobTypeBlueKingJob", language)
			case string(config.JobSQL):
				return getText("jobTypeSql", language)
			case string(config.JobNotification):
				return getText("jobTypeNotification", language)
			case string(config.JobSAEDeploy):
				return getText("jobTypeSaeDeploy", language)
			default:
				return string(jobType)
			}
		},
		"getText": func(key string) string {
			return getText(key, language)
		},
	}).Parse(tplcontent))

	buffer := bytes.NewBufferString("")
	if err := tmpl.Execute(buffer, args); err != nil {
		log.Errorf("getTplExec Execute err:%s", err)
		return "", fmt.Errorf("getTplExec Execute err:%s", err)

	}
	return buffer.String(), nil
}

func getMailTemplate(language string) string {
	if language == string(config.SystemLanguageEnUS) {
		return string(notificationENHTML)
	}
	return string(notificationHTML)
}

func genTestResultText(workflowName, jobTaskName string, taskID int64, language string) (string, error) {
	testResultList, err := commonrepo.NewCustomWorkflowTestReportColl().ListByWorkflowJobTaskName(workflowName, jobTaskName, taskID)
	if err != nil {
		log.Errorf("failed to list junit test report for workflow: %s, error: %s", workflowName, err)
		return "", fmt.Errorf("failed to list junit test report for workflow: %s, error: %s", workflowName, err)
	}

	result := ""
	for _, report := range testResultList {
		totalNum := report.TestCaseNum
		failedNum := report.FailedCaseNum
		successNum := report.SuccessCaseNum
		result += fmt.Sprintf("%d(%s) %d(%s) %d(%s)\n", successNum, getText("testStatusSuccess", language), failedNum, getText("testStatusFailed", language), totalNum, getText("testTotal", language))
	}
	return result, nil
}

func genSonartMetricsText(jobSpec *models.JobTaskFreestyleSpec, language string) (string, string, error) {
	getQualityGateStatusText := func(qualityGateStatus sonar.QualityGateStatus, language string) string {
		if language == string(config.SystemLanguageEnUS) {
			if qualityGateStatus == "" {
				return "NONE"
			} else {
				return string(qualityGateStatus)
			}
		}

		if qualityGateStatus == "OK" {
			return "通过"
		} else if qualityGateStatus == "WARN" {
			return "警告"
		} else if qualityGateStatus == "ERROR" {
			return "未通过"
		} else if qualityGateStatus == "NONE" || qualityGateStatus == "" {
			return "未开启"
		}
		return ""
	}

	result := ""
	mailResult := ""
	for _, jobStep := range jobSpec.Steps {
		if jobStep.StepType == config.StepSonarGetMetrics {
			stepSpec := &step.StepSonarGetMetricsSpec{}
			models.IToi(jobStep.Spec, stepSpec)

			if stepSpec.SonarMetrics == nil {
				return "", "", nil
			}

			result = fmt.Sprintf("**%s**(%s) **%s**(%s) **%s**(%s) **%s**(%s) **%s**(%s) **%s%%**(%s)",
				getQualityGateStatusText(stepSpec.SonarMetrics.QualityGateStatus, language), getText("sonarQualityGateStatus", language),
				stepSpec.SonarMetrics.Ncloc, getText("sonarNcloc", language),
				stepSpec.SonarMetrics.Bugs, getText("sonarBugs", language),
				stepSpec.SonarMetrics.Vulnerabilities, getText("sonarVulnerabilities", language),
				stepSpec.SonarMetrics.CodeSmells, getText("sonarCodeSmells", language),
				stepSpec.SonarMetrics.Coverage, getText("sonarCoverage", language),
			)
			mailResult = fmt.Sprintf("%s(%s) %s(%s) %s(%s) %s(%s) %s(%s) %s%%(%s)",
				getQualityGateStatusText(stepSpec.SonarMetrics.QualityGateStatus, language), getText("sonarQualityGateStatus", language),
				stepSpec.SonarMetrics.Ncloc, getText("sonarNcloc", language),
				stepSpec.SonarMetrics.Bugs, getText("sonarBugs", language),
				stepSpec.SonarMetrics.Vulnerabilities, getText("sonarVulnerabilities", language),
				stepSpec.SonarMetrics.CodeSmells, getText("sonarCodeSmells", language),
				stepSpec.SonarMetrics.Coverage, getText("sonarCoverage", language),
			)
		}
	}

	return result, mailResult, nil
}

func appendInlineNotifyAtContent(content string, notify *models.NotifyCtl) string {
	if notify == nil || notify.WebHookType == setting.NotifyWebHookTypeDingDing {
		return content
	}
	return content + getNotifyAtContent(notify)
}

func (w *Service) sendNotification(title, content string, notify *models.NotifyCtl, card *LarkCard, webhookNotify *webhooknotify.WorkflowNotify, taskStatus config.Status) error {
	link := ""
	if notify.WebHookType == setting.NotifyWebHookTypeDingDing || notify.WebHookType == setting.NotifyWebHookTypeWechatWork || notify.WebHookType == setting.NotifyWebHookTypeMSTeam {
		switch webhookNotify.TaskType {
		case config.WorkflowTaskTypeWorkflow:
			link = fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", configbase.SystemAddress(), webhookNotify.ProjectName, webhookNotify.WorkflowName, webhookNotify.TaskID, url.PathEscape(webhookNotify.WorkflowDisplayName))
		case config.WorkflowTaskTypeScanning:
			segs := strings.Split(webhookNotify.WorkflowName, "-")
			link = fmt.Sprintf("%s/v1/projects/detail/%s/scanner/detail/%s/task/%d?id=%s", configbase.SystemAddress(), webhookNotify.ProjectName, url.PathEscape(webhookNotify.WorkflowDisplayName), webhookNotify.TaskID, segs[len(segs)-1])
		case config.WorkflowTaskTypeTesting:
			link = fmt.Sprintf("%s/v1/projects/detail/%s/test/detail/function/%s/%d", configbase.SystemAddress(), webhookNotify.ProjectName, url.PathEscape(webhookNotify.WorkflowDisplayName), webhookNotify.TaskID)
		default:
			link = fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s?display_name=%s", configbase.SystemAddress(), webhookNotify.ProjectName, webhookNotify.WorkflowName, url.PathEscape(webhookNotify.WorkflowDisplayName))
		}
	}

	switch notify.WebHookType {
	case setting.NotifyWebHookTypeMSTeam:
		if err := w.sendMSTeamsMessage(notify.MSTeamsNotificationConfig.HookAddress, title, content, link, notify.MSTeamsNotificationConfig.AtEmails, taskStatus); err != nil {
			return err
		}
	case setting.NotifyWebHookTypeDingDing:
		if err := w.sendDingDingMessage(notify.DingDingNotificationConfig.HookAddress, title, content, link, notify.DingDingNotificationConfig.AtMobiles, notify.DingDingNotificationConfig.IsAtAll); err != nil {
			return err
		}
	case setting.NotifyWebHookTypeFeishu:
		if err := w.sendFeishuMessage(notify.LarkHookNotificationConfig.HookAddress, card); err != nil {
			return err
		}
		if err := w.sendFeishuMessageOfSingleType("", notify.LarkHookNotificationConfig.HookAddress, getNotifyAtContent(notify)); err != nil {
			return err
		}
	case setting.NotifyWebHookTypeMail:
		if err := w.sendMailMessage(title, content, notify.MailNotificationConfig.TargetUsers); err != nil {
			return err
		}
	case setting.NotifyWebHookTypeWebook:
		webhookclient := webhooknotify.NewClient(notify.WebhookNotificationConfig.Address, notify.WebhookNotificationConfig.Token)
		err := webhookclient.SendWorkflowWebhook(webhookNotify, webhooknotify.WebHookNotifyEventWorkflow)
		if err != nil {
			return fmt.Errorf("failed to send notification to webhook, address %s, token: %s, error: %v", notify.WebhookNotificationConfig.Address, notify.WebhookNotificationConfig.Token, err)
		}
	case setting.NotifyWebhookTypeFeishuApp:
		client, err := larkservice.GetLarkClientByIMAppID(notify.LarkGroupNotificationConfig.AppID)
		if err != nil {
			return fmt.Errorf("failed to send notification by lark app: failed to create lark client appID: %s, error: %s", notify.LarkGroupNotificationConfig.AppID, err)
		}

		messageContent, err := json.Marshal(card)
		if err != nil {
			return fmt.Errorf("failed to send notification by lark app: failed to parse the lark card, error: %s", err)
		}

		err = w.sendFeishuMessageFromClient(client, LarkReceiverTypeChat, notify.LarkGroupNotificationConfig.Chat.ChatID, LarkMessageTypeCard, string(messageContent))
		if err != nil {
			return fmt.Errorf("failed to send notification by lark app: failed to send lark card, error: %s", err)
		}

		err = w.sendFeishuMessageFromClient(client, LarkReceiverTypeChat, notify.LarkGroupNotificationConfig.Chat.ChatID, LarkMessageTypeText, getNotifyAtContent(notify))
		if err != nil {
			return fmt.Errorf("failed to send notification by lark app: failed to send lark at message, error: %s", err)
		}
	case setting.NotifyWebHookTypeFeishuPerson:
		client, err := larkservice.GetLarkClientByIMAppID(notify.LarkPersonNotificationConfig.AppID)
		if err != nil {
			return fmt.Errorf("failed to send notification by lark app: failed to create lark client appID: %s, error: %s", notify.LarkPersonNotificationConfig.AppID, err)
		}

		messageContent, err := json.Marshal(card)
		if err != nil {
			return fmt.Errorf("failed to send notification by lark app: failed to parse the lark card, error: %s", err)
		}

		respErr := new(multierror.Error)
		for _, target := range notify.LarkPersonNotificationConfig.TargetUsers {
			if target == nil || target.ID == "" {
				continue
			}
			err = w.sendFeishuMessageFromClient(client, target.IDType, target.ID, LarkMessageTypeCard, string(messageContent))
			if err != nil {
				respErr = multierror.Append(respErr, err)
			}
		}

		return respErr.ErrorOrNil()
	default:
		if err := w.SendWeChatWorkMessage(WeChatTextTypeMarkdown, notify.WechatNotificationConfig.HookAddress, "", "", content); err != nil {
			return err
		}
	}
	return nil
}

func getText(key, language string) string {
	var textMap map[string]string
	switch language {
	case string(config.SystemLanguageEnUS):
		textMap = enTextMap
	default:
		textMap = zhTextMap
	}

	if text, exists := textMap[key]; exists {
		return text
	}
	return key
}
