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

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/label"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

const (
	// BuildTaskV3Timeout ...
	BuildTaskV3Timeout = 60 * 60 * 1 // 60 minutes
)

// InitializeBuildTaskV3Plugin to initialize build task plugin, and return reference
func InitializeBuildTaskV3Plugin(taskType config.TaskType) TaskPlugin {
	return &BuildTaskV3Plugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
		clientset:  krkubeclient.Clientset(),
		restConfig: krkubeclient.RESTConfig(),
	}
}

// Note: Deprecated.
// BuildTaskV3Plugin is Plugin, name should be compatible with task type
type BuildTaskV3Plugin struct {
	Name          config.TaskType
	KubeNamespace string
	JobName       string
	FileName      string
	kubeClient    client.Client
	clientset     kubernetes.Interface
	restConfig    *rest.Config
	Task          *task.Build
	Log           *zap.SugaredLogger
	Timeout       <-chan time.Time

	ack func()
}

func (p *BuildTaskV3Plugin) SetAckFunc(ack func()) {
	p.ack = ack
}

// Init ...
func (p *BuildTaskV3Plugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	p.JobName = jobname
	p.Log = xl
	p.FileName = filename
}

func (p *BuildTaskV3Plugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *BuildTaskV3Plugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *BuildTaskV3Plugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *BuildTaskV3Plugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = BuildTaskV3Timeout
	} else {
		if !p.Task.IsRestart {
			p.Task.Timeout = p.Task.Timeout * 60
		}
	}
	return p.Task.Timeout
}

func (p *BuildTaskV3Plugin) SetBuildStatusCompleted(status config.Status) {
	p.Task.BuildStatus.Status = status
	p.Task.BuildStatus.EndTime = time.Now().Unix()
}

func (p *BuildTaskV3Plugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	taskIDVar := &task.KeyVal{Key: "TASK_ID", Value: strconv.FormatInt(pipelineTask.TaskID, 10), IsCredential: false}
	p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, taskIDVar)

	p.KubeNamespace = pipelineTask.ConfigPayload.Build.KubeNamespace

	//instantiates variables like ${<REPO>_BRANCH} ${${REPO_index}_BRANCH} ...
	p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, InstantiateBuildSysVariables(&p.Task.JobCtx)...)

	jobCtx := JobCtxBuilder{
		JobName:     p.JobName,
		PipelineCtx: pipelineCtx,
		ArchiveFile: p.Task.JobCtx.PackageFile,
		JobCtx:      p.Task.JobCtx,
		Installs:    p.Task.InstallCtx,
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

	jobLabel := &label.JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     string(p.Type()),
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
	}
	p.Log.Infof("succeed to create cm for build job %s", p.JobName)

	jobImage := getReaperImage(pipelineTask.ConfigPayload.Release.ReaperImage, p.Task.BuildOS, p.Task.ImageFrom)

	//Resource request default value is LOW
	job, err := buildJob(p.Type(), jobImage, p.JobName, serviceName, "", pipelineTask.ConfigPayload.Build.KubeNamespace, p.Task.ResReq, p.Task.ResReqSpec, pipelineCtx, pipelineTask, p.Task.Registries)
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
	if err := createOrUpdateRegistrySecrets(p.KubeNamespace, pipelineTask.ConfigPayload.RegistryID, p.Task.Registries, p.kubeClient); err != nil {
		msg := fmt.Sprintf("create secret error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		p.SetBuildStatusCompleted(config.StatusFailed)
		return
	}
	if err := updater.CreateJob(job, p.kubeClient); err != nil {
		msg := fmt.Sprintf("create build job error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		p.SetBuildStatusCompleted(config.StatusFailed)
		return
	}
	p.Timeout = time.After(time.Duration(p.TaskTimeout()) * time.Second)
	p.Log.Infof("succeed to create build job %s", p.JobName)
}

// Wait ...
func (p *BuildTaskV3Plugin) Wait(ctx context.Context) {
	status, err := waitJobEndWithFile(ctx, p.TaskTimeout(), p.Timeout, p.KubeNamespace, p.JobName, true, p.kubeClient, p.clientset, p.restConfig, p.Log)
	p.SetBuildStatusCompleted(status)
	if err != nil {
		p.Task.Error = err.Error()
	}
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
func (p *BuildTaskV3Plugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
	jobLabel := &label.JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     string(p.Type()),
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

	err := saveContainerLog(pipelineTask, p.KubeNamespace, "", p.FileName, jobLabel, p.kubeClient)
	if err != nil {
		p.Log.Error(err)
		p.Task.Error = err.Error()
		return
	}

	p.Task.LogFile = p.FileName
}

// SetTask ...
func (p *BuildTaskV3Plugin) SetTask(t map[string]interface{}) error {
	task, err := ToBuildTask(t)
	if err != nil {
		return err
	}
	p.Task = task
	return nil
}

// GetTask ...
func (p *BuildTaskV3Plugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *BuildTaskV3Plugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *BuildTaskV3Plugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *BuildTaskV3Plugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *BuildTaskV3Plugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *BuildTaskV3Plugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *BuildTaskV3Plugin) ResetError() {
	p.Task.Error = ""
}
