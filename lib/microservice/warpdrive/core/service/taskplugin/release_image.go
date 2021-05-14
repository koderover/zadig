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
	"strconv"
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

// InitializeReleaseImagePlugin ...
func InitializeReleaseImagePlugin(taskType config.TaskType) TaskPlugin {
	return &ReleaseImagePlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
	}
}

// ReleaseImagePlugin Plugin name should be compatible with task type
type ReleaseImagePlugin struct {
	Name          config.TaskType
	KubeNamespace string
	JobName       string
	FileName      string
	kubeClient    client.Client
	Task          *task.ReleaseImage
	Log           *xlog.Logger
}

func (p *ReleaseImagePlugin) SetAckFunc(func()) {
}

const (
	// RelealseImageTaskTimeout ...
	RelealseImageTaskTimeout = 60 * 5 // 5 minutes
)

// Init ...
func (p *ReleaseImagePlugin) Init(jobname, filename string, xl *xlog.Logger) {
	p.JobName = jobname
	p.FileName = filename
	// SetLogger ...
	p.Log = xl
}

// Type ...
func (p *ReleaseImagePlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *ReleaseImagePlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *ReleaseImagePlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *ReleaseImagePlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		tm := config.ReleaseImageTimeout()
		if tm != "" {
			var err error
			p.Task.Timeout, err = strconv.Atoi(tm)
			if err != nil {
				p.Log.Warnf("failed to parse timeout settings %v", err)
				p.Task.Timeout = RelealseImageTaskTimeout
			}
		} else {
			p.Task.Timeout = RelealseImageTaskTimeout
		}
	}

	return p.Task.Timeout
}

// Run ...
func (p *ReleaseImagePlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	p.KubeNamespace = pipelineTask.ConfigPayload.Build.KubeNamespace
	// 设置本次运行需要配置
	//t.Workspace = fmt.Sprintf("%s/%s", pipelineTask.ConfigPayload.NFS.Path, pipelineTask.PipelineName)
	releases := make([]task.RepoImage, 0)
	for _, v := range p.Task.Releases {
		if cfg, ok := pipelineTask.ConfigPayload.RepoConfigs[v.RepoId]; ok {
			v.Username = cfg.AccessKey
			v.Password = cfg.SecretyKey
			releases = append(releases, v)
		}
	}

	if len(releases) == 0 {
		return
	}

	jobCtx := &types.PredatorContext{
		JobType: setting.ReleaseImageJob,
		//Docker build context
		DockerBuildCtx: &task.DockerBuildCtx{
			ImageName:       p.Task.ImageTest,
			ImageReleaseTag: p.Task.ImageRelease,
		},
		//Registry host/user/password
		DockerRegistry: &types.DockerRegistry{
			Host:     pipelineTask.ConfigPayload.Registry.Addr,
			UserName: pipelineTask.ConfigPayload.Registry.AccessKey,
			Password: pipelineTask.ConfigPayload.Registry.SecretKey,
		},

		ReleaseImages: releases,
	}

	jobCtxBytes, err := yaml.Marshal(jobCtx)
	if err != nil {
		msg := fmt.Sprintf("cannot mashal predetor.Context data: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	// 重置错误信息
	p.Task.Error = ""

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
		p.KubeNamespace, p.JobName, jobLabel, string(jobCtxBytes), p.kubeClient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	} else {
		p.Log.Infof("succeed to create cm for image job %s", p.JobName)
	}

	job, err := buildJob(p.Type(), pipelineTask.ConfigPayload.Release.PredatorImage, p.JobName, serviceName, setting.MinRequest, pipelineCtx, pipelineTask, []*task.RegistryNamespace{})
	if err != nil {
		msg := fmt.Sprintf("create release image job context error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	if err := ensureDeleteJob(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
		msg := fmt.Sprintf("delete release image job error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	job.Namespace = p.KubeNamespace
	if err := updater.CreateJob(job, p.kubeClient); err != nil {
		msg := fmt.Sprintf("create release image job error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	} else {
		p.Log.Infof("succeed to create image job %s", p.JobName)
	}

	return
}

// Wait ...
func (p *ReleaseImagePlugin) Wait(ctx context.Context) {
	status := waitJobEnd(ctx, p.TaskTimeout(), p.KubeNamespace, p.JobName, p.kubeClient, p.Log)
	p.SetStatus(status)
}

// Complete ...
func (p *ReleaseImagePlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
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

// SetTask ...
func (p *ReleaseImagePlugin) SetTask(t map[string]interface{}) error {
	task, err := ToReleaseImageTask(t)
	if err != nil {
		return err
	}
	p.Task = task
	return nil
}

// GetTask ...
func (p *ReleaseImagePlugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *ReleaseImagePlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *ReleaseImagePlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *ReleaseImagePlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *ReleaseImagePlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *ReleaseImagePlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *ReleaseImagePlugin) ResetError() {
	p.Task.Error = ""
}
