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

package wechat

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
	msgType              = "markdown"
	markdownColorInfo    = "info"
	markdownColorComment = "comment"
	markdownColorWarning = "warning"
	singleInfo           = "single"
	multiInfo            = "multi"
	dingDingType         = "dingding"
	feiShuType           = "feishu"
)

//wechat
type Messsage struct {
	MsgType string `json:"msgtype"`
	Text    *Text  `json:"markdown"`
}

type Text struct {
	Content string `json:"content"`
}

//dingding
type DingDingMessage struct {
	MsgType  string            `json:"msgtype"`
	MarkDown *DingDingMarkDown `json:"markdown"`
	At       *DingDingAt       `json:"at"`
}

type DingDingMarkDown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type DingDingAt struct {
	AtMobiles []string `json:"atMobiles"`
	IsAtAll   bool     `json:"isAtAll"`
}

type FeiShuMessage struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type FeiShuMessageV2 struct {
	MsgType string          `json:"msg_type"`
	Content FeiShuContentV2 `json:"content"`
}

type FeiShuContentV2 struct {
	Text string `json:"text"`
}

type Service struct {
	proxyColl    *mongodb.ProxyColl
	workflowColl *mongodb.WorkflowColl
	pipelineColl *mongodb.PipelineColl
}

func NewWeChatClient() *Service {
	return &Service{
		proxyColl:    mongodb.NewProxyColl(),
		workflowColl: mongodb.NewWorkflowColl(),
		pipelineColl: mongodb.NewPipelineColl(),
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

func (w *Service) SendWechatMessage(task *task.Task) error {
	var (
		uri         = ""
		content     = ""
		webHookType = ""
		atMobiles   []string
		isAtAll     bool
	)
	if task.Type == config.SingleType {
		resp, err := w.pipelineColl.Find(&mongodb.PipelineFindOption{Name: task.PipelineName})
		if err != nil {
			log.Errorf("Pipeline find err :%v", err)
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
				log.Errorf("pipeline CreateNotifyBody err :%v", err)
				return err
			}
		}
	} else if task.Type == config.WorkflowType {
		resp, err := w.workflowColl.Find(task.PipelineName)
		if err != nil {
			log.Errorf("Workflow find err :%v", err)
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
			content, err = w.createNotifyBody(&wechatNotification{
				Task:        task,
				BaseURI:     configbase.SystemAddress(),
				IsSingle:    false,
				WebHookType: webHookType,
				TotalTime:   time.Now().Unix() - task.StartTime,
				AtMobiles:   atMobiles,
				IsAtAll:     isAtAll,
			})
			if err != nil {
				log.Errorf("workflow CreateNotifyBody err :%v", err)
				return err
			}
		}
	}

	if uri != "" && content != "" {
		if webHookType == dingDingType {
			message := &DingDingMessage{
				MsgType: msgType,
				MarkDown: &DingDingMarkDown{
					Title: "工作流状态",
					Text:  content,
				},
			}
			if len(atMobiles) > 0 {
				message.At = &DingDingAt{
					AtMobiles: atMobiles,
					IsAtAll:   false,
				}
			} else {
				message.At = &DingDingAt{
					IsAtAll: true,
				}
			}

			_, err := w.SendMessageRequest(uri, message)
			if err != nil {
				log.Errorf("SendDingDingMessageRequest err : %v", err)
				return err
			}

		} else if webHookType == feiShuType {
			var message interface{}
			message = &FeiShuMessage{
				Title: "工作流状态",
				Text:  content,
			}
			if strings.Contains(uri, "bot/v2/hook") {
				message = &FeiShuMessageV2{
					MsgType: "text",
					Content: FeiShuContentV2{
						Text: content,
					},
				}
			}
			_, err := w.SendMessageRequest(uri, message)
			if err != nil {
				log.Errorf("SendFeiShuMessageRequest err : %v", err)
				return err
			}
		} else {
			message := &Messsage{
				MsgType: msgType,
				Text: &Text{
					Content: content,
				},
			}
			_, err := w.SendMessageRequest(uri, message)
			if err != nil {
				log.Errorf("SendWeChatMessageRequest err : %v", err)
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
	}).Parse(tmplSource))
	buffer := bytes.NewBufferString("")

	if err = tmpl.Execute(buffer, &weChatNotification); err != nil {
		return
	}

	return buffer.String(), nil
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
