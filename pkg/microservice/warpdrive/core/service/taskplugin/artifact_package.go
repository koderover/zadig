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

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/label"
	"github.com/koderover/zadig/pkg/tool/kube/podexec"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

// InitializeArtifactPackagePlugin to ini
func InitializeArtifactPackagePlugin(taskType config.TaskType) TaskPlugin {
	return &ArtifactPackageTaskPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
	}
}

type ArtifactPackageTaskPlugin struct {
	Name          config.TaskType
	KubeNamespace string
	JobName       string
	FileName      string
	kubeClient    client.Client
	Task          *task.ArtifactPackage
	AckFunc       func()
	Log           *zap.SugaredLogger

	RealTimeProgress string
}

const (
	// ArtifactPackageBuildImageTaskTimeout ...
	ArtifactPackageBuildImageTaskTimeout = 60 * 5 // 5 minutes
)

// Init ...
func (p *ArtifactPackageTaskPlugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	p.JobName = jobname
	p.FileName = filename
	// SetLogger ...
	p.Log = xl
}

func (p *ArtifactPackageTaskPlugin) SetAckFunc(f func()) {
	p.AckFunc = f
}

// Type ...
func (p *ArtifactPackageTaskPlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *ArtifactPackageTaskPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *ArtifactPackageTaskPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

func (p *ArtifactPackageTaskPlugin) SetProgress(progress string) {
	p.Task.Progress = progress
}

// TaskTimeout ...
func (p *ArtifactPackageTaskPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = ArtifactPackageBuildImageTaskTimeout
	}
	return p.Task.Timeout
}

func (p *ArtifactPackageTaskPlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	p.KubeNamespace = pipelineTask.ConfigPayload.Build.KubeNamespace

	sourceRegistries := make([]*types.DockerRegistry, 0)
	for _, registryID := range pipelineTask.ArtifactPackageTaskArgs.SourceRegistries {
		if registry, ok := pipelineTask.ConfigPayload.RepoConfigs[registryID]; ok {
			sourceRegistries = append(sourceRegistries, &types.DockerRegistry{
				RegistryID: registryID,
				Host:       registry.RegAddr,
				Namespace:  registry.Namespace,
				UserName:   registry.AccessKey,
				Password:   registry.SecretKey,
			})
		}
	}

	targetRegistries := make([]*types.DockerRegistry, 0)
	for _, registryID := range pipelineTask.ArtifactPackageTaskArgs.TargetRegistries {
		if registry, ok := pipelineTask.ConfigPayload.RepoConfigs[registryID]; ok {
			targetRegistries = append(targetRegistries, &types.DockerRegistry{
				RegistryID: registryID,
				Host:       registry.RegAddr,
				Namespace:  registry.Namespace,
				UserName:   registry.AccessKey,
				Password:   registry.SecretKey,
			})
		}
	}

	jobCtx := &types.ArtifactPackagerContext{
		JobType:          setting.BuildChartPackage,
		ProgressFile:     setting.ProgressFile,
		Images:           pipelineTask.ArtifactPackageTaskArgs.Images,
		SourceRegistries: sourceRegistries,
		TargetRegistries: targetRegistries,
	}

	jobCtxBytes, err := yaml.Marshal(jobCtx)
	if err != nil {
		msg := fmt.Sprintf("cannot mashal predetor.Context data: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	p.Task.Error = ""

	jobLabel := &label.JobLabel{
		PipelineName: pipelineTask.PipelineName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     string(p.Type()),
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
	}
	p.Log.Infof("succeed to create cm for artifact package job %s", p.JobName)

	job, err := buildJob(p.Type(), pipelineTask.ConfigPayload.Release.PackagerImage, p.JobName, serviceName, "", pipelineTask.ConfigPayload.Build.KubeNamespace, setting.MinRequest, setting.LowRequestSpec, pipelineCtx, pipelineTask, []*task.RegistryNamespace{})
	if err != nil {
		msg := fmt.Sprintf("create release artifact package job context error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	if err := ensureDeleteJob(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
		msg := fmt.Sprintf("delete release artifact package job error: %v", err)
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
	}
	p.Log.Infof("succeed to create artifact package job %s", p.JobName)
}

// SetTask ...
func (p *ArtifactPackageTaskPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToArtifactTask(t)
	if err != nil {
		return err
	}
	p.Task = task
	return nil
}

// GetTask ...
func (p *ArtifactPackageTaskPlugin) GetTask() interface{} {
	return p.Task
}

// Wait ...
func (p *ArtifactPackageTaskPlugin) Wait(ctx context.Context) {

	status := p.waitJobStart()
	if status != config.StatusRunning {
		p.SetStatus(status)
		return
	}

	status = p.waitJobEnd(ctx, p.TaskTimeout())
	p.SetProgress(p.RealTimeProgress)
	p.SetStatus(status)
}

func (p *ArtifactPackageTaskPlugin) waitJobStart() config.Status {
	namespace, jobName, kubeClient, xl := p.KubeNamespace, p.JobName, p.kubeClient, p.Log
	xl.Infof("wait job to start: %s/%s", namespace, jobName)
	podTimeout := time.After(120 * time.Second)
	var started bool
	for {
		select {
		case <-podTimeout:
			return config.StatusTimeout
		default:
			job, _, err := getter.GetJob(namespace, jobName, kubeClient)
			if err != nil {
				xl.Errorf("get job failed, namespace:%s, jobName:%s, err:%v", namespace, jobName, err)
			}
			if job != nil {
				started = job.Status.Active > 0
			}
		}
		if started {
			break
		}

		time.Sleep(time.Second)
	}
	return config.StatusRunning
}

func (p *ArtifactPackageTaskPlugin) waitJobEnd(ctx context.Context, taskTimeout int) (status config.Status) {
	namespace, jobName, kubeClient, xl := p.KubeNamespace, p.JobName, p.kubeClient, p.Log
	xl.Infof("wait job to start: %s/%s", namespace, jobName)
	timeout := time.After(time.Duration(taskTimeout) * time.Second)

	// 等待job 运行结束
	xl.Infof("wait job to end: %s %s", namespace, jobName)
	for {
		select {
		case <-ctx.Done():
			return config.StatusCancelled

		case <-timeout:
			return config.StatusTimeout

		default:
			job, found, err := getter.GetJob(namespace, jobName, kubeClient)
			if err != nil || !found {
				xl.Errorf("failed to get pod with label job-name=%s %v", jobName, err)
				return config.StatusFailed
			}
			// pod is still running
			if job.Status.Active != 0 {

				pods, err := getter.ListPods(namespace, labels.Set{"job-name": jobName}.AsSelector(), kubeClient)
				if err != nil {
					xl.Errorf("failed to find pod with label job-name=%s %v", jobName, err)
					return config.StatusFailed
				}

				var done bool
				for _, pod := range pods {
					ipod := wrapper.Pod(pod)
					if ipod.Pending() {
						continue
					}
					if ipod.Failed() {
						return config.StatusFailed
					}

					if !ipod.Finished() {
						progressInfo, err := getProgressInfo(namespace, ipod.Name, ipod.ContainerNames()[0], setting.ProgressFile)
						if err != nil {
							xl.Infof("failed to check dog food file %s %v", pods[0].Name, err)
							break
						}
						p.RealTimeProgress = progressInfo
						xl.Infof("find progress info %s", progressInfo)
						p.SetProgress(progressInfo)
						p.SetStatus(config.StatusRunning)

						if p.AckFunc != nil {
							p.AckFunc()
						}
					} else {
						done = true
					}
				}

				if done {
					return config.StatusPassed
				}
			} else if job.Status.Succeeded != 0 {
				return config.StatusPassed
			} else {
				return config.StatusFailed
			}
		}

		time.Sleep(time.Second * 5)
	}

}

// Complete ...
func (p *ArtifactPackageTaskPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
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

	// 保存实时日志到s3
	err := saveContainerLog(pipelineTask, p.KubeNamespace, "", p.FileName, jobLabel, p.kubeClient)
	if err != nil {
		p.Log.Error(err)
		p.Task.Error = err.Error()
		return
	}

	p.Task.LogFile = p.JobName
}

// IsTaskDone ...
func (p *ArtifactPackageTaskPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *ArtifactPackageTaskPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *ArtifactPackageTaskPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *ArtifactPackageTaskPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *ArtifactPackageTaskPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *ArtifactPackageTaskPlugin) ResetError() {
	p.Task.Error = ""
}

func getProgressInfo(namespace string, pod string, container string, progressInfoFile string) (string, error) {
	stdOut, stdErr, success, err := podexec.ExecWithOptions(podexec.ExecOptions{
		Command:       []string{"cat", progressInfoFile},
		Namespace:     namespace,
		PodName:       pod,
		ContainerName: container,
	})

	if err != nil {
		return "", err
	}
	if len(stdErr) != 0 {
		return "", fmt.Errorf(stdErr)
	}
	if !success {
		return "", fmt.Errorf("pod exec execute failed")
	}
	return stdOut, nil
}
