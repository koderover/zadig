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

package jobcontroller

import (
	"context"
	"fmt"
	"strings"
	"time"

	zadigconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/dockerhost"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DindServer              = "dind"
	KoderoverAgentNamespace = "koderover-agent"
)

type FreestyleJobCtl struct {
	job         *commonmodels.JobTask
	jobName     string
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeclient  crClient.Client
	clientset   kubernetes.Interface
	restConfig  *rest.Config
	paths       *string
	ack         func()
}

func NewFreestyleJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *FreestyleJobCtl {
	paths := ""
	return &FreestyleJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		paths:       &paths,
		jobName:     fmt.Sprintf("%s-%s-%d", job.Name, job.JobType, workflowCtx.TaskID),
	}
}

func (c *FreestyleJobCtl) Run(ctx context.Context) {
	// get kube client
	hubServerAddr := config.HubServerAddress()
	switch c.job.Properties.ClusterID {
	case setting.LocalClusterID:
		c.job.Properties.Namespace = zadigconfig.Namespace()
		c.kubeclient = krkubeclient.Client()
		c.clientset = krkubeclient.Clientset()
		c.restConfig = krkubeclient.RESTConfig()
	default:
		c.job.Properties.Namespace = setting.AttachedClusterNamespace

		crClient, clientset, restConfig, err := GetK8sClients(hubServerAddr, c.job.Properties.ClusterID)
		if err != nil {
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.job.EndTime = time.Now().Unix()
			return
		}
		c.kubeclient = crClient
		c.clientset = clientset
		c.restConfig = restConfig
	}

	// decide which docker host to use.
	// TODO: do not use code in warpdrive moudule, should move to a public place
	dockerhosts := dockerhost.NewDockerHosts(hubServerAddr, c.logger)
	c.job.Properties.DockerHost = dockerhosts.GetBestHost(dockerhost.ClusterID(c.job.Properties.ClusterID), "")

	// not local cluster
	var (
		replaceDindServer = "." + DindServer
		dockerHost        = ""
	)

	if c.job.Properties.ClusterID != "" && c.job.Properties.ClusterID != setting.LocalClusterID {
		if strings.Contains(c.job.Properties.DockerHost, config.Namespace()) {
			// replace namespace only
			dockerHost = strings.Replace(c.job.Properties.DockerHost, config.Namespace(), KoderoverAgentNamespace, 1)
		} else {
			// add namespace
			dockerHost = strings.Replace(c.job.Properties.DockerHost, replaceDindServer, replaceDindServer+"."+KoderoverAgentNamespace, 1)
		}
	} else if c.job.Properties.ClusterID == "" || c.job.Properties.ClusterID == setting.LocalClusterID {
		if !strings.Contains(c.job.Properties.DockerHost, config.Namespace()) {
			// add namespace
			dockerHost = strings.Replace(c.job.Properties.DockerHost, replaceDindServer, replaceDindServer+"."+config.Namespace(), 1)
		}
	}

	c.job.Properties.DockerHost = dockerHost

	// TODO: inject vars like task_id and etc, passing them into c.job.Properties.Args
	envNameVar := &commonmodels.KeyVal{Key: "ENV_NAME", Value: c.job.Properties.Namespace, IsCredential: false}
	c.job.Properties.Args = append(c.job.Properties.Args, envNameVar)

	jobCtxBytes, err := yaml.Marshal(BuildJobExcutorContext(c.job, c.workflowCtx, c.logger))
	if err != nil {
		msg := fmt.Sprintf("cannot Jobexcutor.Context data: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}

	jobLabel := &JobLabel{
		WorkflowName: c.workflowCtx.WorkflowName,
		TaskID:       c.workflowCtx.TaskID,
		JobType:      string(c.job.JobType),
		JobName:      c.job.Name,
	}
	if err := ensureDeleteConfigMap(c.job.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
		c.logger.Error(err)
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return
	}

	if err := createJobConfigMap(
		c.job.Properties.Namespace, c.jobName, jobLabel, string(jobCtxBytes), c.kubeclient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}

	c.logger.Infof("succeed to create cm for job %s", c.jobName)

	// TODO: do not use default image
	jobImage := getBaseImage(c.job.Properties.BuildOS, c.job.Properties.ImageFrom)
	// jobImage := "ccr.ccs.tencentyun.com/koderover-rc/job-excutor:guoyu-test2"
	// jobImage := getReaperImage(config.ReaperImage(), c.job.Properties.BuildOS)

	//Resource request default value is LOW
	job, err := buildJob(c.job.JobType, jobImage, c.jobName, c.job.Properties.ClusterID, c.job.Properties.Namespace, c.job.Properties.ResourceRequest, setting.DefaultRequestSpec, c.job, c.workflowCtx, nil)
	if err != nil {
		msg := fmt.Sprintf("create job context error: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}

	job.Namespace = c.job.Properties.Namespace

	if err := ensureDeleteJob(c.job.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
		msg := fmt.Sprintf("delete job error: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}

	// 将集成到KodeRover的私有镜像仓库的访问权限设置到namespace中
	// if err := createOrUpdateRegistrySecrets(p.KubeNamespace, pipelineTask.ConfigPayload.RegistryID, p.Task.Registries, p.kubeClient); err != nil {
	// 	msg := fmt.Sprintf("create secret error: %v", err)
	// 	p.Log.Error(msg)
	// 	p.Task.TaskStatus = config.StatusFailed
	// 	p.Task.Error = msg
	// 	p.SetBuildStatusCompleted(config.StatusFailed)
	// 	return
	// }
	if err := updater.CreateJob(job, c.kubeclient); err != nil {
		msg := fmt.Sprintf("create job error: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}
	c.logger.Infof("succeed to create job %s", c.jobName)
}

func (c *FreestyleJobCtl) Wait(ctx context.Context) {
	status := waitJobEndWithFile(ctx, int(c.job.Properties.Timeout), c.job.Properties.Namespace, c.jobName, true, c.kubeclient, c.clientset, c.restConfig, c.logger)
	c.job.Status = status
}

func (c *FreestyleJobCtl) Complete(ctx context.Context) {
	jobLabel := &JobLabel{
		WorkflowName: c.workflowCtx.WorkflowName,
		TaskID:       c.workflowCtx.TaskID,
		JobType:      string(c.job.JobType),
		JobName:      c.job.Name,
	}

	// 清理用户取消和超时的任务
	defer func() {
		go func() {
			if err := ensureDeleteJob(c.job.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
				c.logger.Error(err)
			}
			if err := ensureDeleteConfigMap(c.job.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
				c.logger.Error(err)
			}
		}()
	}()

	// get job outputs info from pod terminate message.
	outputs, err := getJobOutput(c.job.Properties.Namespace, c.job.Name, jobLabel, c.kubeclient)
	if err != nil {
		c.logger.Error(err)
		c.job.Error = err.Error()
	}

	// write jobs output info to globalcontext so other job can use like this $(jobName.outputName)
	for _, output := range outputs {
		c.workflowCtx.GlobalContextSet(strings.Join([]string{c.job.Name, output.Name}, "."), output.Value)
	}

	if err := saveContainerLog(c.job, c.workflowCtx.WorkflowName, c.workflowCtx.TaskID, jobLabel, c.kubeclient); err != nil {
		c.logger.Error(err)
		c.job.Error = err.Error()
		return
	}
}

func BuildJobExcutorContext(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger) *JobContext {
	var envVars, secretEnvVars []string
	for _, env := range job.Properties.Args {
		if env.IsCredential {
			secretEnvVars = append(secretEnvVars, strings.Join([]string{env.Key, env.Value}, "="))
			continue
		}
		envVars = append(envVars, strings.Join([]string{env.Key, env.Value}, "="))
	}

	outputs := []string{}
	for _, output := range job.Outputs {
		outputs = append(outputs, output.Name)
	}

	return &JobContext{
		Name:         job.Name,
		Envs:         envVars,
		SecretEnvs:   secretEnvVars,
		WorkflowName: workflowCtx.WorkflowName,
		TaskID:       workflowCtx.TaskID,
		Outputs:      outputs,
		Steps:        job.Steps,
		Paths:        job.Properties.Paths,
	}
}
