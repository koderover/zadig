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
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/task"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

const (
	msgType    = "markdown"
	singleInfo = "single"
	multiInfo  = "multi"
)

type BranchTagType string

const (
	BranchTagTypeBranch      BranchTagType = "Branch"
	BranchTagTypeTag         BranchTagType = "Tag"
	CommitMsgInterceptLength               = 60
)

type Service struct {
	proxyColl          *mongodb.ProxyColl
	workflowColl       *mongodb.WorkflowColl
	pipelineColl       *mongodb.PipelineColl
	testingColl        *mongodb.TestingColl
	testTaskStatColl   *mongodb.TestTaskStatColl
	workflowV4Coll     *mongodb.WorkflowV4Coll
	workflowTaskV4Coll *mongodb.WorkflowTaskv4Coll
	scanningColl       *mongodb.ScanningColl
}

func NewWeChatClient() *Service {
	return &Service{
		proxyColl:          mongodb.NewProxyColl(),
		workflowColl:       mongodb.NewWorkflowColl(),
		pipelineColl:       mongodb.NewPipelineColl(),
		testingColl:        mongodb.NewTestingColl(),
		testTaskStatColl:   mongodb.NewTestTaskStatColl(),
		workflowV4Coll:     mongodb.NewWorkflowV4Coll(),
		workflowTaskV4Coll: mongodb.NewworkflowTaskv4Coll(),
		scanningColl:       mongodb.NewScanningColl(),
	}
}

type wechatNotification struct {
	Task               *task.Task                `json:"task"`
	EncodedDisplayName string                    `json:"encoded_display_name"`
	BaseURI            string                    `json:"base_uri"`
	IsSingle           bool                      `json:"is_single"`
	WebHookType        setting.NotifyWebHookType `json:"web_hook_type"`
	TotalTime          int64                     `json:"total_time"`
	AtMobiles          []string                  `json:"atMobiles"`
	IsAtAll            bool                      `json:"is_at_all"`
}

func (w *Service) SendMessageRequest(uri string, message interface{}) ([]byte, error) {
	c := httpclient.New()

	// 使用代理
	proxies, _ := w.proxyColl.List(&mongodb.ProxyArgs{})
	if len(proxies) != 0 && proxies[0].EnableApplicationProxy {
		c.SetProxy(proxies[0].GetProxyURL())
		fmt.Printf("send message is using proxy:%s\n", proxies[0].GetProxyURL())
	}

	res, err := c.Post(uri, httpclient.SetBody(message))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

// @note pipeline notification, deprecated
func (w *Service) SendInstantMessage(task *task.Task, testTaskStatusChanged, scanningTaskStatusChanged bool) error {
	var notifyCtls []*models.NotifyCtl
	var desc, scanningName, scanningID string
	switch task.Type {
	case config.SingleType:
		resp, err := w.pipelineColl.Find(&mongodb.PipelineFindOption{Name: task.PipelineName})
		if err != nil {
			log.Errorf("failed to find Pipeline,err: %s", err)
			return err
		}
		notifyCtls = resp.NotifyCtls

	case config.WorkflowType:
		resp, err := w.workflowColl.Find(task.PipelineName)
		if err != nil {
			log.Errorf("failed to find Workflow,err: %s", err)
			return err
		}
		notifyCtls = resp.NotifyCtls

	case config.TestType:
		resp, err := w.testingColl.Find(strings.TrimSuffix(task.PipelineName, "-job"), task.ProductName)
		if err != nil {
			log.Errorf("failed to find Testing,err: %s", err)
			return err
		}
		notifyCtls = resp.NotifyCtls
		desc = resp.Desc

	case config.ScanningType:
		scanningJobName := strings.TrimSuffix(task.PipelineName, "-scanning-job")
		if lastIndex := strings.LastIndex(scanningJobName, "-"); lastIndex != -1 && lastIndex < len(scanningJobName)-1 {
			scanningID = scanningJobName[lastIndex+1:]
			resp, err := w.scanningColl.GetByID(scanningID)
			if err != nil {
				log.Errorf("failed to find Scanning %s, err: %v", scanningID, err)
				return err
			}
			if resp.AdvancedSetting != nil {
				notifyCtls = resp.AdvancedSetting.NotifyCtls
				desc = resp.Description
				scanningName = strings.TrimSuffix(task.PipelineName, "-"+scanningID+"-scanning-job")
			}
		} else {
			log.Errorf("invalid scanning name: %s", scanningJobName)
			return fmt.Errorf("invalid scanning name: %s", scanningJobName)
		}
	default:
		log.Errorf("task type is not supported!")
		return nil
	}

	for _, notifyCtl := range notifyCtls {
		if err := w.sendMessage(task, notifyCtl, testTaskStatusChanged, scanningTaskStatusChanged, desc, scanningName, scanningID); err != nil {
			log.Errorf("send %s message err: %s", notifyCtl.WebHookType, err)
			continue
		}
	}
	return nil
}

func (w *Service) sendMessage(task *task.Task, notifyCtl *models.NotifyCtl, testTaskStatusChanged, scanningTaskStatusChanged bool, desc, scanningName, scanningID string) error {
	if notifyCtl == nil {
		return nil
	}
	var (
		uri         = ""
		content     = ""
		webHookType setting.NotifyWebHookType
		atMobiles   []string
		isAtAll     bool
		title       = ""
		larkCard    *LarkCard
		err         error
	)
	switch task.Type {
	case config.SingleType:
		if notifyCtl.Enabled && sets.NewString(notifyCtl.NotifyTypes...).Has(string(task.Status)) {
			webHookType = notifyCtl.WebHookType
			if webHookType == setting.NotifyWebHookTypeDingDing {
				uri = notifyCtl.DingDingWebHook
				atMobiles = notifyCtl.AtMobiles
				isAtAll = notifyCtl.IsAtAll
			} else if webHookType == setting.NotifyWebHookTypeFeishu {
				uri = notifyCtl.FeiShuWebHook
			} else {
				uri = notifyCtl.WeChatWebHook
			}
			content, err = w.createNotifyBody(&wechatNotification{
				Task:        task,
				BaseURI:     configbase.SystemAddress(),
				IsSingle:    true,
				WebHookType: webHookType,
				TotalTime:   time.Now().Unix() - task.StartTime,
				AtMobiles:   atMobiles,
				IsAtAll:     isAtAll,
			})
			if err != nil {
				log.Errorf("pipeline createNotifyBody err :%s", err)
				return err
			}
		}
	case config.WorkflowType:
		if notifyCtl.Enabled && sets.NewString(notifyCtl.NotifyTypes...).Has(string(task.Status)) {
			webHookType = notifyCtl.WebHookType
			if webHookType == setting.NotifyWebHookTypeDingDing {
				uri = notifyCtl.DingDingWebHook
				atMobiles = notifyCtl.AtMobiles
				isAtAll = notifyCtl.IsAtAll
			} else if webHookType == setting.NotifyWebHookTypeFeishu {
				uri = notifyCtl.FeiShuWebHook
			} else {
				uri = notifyCtl.WeChatWebHook
			}
			title, content, larkCard, err = w.createNotifyBodyOfWorkflowIM(&wechatNotification{
				Task:        task,
				BaseURI:     configbase.SystemAddress(),
				IsSingle:    false,
				WebHookType: webHookType,
				TotalTime:   time.Now().Unix() - task.StartTime,
				AtMobiles:   atMobiles,
				IsAtAll:     isAtAll,
			}, notifyCtl)
			if err != nil {
				log.Errorf("workflow createNotifyBodyOfWorkflowIM err :%s", err)
				return err
			}
		}
	case config.TestType:
		statusSets := sets.NewString(notifyCtl.NotifyTypes...)
		if notifyCtl.Enabled && (statusSets.Has(string(task.Status)) || (testTaskStatusChanged && statusSets.Has(string(config.StatusChanged)))) {
			webHookType = notifyCtl.WebHookType
			if webHookType == setting.NotifyWebHookTypeDingDing {
				uri = notifyCtl.DingDingWebHook
				atMobiles = notifyCtl.AtMobiles
				isAtAll = notifyCtl.IsAtAll
			} else if webHookType == setting.NotifyWebHookTypeFeishu {
				uri = notifyCtl.FeiShuWebHook
			} else {
				uri = notifyCtl.WeChatWebHook
			}
			title, content, larkCard, err = w.createNotifyBodyOfTestIM(desc, &wechatNotification{
				Task:        task,
				BaseURI:     configbase.SystemAddress(),
				IsSingle:    false,
				WebHookType: webHookType,
				TotalTime:   time.Now().Unix() - task.StartTime,
				AtMobiles:   atMobiles,
				IsAtAll:     isAtAll,
			}, notifyCtl)
			if err != nil {
				log.Errorf("testing createNotifyBodyOfTestIM err :%s", err)
				return err
			}
		}
	case config.ScanningType:
		statusSets := sets.NewString(notifyCtl.NotifyTypes...)
		if notifyCtl.Enabled && (statusSets.Has(string(task.Status)) || (scanningTaskStatusChanged && statusSets.Has(string(config.StatusChanged)))) {
			webHookType = notifyCtl.WebHookType
			if webHookType == setting.NotifyWebHookTypeDingDing {
				uri = notifyCtl.DingDingWebHook
				atMobiles = notifyCtl.AtMobiles
				isAtAll = notifyCtl.IsAtAll
			} else if webHookType == setting.NotifyWebHookTypeFeishu {
				uri = notifyCtl.FeiShuWebHook
			} else {
				uri = notifyCtl.WeChatWebHook
			}
			title, content, larkCard, err = w.createNotifyBodyOfScanningIM(desc, scanningName, scanningID, &wechatNotification{
				Task:        task,
				BaseURI:     configbase.SystemAddress(),
				IsSingle:    false,
				WebHookType: webHookType,
				TotalTime:   time.Now().Unix() - task.StartTime,
				AtMobiles:   atMobiles,
				IsAtAll:     isAtAll,
			}, notifyCtl)
			if err != nil {
				log.Errorf("testing createNotifyBodyOfTestIM err :%s", err)
				return err
			}
		}
	}

	if uri != "" && (content != "" || larkCard != nil) {
		if webHookType == setting.NotifyWebHookTypeDingDing {
			if task.Type == config.SingleType {
				title = "工作流状态"
			}
			content = fmt.Sprintf("%s\n%s", content, getNotifyAtContent(notifyCtl))
			workflowDetailURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/multi/%s/%d?display_name=%s", configbase.SystemAddress(), task.ProductName, task.PipelineName, task.TaskID, url.PathEscape(task.PipelineDisplayName))
			err := w.sendDingDingMessage(uri, title, content, workflowDetailURL, atMobiles, isAtAll)
			if err != nil {
				log.Errorf("sendDingDingMessage err : %s", err)
				return err
			}
		} else if webHookType == setting.NotifyWebHookTypeFeishu {
			if task.Type == config.SingleType {
				err := w.sendFeishuMessageOfSingleType("工作流状态", uri, content)
				if err != nil {
					log.Errorf("sendFeishuMessageOfSingleType Request err : %s", err)
					return err
				}
				return nil
			}

			err := w.sendFeishuMessage(uri, larkCard)
			if err != nil {
				log.Errorf("SendFeiShuMessageRequest err : %s", err)
				return err
			}
			if err := w.sendFeishuMessageOfSingleType("", notifyCtl.FeiShuWebHook, getNotifyAtContent(notifyCtl)); err != nil {
				log.Errorf("SendFeiShu @ message err : %s", err)
				return err
			}
		} else if webHookType == setting.NotifyWebhookTypeFeishuApp {
			client, err := lark.GetLarkClientByIMAppID(notifyCtl.FeiShuAppID)
			if err != nil {
				log.Errorf("send feishu message by application err: %s", err)
				return err
			}

			messageContent, err := json.Marshal(larkCard)
			if err != nil {
				log.Errorf("send feishu message by application err: [marshal content err]: %s", err)
				return err
			}

			err = client.SendMessage(LarkReceiverTypeChat, LarkMessageTypeCard, notifyCtl.FeishuChat.ChatID, string(messageContent))

			atMessage := getNotifyAtContent(notifyCtl)

			larkAtMessage := &FeiShuMessage{
				Text: atMessage,
			}

			atMessageContent, err := json.Marshal(larkAtMessage)
			if err != nil {
				return err
			}

			err = client.SendMessage(LarkReceiverTypeChat, LarkMessageTypeText, notifyCtl.FeishuChat.ChatID, string(atMessageContent))

			if err != nil {
				return err
			}

		} else {
			typeText := WeChatTextTypeTemplateCard
			if task.Type == config.SingleType {
				typeText = weChatTextTypeText
			}
			workflowDetailURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/multi/%s/%d?display_name=%s", configbase.SystemAddress(), task.ProductName, task.PipelineName, task.TaskID, url.PathEscape(task.PipelineDisplayName))
			err := w.SendWeChatWorkMessage(typeText, uri, workflowDetailURL, title, content)
			if err != nil {
				log.Errorf("SendWeChatWorkMessage err : %s", err)
				return err
			}
		}
	}
	return nil
}

func (w *Service) createNotifyBody(weChatNotification *wechatNotification) (content string, err error) {
	tmplSource := "{{if eq .WebHookType \"feishu\"}}触发的工作流: {{.BaseURI}}/v1/projects/detail/{{.Task.ProductName}}/pipelines/{{ isSingle .IsSingle }}/{{.Task.PipelineName}}/{{.Task.TaskID}}{{else}}#### 触发的工作流: [{{.Task.PipelineName}}#{{.Task.TaskID}}]({{.BaseURI}}/v1/projects/detail/{{.Task.ProductName}}/pipelines/{{ isSingle .IsSingle }}/{{.Task.PipelineName}}/{{.Task.TaskID}}){{end}} \n" +
		"- 状态: {{if eq .WebHookType \"feishu\"}}{{.Task.Status}}{{else}}<font color=\"{{ getColor .Task.Status }}\">{{.Task.Status}}</font>{{end}} \n" +
		"- 创建人：{{.Task.TaskCreator}} \n" +
		"- 总运行时长：{{ .TotalTime}} 秒 \n"

	testNames := getHTMLTestReport(weChatNotification.Task)
	if len(testNames) != 0 {
		tmplSource += "- 测试报告：\n"
	}

	for _, testName := range testNames {
		url := fmt.Sprintf("{{.BaseURI}}/api/aslan/testing/report?pipelineName={{.Task.PipelineName}}&pipelineType={{.Task.Type}}&taskID={{.Task.TaskID}}&testName=%s\n", testName)
		if weChatNotification.WebHookType == setting.NotifyWebHookTypeFeishu {
			tmplSource += url
			continue
		}
		tmplSource += fmt.Sprintf("[%s](%s)\n", url, url)
	}

	if setting.NotifyWebHookType(weChatNotification.WebHookType) == setting.NotifyWebHookTypeDingDing {
		if len(weChatNotification.AtMobiles) > 0 && !weChatNotification.IsAtAll {
			tmplSource = fmt.Sprintf("%s - 相关人员：@%s \n", tmplSource, strings.Join(weChatNotification.AtMobiles, "@"))
		}
	}

	tplcontent, err := getTplExec(tmplSource, weChatNotification)
	return tplcontent, err
}

// @note pipeline notification, deprecated
func (w *Service) createNotifyBodyOfWorkflowIM(weChatNotification *wechatNotification, notify *models.NotifyCtl) (string, string, *LarkCard, error) {
	weChatNotification.EncodedDisplayName = url.PathEscape(weChatNotification.Task.PipelineDisplayName)
	tplTitle := "{{if and (ne .WebHookType \"feishu\") (ne .WebHookType \"wechat\")}}#### {{end}}{{getIcon .Task.Status }}工作流 {{.Task.PipelineDisplayName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }} \n"
	tplBaseInfo := []string{"{{if eq .WebHookType \"dingding\"}}##### {{end}}{{if ne .WebHookType \"wechat\"}}**{{end}}执行用户{{if ne .WebHookType \"wechat\"}}**{{end}}：{{.Task.TaskCreator}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}{{if ne .WebHookType \"wechat\"}}**{{end}}环境信息{{if ne .WebHookType \"wechat\"}}**{{end}}：{{.Task.WorkflowArgs.Namespace}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}{{if ne .WebHookType \"wechat\"}}**{{end}}项目名称{{if ne .WebHookType \"wechat\"}}**{{end}}：{{.Task.ProductName}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}{{if ne .WebHookType \"wechat\"}}**{{end}}开始时间{{if ne .WebHookType \"wechat\"}}**{{end}}：{{ getStartTime .Task.StartTime}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}{{if ne .WebHookType \"wechat\"}}**{{end}}持续时间{{if ne .WebHookType \"wechat\"}}**{{end}}：{{ getDuration .TotalTime}} \n",
	}

	build := []string{}
	test := ""
	for _, subStage := range weChatNotification.Task.Stages {
		switch subStage.TaskType {
		case config.TaskBuild:
			for _, sb := range subStage.SubTasks {
				buildElemTemp := ""
				buildSt, err := base.ToBuildTask(sb)
				if err != nil {
					return "", "", nil, err
				}
				var prInfo string
				var prInfoList []string
				branchTag, commitID, commitMsg, gitCommitURL := "", "", "", ""
				for idx, buildRepo := range buildSt.JobCtx.Builds {
					if idx == 0 || buildRepo.IsPrimary {
						branchTag = buildRepo.Branch
						if buildRepo.Tag != "" {
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
						gitCommitURL = fmt.Sprintf("%s/%s/%s/commit/%s", buildRepo.Address, buildRepo.RepoOwner, buildRepo.RepoName, commitID)
						prInfoList = []string{}
						sort.Ints(buildRepo.PRs)
						for _, id := range buildRepo.PRs {
							link := prLinkBuilder(buildRepo.Address, buildRepo.RepoOwner, buildRepo.RepoName, id)
							if link != "" {
								prInfoList = append(prInfoList, fmt.Sprintf("[#%d](%s)", id, link))
							}
						}
					}
				}
				if len(prInfoList) != 0 {
					// need an extra space at the end
					prInfo = strings.Join(prInfoList, " ") + " "
				}

				if buildSt.BuildStatus.Status == "" {
					buildSt.BuildStatus.Status = config.StatusNotRun
				}
				buildElemTemp += fmt.Sprintf("\n\n{{if eq .WebHookType \"dingding\"}}---\n\n##### {{end}}**服务名称**：%s \n", buildSt.Service)
				if !(buildSt.ServiceType == setting.PMDeployType &&
					(buildSt.JobCtx.FileArchiveCtx != nil || buildSt.JobCtx.DockerBuildCtx != nil)) {
					buildElemTemp += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**镜像信息**：%s \n", buildSt.JobCtx.Image)
				}
				buildElemTemp += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**代码信息**：%s %s[%s](%s) \n", branchTag, prInfo, commitID, gitCommitURL)
				buildElemTemp += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**提交信息**：%s \n", commitMsg)
				build = append(build, buildElemTemp)
			}

		case config.TaskJenkinsBuild:
			for _, sb := range subStage.SubTasks {
				buildElemTemp := ""
				buildSt, err := base.ToJenkinsBuildTask(sb)
				if err != nil {
					return "", "", nil, err
				}
				buildElemTemp += fmt.Sprintf("\n\n{{if eq .WebHookType \"dingding\"}}---\n\n##### {{end}}**服务名称**：%s \n", buildSt.Service)
				buildElemTemp += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**镜像信息**：%s \n", buildSt.Image)
				build = append(build, buildElemTemp)
			}

		case config.TaskTestingV2:
			test = "{{if eq .WebHookType \"dingding\"}}##### {{end}}**测试结果** \n"
			for _, sb := range subStage.SubTasks {
				test = genTestCaseText(test, sb, weChatNotification.Task.TestReports)
			}
		}
	}

	buttonContent := "点击查看更多信息"
	workflowDetailURL := "{{.BaseURI}}/v1/projects/detail/{{.Task.ProductName}}/pipelines/{{ isSingle .IsSingle }}/{{.Task.PipelineName}}/{{.Task.TaskID}}?display_name={{.EncodedDisplayName}}"
	//moreInformation := fmt.Sprintf("[%s](%s)", buttonContent, workflowDetailURL)
	tplTitle, _ = getTplExec(tplTitle, weChatNotification)

	if weChatNotification.WebHookType != setting.NotifyWebHookTypeFeishu {
		tplcontent := strings.Join(tplBaseInfo, "")
		tplcontent += strings.Join(build, "")
		tplcontent = fmt.Sprintf("%s%s", tplcontent, test)
		tplcontent = tplcontent + getNotifyAtContent(notify)
		if weChatNotification.WebHookType != setting.NotifyWebHookTypeWechatWork {
			tplcontent = fmt.Sprintf("%s%s", tplTitle, tplcontent)
		}
		tplExecContent, _ := getTplExec(tplcontent, weChatNotification)
		return tplTitle, tplExecContent, nil, nil
	}

	lc := NewLarkCard()
	lc.SetConfig(true)
	lc.SetHeader(getColorTemplateWithStatus(weChatNotification.Task.Status), tplTitle, feiShuTagText)
	for idx, feildContent := range tplBaseInfo {
		feildExecContent, _ := getTplExec(feildContent, weChatNotification)
		lc.AddI18NElementsZhcnFeild(feildExecContent, idx == 0)
	}
	for _, feildContent := range build {
		feildExecContent, _ := getTplExec(feildContent, weChatNotification)
		lc.AddI18NElementsZhcnFeild(feildExecContent, true)
	}
	if test != "" {
		test, _ = getTplExec(test, weChatNotification)
		lc.AddI18NElementsZhcnFeild(test, true)
	}
	workflowDetailURL, _ = getTplExec(workflowDetailURL, weChatNotification)
	lc.AddI18NElementsZhcnAction(buttonContent, workflowDetailURL)
	return "", "", lc, nil
}

func (w *Service) createNotifyBodyOfTestIM(desc string, weChatNotification *wechatNotification, notify *models.NotifyCtl) (string, string, *LarkCard, error) {

	tplTitle := "{{if and (ne .WebHookType \"feishu\") (ne .WebHookType \"wechat\")}}#### {{end}}{{getIcon .Task.Status }}{{if eq .WebHookType \"wechat\"}}<font color=\"{{ getColor .Task.Status }}\">工作流{{.Task.PipelineName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }}</font>{{else}}工作流 {{.Task.PipelineName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }}{{end}} \n"
	tplBaseInfo := []string{"{{if eq .WebHookType \"dingding\"}}##### {{end}}**执行用户**：{{.Task.TaskCreator}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**项目名称**：{{.Task.ProductName}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**持续时间**：{{ getDuration .TotalTime}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**开始时间**：{{ getStartTime .Task.StartTime}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**测试描述**：" + desc + " \n",
	}

	tplTestCaseInfo := "{{if eq .WebHookType \"dingding\"}}##### {{end}}**测试结果** \n"
	for _, stage := range weChatNotification.Task.Stages {
		if stage.TaskType != config.TaskTestingV2 {
			continue
		}
		for _, subTask := range stage.SubTasks {
			tplTestCaseInfo = genTestCaseText(tplTestCaseInfo, subTask, weChatNotification.Task.TestReports)
		}
	}

	buttonContent := "点击查看更多信息"
	workflowDetailURL := "{{.BaseURI}}/v1/projects/detail/{{.Task.ProductName}}/test/detail/function/{{.Task.PipelineName}}/{{.Task.TaskID}}"
	moreInformation := fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}[%s](%s)", buttonContent, workflowDetailURL)

	tplTitle, _ = getTplExec(tplTitle, weChatNotification)

	if weChatNotification.WebHookType != setting.NotifyWebHookTypeFeishu {
		tplcontent := strings.Join(tplBaseInfo, "")
		tplcontent = fmt.Sprintf("%s%s", tplcontent, tplTestCaseInfo)
		tplcontent = tplcontent + getNotifyAtContent(notify)
		tplcontent = fmt.Sprintf("%s%s%s", tplTitle, tplcontent, moreInformation)
		tplExecContent, _ := getTplExec(tplcontent, weChatNotification)
		return tplTitle, tplExecContent, nil, nil
	}
	lc := NewLarkCard()
	lc.SetConfig(true)
	lc.SetHeader(getColorTemplateWithStatus(weChatNotification.Task.Status), tplTitle, feiShuTagText)
	for idx, feildContent := range tplBaseInfo {
		feildExecContent, _ := getTplExec(feildContent, weChatNotification)
		lc.AddI18NElementsZhcnFeild(feildExecContent, idx == 0)
	}
	if tplTestCaseInfo != "" {
		tplTestCaseInfo, _ = getTplExec(tplTestCaseInfo, weChatNotification)
		lc.AddI18NElementsZhcnFeild(tplTestCaseInfo, true)
	}
	workflowDetailURL, _ = getTplExec(workflowDetailURL, weChatNotification)
	lc.AddI18NElementsZhcnAction(buttonContent, workflowDetailURL)

	return "", "", lc, nil
}

func (w *Service) createNotifyBodyOfScanningIM(desc, scanningName, scanningID string, weChatNotification *wechatNotification, notify *models.NotifyCtl) (string, string, *LarkCard, error) {
	tplTitle := "{{if ne .WebHookType \"feishu\"}}#### {{end}}{{getIcon .Task.Status }}{{if eq .WebHookType \"wechat\"}}<font color=\"{{ getColor .Task.Status }}\">代码扫描 " + scanningName + " #{{.Task.TaskID}} {{ taskStatus .Task.Status }}</font>{{else}}代码扫描 " + scanningName + " #{{.Task.TaskID}} {{ taskStatus .Task.Status }}{{end}} \n"
	tplBaseInfo := []string{"{{if eq .WebHookType \"dingding\"}}##### {{end}}**执行用户**：{{.Task.TaskCreator}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**项目名称**：{{.Task.ProductName}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**持续时间**：{{ getDuration .TotalTime}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**开始时间**：{{ getStartTime .Task.StartTime}} \n",
		"{{if eq .WebHookType \"dingding\"}}##### {{end}}**代码扫描描述**：" + desc + " \n",
	}

	tplTestCaseInfo := "{{if eq .WebHookType \"dingding\"}}##### {{end}}**代码扫描结果**: "
	for _, stage := range weChatNotification.Task.Stages {
		if stage.TaskType != config.TaskScanning {
			continue
		}
		if stage.Status == "" {
			tplTestCaseInfo = ""
			break
		}
		tplTestCaseInfo += string(stage.Status) + "\n"
	}

	buttonContent := "点击查看更多信息"
	workflowDetailURL := "{{.BaseURI}}/v1/projects/detail/{{.Task.ProductName}}/scanner/detail/" + scanningName + "/task/{{.Task.TaskID}}?id=" + scanningID
	moreInformation := fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}[%s](%s)", buttonContent, workflowDetailURL)

	tplTitle, _ = getTplExec(tplTitle, weChatNotification)

	if weChatNotification.WebHookType != setting.NotifyWebHookTypeFeishu {
		tplcontent := strings.Join(tplBaseInfo, "")
		tplcontent = fmt.Sprintf("%s%s", tplcontent, tplTestCaseInfo)
		tplcontent = tplcontent + getNotifyAtContent(notify)
		tplcontent = fmt.Sprintf("%s%s%s", tplTitle, tplcontent, moreInformation)
		tplExecContent, _ := getTplExec(tplcontent, weChatNotification)
		return tplTitle, tplExecContent, nil, nil
	}
	lc := NewLarkCard()
	lc.SetConfig(true)
	lc.SetHeader(getColorTemplateWithStatus(weChatNotification.Task.Status), tplTitle, feiShuTagText)
	for idx, feildContent := range tplBaseInfo {
		feildExecContent, _ := getTplExec(feildContent, weChatNotification)
		lc.AddI18NElementsZhcnFeild(feildExecContent, idx == 0)
	}
	if tplTestCaseInfo != "" {
		tplTestCaseInfo, _ = getTplExec(tplTestCaseInfo, weChatNotification)
		lc.AddI18NElementsZhcnFeild(tplTestCaseInfo, true)
	}
	workflowDetailURL, _ = getTplExec(workflowDetailURL, weChatNotification)
	lc.AddI18NElementsZhcnAction(buttonContent, workflowDetailURL)

	return "", "", lc, nil
}

func getHTMLTestReport(task *task.Task) []string {
	if task.Type != config.WorkflowType {
		return nil
	}

	testNames := make([]string, 0)
	for _, stage := range task.Stages {
		if stage.TaskType != config.TaskTestingV2 {
			continue
		}

		for testName, subTask := range stage.SubTasks {
			testInfo, err := base.ToTestingTask(subTask)
			if err != nil {
				log.Errorf("parse testInfo failed, err:%s", err)
				continue
			}

			if testInfo.JobCtx.TestType == setting.FunctionTest && testInfo.JobCtx.TestReportPath != "" {
				testNames = append(testNames, testName)
			}
		}
	}

	return testNames
}

func getTplExec(tplcontent string, weChatNotification *wechatNotification) (string, error) {
	tmpl := template.Must(template.New("notify").Funcs(template.FuncMap{
		"getColor": func(status config.Status) string {
			if status == config.StatusPassed || status == config.StatusCreated {
				return markdownColorInfo
			} else if status == config.StatusTimeout || status == config.StatusCancelled {
				return markdownColorComment
			} else if status == config.StatusFailed {
				return markdownColorWarning
			}
			return markdownColorComment
		},
		"isSingle": func(isSingle bool) string {
			if isSingle {
				return singleInfo
			}
			return multiInfo
		},
		"taskStatus": func(status config.Status) string {
			if status == config.StatusPassed {
				return "执行成功"
			} else if status == config.StatusCancelled {
				return "执行取消"
			} else if status == config.StatusTimeout {
				return "执行超时"
			} else if status == config.StatusCreated {
				return "开始执行"
			}
			return "执行失败"
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
	}).Parse(tplcontent))

	buffer := bytes.NewBufferString("")
	if err := tmpl.Execute(buffer, &weChatNotification); err != nil {
		log.Errorf("getTplExec Execute err:%s", err)
		return "", fmt.Errorf("getTplExec Execute err:%s", err)

	}
	return buffer.String(), nil
}

func checkTestReportsExist(testModuleName string, testReports map[string]interface{}) bool {
	for testname := range testReports {
		if testname == testModuleName {
			return true
		}
	}
	return false
}

func genTestCaseText(test string, subTask, testReports map[string]interface{}) string {
	testSt, err := base.ToTestingTask(subTask)
	if err != nil {
		log.Errorf("parse testInfo failed, err:%s", err)
		return test
	}
	if testSt.TaskStatus == "" {
		testSt.TaskStatus = config.StatusNotRun
	}
	if testSt.JobCtx.TestType == setting.FunctionTest && testSt.JobCtx.TestReportPath != "" && testSt.TaskStatus == config.StatusPassed {
		url := fmt.Sprintf("{{.BaseURI}}/api/aslan/testing/report?pipelineName={{.Task.PipelineName}}&pipelineType={{.Task.Type}}&taskID={{.Task.TaskID}}&testName=%s", testSt.TestModuleName)
		test += fmt.Sprintf("{{if ne .WebHookType \"feishu\"}} - {{end}}[%s](%s): ", testSt.TestModuleName, url)
	} else {
		test += fmt.Sprintf("{{if ne .WebHookType \"feishu\"}} - {{end}}%s: ", testSt.TestModuleName)
	}
	if testReports == nil || !checkTestReportsExist(testSt.TestModuleName, testReports) {
		test += fmt.Sprintf("%s \n", testSt.TaskStatus)
		return test
	}

	for testname, report := range testReports {
		if testname != testSt.TestModuleName {
			continue
		}
		tr := &task.TestReport{}
		if task.IToi(report, tr) != nil {
			log.Errorf("parse TestReport failed, err:%s", err)
			continue
		}
		if tr.FunctionTestSuite == nil {
			test += fmt.Sprintf("%s \n", testSt.TaskStatus)
			continue
		}
		totalNum := tr.FunctionTestSuite.Tests + tr.FunctionTestSuite.Skips
		failedNum := tr.FunctionTestSuite.Failures + tr.FunctionTestSuite.Errors
		successNum := tr.FunctionTestSuite.Tests - failedNum
		test += fmt.Sprintf("%d(成功)%d(失败)%d(总数) \n", successNum, failedNum, totalNum)
	}
	return test
}

func getNotifyAtContent(notify *models.NotifyCtl) string {
	resp := ""
	if notify.WebHookType == setting.NotifyWebHookTypeDingDing {
		notify.DingDingNotificationConfig.AtMobiles = lo.Filter(notify.DingDingNotificationConfig.AtMobiles, func(s string, _ int) bool { return s != "All" })
		if len(notify.AtMobiles) > 0 {
			resp = fmt.Sprintf("##### **相关人员**: @%s \n", strings.Join(notify.DingDingNotificationConfig.AtMobiles, "@"))
		}
	}
	if notify.WebHookType == setting.NotifyWebHookTypeWechatWork && len(notify.WechatUserIDs) > 0 {
		atUserList := []string{}
		notify.WechatNotificationConfig.AtUsers = lo.Filter(notify.WechatNotificationConfig.AtUsers, func(s string, _ int) bool { return s != "All" })
		for _, userID := range notify.WechatUserIDs {
			atUserList = append(atUserList, fmt.Sprintf("<@%s>", userID))
		}
		resp = fmt.Sprintf("##### **相关人员**: %s \n", strings.Join(atUserList, " "))
	}
	if notify.WebHookType == setting.NotifyWebHookTypeFeishu {
		atUserList := []string{}
		notify.LarkHookNotificationConfig.AtUsers = lo.Filter(notify.LarkHookNotificationConfig.AtUsers, func(s string, _ int) bool { return s != "All" })
		for _, userID := range notify.LarkUserIDs {
			atUserList = append(atUserList, fmt.Sprintf("<at user_id=\"%s\"></at>", userID))
		}
		resp = strings.Join(atUserList, " ")
		if notify.LarkHookNotificationConfig.IsAtAll {
			resp += "<at user_id=\"all\"></at>"
		}
	}
	if notify.WebHookType == setting.NotifyWebhookTypeFeishuApp {
		atUserList := []string{}
		for _, userID := range notify.LarkGroupNotificationConfig.AtUsers {
			atUserList = append(atUserList, fmt.Sprintf("<at user_id=\"%s\"></at>", userID.ID))
		}
		msg := strings.Join(atUserList, " ")
		if notify.LarkHookNotificationConfig.IsAtAll {
			msg += "<at user_id=\"all\"></at>"
		}

		larkAtMessage := &FeiShuMessage{
			Text: msg,
		}

		atMessageContent, err := json.Marshal(larkAtMessage)
		if err != nil {
			log.Errorf("failed to generate lark at info, error: %s", err)
			return ""
		}
		return string(atMessageContent)
	}
	return resp
}
