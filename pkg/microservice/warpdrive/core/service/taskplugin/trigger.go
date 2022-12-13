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
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/taskplugin/s3"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
)

const (
	// TriggerTaskTimeout ...
	TriggerTaskTimeout = 60 * 60 * 1 // 60 minutes
)

// InitializeTriggerTaskPlugin to initialize build task plugin, and return reference
func InitializeTriggerTaskPlugin(taskType config.TaskType) TaskPlugin {
	return &TriggerTaskPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
	}
}

// TriggerTaskPlugin is Plugin, name should be compatible with task type
type TriggerTaskPlugin struct {
	Name          config.TaskType
	KubeNamespace string
	JobName       string
	FileName      string
	kubeClient    client.Client
	Task          *task.Trigger
	Log           *zap.SugaredLogger
	cancel        context.CancelFunc
	ack           func()
	pipelineName  string
	taskId        int64
}

func (p *TriggerTaskPlugin) SetAckFunc(ack func()) {
	p.ack = ack
}

// Init ...
func (p *TriggerTaskPlugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	p.JobName = jobname
	p.Log = xl
	p.FileName = filename
}

func (p *TriggerTaskPlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *TriggerTaskPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *TriggerTaskPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *TriggerTaskPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = TriggerTaskTimeout
	} else {
		if !p.Task.IsRestart {
			p.Task.Timeout = p.Task.Timeout * 60
		}
	}
	return p.Task.Timeout
}

func (p *TriggerTaskPlugin) SetTriggerStatusCompleted(status config.Status) {
	p.Task.TaskStatus = status
	p.Task.EndTime = time.Now().Unix()
}

func (p *TriggerTaskPlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	var (
		err          error
		body         []byte
		artifactPath string
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
	p.Log.Infof("succeed to create trigger task %s", p.JobName)
	_, p.cancel = context.WithCancel(context.Background())
	httpClient := httpclient.New(
		httpclient.SetHostURL(p.Task.URL),
	)
	url := p.Task.Path
	artifactPath, err = p.getS3Storage(pipelineTask)
	if err != nil {
		return
	}
	taskOutput := &task.TaskOutput{
		Type:  "object_storage",
		Value: artifactPath,
	}
	webhookPayload := &task.WebhookPayload{
		EventName:   "workflow",
		ProjectName: pipelineTask.ProductName,
		TaskName:    pipelineTask.PipelineName,
		TaskID:      pipelineTask.TaskID,
		TaskOutput:  []*task.TaskOutput{taskOutput},
		TaskEnvs:    pipelineTask.TaskArgs.BuildArgs,
	}
	body, err = json.Marshal(webhookPayload)

	headers := make(map[string]string)
	for _, header := range p.Task.Headers {
		headers[header.Key] = header.Value
	}
	headers[ZadigEvent] = EventName
	_, err = httpClient.Post(url, httpclient.SetHeaders(headers), httpclient.SetBody(body))
	if err != nil {
		return
	}
	if !p.Task.IsCallback {
		p.SetTriggerStatusCompleted(config.StatusPassed)
	}
}

func (p *TriggerTaskPlugin) getS3Storage(pipelineTask *task.Task) (string, error) {
	var err error
	var store *s3.S3
	if store, err = s3.NewS3StorageFromEncryptedURI(pipelineTask.StorageURI); err != nil {
		log.Errorf("Archive failed to create s3 storage %s", pipelineTask.StorageURI)
		return "", err
	}
	if store.Subfolder != "" {
		store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, pipelineTask.PipelineName, pipelineTask.TaskID, "artifact")
	} else {
		store.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineTask.PipelineName, pipelineTask.TaskID, "artifact")
	}
	forcedPathStyle := true
	if store.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Region, store.Insecure, forcedPathStyle)
	if err != nil {
		return "", err
	}
	prefix := store.GetObjectPath("")
	files, err := s3client.ListFiles(store.Bucket, prefix, true)
	if err != nil {
		return "", err
	}
	fileName := "artifact.tar.gz"
	if len(files) > 0 {
		fileName = files[0]
	}
	return fmt.Sprintf("%s://%s.%s/%s", store.GetSchema(), store.Bucket, store.Endpoint, fileName), nil
}

// Wait ...
func (p *TriggerTaskPlugin) Wait(ctx context.Context) {
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
				p.Task.CallbackType = callbackPayloadObj.Type
				p.Task.CallbackPayload = callbackPayloadObj.Payload
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

func (p *TriggerTaskPlugin) getCallbackObj(taskID int64, pipelineName string) (*task.CallbackPayloadObj, error) {
	url := fmt.Sprintf("/api/workflow/v3/workflowtask/callback/id/%d/name/%s", taskID, pipelineName)
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
func (p *TriggerTaskPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
}

// SetTask ...
func (p *TriggerTaskPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToTriggerTask(t)
	if err != nil {
		return err
	}
	p.Task = task
	return nil
}

// GetTask ...
func (p *TriggerTaskPlugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *TriggerTaskPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *TriggerTaskPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *TriggerTaskPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *TriggerTaskPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *TriggerTaskPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *TriggerTaskPlugin) ResetError() {
	p.Task.Error = ""
}
