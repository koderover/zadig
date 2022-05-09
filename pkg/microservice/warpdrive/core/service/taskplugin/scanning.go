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
	"fmt"
	zadigconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func InitializeScanningTaskPlugin(taskType config.TaskType) TaskPlugin {
	return &ScanPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
		clientset:  krkubeclient.Clientset(),
		restConfig: krkubeclient.RESTConfig(),
	}
}

type ScanPlugin struct {
	Name          config.TaskType
	KubeNamespace string
	JobName       string
	FileName      string
	kubeClient    client.Client
	clientset     kubernetes.Interface
	restConfig    *rest.Config
	Task          *task.Scanning
	Log           *zap.SugaredLogger
}

func (p *ScanPlugin) SetAckFunc(func()) {
}

const (
	ScanningTaskTimeout = 60 * 60 // 60 minutes
)

func (p *ScanPlugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	p.JobName = jobname
	p.FileName = filename
	p.Log = xl
}

func (p *ScanPlugin) Type() config.TaskType {
	return p.Name
}

func (p *ScanPlugin) Status() config.Status {
	return p.Task.Status
}

func (p *ScanPlugin) SetStatus(status config.Status) {
	p.Task.Status = status
}

func (p *ScanPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = ScanningTaskTimeout
	}
	return int(p.Task.Timeout)
}

func (p *ScanPlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	switch p.Task.ClusterID {
	case setting.LocalClusterID:
		p.KubeNamespace = zadigconfig.Namespace()
	default:
		p.KubeNamespace = setting.AttachedClusterNamespace

		crClient, clientset, restConfig, err := GetK8sClients(pipelineTask.ConfigPayload.HubServerAddr, p.Task.ClusterID)
		if err != nil {
			p.Log.Error(err)
			p.Task.Status = config.StatusFailed
			p.Task.Error = err.Error()
			return
		}

		p.kubeClient = crClient
		p.clientset = clientset
		p.restConfig = restConfig
	}

	additionalKV := make([]*task.KeyVal, 0)
	if p.Task.SonarInfo != nil {
		additionalKV = append(additionalKV, &task.KeyVal{
			Key:   "SONAR_HOST_URL",
			Value: p.Task.SonarInfo.ServerAddress,
		})
		additionalKV = append(additionalKV, &task.KeyVal{
			Key:   "SONAR_LOGIN",
			Value: p.Task.SonarInfo.Token,
		})
	}

	jobCtx := JobCtxBuilder{
		JobName:     p.JobName,
		PipelineCtx: pipelineCtx,
		JobCtx: task.JobCtx{
			EnvVars: additionalKV,
		},
	}

	jobCtxBytes, err := yaml.Marshal(jobCtx.BuildReaperContext(pipelineTask, serviceName))
	if err != nil {
		msg := fmt.Sprintf("cannot reaper.Context data: %v", err)
		p.Log.Error(msg)
		p.Task.Status = config.StatusFailed
		p.Task.Error = msg
		return
	}

	jobLabel := &JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     string(p.Type()),
		PipelineType: string(pipelineTask.Type),
	}

	if err := ensureDeleteConfigMap(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
		p.Log.Error(err)
		p.Task.Status = config.StatusFailed
		p.Task.Error = err.Error()
		return
	}

	if err := createJobConfigMap(p.KubeNamespace, p.JobName, jobLabel, string(jobCtxBytes), p.kubeClient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		p.Log.Error(msg)
		p.Task.Status = config.StatusFailed
		p.Task.Error = msg
		return
	}

	// search namespace should also include desired namespace
	job, err := buildJobWithLinkedNs(
		p.Type(), p.Task.ImageInfo, p.JobName, serviceName, p.Task.ClusterID, pipelineTask.ConfigPayload.Test.KubeNamespace, p.Task.ResReq, p.Task.ResReqSpec, pipelineCtx, pipelineTask, p.Task.Registries,
		p.KubeNamespace,
		// this is a useless field, so we will just leave it empty
		"",
	)

	job.Namespace = p.KubeNamespace

	if err != nil {
		msg := fmt.Sprintf("create scanning job context error: %v", err)
		p.Log.Error(msg)
		p.Task.Status = config.StatusFailed
		p.Task.Error = msg
		return
	}

	if err := ensureDeleteJob(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
		msg := fmt.Sprintf("delete scanning job error: %v", err)
		p.Log.Error(msg)
		p.Task.Status = config.StatusFailed
		p.Task.Error = msg
		return
	}

	// 将集成到KodeRover的私有镜像仓库的访问权限设置到namespace中
	if err := createOrUpdateRegistrySecrets(p.KubeNamespace, pipelineTask.ConfigPayload.RegistryID, p.Task.Registries, p.kubeClient); err != nil {
		p.Log.Errorf("create secret error: %v", err)
	}
	if err := updater.CreateJob(job, p.kubeClient); err != nil {
		msg := fmt.Sprintf("create testing job error: %v", err)
		p.Log.Error(msg)
		p.Task.Status = config.StatusFailed
		p.Task.Error = msg
		return
	}

	p.Task.Status = waitJobReady(ctx, p.KubeNamespace, p.JobName, p.kubeClient, p.Log)
}

func (p *ScanPlugin) Wait(ctx context.Context) {

}

func (p *ScanPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {

}

func (p *ScanPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToScanningTask(t)
	if err != nil {
		return err
	}
	p.Task = task

	return nil
}

func (p *ScanPlugin) GetTask() interface{} {
	return p.Task
}

func (p *ScanPlugin) IsTaskDone() bool {
	if p.Task.Status != config.StatusCreated && p.Task.Status != config.StatusRunning {
		return true
	}
	return false
}

func (p *ScanPlugin) IsTaskFailed() bool {
	if p.Task.Status == config.StatusFailed || p.Task.Status == config.StatusTimeout || p.Task.Status == config.StatusCancelled {
		return true
	}
	return false
}

// since scan is a standalone job, a subtask should not have startTime and endtime

func (p *ScanPlugin) SetStartTime() {
	//p.Task.S = time.Now().Unix()
}

func (p *ScanPlugin) SetEndTime() {
	//p.Task.EndTime = time.Now().Unix()
}

// scan job is a standalone job so it is always enabled
func (p *ScanPlugin) IsTaskEnabled() bool {
	return true
}

func (p *ScanPlugin) ResetError() {
	p.Task.Error = ""
}
