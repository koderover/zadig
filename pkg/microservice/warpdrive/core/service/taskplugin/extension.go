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

package taskplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
)

const (
	ExtensionTaskTimeout = 60 * 60 * 1 // 60 minutes
	ZadigEvent           = "X-Zadig-Event"
	EventName            = "Workflow"
)

// InitializeExtensionTaskPlugin to initialize build task plugin, and return reference
func InitializeExtensionTaskPlugin(taskType config.TaskType) TaskPlugin {
	return &ExtensionTaskPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
	}
}

// ExtensionTaskPlugin is Plugin, name should be compatible with task type
type ExtensionTaskPlugin struct {
	Name          config.TaskType
	KubeNamespace string
	JobName       string
	FileName      string
	kubeClient    client.Client
	Task          *task.Extension
	Log           *zap.SugaredLogger
	cancel        context.CancelFunc
	ack           func()
	pipelineName  string
	taskId        int64
}

func (p *ExtensionTaskPlugin) SetAckFunc(ack func()) {
	p.ack = ack
}

// Init ...
func (p *ExtensionTaskPlugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	p.JobName = jobname
	p.Log = xl
	p.FileName = filename
}

func (p *ExtensionTaskPlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *ExtensionTaskPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *ExtensionTaskPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *ExtensionTaskPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = ExtensionTaskTimeout
	} else {
		if !p.Task.IsRestart {
			p.Task.Timeout = p.Task.Timeout * 60
		}
	}
	return p.Task.Timeout
}

func (p *ExtensionTaskPlugin) SetExtensionStatusCompleted(status config.Status) {
	p.Task.TaskStatus = status
	p.Task.EndTime = time.Now().Unix()
}

func (p *ExtensionTaskPlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	var (
		err  error
		body []byte
	)
	defer func() {
		if err != nil {
			p.Log.Error(err)
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = err.Error()
			return
		}
	}()
	p.pipelineName = pipelineTask.PipelineName
	p.taskId = pipelineTask.TaskID
	p.Log.Infof("succeed to create extension task %s", p.JobName)
	_, p.cancel = context.WithCancel(context.Background())
	httpClient := httpclient.New(
		httpclient.SetHostURL(p.Task.URL),
	)
	url := p.Task.Path
	webhookPayload := &task.WebhookPayload{
		EventName:      "workflow",
		ProjectName:    pipelineTask.ProductName,
		TaskName:       pipelineTask.PipelineName,
		TaskID:         pipelineTask.TaskID,
		ServiceInfos:   p.Task.ServiceInfos,
		Creator:        pipelineTask.TaskCreator,
		WorkflowStatus: getPipelineStatus(pipelineTask),
	}
	body, err = json.Marshal(webhookPayload)
	if err != nil {
		return
	}

	p.Task.Payload = string(body)
	headers := make(map[string]string)
	for _, header := range p.Task.Headers {
		headers[header.Key] = header.Value
	}
	headers[ZadigEvent] = EventName
	response, err := httpClient.Post(url, httpclient.SetHeaders(headers), httpclient.SetBody(body))
	if err != nil {
		return
	}
	responseBody := string(response.Body())
	responseCode := response.StatusCode()
	p.Task.ResponseBody = &responseBody
	p.Task.ResponseCode = &responseCode
	if !p.Task.IsCallback {
		p.SetExtensionStatusCompleted(config.StatusPassed)
	}
}

func getPipelineStatus(pipelineTask *task.Task) config.Status {
	for _, stage := range pipelineTask.Stages {
		if stage.Status == config.StatusFailed || stage.Status == config.StatusCancelled || stage.Status == config.StatusTimeout {
			return stage.Status
		}
	}
	return config.StatusPassed
}

// Wait ...
func (p *ExtensionTaskPlugin) Wait(ctx context.Context) {
	timeout := time.After(time.Duration(p.TaskTimeout()) * time.Second)
	defer p.cancel()

	for {
		select {
		case <-ctx.Done():
			p.Task.TaskStatus = config.StatusCancelled
			return
		case <-timeout:
			p.Task.TaskStatus = config.StatusTimeout
			p.Task.Error = "timeout"
			return
		default:
			time.Sleep(time.Second * 3)
			callbackPayloadObj, _ := p.getCallbackObj(p.taskId, p.pipelineName)
			if callbackPayloadObj != nil {
				if callbackPayloadObj.Status == "success" {
					p.Task.TaskStatus = config.StatusPassed
					return
				}
				p.Task.TaskStatus = config.StatusFailed
				p.Task.Error = callbackPayloadObj.StatusMessage
				return
			}

			if p.IsTaskDone() {
				return
			}
		}
	}
}

func (p *ExtensionTaskPlugin) getCallbackObj(taskID int64, pipelineName string) (*task.CallbackPayloadObj, error) {
	url := fmt.Sprintf("/api/workflow/workflowtask/callback/id/%d/name/%s", taskID, pipelineName)
	httpClient := httpclient.New(
		httpclient.SetHostURL(configbase.AslanServiceAddress()),
	)

	CallbackPayloadObj := new(task.CallbackPayloadObj)
	_, err := httpClient.Get(url, httpclient.SetResult(&CallbackPayloadObj))
	if err != nil {
		return nil, err
	}
	return CallbackPayloadObj, nil
}

// Complete ...
func (p *ExtensionTaskPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
}

// SetTask ...
func (p *ExtensionTaskPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToExtensionTask(t)
	if err != nil {
		return err
	}
	p.Task = task
	return nil
}

// GetTask ...
func (p *ExtensionTaskPlugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *ExtensionTaskPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *ExtensionTaskPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *ExtensionTaskPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *ExtensionTaskPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *ExtensionTaskPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *ExtensionTaskPlugin) ResetError() {
	p.Task.Error = ""
}
