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

package instantmessage

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

const (
	msgType    = "markdown"
	singleInfo = "single"
	multiInfo  = "multi"
)

type TextType string

type Service struct {
	proxyColl        *mongodb.ProxyColl
	workflowColl     *mongodb.WorkflowColl
	pipelineColl     *mongodb.PipelineColl
	testingColl      *mongodb.TestingColl
	testTaskStatColl *mongodb.TestTaskStatColl
}

func NewWeChatClient() *Service {
	return &Service{
		proxyColl:        mongodb.NewProxyColl(),
		workflowColl:     mongodb.NewWorkflowColl(),
		pipelineColl:     mongodb.NewPipelineColl(),
		testingColl:      mongodb.NewTestingColl(),
		testTaskStatColl: mongodb.NewTestTaskStatColl(),
	}
}

type wechatNotification struct {
	Task        *task.Task `json:"task"`
	BaseURI     string     `json:"base_uri"`
	IsSingle    bool       `json:"is_single"`
	WebHookType string     `json:"web_hook_type"`
	TotalTime   int64      `json:"total_time"`
	AtMobiles   []string   `json:"atMobiles"`
	IsAtAll     bool       `json:"is_at_all"`
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

func (w *Service) SendInstantMessage(task *task.Task, testTaskStatusChanged bool) error {
	var (
		uri           = ""
		content       = ""
		webHookType   = ""
		atMobiles     []string
		isAtAll       bool
		title         = ""
		buttonContent = ""
		buttonURL     = ""
	)
	if task.Type == config.SingleType {
		resp, err := w.pipelineColl.Find(&mongodb.PipelineFindOption{Name: task.PipelineName})
		if err != nil {
			log.Errorf("Pipeline find err :%s", err)
			return err
		}
		if resp.NotifyCtl == nil {
			log.Infof("pipeline notifyCtl is not set!")
			return nil
		}
		if resp.NotifyCtl.Enabled && sets.NewString(resp.NotifyCtl.NotifyTypes...).Has(string(task.Status)) {
			webHookType = resp.NotifyCtl.WebHookType
			if webHookType == dingDingType {
				uri = resp.NotifyCtl.DingDingWebHook
				atMobiles = resp.NotifyCtl.AtMobiles
				isAtAll = resp.NotifyCtl.IsAtAll
			} else if webHookType == feiShuType {
				uri = resp.NotifyCtl.FeiShuWebHook
			} else {
				uri = resp.NotifyCtl.WeChatWebHook
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
	} else if task.Type == config.WorkflowType {
		resp, err := w.workflowColl.Find(task.PipelineName)
		if err != nil {
			log.Errorf("Workflow find err :%s", err)
			return err
		}
		if resp.NotifyCtl == nil {
			log.Infof("Workflow notifyCtl is not set!")
			return nil
		}
		if resp.NotifyCtl.Enabled && sets.NewString(resp.NotifyCtl.NotifyTypes...).Has(string(task.Status)) {
			webHookType = resp.NotifyCtl.WebHookType
			if webHookType == dingDingType {
				uri = resp.NotifyCtl.DingDingWebHook
				atMobiles = resp.NotifyCtl.AtMobiles
				isAtAll = resp.NotifyCtl.IsAtAll
			} else if webHookType == feiShuType {
				uri = resp.NotifyCtl.FeiShuWebHook
			} else {
				uri = resp.NotifyCtl.WeChatWebHook
			}
			title, content, buttonContent, buttonURL, err = w.createNotifyBodyOfWorkflowIM(&wechatNotification{
				Task:        task,
				BaseURI:     configbase.SystemAddress(),
				IsSingle:    false,
				WebHookType: webHookType,
				TotalTime:   time.Now().Unix() - task.StartTime,
				AtMobiles:   atMobiles,
				IsAtAll:     isAtAll,
			})
			if err != nil {
				log.Errorf("workflow createNotifyBodyOfWorkflowIM err :%s", err)
				return err
			}
		}
	} else if task.Type == config.TestType {
		resp, err := w.testingColl.Find(task.PipelineName, task.ProductName)
		if err != nil {
			log.Errorf("testing find err :%s", err)
			return err
		}
		if resp.NotifyCtl == nil {
			log.Infof("testing notifyCtl is not set!")
			return nil
		}
		statusSets := sets.NewString(resp.NotifyCtl.NotifyTypes...)
		if resp.NotifyCtl.Enabled && (statusSets.Has(string(task.Status)) || (testTaskStatusChanged && statusSets.Has(string(config.StatusChanged)))) {
			webHookType = resp.NotifyCtl.WebHookType
			if webHookType == dingDingType {
				uri = resp.NotifyCtl.DingDingWebHook
				atMobiles = resp.NotifyCtl.AtMobiles
				isAtAll = resp.NotifyCtl.IsAtAll
			} else if webHookType == feiShuType {
				uri = resp.NotifyCtl.FeiShuWebHook
			} else {
				uri = resp.NotifyCtl.WeChatWebHook
			}
			title, content, err = w.createNotifyBodyOfTestIM(resp.Desc, &wechatNotification{
				Task:        task,
				BaseURI:     configbase.SystemAddress(),
				IsSingle:    false,
				WebHookType: webHookType,
				TotalTime:   time.Now().Unix() - task.StartTime,
				AtMobiles:   atMobiles,
				IsAtAll:     isAtAll,
			})
			if err != nil {
				log.Errorf("testing createNotifyBodyOfTestIM err :%s", err)
				return err
			}
		}
	}

	if uri != "" && content != "" {
		if webHookType == dingDingType {
			if task.Type == config.SingleType {
				title = "工作流状态"
			}
			err := w.sendDingDingMessage(uri, title, content, atMobiles)
			if err != nil {
				log.Errorf("sendDingDingMessage err : %s", err)
				return err
			}
		} else if webHookType == feiShuType {
			if task.Type == config.SingleType {
				err := w.sendFeishuMessageOfSingleType("工作流状态", uri, content)
				if err != nil {
					log.Errorf("sendFeishuMessageOfSingleType Request err : %s", err)
					return err
				}
				return nil
			}
			lc := NewLarkCard()
			lc.SetConfig(true)
			lc.SetHeader(feishuHeaderTemplate, title, feiShuTagText)
			lc.AddI18NElementsZhcnFeild(content)
			lc.AddI18NElementsZhcnAction(buttonContent, buttonURL)
			err := w.sendFeishuMessage(uri, lc)
			if err != nil {
				log.Errorf("SendFeiShuMessageRequest err : %s", err)
				return err
			}
		} else {
			typeText := weChatTextTypeMarkdown
			if task.Type == config.SingleType {
				typeText = weChatTextTypeText
			}
			err := w.SendWeChatWorkMessage(typeText, uri, title+content)
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
		if weChatNotification.WebHookType == feiShuType {
			tmplSource += url
			continue
		}
		tmplSource += fmt.Sprintf("[%s](%s)\n", url, url)
	}

	if weChatNotification.WebHookType == dingDingType {
		if len(weChatNotification.AtMobiles) > 0 && !weChatNotification.IsAtAll {
			tmplSource = fmt.Sprintf("%s - 相关人员：@%s \n", tmplSource, strings.Join(weChatNotification.AtMobiles, "@"))
		}
	}

	tplcontent, err := getTplExec(tmplSource, weChatNotification)
	return tplcontent, err
}

func (w *Service) createNotifyBodyOfWorkflowIM(weChatNotification *wechatNotification) (string, string, string, string, error) {

	tplTitle := " #### <table><tr><td bgcolor=#7FFFD4> 👍{{.Task.PipelineName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }}</td></tr></table>\n"
	tplcontent := "**创建者**：{{.Task.TaskCreator}}  **持续时间**：{{ .TotalTime}} 秒 \n" +
		"**环境信息**：{{.Task.WorkflowArgs.EnvName}}  **开始时间**：{{.Task.StartTime}} \n"
	build := ""
	deploy := ""
	test := ""
	distribute := ""
	for _, subStage := range weChatNotification.Task.Stages {
		switch subStage.TaskType {
		case config.TaskBuild:
			build = "**构建**：\n"
			for _, sb := range subStage.SubTasks {
				buildSt, err := base.ToBuildTask(sb)
				if err != nil {
					return "", "", "", "", err
				}
				branch := ""
				commitID := ""
				commitMsg := ""
				gitCommitURL := ""
				for idx, build := range buildSt.JobCtx.Builds {
					if idx == 0 || build.IsPrimary {
						branch = "Branch-" + build.Branch
						commitID = build.CommitID
						commitMsg = build.CommitMessage
						gitCommitURL = fmt.Sprintf("%s/%s/%s/commit/%s", build.Address, build.RepoOwner, build.RepoName, commitID)
					}
				}
				build += fmt.Sprintf("- %s/%s status:%s [%s-%s](%s) commmitMsg:%s\n", buildSt.ServiceName, buildSt.Service, buildSt.BuildStatus.Status, branch, commitID, gitCommitURL, commitMsg)
			}
		case config.TaskArtifact:

		case config.TaskDeploy:
			deploy = "**部署**\n"
			for svrModule, sb := range subStage.SubTasks {
				deploySt, err := base.ToDeployTask(sb)
				if err != nil {
					return "", "", "", "", err
				}
				deploy += fmt.Sprintf("- %s/%s status:%s image:%s\n", svrModule, deploySt.ServiceName, deploySt.TaskStatus, deploySt.Image)
			}
		case config.TaskTestingV2:
			test = "**测试**\n"
			for _, sb := range subStage.SubTasks {
				testSt, err := base.ToTestingTask(sb)
				if err != nil {
					return "", "", "", "", err
				}
				if weChatNotification.Task.TestReports == nil {
					continue
				}
				for testname, report := range weChatNotification.Task.TestReports {
					if testname != testSt.TestName {
						continue
					}
					tr := task.TestReport{}
					if task.IToi(report, tr) != nil {
						log.Errorf("parse TestReport failed, err:%s", err)
						continue
					}
					url := fmt.Sprintf("{{.BaseURI}}/api/aslan/testing/report?pipelineName={{.Task.PipelineName}}&pipelineType={{.Task.Type}}&taskID={{.Task.TaskID}}&testName=%s\n", testSt.TestName)
					//status:="{{if eq .WebHookType \"feishu\"}}{{.Task.Status}}{{else}}<font color=\"{{ getColor .Task.Status }}\">{{.Task.Status}}</font>{{end}} \n"
					test += fmt.Sprintf("- [%s](%s) status:%s", testSt.TestName, url, testSt.TaskStatus)
					if tr.FunctionTestSuite == nil {
						test += "\n"
						continue
					}
					test += fmt.Sprintf(":%d(成功)%d(失败)%d(总数)\n", tr.FunctionTestSuite.Successes, tr.FunctionTestSuite.Failures, tr.FunctionTestSuite.Tests)
				}
			}
		case config.TaskDistribute, config.TaskDistributeToS3:
			build = "**分发**：\n"
			for _, sb := range subStage.SubTasks {
				distributeSt, err := base.ToDistributeToS3Task(sb)
				if err != nil {
					return "", "", "", "", err
				}
				distribute += fmt.Sprintf("- %s status:%s", distributeSt.ServiceName, distributeSt.TaskStatus)
			}
		}
	}

	if weChatNotification.WebHookType == dingDingType {
		if len(weChatNotification.AtMobiles) > 0 && !weChatNotification.IsAtAll {
			tplcontent = fmt.Sprintf("%s \n 相关人员：@%s \n", tplcontent, strings.Join(weChatNotification.AtMobiles, "@"))
		}
	}
	buttonContent := "点击查看更多信息"
	workflowDetailURL := "{{.BaseURI}}/v1/projects/detail/{{.Task.ProductName}}/pipelines/{{ isSingle .IsSingle }}/{{.Task.PipelineName}}/{{.Task.TaskID}}"
	moreInformation := fmt.Sprintf("[%s](%s)", buttonContent, workflowDetailURL)
	if weChatNotification.WebHookType == feiShuType {
		tplcontent = fmt.Sprintf("%s%s%s%s%s", tplcontent, build, deploy, test, distribute)
	} else {
		tplcontent = fmt.Sprintf("%s%s%s%s%s%s", tplcontent, build, deploy, test, distribute, moreInformation)
	}

	tplExecContent, _ := getTplExec(tplcontent, weChatNotification)
	tplExecTitle, _ := getTplExec(tplTitle, weChatNotification)
	execButtonContent, _ := getTplExec(buttonContent, weChatNotification)
	execButtonURL, _ := getTplExec(workflowDetailURL, weChatNotification)
	return tplExecTitle, tplExecContent, execButtonContent, execButtonURL, nil
}

func (w *Service) createNotifyBodyOfTestIM(desc string, weChatNotification *wechatNotification) (string, string, error) {
	tplTitle := " #### <table><tr><td bgcolor=#7FFFD4> 👍{{.Task.PipelineName}} #{{.Task.TaskID}} {{ taskStatus .Task.Status }}</td></tr></table>\n"
	tplcontent := "**创建者**：{{.Task.TaskCreator}}  **持续时间**：{{ .TotalTime}} 秒 \n" +
		"**测试描述**: \n " + desc +
		"**测试结果** \n"
	for _, stage := range weChatNotification.Task.Stages {
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
				url := fmt.Sprintf("{{.BaseURI}}/api/aslan/testing/report?pipelineName={{.Task.PipelineName}}&pipelineType={{.Task.Type}}&taskID={{.Task.TaskID}}&testName=%s\n", testInfo.TestName)
				//status:="{{if eq .WebHookType \"feishu\"}}{{.Task.Status}}{{else}}<font color=\"{{ getColor .Task.Status }}\">{{.Task.Status}}</font>{{end}} \n"
				tplcontent += fmt.Sprintf("- [%s](%s) status:%s", testName, url, testInfo.TaskStatus)
				if weChatNotification.Task.TestReports == nil {
					continue
				}
				for testname, report := range weChatNotification.Task.TestReports {
					if testname != testInfo.TestName {
						continue
					}
					tr := task.TestReport{}
					if task.IToi(report, tr) != nil {
						log.Errorf("parse TestReport failed, err:%s", err)
						continue
					}
					if tr.FunctionTestSuite == nil {
						tplcontent += "\n"
						continue
					}
					tplcontent += fmt.Sprintf(":%d(成功)%d(失败)%d(总数)\n", tr.FunctionTestSuite.Successes, tr.FunctionTestSuite.Failures, tr.FunctionTestSuite.Tests)
				}
			}
		}
	}

	if weChatNotification.WebHookType == dingDingType {
		if len(weChatNotification.AtMobiles) > 0 && !weChatNotification.IsAtAll {
			tplcontent = fmt.Sprintf("%s - 相关人员：@%s \n", tplcontent, strings.Join(weChatNotification.AtMobiles, "@"))
		}
	}

	workflowDetailURL := "{{.BaseURI}}/v1/projects/detail/{{.Task.ProductName}}/pipelines/{{ isSingle .IsSingle }}/{{.Task.PipelineName}}/{{.Task.TaskID}}"
	moreInformation := fmt.Sprintf("[点击查看更多信息](%s)", workflowDetailURL)
	tplcontent += moreInformation

	tplExecContent, err := getTplExec(tplcontent, weChatNotification)
	tplExecTitle, err := getTplExec(tplTitle, weChatNotification)
	return tplExecTitle, tplExecContent, err
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
			if status == config.StatusPassed {
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
			}
			return "执行失败"
		},
	}).Parse(tplcontent))

	buffer := bytes.NewBufferString("")
	if err := tmpl.Execute(buffer, &weChatNotification); err != nil {
		log.Errorf("getTplExec Execute err:%s", err)
		return "", fmt.Errorf("getTplExec Execute err:%s", err)

	}
	return buffer.String(), nil
}
