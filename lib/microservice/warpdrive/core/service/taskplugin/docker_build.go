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
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/lib/setting"
	krkubeclient "github.com/koderover/zadig/lib/tool/kube/client"
	"github.com/koderover/zadig/lib/tool/kube/updater"
	"github.com/koderover/zadig/lib/tool/xlog"
)

// DockerBuildPlugin Plugin name should be compatible with task type
type DockerBuildPlugin struct {
	Name          config.TaskType
	KubeNamespace string
	JobName       string
	FileName      string
	kubeClient    client.Client
	Task          *task.DockerBuild
	Log           *xlog.Logger
}

func (p *DockerBuildPlugin) SetAckFunc(func()) {
}

const (
	// DockerBuildTimeout ...
	DockerBuildTimeout = 60 * 60 // 60 minutes
)

// InitializeDockerBuildTaskPlugin init docker build plugin
func InitializeDockerBuildTaskPlugin(taskType config.TaskType) TaskPlugin {
	return &DockerBuildPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
	}
}

// Init ...
func (p *DockerBuildPlugin) Init(jobname, filename string, xl *xlog.Logger) {
	p.JobName = jobname
	p.FileName = filename
	// SetLogger ...
	p.Log = xl
}

// Type ...
func (p *DockerBuildPlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *DockerBuildPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetTask ...
func (p *DockerBuildPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToDockerBuildTask(t)
	if err != nil {
		return err
	}
	p.Task = task
	return nil
}

// GetTask ...
func (p *DockerBuildPlugin) GetTask() interface{} {
	return p.Task
}

// SetStatus ...
func (p *DockerBuildPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *DockerBuildPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = DockerBuildTimeout
	}
	return p.Task.Timeout
}

func (p *DockerBuildPlugin) setProxy(cfg task.Proxy, dockerHost string) {
	if cfg.EnableRepoProxy && cfg.Type == "http" {
		httpAddr := cfg.GetProxyUrl()
		if !strings.Contains(strings.ToLower(p.Task.BuildArgs), "--build-arg http_proxy=") {
			p.Task.BuildArgs = fmt.Sprintf("%s --build-arg http_proxy=%s", p.Task.BuildArgs, httpAddr)
		}
		if !strings.Contains(strings.ToLower(p.Task.BuildArgs), "--build-arg https_proxy=") {
			p.Task.BuildArgs = fmt.Sprintf("%s --build-arg https_proxy=%s", p.Task.BuildArgs, httpAddr)
		}
	}
}

func (p *DockerBuildPlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	p.KubeNamespace = pipelineTask.ConfigPayload.Build.KubeNamespace

	//t.Workspace = pipelineCtx.Workspace
	//t.ConfigMapMountDir = pipelineCtx.Workspace

	if p.Task.BuildArgs != "" {
		p.Task.BuildArgs = strings.Join(strings.Fields(p.Task.BuildArgs), " ")
	}
	p.setProxy(pipelineTask.ConfigPayload.Proxy, pipelineCtx.DockerHost)

	configCtx := &types.PredatorContext{
		JobType: setting.BuildImageJob,
		//Docker build context
		DockerBuildCtx: &task.DockerBuildCtx{
			WorkDir:    fmt.Sprintf("%s/%s", pipelineCtx.Workspace, p.Task.WorkDir),
			DockerFile: fmt.Sprintf("%s/%s", pipelineCtx.Workspace, p.Task.DockerFile),
			ImageName:  p.Task.Image,
			BuildArgs:  p.Task.BuildArgs,
		},
		//Registry host/user/password
		DockerRegistry: &types.DockerRegistry{
			Host:     pipelineTask.ConfigPayload.Registry.Addr,
			UserName: pipelineTask.ConfigPayload.Registry.AccessKey,
			Password: pipelineTask.ConfigPayload.Registry.SecretKey,
		},
	}

	if p.Task.OnSetup != "" {
		configCtx.OnSetup = p.Task.OnSetup
	}

	//生成Predator相关Configmap
	predatorConfigCtx, err := yaml.Marshal(configCtx)
	if err != nil {
		msg := fmt.Sprintf("cannot mashal predator.Context data: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	jobLabel := &JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     fmt.Sprintf("%s", p.Type()),
		PipelineType: string(pipelineTask.Type),
	}

	if err := ensureDeleteConfigMap(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
		msg := fmt.Sprintf("ensureDeleteConfigMap error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	if err := createJobConfigMap(
		p.KubeNamespace, p.JobName, jobLabel, string(predatorConfigCtx), p.kubeClient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	if err := ensureDeleteJob(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
		msg := fmt.Sprintf("delete docker build job error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	job, err := buildJob(p.Type(), pipelineTask.ConfigPayload.Release.PredatorImage, p.JobName, serviceName, setting.MinRequest, pipelineCtx, pipelineTask, []*task.RegistryNamespace{})
	if err != nil {
		msg := fmt.Sprintf("create build job context error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	job.Namespace = p.KubeNamespace

	if err := updater.CreateJob(job, p.kubeClient); err != nil {
		msg := fmt.Sprintf("create docker build job error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}
	return
}

// Wait ...
func (p *DockerBuildPlugin) Wait(ctx context.Context) {
	status := waitJobEnd(ctx, p.TaskTimeout(), p.KubeNamespace, p.JobName, p.kubeClient, p.Log)
	p.SetStatus(status)
}

// Complete ...
func (p *DockerBuildPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
	jobLabel := &JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     fmt.Sprintf("%s", p.Type()),
		PipelineType: string(pipelineTask.Type),
	}

	// 清理用户取消和超时的任务
	defer func() {
		if p.Task.TaskStatus == config.StatusCancelled || p.Task.TaskStatus == config.StatusTimeout {
			if err := ensureDeleteJob(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
				p.Log.Error(err)
				p.Task.Error = err.Error()
			}
			return
		}
	}()

	// 保存实时日志到s3
	err := saveContainerLog(pipelineTask, p.KubeNamespace, p.FileName, jobLabel, p.kubeClient)
	if err != nil {
		p.Log.Error(err)
		p.Task.Error = err.Error()
		return
	}
	p.Task.LogFile = p.JobName

}

// IsTaskDone ...
func (p *DockerBuildPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *DockerBuildPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *DockerBuildPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *DockerBuildPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *DockerBuildPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *DockerBuildPlugin) ResetError() {
	p.Task.Error = ""
}
