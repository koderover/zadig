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

package taskplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/xlog"
)

// InitializeSecurityPlugin ...
func InitializeSecurityPlugin(taskType config.TaskType) TaskPlugin {
	return &SecurityPlugin{
		Name:      taskType,
		errorChan: make(chan error, 1),
	}
}

const (
	SecurityTaskTimeout = 60 * 3 // 3 minutes
)

// SecurityPlugin Plugin name should be compatible with task type
type SecurityPlugin struct {
	Name          config.TaskType
	KubeNamespace string
	JobName       string
	FileName      string
	Task          *task.Security
	Log           *xlog.Logger
	cancel        context.CancelFunc
	errorChan     chan error
}

type DeliverySecurityInfo struct {
	Result string `json:"result,omitempty"`
}

func (p *SecurityPlugin) SetAckFunc(func()) {
}

// Init ...
func (p *SecurityPlugin) Init(jobname, filename string, xl *xlog.Logger) {
	p.JobName = jobname
	p.FileName = filename
	// SetLogger ...
	p.Log = xl
}

// Type ...
func (p *SecurityPlugin) Type() config.TaskType {
	return p.Name
}

func (p *SecurityPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *SecurityPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *SecurityPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = SecurityTaskTimeout
	}
	return p.Task.Timeout
}

// Run ...
func (p *SecurityPlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	ctx, p.cancel = context.WithCancel(context.Background())
	p.KubeNamespace = pipelineTask.ConfigPayload.Build.KubeNamespace
	// 设置本次运行需要配置
	var namespace string
	if namespaceData, err := ioutil.ReadFile(
		"/var/run/secrets/kubernetes.io/serviceaccount/namespace",
	); err == nil {
		namespace = strings.TrimSpace(string(namespaceData))
	}

	imageName := p.Task.ImageName
	go func() {
		// send request to clair client to analysis image
		body, err := p.analysis(ctx, imageName, pipelineCtx.DockerHost, namespace)
		if err != nil {
			p.errorChan <- err
			return
		}

		var imageID string
		// send analysis result to aslan to store
		if imageID, err = p.report(ctx, imageName, body); err != nil {
			p.errorChan <- err
			return
		} else {
			p.Task.ImageID = imageID
		}

		// get analysis summary and save to task
		if summary, err := p.getSummary(ctx, imageID); err != nil {
			p.errorChan <- err
			return
		} else {
			p.Task.Summary = summary
			p.Task.TaskStatus = config.StatusPassed
		}
	}()

	return
}

func getRootAuthHeader() http.Header {
	header := http.Header{}
	header.Set(setting.Auth, fmt.Sprintf("%s%s", setting.AuthPrefix, config.PoetryAPIRootKey()))
	return header
}

func (p *SecurityPlugin) getSummary(ctx context.Context, imageId string) (map[string]int, error) {
	client := &httpClient{
		ctx: ctx,
		log: p.Log,
	}
	url := fmt.Sprintf("%s/delivery/security/stats?imageId=%s", config.AslanAddr(), imageId)

	if body, err := client.Do("GET", url, getRootAuthHeader(), nil); err != nil {
		return nil, err
	} else {
		var summary map[string]int
		if err := json.Unmarshal(body, &summary); err != nil {
			p.Log.Error("json Unmarshal err :", err)
			return nil, err
		}
		return summary, nil
	}
}

func (p *SecurityPlugin) report(ctx context.Context, imageName string, body []byte) (string, error) {
	client := &httpClient{
		ctx: ctx,
		log: p.Log,
	}

	url := fmt.Sprintf("%s/delivery/security", config.AslanAddr())
	if body, err := client.Do("POST", url, getRootAuthHeader(), body); err != nil {
		return "", err
	} else {
		p.Log.Info("security scan success !!! imageName :", imageName)
		var imageID string
		if err := json.Unmarshal(body, &imageID); err != nil {
			p.Log.Error("json Unmarshal err :", err)
			return "", err
		}
		p.Task.ImageID = imageID
		return imageID, nil
	}
}

type httpClient struct {
	ctx context.Context
	log *xlog.Logger
}

func (hc *httpClient) Do(method, url string, header http.Header, body []byte) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		hc.log.Errorf("http.NewRequest err %s :%v", url, err)
		return nil, err
	}
	req.Close = true
	req = req.WithContext(hc.ctx)
	req.Header = header

	var resp *http.Response
	if resp, err = client.Do(req); err != nil {
		hc.log.Errorf("client.Do err %s :%v", url, err)
		return nil, err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if body, err := ioutil.ReadAll(resp.Body); err != nil || resp.StatusCode/100 != 2 {
		if err == nil {
			err = fmt.Errorf("non 200 status found %s, %d", url, resp.StatusCode)
		}
		hc.log.Errorf("error response %s, %v", url, err)
		return nil, err
	} else {
		return body, nil
	}
}

func (p *SecurityPlugin) analysis(ctx context.Context, imageName, dockerHost, namespace string) ([]byte, error) {
	client := &httpClient{
		ctx: ctx,
		log: p.Log,
	}

	clairUrl := fmt.Sprintf(
		"%sanalyzeLocalImage?imageName=%s&dockerHost=%s&namespace=%s",
		config.ClairClientAddr(),
		imageName,
		dockerHost,
		namespace,
	)

	body, err := client.Do("GET", clairUrl, nil, nil)
	if err != nil {
		return nil, err
	} else {
		//调用backend api
		var deliverySecurityInfo DeliverySecurityInfo
		if err := json.Unmarshal(body, &deliverySecurityInfo); err != nil {
			p.Log.Error("json Unmarshal err :", err)
			return nil, err
		}
		if deliverySecurityInfo.Result == "success" {
			return body, nil
		}

		return nil, fmt.Errorf("failed to analysis %s", imageName)
	}
}

// Wait ...
func (p *SecurityPlugin) Wait(ctx context.Context) {
	timeout := time.After(time.Duration(p.TaskTimeout()) * time.Second)
	defer p.cancel()
	for {
		select {
		case <-ctx.Done():
			p.Task.TaskStatus = config.StatusCancelled
			return
		case err := <-p.errorChan:
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = err.Error()
			p.Log.Errorf("failed to scan image %s %v", p.Task.ImageName, err)
			return
		case <-timeout:
			p.Task.TaskStatus = config.StatusTimeout
			p.Task.Error = "timeout"
			return
		default:
			time.Sleep(time.Second * 2)
			if p.IsTaskDone() {
				return
			}
		}
	}
}

// Complete ...
func (p *SecurityPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
	if pipelineTask.TestReports == nil {
		pipelineTask.TestReports = make(map[string]interface{})
	}
	if p.Task.Summary != nil {
		testReport := new(types.TestReport)
		security := make(map[string]map[string]int)
		security[p.Task.ImageID] = p.Task.Summary
		testReport.Security = security
		//安全测试报告
		pipelineTask.TestReports[p.Task.ImageID] = testReport
	}
}

// SetTask ...
func (p *SecurityPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToSecurityTask(t)
	if err != nil {
		return err
	}
	p.Task = task
	return nil
}

// GetTask ...
func (p *SecurityPlugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *SecurityPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *SecurityPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *SecurityPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *SecurityPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *SecurityPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *SecurityPlugin) ResetError() {
	p.Task.Error = ""
}
