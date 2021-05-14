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
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/qiniu/x/log.v7"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/task"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/util"
)

const (
	METHOD                 = "POST"
	MSGTYPE                = "markdown"
	MARKDOWN_COLOR_INFO    = "info"
	MARKDOWN_COLOR_COMMENT = "comment"
	MARKDOWN_COLOR_Warning = "warning"
	SINGLE_INFO            = "single"
	MULTI_INFO             = "multi"
	DingDingType           = "dingding"
	FeiShuType             = "feishu"
)

//wechat
type WechatMesssage struct {
	MsgType    string      `json:"msgtype"`
	WechatText *WechatText `json:"markdown"`
}

type WechatText struct {
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

type WeChatService struct {
	proxyColl    *repo.ProxyColl
	workflowColl *repo.WorkflowColl
	pipelineColl *repo.PipelineColl
}

func NewWeChatClient() *WeChatService {
	return &WeChatService{
		proxyColl:    repo.NewProxyColl(),
		workflowColl: repo.NewWorkflowColl(),
		pipelineColl: repo.NewPipelineColl(),
	}
}

type wechatNotification struct {
	Task        *task.Task `json:"task"`
	BaseUri     string     `json:"base_uri"`
	IsSingle    bool       `json:"is_single"`
	WebHookType string     `json:"web_hook_type"`
	TotalTime   int64      `json:"total_time"`
	AtMobiles   []string   `json:"atMobiles"`
	IsAtAll     bool       `json:"is_at_all"`
}

func (w *WeChatService) SendMessageRequest(uri string, message interface{}) ([]byte, error) {
	header := http.Header{}
	header.Set("Content-Type", "application/json")

	req, err := http.NewRequest(METHOD, uri, util.GetRequestBody(message))
	if err != nil {
		return nil, err
	}

	req.Header = header
	client := &http.Client{}

	// 使用代理
	proxies, _ := w.proxyColl.List(&repo.ProxyArgs{})
	if len(proxies) != 0 && proxies[0].EnableApplicationProxy {
		p, err := url.Parse(proxies[0].GetProxyUrl())
		if err == nil {
			proxy := http.ProxyURL(p)
			trans := &http.Transport{
				Proxy: proxy,
			}
			client.Transport = trans
		}
		fmt.Printf("send message is using proxy:%s\n", proxies[0].GetProxyUrl())
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode/100 == 2 {
		return body, nil
	}

	return nil, fmt.Errorf("response status error: %s %d", uri, resp.StatusCode)
}

func (w *WeChatService) SendWechatMessage(task *task.Task) error {
	var (
		uri         = ""
		content     = ""
		webHookType = ""
		atMobiles   []string
		isAtAll     bool
	)
	if task.Type == config.SingleType {
		resp, err := w.pipelineColl.Find(&repo.PipelineFindOption{Name: task.PipelineName})
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
			if webHookType == DingDingType {
				uri = resp.NotifyCtl.DingDingWebHook
				atMobiles = resp.NotifyCtl.AtMobiles
				isAtAll = resp.NotifyCtl.IsAtAll
			} else if webHookType == FeiShuType {
				uri = resp.NotifyCtl.FeiShuWebHook
			} else {
				uri = resp.NotifyCtl.WeChatWebHook
			}
			content, err = w.createNotifyBody(&wechatNotification{
				Task:        task,
				BaseUri:     config.AslanURL(),
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
			if webHookType == DingDingType {
				uri = resp.NotifyCtl.DingDingWebHook
				atMobiles = resp.NotifyCtl.AtMobiles
				isAtAll = resp.NotifyCtl.IsAtAll
			} else if webHookType == FeiShuType {
				uri = resp.NotifyCtl.FeiShuWebHook
			} else {
				uri = resp.NotifyCtl.WeChatWebHook
			}
			content, err = w.createNotifyBody(&wechatNotification{
				Task:        task,
				BaseUri:     config.AslanURL(),
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
		if webHookType == DingDingType {
			message := &DingDingMessage{
				MsgType: MSGTYPE,
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

		} else if webHookType == FeiShuType {
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
			message := &WechatMesssage{
				MsgType: MSGTYPE,
				WechatText: &WechatText{
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

func (w *WeChatService) createNotifyBody(weChatNotification *wechatNotification) (content string, err error) {
	tmplSource := "{{if eq .WebHookType \"feishu\"}}触发的工作流: {{.BaseUri}}/v1/projects/detail/{{.Task.ProductName}}/pipelines/{{ isSingle .IsSingle }}/{{.Task.PipelineName}}/{{.Task.TaskID}}{{else}}#### 触发的工作流: [{{.Task.PipelineName}}#{{.Task.TaskID}}]({{.BaseUri}}/v1/projects/detail/{{.Task.ProductName}}/pipelines/{{ isSingle .IsSingle }}/{{.Task.PipelineName}}/{{.Task.TaskID}}){{end}} \n" +
		"- 状态: {{if eq .WebHookType \"feishu\"}}{{.Task.Status}}{{else}}<font color=\"{{ getColor .Task.Status }}\">{{.Task.Status}}</font>{{end}} \n" +
		"- 创建人：{{.Task.TaskCreator}} \n" +
		"- 总运行时长：{{ .TotalTime}} 秒 \n"

	if weChatNotification.WebHookType == DingDingType {
		if len(weChatNotification.AtMobiles) > 0 && !weChatNotification.IsAtAll {
			tmplSource = fmt.Sprintf("%s - 相关人员：@%s \n", tmplSource, strings.Join(weChatNotification.AtMobiles, "@"))
		}
	}

	tmpl := template.Must(template.New("notify").Funcs(template.FuncMap{
		"getColor": func(status config.Status) string {
			if status == config.StatusPassed {
				return MARKDOWN_COLOR_INFO
			} else if status == config.StatusTimeout || status == config.StatusCancelled {
				return MARKDOWN_COLOR_COMMENT
			} else if status == config.StatusFailed {
				return MARKDOWN_COLOR_Warning
			}
			return MARKDOWN_COLOR_COMMENT
		},
		"isSingle": func(isSingle bool) string {
			if isSingle {
				return SINGLE_INFO
			} else {
				return MULTI_INFO
			}
		},
	}).Parse(tmplSource))
	buffer := bytes.NewBufferString("")

	if err = tmpl.Execute(buffer, &weChatNotification); err != nil {
		return
	}

	return buffer.String(), nil
}
