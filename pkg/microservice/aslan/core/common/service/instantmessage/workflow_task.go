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
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/types/step"
)

func (w *Service) SendWorkflowTaskAproveNotifications(workflowName string, taskID int64) error {
	resp, err := w.workflowV4Coll.Find(workflowName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to find workflowv4, err: %s", err)
		log.Error(errMsg)
		return errors.New(errMsg)
	}
	task, err := w.workflowTaskV4Coll.Find(workflowName, taskID)
	if err != nil {
		errMsg := fmt.Sprintf("failed to find workflowv4 task, err: %s", err)
		log.Error(errMsg)
		return errors.New(errMsg)
	}
	for _, notify := range resp.NotifyCtls {
		statusSets := sets.NewString(notify.NotifyTypes...)
		if !statusSets.Has(string(config.StatusWaitingApprove)) {
			continue
		}
		if !notify.Enabled {
			continue
		}
		title, content, larkCard, err := w.getApproveNotificationContent(notify, task)
		if err != nil {
			errMsg := fmt.Sprintf("failed to get notification content, err: %s", err)
			log.Error(errMsg)
			return errors.New(errMsg)
		}
		if err := w.sendNotification(title, content, notify, larkCard); err != nil {
			log.Errorf("failed to send notification, err: %s", err)
		}
	}
	return nil
}

func (w *Service) SendWorkflowTaskNotifications(task *models.WorkflowTask) error {
	resp, err := w.workflowV4Coll.Find(task.WorkflowName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to find workflowv4, err: %s", err)
		log.Error(errMsg)
		return errors.New(errMsg)
	}
	if len(resp.NotifyCtls) == 0 {
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
	for _, notify := range resp.NotifyCtls {
		if !notify.Enabled {
			continue
		}
		statusSets := sets.NewString(notify.NotifyTypes...)
		if statusSets.Has(string(task.Status)) || (statusChanged && statusSets.Has(string(config.StatusChanged))) {
			title, content, larkCard, err := w.getNotificationContent(notify, task)
			if err != nil {
				errMsg := fmt.Sprintf("failed to get notification content, err: %s", err)
				log.Error(errMsg)
				return errors.New(errMsg)
			}
			if err := w.sendNotification(title, content, notify, larkCard); err != nil {
				log.Errorf("failed to send notification, err: %s", err)
			}
		}
	}
	return nil
}
func (w *Service) getApproveNotificationContent(notify *models.NotifyCtl, task *models.WorkflowTask) (string, string, *LarkCard, error) {
	workflowNotification := &workflowTaskNotification{
		Task:               task,
		EncodedDisplayName: url.PathEscape(task.WorkflowDisplayName),
		BaseURI:            configbase.SystemAddress(),
		WebHookType:        notify.WebHookType,
		TotalTime:          time.Now().Unix() - task.StartTime,
	}

	tplTitle := "{{if ne .WebHookType \"feishu\"}}#### {{end}}{{getIcon .Task.Status }}{{if eq .WebHookType \"wechat\"}}<font color=\"markdownColorInfo\">工作流{{.Task.WorkflowDisplayName}} #{{.Task.TaskID}} 等待审批</font>{{else}}工作流 {{.Task.WorkflowDisplayName}} #{{.Task.TaskID}} 等待审批{{end}} \n"
	tplBaseInfo := []string{"{{if eq .WebHookType \"dingding\"}}##### {{end}}**执行用户**：{{.Task.TaskCreator}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**项目名称**：{{.Task.ProjectName}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**开始时间**：{{ getStartTime .Task.StartTime}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**持续时间**：{{ getDuration .TotalTime}} \n",
	}
	title, err := getWorkflowTaskTplExec(tplTitle, workflowNotification)
	if err != nil {
		return "", "", nil, err
	}
	buttonContent := "点击查看更多信息"
	workflowDetailURL := "{{.BaseURI}}/v1/projects/detail/{{.Task.ProjectName}}/pipelines/custom/{{.Task.WorkflowName}}/{{.Task.TaskID}}?display_name={{.EncodedDisplayName}}"
	moreInformation := fmt.Sprintf("[%s](%s)", buttonContent, workflowDetailURL)
	if notify.WebHookType != feiShuType {
		tplcontent := strings.Join(tplBaseInfo, "")
		tplcontent = tplcontent + getNotifyAtContent(notify)
		tplcontent = fmt.Sprintf("%s%s%s", title, tplcontent, moreInformation)
		content, err := getWorkflowTaskTplExec(tplcontent, workflowNotification)
		if err != nil {
			return "", "", nil, err
		}
		return title, content, nil, nil
	}

	lc := NewLarkCard()
	lc.SetConfig(true)
	lc.SetHeader(feishuHeaderTemplateGreen, title, feiShuTagText)
	for idx, feildContent := range tplBaseInfo {
		feildExecContent, _ := getWorkflowTaskTplExec(feildContent, workflowNotification)
		lc.AddI18NElementsZhcnFeild(feildExecContent, idx == 0)
	}
	workflowDetailURL, _ = getWorkflowTaskTplExec(workflowDetailURL, workflowNotification)
	lc.AddI18NElementsZhcnAction(buttonContent, workflowDetailURL)
	return "", "", lc, nil
}

func (w *Service) getNotificationContent(notify *models.NotifyCtl, task *models.WorkflowTask) (string, string, *LarkCard, error) {
	workflowNotification := &workflowTaskNotification{
		Task:               task,
		EncodedDisplayName: url.PathEscape(task.WorkflowDisplayName),
		BaseURI:            configbase.SystemAddress(),
		WebHookType:        notify.WebHookType,
		TotalTime:          time.Now().Unix() - task.StartTime,
	}

	tplTitle := "{{if ne .WebHookType \"feishu\"}}#### {{end}}{{getIcon .Task.Status }}{{if eq .WebHookType \"wechat\"}}<font color=\"{{ getColor .Task.Status }}\">工作流{{.Task.WorkflowDisplayName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }}</font>{{else}}工作流 {{.Task.WorkflowDisplayName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }}{{end}} \n"
	tplBaseInfo := []string{"{{if eq .WebHookType \"dingding\"}}##### {{end}}**执行用户**：{{.Task.TaskCreator}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**项目名称**：{{.Task.ProjectName}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**开始时间**：{{ getStartTime .Task.StartTime}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**持续时间**：{{ getDuration .TotalTime}} \n",
	}
	jobContents := []string{}
	for _, stage := range task.Stages {
		for _, job := range stage.Jobs {
			jobTplcontent := "\n\n{{if eq .WebHookType \"dingding\"}}---\n\n##### {{end}}**{{jobType .Job.JobType }}**: {{.Job.Name}}    **状态**: {{taskStatus .Job.Status }} \n"
			switch job.JobType {
			case string(config.JobZadigBuild):
				fallthrough
			case string(config.JobFreestyle):
				jobSpec := &models.JobTaskFreestyleSpec{}
				models.IToi(job.Spec, jobSpec)
				repos := []*types.Repository{}
				for _, stepTask := range jobSpec.Steps {
					if stepTask.StepType == config.StepGit {
						stepSpec := &step.StepGitSpec{}
						models.IToi(stepTask.Spec, stepSpec)
						repos = stepSpec.Repos
					}
				}
				branchTag, branchTagType, commitID, commitMsg, gitCommitURL := "", BranchTagTypeBranch, "", "", ""
				for idx, buildRepo := range repos {
					if idx == 0 || buildRepo.IsPrimary {
						branchTag = buildRepo.Branch
						if buildRepo.Tag != "" {
							branchTagType = BranchTagTypeTag
							branchTag = buildRepo.Tag
						}
						if len(buildRepo.CommitID) > 8 {
							commitID = buildRepo.CommitID[0:8]
						}
						commitMsgs := strings.Split(buildRepo.CommitMessage, "\n")
						if len(commitMsgs) > 0 {
							commitMsg = commitMsgs[0]
						}
						if len(commitMsg) > CommitMsgInterceptLength {
							commitMsg = commitMsg[0:CommitMsgInterceptLength]
						}
						gitCommitURL = fmt.Sprintf("%s/%s/%s/commit/%s", buildRepo.Address, buildRepo.RepoOwner, buildRepo.RepoName, commitID)
					}
				}
				image := ""
				for _, env := range jobSpec.Properties.Envs {
					if env.Key == "IMAGE" {
						image = env.Value
					}
				}
				if len(commitID) > 0 {
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**代码信息**：[%s-%s %s](%s) \n", branchTagType, branchTag, commitID, gitCommitURL)
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**提交信息**：%s \n", commitMsg)
				}
				if image != "" {
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**镜像信息**：%s \n", image)
				}
			case string(config.JobZadigDeploy):
				jobSpec := &models.JobTaskDeploySpec{}
				models.IToi(job.Spec, jobSpec)
				jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**环境**：%s \n", jobSpec.Env)
			case string(config.JobZadigHelmDeploy):
				jobSpec := &models.JobTaskHelmDeploySpec{}
				models.IToi(job.Spec, jobSpec)
				jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**环境**：%s \n", jobSpec.Env)
			}
			jobNotifaication := &jobTaskNotification{
				Job:         job,
				WebHookType: notify.WebHookType,
			}

			jobContent, err := getJobTaskTplExec(jobTplcontent, jobNotifaication)
			if err != nil {
				return "", "", nil, err
			}
			jobContents = append(jobContents, jobContent)
		}
	}
	title, err := getWorkflowTaskTplExec(tplTitle, workflowNotification)
	if err != nil {
		return "", "", nil, err
	}
	buttonContent := "点击查看更多信息"
	workflowDetailURL := "{{.BaseURI}}/v1/projects/detail/{{.Task.ProjectName}}/pipelines/custom/{{.Task.WorkflowName}}/{{.Task.TaskID}}?display_name={{.EncodedDisplayName}}"
	moreInformation := fmt.Sprintf("\n\n{{if eq .WebHookType \"dingding\"}}---\n\n{{end}}[%s](%s)", buttonContent, workflowDetailURL)
	if notify.WebHookType != feiShuType {
		tplcontent := strings.Join(tplBaseInfo, "")
		tplcontent += strings.Join(jobContents, "")
		tplcontent = tplcontent + getNotifyAtContent(notify)
		tplcontent = fmt.Sprintf("%s%s%s", title, tplcontent, moreInformation)
		content, err := getWorkflowTaskTplExec(tplcontent, workflowNotification)
		if err != nil {
			return "", "", nil, err
		}
		return title, content, nil, nil
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
	workflowDetailURL, _ = getWorkflowTaskTplExec(workflowDetailURL, workflowNotification)
	lc.AddI18NElementsZhcnAction(buttonContent, workflowDetailURL)
	return "", "", lc, nil
}

type workflowTaskNotification struct {
	Task               *models.WorkflowTask `json:"task"`
	EncodedDisplayName string               `json:"encoded_display_name"`
	BaseURI            string               `json:"base_uri"`
	WebHookType        string               `json:"web_hook_type"`
	TotalTime          int64                `json:"total_time"`
}

func getWorkflowTaskTplExec(tplcontent string, args *workflowTaskNotification) (string, error) {
	tmpl := template.Must(template.New("notify").Funcs(template.FuncMap{
		"getColor": func(status config.Status) string {
			if status == config.StatusPassed {
				return markdownColorInfo
			} else if status == config.StatusTimeout || status == config.StatusCancelled {
				return markdownColorComment
			} else if status == config.StatusFailed {
				return markdownColorWarning
			}
			return markdownColorComment
		},
		"taskStatus": func(status config.Status) string {
			if status == config.StatusPassed {
				return "执行成功"
			} else if status == config.StatusCancelled {
				return "执行取消"
			} else if status == config.StatusTimeout {
				return "执行超时"
			} else if status == config.StatusReject {
				return "执行被拒绝"
			}
			return "执行失败"
		},
		"getIcon": func(status config.Status) string {
			if status == config.StatusPassed {
				return "👍"
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
	}).Parse(tplcontent))

	buffer := bytes.NewBufferString("")
	if err := tmpl.Execute(buffer, args); err != nil {
		log.Errorf("getTplExec Execute err:%s", err)
		return "", fmt.Errorf("getTplExec Execute err:%s", err)

	}
	return buffer.String(), nil
}

type jobTaskNotification struct {
	Job         *models.JobTask `json:"task"`
	WebHookType string          `json:"web_hook_type"`
}

func getJobTaskTplExec(tplcontent string, args *jobTaskNotification) (string, error) {
	tmpl := template.Must(template.New("notify").Funcs(template.FuncMap{
		"taskStatus": func(status config.Status) string {
			if status == config.StatusPassed {
				return "执行成功"
			} else if status == config.StatusCancelled {
				return "执行取消"
			} else if status == config.StatusTimeout {
				return "执行超时"
			} else if status == config.StatusReject {
				return "执行被拒绝"
			} else if status == "" {
				return "未执行"
			}
			return "执行失败"
		},
		"jobType": func(jobType string) string {
			if jobType == string(config.JobZadigBuild) {
				return "构建"
			} else if jobType == string(config.JobZadigDeploy) {
				return "部署"
			} else if jobType == string(config.JobZadigHelmDeploy) {
				return "helm部署"
			} else if jobType == string(config.JobCustomDeploy) {
				return "自定义部署"
			} else if jobType == string(config.JobFreestyle) {
				return "通用任务"
			} else if jobType == string(config.JobPlugin) {
				return "自定义任务"
			} else if jobType == string(config.JobZadigTesting) {
				return "测试"
			}
			return string(jobType)
		},
	}).Parse(tplcontent))

	buffer := bytes.NewBufferString("")
	if err := tmpl.Execute(buffer, args); err != nil {
		log.Errorf("getTplExec Execute err:%s", err)
		return "", fmt.Errorf("getTplExec Execute err:%s", err)

	}
	return buffer.String(), nil
}

func (w *Service) sendNotification(title, content string, notify *models.NotifyCtl, card *LarkCard) error {
	switch notify.WebHookType {
	case dingDingType:
		if err := w.sendDingDingMessage(notify.DingDingWebHook, title, content, notify.AtMobiles); err != nil {
			return err
		}
	case feiShuType:
		if err := w.sendFeishuMessage(notify.FeiShuWebHook, card); err != nil {
			return err
		}
		if err := w.sendFeishuMessageOfSingleType("", notify.FeiShuWebHook, getNotifyAtContent(notify)); err != nil {
			return err
		}
	default:
		if err := w.SendWeChatWorkMessage(weChatTextTypeMarkdown, notify.WeChatWebHook, content); err != nil {
			return err
		}
	}
	return nil
}
