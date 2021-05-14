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
	"time"

	"github.com/bndr/gojenkins"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/lib/microservice/jenkinsplugin/core/service"
	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/lib/setting"
	krkubeclient "github.com/koderover/zadig/lib/tool/kube/client"
	"github.com/koderover/zadig/lib/tool/kube/updater"
	"github.com/koderover/zadig/lib/tool/xlog"
)

const (
	// JenkinsBuildTimeout ...
	JenkinsBuildTimeout = 60 * 60 * 3 // 60 minutes
)

// InitializeJenkinsBuildPlugin to initialize jenkins build, and return reference
func InitializeJenkinsBuildPlugin(taskType config.TaskType) TaskPlugin {
	return &JenkinsBuildPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
	}
}

// JenkinsBuildPlugin is Plugin, name should be compatible with task type
type JenkinsBuildPlugin struct {
	Name          config.TaskType
	KubeNamespace string
	kubeClient    client.Client
	JobName       string
	FileName      string
	Task          *task.JenkinsBuild
	Log           *xlog.Logger

	ack func()
}

func (j *JenkinsBuildPlugin) SetAckFunc(ack func()) {
	j.ack = ack
}

// Init ...
func (j *JenkinsBuildPlugin) Init(jobname, filename string, xl *xlog.Logger) {
	j.JobName = jobname
	j.Log = xl
	j.FileName = filename
}

func (j *JenkinsBuildPlugin) Type() config.TaskType {
	return j.Name
}

func (j *JenkinsBuildPlugin) Status() config.Status {
	return j.Task.TaskStatus
}

func (j *JenkinsBuildPlugin) SetStatus(status config.Status) {
	j.Task.TaskStatus = status
}

// TaskTimeout ...
func (j *JenkinsBuildPlugin) TaskTimeout() int {
	if j.Task.Timeout == 0 {
		j.Task.Timeout = JenkinsBuildTimeout
	} else {
		if !j.Task.IsRestart {
			j.Task.Timeout = j.Task.Timeout * 60
		}
	}
	return j.Task.Timeout
}

func (j *JenkinsBuildPlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	j.KubeNamespace = pipelineTask.ConfigPayload.Build.KubeNamespace

	jenkinsBuildParams := make([]*service.JenkinsBuildParam, 0)
	for _, jenkinsBuildParam := range j.Task.JenkinsBuildArgs.JenkinsBuildParams {
		jenkinsBuildParams = append(jenkinsBuildParams, &service.JenkinsBuildParam{
			Name:  jenkinsBuildParam.Name,
			Value: jenkinsBuildParam.Value,
		})
	}

	jobCtx := &service.Config{
		JobType: setting.JenkinsBuildJob,
		JenkinsIntegration: &service.JenkinsIntegration{
			Url:      j.Task.JenkinsIntegration.URL,
			Username: j.Task.JenkinsIntegration.Username,
			Password: j.Task.JenkinsIntegration.Password,
		},
		JenkinsBuild: &service.JenkinsBuild{
			JobName:           j.Task.JenkinsBuildArgs.JobName,
			JenkinsBuildParam: jenkinsBuildParams,
		},
	}

	jobCtxBytes, err := yaml.Marshal(jobCtx)
	if err != nil {
		msg := fmt.Sprintf("cannot mashal jenkins.Context data: %v", err)
		j.Log.Error(msg)
		j.Task.TaskStatus = config.StatusFailed
		j.Task.Error = msg
		return
	}

	// 重置错误信息
	j.Task.Error = ""

	jobLabel := &JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     fmt.Sprintf("%s", j.Type()),
		PipelineType: string(pipelineTask.Type),
	}

	if err := ensureDeleteConfigMap(j.KubeNamespace, jobLabel, j.kubeClient); err != nil {
		msg := fmt.Sprintf("ensureDeleteConfigMap error: %v", err)
		j.Log.Error(msg)
		j.Task.TaskStatus = config.StatusFailed
		j.Task.Error = msg
		return
	}

	if err := createJobConfigMap(
		j.KubeNamespace, j.JobName, jobLabel, string(jobCtxBytes), j.kubeClient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		j.Log.Error(msg)
		j.Task.TaskStatus = config.StatusFailed
		j.Task.Error = msg
		return
	} else {
		j.Log.Infof("succeed to create cm for jenkins build job %s", j.JobName)
	}

	j.Log.Infof("JenkinsBuildConfig : %+v", pipelineTask.ConfigPayload.JenkinsBuildConfig)
	job, err := buildJob(j.Type(), pipelineTask.ConfigPayload.JenkinsBuildConfig.JenkinsBuildImage, j.JobName, serviceName, setting.MinRequest, pipelineCtx, pipelineTask, []*task.RegistryNamespace{})
	if err != nil {
		msg := fmt.Sprintf("create jenkins build job context error: %v", err)
		j.Log.Error(msg)
		j.Task.TaskStatus = config.StatusFailed
		j.Task.Error = msg
		return
	}

	if err := ensureDeleteJob(j.KubeNamespace, jobLabel, j.kubeClient); err != nil {
		msg := fmt.Sprintf("delete jenkins build job error: %v", err)
		j.Log.Error(msg)
		j.Task.TaskStatus = config.StatusFailed
		j.Task.Error = msg
		return
	}
	job.Namespace = j.KubeNamespace
	if err := updater.CreateJob(job, j.kubeClient); err != nil {
		msg := fmt.Sprintf("create jenkins build job error: %v", err)
		j.Log.Error(msg)
		j.Task.TaskStatus = config.StatusFailed
		j.Task.Error = msg
		return
	} else {
		j.Log.Infof("succeed to create jenkins build job %s", j.JobName)
	}
	return
}

// Wait ...
func (j *JenkinsBuildPlugin) Wait(ctx context.Context) {
	jobStatus := waitJobEnd(ctx, j.TaskTimeout(), j.KubeNamespace, j.JobName, j.kubeClient, j.Log)
	jenkinsClient, err := gojenkins.CreateJenkins(nil, j.Task.JenkinsIntegration.URL, j.Task.JenkinsIntegration.Username, j.Task.JenkinsIntegration.Password).Init(ctx)
	if err != nil {
		j.SetStatus(config.StatusFailed)
	}
	job, err := jenkinsClient.GetJob(ctx, j.Task.JenkinsBuildArgs.JobName)
	if err != nil {
		j.SetStatus(config.StatusFailed)
	}
	build, err := job.GetLastCompletedBuild(ctx)
	if err != nil {
		j.SetStatus(config.StatusFailed)
	}
	jenkinsBuildStatus := j.matchStatus(jobStatus, build.GetResult())
	j.SetStatus(jenkinsBuildStatus)
}

func (j *JenkinsBuildPlugin) matchStatus(jobStatus config.Status, jenkinsBuildStatus string) config.Status {
	switch jenkinsBuildStatus {
	case "FAILURE":
		return config.StatusFailed
	}

	return jobStatus
}

func (j *JenkinsBuildPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
	jobLabel := &JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     fmt.Sprintf("%s", j.Type()),
		PipelineType: string(pipelineTask.Type),
	}

	// 清理用户取消和超时的任务
	defer func() {
		if j.Task.TaskStatus == config.StatusCancelled || j.Task.TaskStatus == config.StatusTimeout {
			if err := ensureDeleteJob(j.KubeNamespace, jobLabel, j.kubeClient); err != nil {
				j.Log.Error(err)
				j.Task.Error = err.Error()
			}
			return
		}
	}()

	// 保存实时日志到s3
	err := saveContainerLog(pipelineTask, j.KubeNamespace, j.FileName, jobLabel, j.kubeClient)
	if err != nil {
		j.Log.Error(err)
		j.Task.Error = err.Error()
		return
	}

	j.Task.LogFile = j.FileName
}

func (j *JenkinsBuildPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToJenkinsBuildTask(t)
	if err != nil {
		return err
	}
	j.Task = task
	return nil
}

// GetTask ...
func (j *JenkinsBuildPlugin) GetTask() interface{} {
	return j.Task
}

func (j *JenkinsBuildPlugin) IsTaskDone() bool {
	if j.Task.TaskStatus != config.StatusCreated && j.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

func (j *JenkinsBuildPlugin) IsTaskFailed() bool {
	if j.Task.TaskStatus == config.StatusFailed || j.Task.TaskStatus == config.StatusTimeout || j.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (j *JenkinsBuildPlugin) SetStartTime() {
	j.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (j *JenkinsBuildPlugin) SetEndTime() {
	j.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (j *JenkinsBuildPlugin) IsTaskEnabled() bool {
	return j.Task.Enabled
}

// ResetError ...
func (j *JenkinsBuildPlugin) ResetError() {
	j.Task.Error = ""
}
