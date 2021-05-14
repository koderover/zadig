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
	"math/rand"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/lib/setting"
	krkubeclient "github.com/koderover/zadig/lib/tool/kube/client"
	"github.com/koderover/zadig/lib/tool/kube/updater"
	"github.com/koderover/zadig/lib/tool/xlog"
)

const (
	// BuildTaskV2Timeout ...
	BuildTaskV2Timeout = 60 * 60 * 3 // 60 minutes
)

// InitializeBuildTaskPlugin to initialize build task plugin, and return refrence
func InitializeBuildTaskPlugin(taskType config.TaskType) TaskPlugin {
	return &BuildTaskPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
	}
}

// BuildTaskPlugin is Plugin, name should be compatible with task type
type BuildTaskPlugin struct {
	Name          config.TaskType
	KubeNamespace string
	JobName       string
	FileName      string
	kubeClient    client.Client
	Task          *task.Build
	Log           *xlog.Logger

	ack func()
}

func (p *BuildTaskPlugin) SetAckFunc(ack func()) {
	p.ack = ack
}

// Init ...
func (p *BuildTaskPlugin) Init(jobname, filename string, xl *xlog.Logger) {
	p.JobName = jobname
	p.Log = xl
	p.FileName = filename
}

func (p *BuildTaskPlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *BuildTaskPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *BuildTaskPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *BuildTaskPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = BuildTaskV2Timeout
	} else {
		if !p.Task.IsRestart {
			p.Task.Timeout = p.Task.Timeout * 60
		}
	}
	return p.Task.Timeout
}

func (p *BuildTaskPlugin) SetBuildStatusCompleted(status config.Status) {
	p.Task.BuildStatus.Status = status
	p.Task.BuildStatus.EndTime = time.Now().Unix()
}

//TODO: Binded Archive File logic
func (p *BuildTaskPlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	p.KubeNamespace = pipelineTask.ConfigPayload.Build.KubeNamespace

	if pipelineTask.Type == config.WorkflowType {
		envName := pipelineTask.WorkflowArgs.Namespace
		envNameVar := &task.KeyVal{Key: "ENV_NAME", Value: envName, IsCredential: false}
		p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, envNameVar)
		if p.Task.Namespace != "" {
			xlog := new(xlog.Logger)
			if dockerHost, err := GetBestDockerHost(pipelineTask.ConfigPayload.Docker.HostList, string(config.ServiceType), pipelineTask.ConfigPayload.Build.KubeNamespace, xlog); err == nil {
				pipelineCtx.DockerHost = dockerHost
			}
			p.KubeNamespace = p.Task.Namespace
		}
	} else if pipelineTask.Type == config.ServiceType {
		envName := pipelineTask.ServiceTaskArgs.Namespace
		envNameVar := &task.KeyVal{Key: "ENV_NAME", Value: envName, IsCredential: false}
		p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, envNameVar)
		p.KubeNamespace = pipelineTask.ServiceTaskArgs.K8sNamespace
	}

	for _, repo := range p.Task.JobCtx.Builds {
		if len(repo.Branch) > 0 {
			branchVar := &task.KeyVal{Key: fmt.Sprintf("%s_BRANCH", repo.RepoName), Value: repo.Branch, IsCredential: false}
			p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, branchVar)
		}

		if len(repo.Tag) > 0 {
			tagVar := &task.KeyVal{Key: fmt.Sprintf("%s_TAG", repo.RepoName), Value: repo.Tag, IsCredential: false}
			p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, tagVar)
		}

		if repo.PR > 0 {
			prVar := &task.KeyVal{Key: fmt.Sprintf("%s_PR", repo.RepoName), Value: strconv.Itoa(repo.PR), IsCredential: false}
			p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, prVar)
		}
	}

	jobCtx := JobCtxBuilder{
		JobName:     p.JobName,
		PipelineCtx: pipelineCtx,
		ArchiveFile: p.Task.JobCtx.PackageFile,
		JobCtx:      p.Task.JobCtx,
		Installs:    p.Task.InstallCtx,
	}

	if featuresObj, err := GetFeatures(); err == nil {
		pipelineTask.Features = featuresObj.Features
	}

	if p.Task.BuildStatus == nil {
		p.Task.BuildStatus = &task.BuildStatus{}
	}

	p.Task.BuildStatus.Status = config.StatusRunning
	p.Task.BuildStatus.StartTime = time.Now().Unix()
	p.ack()

	jobCtxBytes, err := yaml.Marshal(jobCtx.BuildReaperContext(pipelineTask, serviceName))
	if err != nil {
		msg := fmt.Sprintf("cannot reaper.Context data: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		p.SetBuildStatusCompleted(config.StatusFailed)
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
		p.Log.Error(err)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = err.Error()
		p.SetBuildStatusCompleted(config.StatusFailed)
		return
	}

	if err := createJobConfigMap(
		p.KubeNamespace, p.JobName, jobLabel, string(jobCtxBytes), p.kubeClient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		p.SetBuildStatusCompleted(config.StatusFailed)
		return
	} else {
		p.Log.Infof("succeed to create cm for build job %s", p.JobName)
	}

	jobImage := fmt.Sprintf("%s-%s", pipelineTask.ConfigPayload.Release.ReaperImage, p.Task.BuildOS)
	if p.Task.ImageFrom == setting.ImageFromCustom {
		jobImage = p.Task.BuildOS
	}

	//Resource request default value is LOW
	job, err := buildJob(p.Type(), jobImage, p.JobName, serviceName, p.Task.ResReq, pipelineCtx, pipelineTask, p.Task.Registries)
	if err != nil {
		msg := fmt.Sprintf("create build job context error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		p.SetBuildStatusCompleted(config.StatusFailed)
		return
	}

	job.Namespace = p.KubeNamespace

	if err := ensureDeleteJob(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
		msg := fmt.Sprintf("delete build job error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		p.SetBuildStatusCompleted(config.StatusFailed)
		return
	}

	// 将集成到KodeRover的私有镜像仓库的访问权限设置到namespace中
	if err := createOrUpdateRegistrySecrets(p.KubeNamespace, p.Task.Registries, p.kubeClient); err != nil {
		p.Log.Errorf("create secret error: %v", err)
	}
	if err := updater.CreateJob(job, p.kubeClient); err != nil {
		msg := fmt.Sprintf("create build job error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		p.SetBuildStatusCompleted(config.StatusFailed)
		return
	} else {
		p.Log.Infof("succeed to create build job %s", p.JobName)
	}

	return
}

// Wait ...
func (p *BuildTaskPlugin) Wait(ctx context.Context) {
	status := waitJobEndWithFile(ctx, p.TaskTimeout(), p.KubeNamespace, p.JobName, true, p.kubeClient, p.Log)
	p.SetBuildStatusCompleted(status)

	if status == config.StatusPassed {
		if p.Task.DockerBuildStatus == nil {
			p.Task.DockerBuildStatus = &task.DockerBuildStatus{}
		}

		p.Task.DockerBuildStatus.StartTime = time.Now().Unix()
		p.Task.DockerBuildStatus.Status = config.StatusRunning
		p.ack()

		select {
		case <-ctx.Done():
			p.Task.DockerBuildStatus.EndTime = time.Now().Unix()
			p.Task.DockerBuildStatus.Status = config.StatusCancelled
			p.Task.TaskStatus = config.StatusCancelled
			p.ack()
			return
		case <-time.After(time.Duration(rand.Int()%2) * time.Second):
			p.Task.DockerBuildStatus.EndTime = time.Now().Unix()
			p.Task.DockerBuildStatus.Status = config.StatusPassed
		}
	}

	p.SetStatus(status)
}

// Complete ...
func (p *BuildTaskPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
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

	err := saveContainerLog(pipelineTask, p.KubeNamespace, p.FileName, jobLabel, p.kubeClient)
	if err != nil {
		p.Log.Error(err)
		p.Task.Error = err.Error()
		return
	}

	p.Task.LogFile = p.FileName
}

// SetTask ...
func (p *BuildTaskPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToBuildTask(t)
	if err != nil {
		return err
	}
	p.Task = task
	return nil
}

// GetTask ...
func (p *BuildTaskPlugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *BuildTaskPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *BuildTaskPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *BuildTaskPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *BuildTaskPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *BuildTaskPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *BuildTaskPlugin) ResetError() {
	p.Task.Error = ""
}
