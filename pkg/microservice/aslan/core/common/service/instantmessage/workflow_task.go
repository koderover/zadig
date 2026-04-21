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
	"sort"
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
	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhooknotify"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/sonar"
	"github.com/koderover/zadig/v2/pkg/types"
	jobspec "github.com/koderover/zadig/v2/pkg/types/job"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

var (
	zhTextMap = map[string]string{
		"taskTypeWorkflow": "工作流",
		"taskTypeScanning": "代码扫描",
		"taskTypeTesting":  "测试",

		"taskStatusSuccess":          "执行成功",
		"taskStatusFailed":           "执行失败",
		"taskStatusCancelled":        "执行取消",
		"taskStatusTimeout":          "执行超时",
		"taskStatusRejected":         "执行被拒绝",
		"taskStatusExecutionStarted": "开始执行",
		"taskStatusManualApproval":   "待确认",
		"taskStatusPause":            "暂停",
		"jobStatusUnstarted":         "未执行",

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
		"notificationTextManualExecPending":  "等待手动执行",
		"notificationTextExecutor":           "执行用户",
		"notificationTextNotifiedUsers":      "被通知人",
		"notificationTextJobs":               "任务信息",
		"notificationTextProjectName":        "项目名称",
		"notificationTextStageName":          "阶段名称",
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

		"taskStatusSuccess":          "Passed",
		"taskStatusFailed":           "Failed",
		"taskStatusCancelled":        "Cancelled",
		"taskStatusTimeout":          "Timeout",
		"taskStatusRejected":         "Rejected",
		"taskStatusExecutionStarted": "Created",
		"taskStatusManualApproval":   "Waiting for confirmation",
		"taskStatusPause":            "Pause",
		"jobStatusUnstarted":         "Unstarted",

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
		"notificationTextManualExecPending":  "Waiting for Manual Execution",
		"notificationTextExecutor":           "Executor",
		"notificationTextNotifiedUsers":      "Notified Users",
		"notificationTextJobs":               "Jobs",
		"notificationTextProjectName":        "Project Name",
		"notificationTextStageName":          "Stage Name",
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

	for _, notify := range resp.NotifyCtls {
		statusSets := sets.NewString(notify.NotifyTypes...)
		if !statusSets.Has(string(config.StatusWaitingApprove)) {
			continue
		}
		if !notify.Enabled {
			continue
		}

		err := notify.GenerateNewNotifyConfigWithOldData()
		if err != nil {
			return err
		}

		title, content, larkCard, webhookNotify, err := w.getApproveNotificationContent(notify, task)
		if err != nil {
			errMsg := fmt.Sprintf("failed to get notification content, err: %s", err)
			log.Error(errMsg)
			return errors.New(errMsg)
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
					if task.TaskCreatorID == "" {
						errMsg := fmt.Errorf("executor id is empty, cannot send message")
						log.Error(errMsg)
						continue
					}

					userInfo, err := userclient.New().GetUserByID(task.TaskCreatorID)
					if err != nil {
						log.Errorf("failed to find user %s, error: %s", task.TaskCreatorID, err)
						return fmt.Errorf("failed to find user %s, error: %s", task.TaskCreatorID, err)
					}

					if len(userInfo.Phone) == 0 {
						return fmt.Errorf("executor phone not configured")
					}

					client, err := larkservice.GetLarkClientByIMAppID(notify.LarkPersonNotificationConfig.AppID)
					if err != nil {
						return fmt.Errorf("failed to get notify target info: create feishu client error: %s", err)
					}

					larkUser, err := client.GetUserIDByEmailOrMobile(lark.QueryTypeMobile, userInfo.Phone, setting.LarkUserID)
					if err != nil {
						return fmt.Errorf("find lark user with phone %s error: %v", userInfo.Phone, err)
					}

					userDetailedInfo, err := client.GetUserInfoByID(util.GetStringFromPointer(larkUser.UserId), setting.LarkUserID)
					if err != nil {
						return fmt.Errorf("find lark user info for userID %s error: %v", util.GetStringFromPointer(larkUser.UserId), err)
					}

					target.ID = util.GetStringFromPointer(larkUser.UserId)
					target.Name = userDetailedInfo.Name
					target.Avatar = userDetailedInfo.Avatar
					target.IDType = setting.LarkUserID
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
	for _, notify := range task.OriginWorkflowArgs.NotifyCtls {
		if !notify.Enabled {
			continue
		}

		err := notify.GenerateNewNotifyConfigWithOldData()
		if err != nil {
			return err
		}

		statusSets := sets.NewString(notify.NotifyTypes...)
		if statusSets.Has(string(task.Status)) || (statusChanged && statusSets.Has(string(config.StatusChanged))) {
			title, content, larkCard, webhookNotify, err := w.getNotificationContent(notify, task)
			if err != nil {
				errMsg := fmt.Sprintf("failed to get notification content, err: %s", err)
				log.Error(errMsg)
				return errors.New(errMsg)
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
						if task.TaskCreatorID == "" {
							errMsg := fmt.Errorf("executor id is empty, cannot send message")
							log.Error(errMsg)
							continue
						}

						userInfo, err := userclient.New().GetUserByID(task.TaskCreatorID)
						if err != nil {
							log.Errorf("failed to find user %s, error: %s", task.TaskCreatorID, err)
							return fmt.Errorf("failed to find user %s, error: %s", task.TaskCreatorID, err)
						}

						if len(userInfo.Phone) == 0 {
							return fmt.Errorf("executor phone not configured")
						}

						client, err := larkservice.GetLarkClientByIMAppID(notify.LarkPersonNotificationConfig.AppID)
						if err != nil {
							return fmt.Errorf("failed to get notify target info: create feishu client error: %s", err)
						}

						larkUser, err := client.GetUserIDByEmailOrMobile(lark.QueryTypeMobile, userInfo.Phone, setting.LarkUserID)
						if err != nil {
							return fmt.Errorf("find lark user with phone %s error: %v", userInfo.Phone, err)
						}

						userDetailedInfo, err := client.GetUserInfoByID(util.GetStringFromPointer(larkUser.UserId), setting.LarkUserID)
						if err != nil {
							return fmt.Errorf("find lark user info for userID %s error: %v", util.GetStringFromPointer(larkUser.UserId), err)
						}

						target.ID = util.GetStringFromPointer(larkUser.UserId)
						target.Name = userDetailedInfo.Name
						target.Avatar = userDetailedInfo.Avatar
						target.IDType = setting.LarkUserID
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

func (w *Service) SendManualExecStageNotifications(workflowCtx *models.WorkflowTaskCtx, stage *models.StageTask) error {
	if workflowCtx == nil || stage == nil || stage.ManualExec == nil {
		return nil
	}
	notifyCfg := stage.ManualExec.LarkPersonNotificationConfig
	if notifyCfg == nil || notifyCfg.AppID == "" || len(stage.ManualExec.ManualExecUsers) == 0 {
		return nil
	}

	systemSetting, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		return fmt.Errorf("get system language error: %w", err)
	}
	language := systemSetting.Language

	client, err := larkservice.GetLarkClientByIMAppID(notifyCfg.AppID)
	if err != nil {
		return fmt.Errorf("create feishu client error: %w", err)
	}

	manualExecUsers, userInfoMap := commonutil.GeneFlatUsersWithCaller(stage.ManualExec.ManualExecUsers, workflowCtx.WorkflowTaskCreatorUserID)
	notifiedUsers := formatManualExecNotifiedUsers(manualExecUsers, userInfoMap)

	messageContent, err := json.Marshal(w.getManualExecStageLarkCard(workflowCtx, stage, language, notifiedUsers))
	if err != nil {
		return fmt.Errorf("marshal manual exec stage notification card error: %w", err)
	}

	respErr := new(multierror.Error)
	sentTargets := sets.NewString()
	for _, execUser := range manualExecUsers {
		if execUser == nil || execUser.UserID == "" {
			continue
		}
		userInfo, ok := userInfoMap[execUser.UserID]
		if !ok {
			var getErr error
			userInfo, getErr = userclient.New().GetUserByID(execUser.UserID)
			if getErr != nil {
				respErr = multierror.Append(respErr, fmt.Errorf("find manual executor %s error: %w", execUser.UserID, getErr))
				continue
			}
		}
		if userInfo.Phone == "" {
			respErr = multierror.Append(respErr, fmt.Errorf("manual executor %s phone not configured", execUser.UserID))
			continue
		}

		larkUser, err := client.GetUserIDByEmailOrMobile(lark.QueryTypeMobile, userInfo.Phone, setting.LarkUserID)
		if err != nil {
			respErr = multierror.Append(respErr, fmt.Errorf("find lark user with phone %s error: %w", userInfo.Phone, err))
			continue
		}
		targetID := util.GetStringFromPointer(larkUser.UserId)
		if targetID == "" {
			continue
		}
		if sentTargets.Has(targetID) {
			continue
		}
		sentTargets.Insert(targetID)

		if err := w.sendFeishuMessageFromClient(client, setting.LarkUserID, targetID, LarkMessageTypeCard, string(messageContent)); err != nil {
			respErr = multierror.Append(respErr, err)
		}
	}

	return respErr.ErrorOrNil()
}

func formatManualExecNotifiedUsers(users []*models.User, userInfoMap map[string]*types.UserInfo) string {
	if len(users) == 0 {
		return ""
	}

	names := make([]string, 0, len(users))
	nameSet := sets.NewString()
	for _, user := range users {
		if user == nil || user.UserID == "" {
			continue
		}

		name := user.UserName
		if info, ok := userInfoMap[user.UserID]; ok && info != nil && info.Name != "" {
			name = info.Name
		}
		if name == "" {
			name = user.UserID
		}
		if nameSet.Has(name) {
			continue
		}
		nameSet.Insert(name)
		names = append(names, name)
	}

	sort.Strings(names)
	return strings.Join(names, ", ")
}

func formatManualExecStageJobs(stage *models.StageTask, language string) string {
	if stage == nil || len(stage.Jobs) == 0 {
		return ""
	}

	lines := make([]string, 0, len(stage.Jobs))
	for _, job := range stage.Jobs {
		if job == nil {
			continue
		}

		jobName := job.DisplayName
		if jobName == "" {
			jobName = job.Name
		}
		jobLine := fmt.Sprintf("%s: %s", formatManualExecJobType(job.JobType, language), jobName)

		switch job.JobType {
		case string(config.JobZadigDeploy):
			jobSpec := &models.JobTaskDeploySpec{}
			models.IToi(job.Spec, jobSpec)
			if jobSpec.Env != "" {
				jobLine = fmt.Sprintf("%s (env: %s)", jobLine, jobSpec.Env)
			}
		case string(config.JobZadigHelmDeploy):
			jobSpec := &models.JobTaskHelmDeploySpec{}
			models.IToi(job.Spec, jobSpec)
			if jobSpec.Env != "" {
				jobLine = fmt.Sprintf("%s (env: %s)", jobLine, jobSpec.Env)
			}
		}

		lines = append(lines, jobLine)
	}

	return strings.Join(lines, "\n")
}

func formatManualExecJobType(jobType, language string) string {
	switch jobType {
	case string(config.JobZadigBuild):
		return getText("jobTypeBuild", language)
	case string(config.JobZadigDeploy):
		return getText("jobTypeDeploy", language)
	case string(config.JobCustomDeploy):
		return getText("jobTypeCustomDeploy", language)
	case string(config.JobZadigTesting):
		return getText("jobTypeTest", language)
	case string(config.JobZadigScanning):
		return getText("jobTypeScan", language)
	case string(config.JobFreestyle):
		return getText("jobTypeFreestyle", language)
	case string(config.JobPlugin):
		return getText("jobTypePlugin", language)
	case string(config.JobWorkflowTrigger):
		return getText("jobTypeWorkflowTrigger", language)
	case string(config.JobApproval):
		return getText("jobTypeApproval", language)
	case string(config.JobZadigHelmDeploy):
		return getText("jobTypeDeploy", language)
	case string(config.JobZadigHelmChartDeploy):
		return getText("jobTypeHelmChartDeploy", language)
	default:
		return jobType
	}
}

func (w *Service) getManualExecStageLarkCard(workflowCtx *models.WorkflowTaskCtx, stage *models.StageTask, language, notifiedUsers string) *LarkCard {
	title := fmt.Sprintf("%s %s #%d %s", getText("notificationTextWorkflow", language), workflowCtx.WorkflowDisplayName, workflowCtx.TaskID, getText("notificationTextManualExecPending", language))
	detailURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		configbase.SystemAddress(),
		workflowCtx.ProjectName,
		workflowCtx.WorkflowName,
		workflowCtx.TaskID,
		url.QueryEscape(workflowCtx.WorkflowDisplayName),
	)

	lc := NewLarkCard()
	lc.SetConfig(true)
	lc.SetHeader(feishuHeaderTemplateTurquoise, title, feiShuTagText)
	lc.AddI18NElementsZhcnFeild(fmt.Sprintf("**%s**：%s", getText("notificationTextProjectName", language), workflowCtx.ProjectDisplayName), true)
	lc.AddI18NElementsZhcnFeild(fmt.Sprintf("**%s**：%s", getText("notificationTextStageName", language), stage.Name), true)
	lc.AddI18NElementsZhcnFeild(fmt.Sprintf("**%s**：%s", getText("notificationTextExecutor", language), workflowCtx.WorkflowTaskCreatorUsername), true)
	if notifiedUsers != "" {
		lc.AddI18NElementsZhcnFeild(fmt.Sprintf("**%s**：%s", getText("notificationTextNotifiedUsers", language), notifiedUsers), true)
	}
	if jobsSummary := formatManualExecStageJobs(stage, language); jobsSummary != "" {
		lc.AddI18NElementsZhcnFeild(fmt.Sprintf("**%s**：%s", getText("notificationTextJobs", language), jobsSummary), true)
	}
	lc.AddI18NElementsZhcnFeild(fmt.Sprintf("**%s**：%s", getText("notificationTextStartTime", language), workflowCtx.StartTime.Format(time.DateTime)), true)
	lc.AddI18NElementsZhcnFeild(fmt.Sprintf("**%s**：%s", getText("notificationTextRemark", language), workflowCtx.Remark), true)
	lc.AddI18NElementsZhcnAction(getText("notificationTextClickForMore", language), detailURL)
	return lc
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
		Status:              task.Status,
		Remark:              task.Remark,
		Error:               task.Error,
		CreateTime:          task.CreateTime,
		StartTime:           task.StartTime,
		EndTime:             task.EndTime,
		TaskCreator:         task.TaskCreator,
		TaskCreatorID:       task.TaskCreatorID,
		TaskCreatorPhone:    task.TaskCreatorPhone,
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
		tplcontent = tplcontent + getNotifyAtContent(notify)
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
		Status:              task.Status,
		Remark:              task.Remark,
		Error:               task.Error,
		CreateTime:          task.CreateTime,
		StartTime:           task.StartTime,
		EndTime:             task.EndTime,
		TaskCreator:         task.TaskCreator,
		TaskCreatorID:       task.TaskCreatorID,
		TaskCreatorPhone:    task.TaskCreatorPhone,
		TaskCreatorEmail:    task.TaskCreatorEmail,
		TaskType:            task.Type,
	}

	tplTitle := "{{if and (ne .WebHookType \"feishu\") (ne .WebHookType \"feishu_app\") (ne .WebHookType \"feishu_person\")}}### {{end}}{{if eq .WebHookType \"dingding\"}}<font color=\"{{ getColor .Task.Status }}\"><b>{{end}}{{getIcon .Task.Status }}{{getTaskType .Task.Type}} {{.Task.WorkflowDisplayName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }}{{if eq .WebHookType \"dingding\"}}</b></font>{{end}} \n"
	mailTplTitle := "{{getIcon .Task.Status }} {{getTaskType .Task.Type}} {{.Task.WorkflowDisplayName}}#{{.Task.TaskID}} {{ taskStatus .Task.Status }}"

	tplBaseInfo := []string{"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextExecutor\"}}**：{{.Task.TaskCreator}}  \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextProjectName\"}}**：{{.ProjectDisplayName}}  \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextStartTime\"}}**：{{ getStartTime .Task.StartTime}}  \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextDuration\"}}**：{{ getDuration .TotalTime}}  \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextRemark\"}}**：{{.Task.Remark}} \n",
	}
	mailTplBaseInfo := []string{"{{getText \"notificationTextExecutor\"}}：{{.Task.TaskCreator}} \n",
		"{{getText \"notificationTextProjectName\"}}：{{.ProjectDisplayName}} \n",
		"{{getText \"notificationTextStartTime\"}}：{{ getStartTime .Task.StartTime}} \n",
		"{{getText \"notificationTextDuration\"}}：{{ getDuration .TotalTime}} \n",
		"{{getText \"notificationTextRemark\"}}：{{ .Task.Remark}} \n",
	}

	jobContents := []string{}
	workflowNotifyStages := []*webhooknotify.WorkflowNotifyStage{}
	for _, stage := range task.Stages {
		workflowNotifyStage := &webhooknotify.WorkflowNotifyStage{
			Name:      stage.Name,
			Status:    stage.Status,
			StartTime: stage.StartTime,
			EndTime:   stage.EndTime,
			Error:     stage.Error,
		}

		for _, job := range stage.Jobs {
			workflowNotifyJob := &webhooknotify.WorkflowNotifyJobTask{
				Name:        job.Name,
				DisplayName: job.DisplayName,
				JobType:     job.JobType,
				Status:      job.Status,
				StartTime:   job.StartTime,
				EndTime:     job.EndTime,
				Error:       job.Error,
			}

			jobTplcontent := "{{if and (ne .WebHookType \"feishu\") (ne .WebHookType \"feishu_app\") (ne .WebHookType \"feishu_person\")}}\n\n{{end}}{{if eq .WebHookType \"dingding\"}}---\n\n##### {{end}}**{{jobType .Job.JobType }}**: {{.Job.DisplayName}}    **{{getText \"notificationTextStatus\"}}**: {{taskStatus .Job.Status }}  \n"
			mailJobTplcontent := "{{jobType .Job.JobType }}：{{.Job.DisplayName}}    {{getText \"notificationTextStatus\"}}：{{taskStatus .Job.Status }} \n"
			switch job.JobType {
			case string(config.JobZadigBuild):
				fallthrough
			case string(config.JobFreestyle):
				jobSpec := &models.JobTaskFreestyleSpec{}
				models.IToi(job.Spec, jobSpec)

				workflowNotifyJobTaskSpec := &webhooknotify.WorkflowNotifyJobTaskBuildSpec{}

				repos := []*types.Repository{}
				for _, stepTask := range jobSpec.Steps {
					if stepTask.StepType == config.StepGit {
						stepSpec := &step.StepGitSpec{}
						models.IToi(stepTask.Spec, stepSpec)
						repos = stepSpec.Repos
					}
				}

				branchTag, commitID, gitCommitURL := "", "", ""
				commitMsgs := []string{}
				var prInfoList []string
				var prInfo string
				for idx, buildRepo := range repos {
					workflowNotifyRepository := &webhooknotify.WorkflowNotifyRepository{
						Source:        buildRepo.Source,
						RepoOwner:     buildRepo.RepoOwner,
						RepoNamespace: buildRepo.RepoNamespace,
						RepoName:      buildRepo.RepoName,
						Branch:        buildRepo.Branch,
						Tag:           buildRepo.Tag,
						AuthorName:    buildRepo.AuthorName,
						CommitID:      buildRepo.CommitID,
						CommitMessage: buildRepo.CommitMessage,
					}
					if idx == 0 || buildRepo.IsPrimary {
						branchTag = buildRepo.Branch
						if buildRepo.Tag != "" {
							branchTag = buildRepo.Tag
						}
						if len(buildRepo.CommitID) > 8 {
							commitID = buildRepo.CommitID[0:8]
						}
						var prLinkBuilder func(baseURL, owner, repoName string, prID int) string
						switch buildRepo.Source {
						case types.ProviderGithub:
							prLinkBuilder = func(baseURL, owner, repoName string, prID int) string {
								return fmt.Sprintf("%s/%s/%s/pull/%d", baseURL, owner, repoName, prID)
							}
						case types.ProviderGitee:
							prLinkBuilder = func(baseURL, owner, repoName string, prID int) string {
								return fmt.Sprintf("%s/%s/%s/pulls/%d", baseURL, owner, repoName, prID)
							}
						case types.ProviderGitlab:
							prLinkBuilder = func(baseURL, owner, repoName string, prID int) string {
								return fmt.Sprintf("%s/%s/%s/merge_requests/%d", baseURL, owner, repoName, prID)
							}
						case types.ProviderGerrit:
							prLinkBuilder = func(baseURL, owner, repoName string, prID int) string {
								return fmt.Sprintf("%s/%d", baseURL, prID)
							}
						default:
							prLinkBuilder = func(baseURL, owner, repoName string, prID int) string {
								return ""
							}
						}
						prInfoList = []string{}
						sort.Ints(buildRepo.PRs)
						for _, id := range buildRepo.PRs {
							link := prLinkBuilder(buildRepo.Address, buildRepo.RepoOwner, buildRepo.RepoName, id)
							if link != "" {
								prInfoList = append(prInfoList, fmt.Sprintf("[#%d](%s)", id, link))
							}
						}
						commitMsg := strings.Trim(buildRepo.CommitMessage, "\n")
						commitMsgs = strings.Split(commitMsg, "\n")
						gitCommitURL = fmt.Sprintf("%s/%s/%s/commit/%s", buildRepo.Address, buildRepo.RepoOwner, buildRepo.RepoName, commitID)
						workflowNotifyRepository.CommitURL = gitCommitURL
					}

					workflowNotifyJobTaskSpec.Repositories = append(workflowNotifyJobTaskSpec.Repositories, workflowNotifyRepository)
				}
				if len(prInfoList) != 0 {
					// need an extra space at the end
					prInfo = strings.Join(prInfoList, " ") + " "
				}
				image := ""
				imageContextKey := strings.Join(strings.Split(jobspec.GetJobOutputKey(job.Key, "IMAGE"), "."), "@?")
				if task.GlobalContext != nil {
					image = task.GlobalContext[imageContextKey]
				}
				if len(commitID) > 0 {
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextRepositoryInfo\"}}**：%s %s[%s](%s)  ", branchTag, prInfo, commitID, gitCommitURL)
					jobTplcontent += "{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextCommitMessage\"}}**："
					mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextRepositoryInfo\"}}：%s %s[%s]( %s )  ", branchTag, prInfo, commitID, gitCommitURL)
					if len(commitMsgs) == 1 {
						jobTplcontent += fmt.Sprintf("%s \n", commitMsgs[0])
					} else {
						jobTplcontent += "\n"
						for _, commitMsg := range commitMsgs {
							jobTplcontent += fmt.Sprintf("%s \n", commitMsg)
						}
					}
				}
				if job.Status == config.StatusPassed && image != "" && !strings.HasPrefix(image, "{{.") && !strings.Contains(image, "}}") {
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextImageInfo\"}}**：%s  \n", image)
					mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextImageInfo\"}}：%s \n", image)
					workflowNotifyJobTaskSpec.Image = image
				}

				workflowNotifyJob.Spec = workflowNotifyJobTaskSpec
			case string(config.JobZadigDeploy):
				jobSpec := &models.JobTaskDeploySpec{}
				models.IToi(job.Spec, jobSpec)
				jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextEnvironment\"}}**：%s  \n", jobSpec.Env)
				mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextEnvironment\"}}：%s \n", jobSpec.Env)

				if job.Status == config.StatusPassed && len(jobSpec.ServiceAndImages) > 0 {
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextImageInfo\"}}**：  \n")
					mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextImageInfo\"}}：  \n")
				}

				serviceModules := []*webhooknotify.WorkflowNotifyDeployServiceModule{}
				for _, serviceAndImage := range jobSpec.ServiceAndImages {
					if job.Status == config.StatusPassed && !strings.HasPrefix(serviceAndImage.Image, "{{.") && !strings.Contains(serviceAndImage.Image, "}}") {
						jobTplcontent += fmt.Sprintf("%s  \n", serviceAndImage.Image)
						mailJobTplcontent += fmt.Sprintf("%s  \n", serviceAndImage.Image)
					}

					serviceModule := &webhooknotify.WorkflowNotifyDeployServiceModule{
						ServiceModule: serviceAndImage.ServiceModule,
						Image:         serviceAndImage.Image,
					}
					serviceModules = append(serviceModules, serviceModule)
				}

				workflowNotifyJobTaskSpec := &webhooknotify.WorkflowNotifyJobTaskDeploySpec{
					Env:            jobSpec.Env,
					ServiceName:    jobSpec.ServiceName,
					ServiceModules: serviceModules,
				}
				workflowNotifyJob.Spec = workflowNotifyJobTaskSpec
			case string(config.JobZadigHelmDeploy):
				jobSpec := &models.JobTaskHelmDeploySpec{}
				models.IToi(job.Spec, jobSpec)
				jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextEnvironment\"}}**：%s  \n", jobSpec.Env)
				mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextEnvironment\"}}：%s \n", jobSpec.Env)

				if job.Status == config.StatusPassed && len(jobSpec.ImageAndModules) > 0 {
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextImageInfo\"}}**：  \n")
					mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextImageInfo\"}}：  \n")
				}

				serviceModules := []*webhooknotify.WorkflowNotifyDeployServiceModule{}
				for _, serviceAndImage := range jobSpec.ImageAndModules {
					if !strings.HasPrefix(serviceAndImage.Image, "{{.") && !strings.Contains(serviceAndImage.Image, "}}") {
						jobTplcontent += fmt.Sprintf("%s  \n", serviceAndImage.Image)
						mailJobTplcontent += fmt.Sprintf("%s  \n", serviceAndImage.Image)
					}

					serviceModule := &webhooknotify.WorkflowNotifyDeployServiceModule{
						ServiceModule: serviceAndImage.ServiceModule,
						Image:         serviceAndImage.Image,
					}
					serviceModules = append(serviceModules, serviceModule)
				}

				workflowNotifyJobTaskSpec := &webhooknotify.WorkflowNotifyJobTaskDeploySpec{
					Env:            jobSpec.Env,
					ServiceName:    jobSpec.ServiceName,
					ServiceModules: serviceModules,
				}
				workflowNotifyJob.Spec = workflowNotifyJobTaskSpec
			case string(config.JobZadigTesting):
				testResult, err := genTestResultText(task.WorkflowName, job.Name, task.TaskID, language)
				if err != nil {
					log.Errorf("genTestResultText err:%s", err)
					return "", "", nil, nil, fmt.Errorf("genTestResultText err:%s", err)
				}

				jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextTestResult\"}}**: %s  \n", testResult)
				mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextTestResult\"}}: %s \n", testResult)
			case string(config.JobZadigScanning):
				jobSpec := &models.JobTaskFreestyleSpec{}
				models.IToi(job.Spec, jobSpec)
				sonarMetricsText, mailSonarMetricsText, err := genSonartMetricsText(jobSpec, language)
				if err != nil {
					log.Errorf("genTestResultText err:%s", err)
					return "", "", nil, nil, fmt.Errorf("genTestResultText err:%s", err)
				}

				if sonarMetricsText != "" {
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextSonarMetrics\"}}**: %s  \n", sonarMetricsText)
					mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextSonarMetrics\"}}: %s \n", mailSonarMetricsText)
				}
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

			workflowNotifyStage.Jobs = append(workflowNotifyStage.Jobs, workflowNotifyJob)
		}
		workflowNotifyStages = append(workflowNotifyStages, workflowNotifyStage)
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
		tplcontent = tplcontent + getNotifyAtContent(notify)
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
	Task               *models.WorkflowTask      `json:"task"`
	ProjectDisplayName string                    `json:"project_display_name"`
	EncodedDisplayName string                    `json:"encoded_display_name"`
	BaseURI            string                    `json:"base_uri"`
	WebHookType        setting.NotifyWebHookType `json:"web_hook_type"`
	TotalTime          int64                     `json:"total_time"`
	ScanningID         string                    `json:"scanning_id"`
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
			if status == config.StatusPassed || status == config.StatusCreated {
				return textColorGreen
			} else {
				return textColorRed
			}
		},
		"taskStatus": func(status config.Status) string {
			if status == config.StatusPassed {
				return getText("taskStatusSuccess", language)
			} else if status == config.StatusCancelled {
				return getText("taskStatusCancelled", language)
			} else if status == config.StatusTimeout {
				return getText("taskStatusTimeout", language)
			} else if status == config.StatusReject {
				return getText("taskStatusRejected", language)
			} else if status == config.StatusCreated {
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
			if status == config.StatusPassed || status == config.StatusCreated {
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
			} else if status == config.StatusCreated {
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
		err := webhookclient.SendWorkflowWebhook(webhookNotify)
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
			return fmt.Errorf("failed to send notification by lark app: failed to create lark client appID: %s, error: %s", notify.LarkGroupNotificationConfig.AppID, err)
		}

		messageContent, err := json.Marshal(card)
		if err != nil {
			return fmt.Errorf("failed to send notification by lark app: failed to parse the lark card, error: %s", err)
		}

		respErr := new(multierror.Error)
		for _, target := range notify.LarkPersonNotificationConfig.TargetUsers {
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
