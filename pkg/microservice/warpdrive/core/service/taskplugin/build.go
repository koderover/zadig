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
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	zadigconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/common"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/label"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/types"
)

const (
	BuildTaskV2Timeout = 60 * 60 * 3 // 180 minutes
)

// InitializeBuildTaskPlugin to initialize build task plugin, and return reference
func InitializeBuildTaskPlugin(taskType config.TaskType) TaskPlugin {
	return &BuildTaskPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
		clientset:  krkubeclient.Clientset(),
		restConfig: krkubeclient.RESTConfig(),
		apiReader:  krkubeclient.APIReader(),
	}
}

// BuildTaskPlugin is Plugin, name should be compatible with task type
type BuildTaskPlugin struct {
	Name          config.TaskType
	KubeNamespace string
	JobName       string
	FileName      string
	kubeClient    client.Client
	clientset     kubernetes.Interface
	restConfig    *rest.Config
	apiReader     client.Reader
	Task          *task.Build
	Log           *zap.SugaredLogger
	Timeout       <-chan time.Time

	ack func()
}

func (p *BuildTaskPlugin) SetAckFunc(ack func()) {
	p.ack = ack
}

func (p *BuildTaskPlugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	p.JobName = jobname
	p.Log = xl
	p.FileName = filename
}

func (p *BuildTaskPlugin) Type() config.TaskType {
	return p.Name
}

func (p *BuildTaskPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

func (p *BuildTaskPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// Note: Unit of input `p.Task.Timeout` is `minutes` and convert it to `seconds` internally.
// TODO: Using time implicitly is easy to generate bugs. We need use `time.Duration` instead.
func (p *BuildTaskPlugin) TaskTimeout() int {
	p.Log.Infof("IsRestart: %t. TaskTimeout: %d", p.Task.IsRestart, p.Task.Timeout)

	if p.Task.Timeout == 0 {
		p.Task.Timeout = BuildTaskV2Timeout
	} else {
		if !p.Task.IsRestart {
			p.Task.Timeout = p.Task.Timeout * 60
		}
	}
	return p.Task.Timeout
}

// Note: This is a temporary function to be compatible with the `TaskTimeout()` method's process of time.
// TODO: Remove this function after `TaskTimeout` uses `time.Duration`.
func (p *BuildTaskPlugin) tmpSetTaskTimeout(durationInSeconds int) {
	p.Task.Timeout = int(math.Ceil(float64(durationInSeconds) / 60.0))
	p.Task.IsRestart = false
}

func (p *BuildTaskPlugin) SetBuildStatusCompleted(status config.Status) {
	p.Task.BuildStatus.Status = status
	p.Task.BuildStatus.EndTime = time.Now().Unix()
}

// TODO: Binded Archive File logic
func (p *BuildTaskPlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	if p.Task.CacheEnable && !pipelineTask.ConfigPayload.ResetCache {
		pipelineCtx.CacheEnable = true
		pipelineCtx.Cache = p.Task.Cache
		pipelineCtx.CacheDirType = p.Task.CacheDirType
		pipelineCtx.CacheUserDir = p.Task.CacheUserDir
	} else {
		pipelineCtx.CacheEnable = false
	}

	// TODO: Since the namespace field has been used continuously since v1.10.0, the processing logic related to namespace needs to
	// be deleted in v1.11.0.
	switch p.Task.ClusterID {
	case setting.LocalClusterID:
		p.KubeNamespace = zadigconfig.Namespace()
	default:
		p.KubeNamespace = setting.AttachedClusterNamespace

		crClient, clientset, restConfig, apiReader, err := GetK8sClients(pipelineTask.ConfigPayload.HubServerAddr, p.Task.ClusterID)
		if err != nil {
			p.Log.Error(err)
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = err.Error()
			p.SetBuildStatusCompleted(config.StatusFailed)
			return
		}

		p.kubeClient = crClient
		p.clientset = clientset
		p.restConfig = restConfig
		p.apiReader = apiReader
	}

	dockerhosts := common.NewDockerHosts(pipelineTask.ConfigPayload.HubServerAddr, p.Log)
	pipelineTask.DockerHost = dockerhosts.GetBestHost(common.ClusterID(p.Task.ClusterID), serviceName)

	// not local cluster
	var (
		replaceDindServer = "." + DindServer
		dockerHost        = ""
	)

	if p.Task.ClusterID != "" && p.Task.ClusterID != setting.LocalClusterID {
		if strings.Contains(pipelineTask.DockerHost, pipelineTask.ConfigPayload.Build.KubeNamespace) {
			// replace namespace only
			dockerHost = strings.Replace(pipelineTask.DockerHost, pipelineTask.ConfigPayload.Build.KubeNamespace, KoderoverAgentNamespace, 1)
		} else {
			// add namespace
			dockerHost = strings.Replace(pipelineTask.DockerHost, replaceDindServer, replaceDindServer+"."+KoderoverAgentNamespace, 1)
		}
	} else if p.Task.ClusterID == "" || p.Task.ClusterID == setting.LocalClusterID {
		if !strings.Contains(pipelineTask.DockerHost, pipelineTask.ConfigPayload.Build.KubeNamespace) {
			// add namespace
			dockerHost = strings.Replace(pipelineTask.DockerHost, replaceDindServer, replaceDindServer+"."+pipelineTask.ConfigPayload.Build.KubeNamespace, 1)
		}
	}
	pipelineCtx.DockerHost = dockerHost

	pipelineCtx.UseHostDockerDaemon = p.Task.UseHostDockerDaemon

	if pipelineTask.Type == config.WorkflowType {
		envName := pipelineTask.WorkflowArgs.Namespace
		envNameVar := &task.KeyVal{Key: "ENV_NAME", Value: envName, IsCredential: false}
		p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, envNameVar)
	} else if pipelineTask.Type == config.ServiceType {
		// Note:
		// In cloud host scenarios, this type of task is performed when creating the environment.
		// This type of task does not occur in other scenarios.

		envName := pipelineTask.ServiceTaskArgs.Namespace
		envNameVar := &task.KeyVal{Key: "ENV_NAME", Value: envName, IsCredential: false}
		p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, envNameVar)
	}

	// Note: When 'pipelinetask.type == config.ServiceType', it may be `nil`.
	if pipelineTask.WorkflowArgs != nil {
		p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, &task.KeyVal{Key: "WORKFLOW", Value: pipelineTask.WorkflowArgs.WorkflowName})
		p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, &task.KeyVal{Key: "PROJECT", Value: pipelineTask.WorkflowArgs.ProductTmplName})
	}

	taskIDVar := &task.KeyVal{Key: "TASK_ID", Value: strconv.FormatInt(pipelineTask.TaskID, 10), IsCredential: false}
	p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, taskIDVar)

	privateKeys := sets.String{}
	for _, privateKey := range pipelineTask.ConfigPayload.PrivateKeys {
		privateKeys.Insert(privateKey.Name)
	}

	privateKeysVar := &task.KeyVal{Key: "AGENTS", Value: strings.Join(privateKeys.List(), ","), IsCredential: false}
	p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, privateKeysVar)

	// env host ips
	for envName, HostIPs := range p.Task.EnvHostInfo {
		envHostKeysVar := &task.KeyVal{Key: envName + "_HOST_IPs", Value: strings.Join(HostIPs, ","), IsCredential: false}
		p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, envHostKeysVar)
	}

	// env host names
	for envName, names := range p.Task.EnvHostNames {
		envHostKeysVar := &task.KeyVal{Key: envName + "_HOST_NAMEs", Value: strings.Join(names, ","), IsCredential: false}
		p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, envHostKeysVar)
	}

	// ARTIFACT
	if p.Task.JobCtx.FileArchiveCtx != nil {
		var workspace = "/workspace"
		if pipelineTask.ConfigPayload.ClassicBuild {
			workspace = pipelineCtx.Workspace
		}
		artifactKeysVar := &task.KeyVal{Key: "ARTIFACT", Value: fmt.Sprintf("%s/%s/%s", workspace, p.Task.JobCtx.FileArchiveCtx.FileLocation, p.Task.JobCtx.FileArchiveCtx.FileName), IsCredential: false}
		p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, artifactKeysVar)
	}

	//instantiates variables like ${<REPO>_BRANCH} ${${REPO_index}_BRANCH} ..
	p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, InstantiateBuildSysVariables(&p.Task.JobCtx)...)

	// Note: Currently, `SERVICE` in the environment variable represents a service module.
	// Since variable rendering is required next, the `SERVICE_MODULE` environment variable is added to accurately
	// characterize the service module.
	// Do not use 'p.task. ServiceName' as the value because it is in the 'ServiceModule_ServiceName' format.
	for _, env := range p.Task.JobCtx.EnvVars {
		if env.Key == "SERVICE" {
			p.Task.JobCtx.EnvVars = append(p.Task.JobCtx.EnvVars, &task.KeyVal{Key: "SERVICE_MODULE", Value: env.Value})
			break
		}
	}

	// Since we allow users to use custom environment variables, variable resolution is required.
	if pipelineCtx.CacheEnable && pipelineCtx.Cache.MediumType == types.NFSMedium {
		pipelineCtx.CacheUserDir = p.renderEnv(pipelineCtx.CacheUserDir)
		pipelineCtx.Cache.NFSProperties.Subpath = p.renderEnv(pipelineCtx.Cache.NFSProperties.Subpath)
	}

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

	jobObj, jobExist, err := checkJobExists(ctx, p.KubeNamespace, jobLabel, p.kubeClient)
	if err != nil {
		msg := fmt.Sprintf("failed to check whether Job exist for %s:%d: %s", pipelineTask.PipelineName, pipelineTask.TaskID, err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		p.SetBuildStatusCompleted(config.StatusFailed)
		return
	}
	if jobExist {
		p.Log.Infof("Job %s:%d eixsts.", pipelineTask.PipelineName, pipelineTask.TaskID)

		p.JobName = jobObj.Name

		// If the code is executed at this point, it indicates that the `wd` instance that executed the Job has been restarted and the
		// Job timeout period needs to be corrected.
		//
		// Rule of reseting timeout: `timeout - (now - start_time_of_job) + compensate_duration`
		// For now, `compensate_duration` is 2min.
		taskTimeout := p.TaskTimeout()
		elaspedTime := time.Now().Unix() - jobObj.Status.StartTime.Time.Unix()
		timeout := taskTimeout + 120 - int(elaspedTime)
		p.Log.Infof("Timeout before normalization: %d seconds", timeout)
		if timeout < 0 {
			// That shouldn't happen in theory, but a protection is needed.
			timeout = 0
		}

		p.Log.Infof("Timeout after normalization: %d seconds", timeout)
		p.tmpSetTaskTimeout(timeout)
	}

	_, cmExist, err := checkConfigMapExists(ctx, p.KubeNamespace, jobLabel, p.kubeClient)
	if err != nil {
		msg := fmt.Sprintf("failed to check whether ConfigMap exist for %s:%d: %s", pipelineTask.PipelineName, pipelineTask.TaskID, err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		p.SetBuildStatusCompleted(config.StatusFailed)
		return
	}

	if !cmExist {
		p.Log.Infof("ConfigMap for Job %s:%d does not exist. Create.", pipelineTask.PipelineName, pipelineTask.TaskID)

		err = createJobConfigMap(p.KubeNamespace, p.JobName, jobLabel, string(jobCtxBytes), p.kubeClient)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			msg := fmt.Sprintf("createJobConfigMap error: %v", err)
			p.Log.Error(msg)
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = msg
			p.SetBuildStatusCompleted(config.StatusFailed)
			return
		}
	}
	p.Log.Infof("succeed to create cm for build job %s", p.JobName)

	if !jobExist {
		p.Log.Infof("Job %s:%d does not exist. Create.", pipelineTask.PipelineName, pipelineTask.TaskID)

		jobImage := getReaperImage(pipelineTask.ConfigPayload.Release.ReaperImage, p.Task.BuildOS, p.Task.ImageFrom)
		p.Task.Registries = getMatchedRegistries(jobImage, p.Task.Registries)

		//Resource request default value is LOW

		job, err := buildJob(p.Type(), jobImage, p.JobName, serviceName, p.Task.ClusterID, pipelineTask.ConfigPayload.Build.KubeNamespace, p.Task.ResReq, p.Task.ResReqSpec, pipelineCtx, pipelineTask, p.Task.Registries)
		if err != nil {
			msg := fmt.Sprintf("create build job context error: %v", err)
			p.Log.Error(msg)
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = msg
			p.SetBuildStatusCompleted(config.StatusFailed)
			return
		}

		job.Namespace = p.KubeNamespace

		// Set imagePullSecrets of the private image registry integrated with KodeRover into the namespace.
		if err := createOrUpdateRegistrySecrets(p.KubeNamespace, pipelineTask.ConfigPayload.RegistryID, p.Task.Registries, p.kubeClient); err != nil {
			msg := fmt.Sprintf("create secret error: %v", err)
			p.Log.Error(msg)
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = msg
			p.SetBuildStatusCompleted(config.StatusFailed)
			return
		}

		err = updater.CreateJob(job, p.kubeClient)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			msg := fmt.Sprintf("create build job error: %v", err)
			p.Log.Error(msg)
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = msg
			p.SetBuildStatusCompleted(config.StatusFailed)
			return
		}
	}
	p.Log.Infof("succeed to create build job %s", p.JobName)

	p.Timeout = time.After(time.Duration(p.TaskTimeout()) * time.Second)
	p.Task.TaskStatus, err = waitJobReady(ctx, p.KubeNamespace, p.JobName, p.kubeClient, p.apiReader, p.Timeout, p.Log)
	if err != nil {
		p.Task.Error = err.Error()
	}
}

func (p *BuildTaskPlugin) Wait(ctx context.Context) {
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

func (p *BuildTaskPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
	jobLabel := &label.JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     string(p.Type()),
		PipelineType: string(pipelineTask.Type),
	}

	// Clean up tasks that user canceled or timed out.
	defer func() {
		go func() {
			if err := ensureDeleteJob(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
				p.Log.Error(err)
			}

			if err := ensureDeleteConfigMap(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
				p.Log.Error(err)
			}
		}()
	}()

	err := saveContainerLog(pipelineTask, p.KubeNamespace, p.Task.ClusterID, p.FileName, jobLabel, p.kubeClient)
	if err != nil {
		p.Log.Error(err)
		if p.Task.Error == "" {
			p.Task.Error = err.Error()
		}
		return
	}

	p.Task.LogFile = p.FileName
}

func (p *BuildTaskPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToBuildTask(t)
	if err != nil {
		return err
	}
	p.Task = task

	return nil
}

func (p *BuildTaskPlugin) GetTask() interface{} {
	return p.Task
}

func (p *BuildTaskPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

func (p *BuildTaskPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

func (p *BuildTaskPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

func (p *BuildTaskPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

func (p *BuildTaskPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

func (p *BuildTaskPlugin) ResetError() {
	p.Task.Error = ""
}

// Note: Since there are few environment variables and few variables to be replaced,
// this method is temporarily used.
func (p *BuildTaskPlugin) renderEnv(data string) string {
	mapper := func(data string) string {
		for _, envar := range p.Task.JobCtx.EnvVars {
			if data != envar.Key {
				continue
			}

			return envar.Value
		}

		return fmt.Sprintf("$%s", data)
	}

	return os.Expand(data, mapper)
}
