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
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	zadigconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/workflowcontroller/stepcontroller"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/dockerhost"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

const (
	DindServer              = "dind"
	KoderoverAgentNamespace = "koderover-agent"
)

type FreestyleJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeclient  crClient.Client
	clientset   kubernetes.Interface
	restConfig  *rest.Config
	apiServer   crClient.Reader
	paths       *string
	jobTaskSpec *commonmodels.JobTaskFreestyleSpec
	ack         func()
}

func NewFreestyleJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *FreestyleJobCtl {
	paths := ""
	jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &FreestyleJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		paths:       &paths,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *FreestyleJobCtl) Clean(ctx context.Context) {}

func (c *FreestyleJobCtl) Run(ctx context.Context) {
	if err := c.prepare(ctx); err != nil {
		return
	}
	if err := c.run(ctx); err != nil {
		return
	}
	c.wait(ctx)
	c.complete(ctx)
}

func (c *FreestyleJobCtl) prepare(ctx context.Context) error {
	// set default timeout
	if c.jobTaskSpec.Properties.Timeout <= 0 {
		c.jobTaskSpec.Properties.Timeout = 600
	}
	// set default resource
	if c.jobTaskSpec.Properties.ResourceRequest == setting.Request("") {
		c.jobTaskSpec.Properties.ResourceRequest = setting.MinRequest
	}
	// set default resource
	if c.jobTaskSpec.Properties.ClusterID == "" {
		c.jobTaskSpec.Properties.ClusterID = setting.LocalClusterID
	}
	// init step configration.
	if err := stepcontroller.PrepareSteps(ctx, c.workflowCtx, &c.jobTaskSpec.Properties.Paths, c.job.Name, c.jobTaskSpec.Steps, c.logger); err != nil {
		logError(c.job, err.Error(), c.logger)
		return err
	}
	c.ack()
	return nil
}

func (c *FreestyleJobCtl) run(ctx context.Context) error {
	// get kube client
	hubServerAddr := config.HubServerAddress()
	switch c.jobTaskSpec.Properties.ClusterID {
	case setting.LocalClusterID:
		c.jobTaskSpec.Properties.Namespace = zadigconfig.Namespace()
		c.kubeclient = krkubeclient.Client()
		c.clientset = krkubeclient.Clientset()
		c.restConfig = krkubeclient.RESTConfig()
		c.apiServer = krkubeclient.APIReader()
	default:
		c.jobTaskSpec.Properties.Namespace = setting.AttachedClusterNamespace

		crClient, clientset, restConfig, apiServer, err := GetK8sClients(hubServerAddr, c.jobTaskSpec.Properties.ClusterID)
		if err != nil {
			logError(c.job, err.Error(), c.logger)
			return err
		}
		c.kubeclient = crClient
		c.clientset = clientset
		c.restConfig = restConfig
		c.apiServer = apiServer
	}

	// decide which docker host to use.
	// TODO: do not use code in warpdrive moudule, should move to a public place
	dockerhosts := dockerhost.NewDockerHosts(hubServerAddr, c.logger)
	c.jobTaskSpec.Properties.DockerHost = dockerhosts.GetBestHost(dockerhost.ClusterID(c.jobTaskSpec.Properties.ClusterID), "")

	// not local cluster
	var (
		replaceDindServer = "." + DindServer
		dockerHost        = ""
	)

	if c.jobTaskSpec.Properties.ClusterID != "" && c.jobTaskSpec.Properties.ClusterID != setting.LocalClusterID {
		if strings.Contains(c.jobTaskSpec.Properties.DockerHost, config.Namespace()) {
			// replace namespace only
			dockerHost = strings.Replace(c.jobTaskSpec.Properties.DockerHost, config.Namespace(), KoderoverAgentNamespace, 1)
		} else {
			// add namespace
			dockerHost = strings.Replace(c.jobTaskSpec.Properties.DockerHost, replaceDindServer, replaceDindServer+"."+KoderoverAgentNamespace, 1)
		}
	} else if c.jobTaskSpec.Properties.ClusterID == "" || c.jobTaskSpec.Properties.ClusterID == setting.LocalClusterID {
		if !strings.Contains(c.jobTaskSpec.Properties.DockerHost, config.Namespace()) {
			// add namespace
			dockerHost = strings.Replace(c.jobTaskSpec.Properties.DockerHost, replaceDindServer, replaceDindServer+"."+config.Namespace(), 1)
		}
	}

	c.jobTaskSpec.Properties.DockerHost = dockerHost

	jobCtxBytes, err := yaml.Marshal(BuildJobExcutorContext(c.jobTaskSpec, c.job, c.workflowCtx, c.logger))
	if err != nil {
		msg := fmt.Sprintf("cannot Jobexcutor.Context data: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	jobLabel := &JobLabel{
		JobType: string(c.job.JobType),
		JobName: c.job.K8sJobName,
	}
	if err := ensureDeleteConfigMap(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
		logError(c.job, err.Error(), c.logger)
		return err
	}

	if err := createJobConfigMap(
		c.jobTaskSpec.Properties.Namespace, c.job.K8sJobName, jobLabel, string(jobCtxBytes), c.kubeclient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	c.logger.Infof("succeed to create cm for job %s", c.job.K8sJobName)

	jobImage := getBaseImage(c.jobTaskSpec.Properties.BuildOS, c.jobTaskSpec.Properties.ImageFrom)

	c.jobTaskSpec.Properties.Registries = getMatchedRegistries(jobImage, c.jobTaskSpec.Properties.Registries)
	//Resource request default value is LOW
	job, err := buildJob(c.job.JobType, jobImage, c.job.K8sJobName, c.jobTaskSpec.Properties.ClusterID, c.jobTaskSpec.Properties.Namespace, c.jobTaskSpec.Properties.ResourceRequest, c.jobTaskSpec.Properties.ResReqSpec, c.job, c.jobTaskSpec, c.workflowCtx, nil)
	if err != nil {
		msg := fmt.Sprintf("create job context error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	job.Namespace = c.jobTaskSpec.Properties.Namespace

	if err := ensureDeleteJob(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
		msg := fmt.Sprintf("delete job error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	if err := createOrUpdateRegistrySecrets(c.jobTaskSpec.Properties.Namespace, c.jobTaskSpec.Properties.Registries, c.kubeclient); err != nil {
		msg := fmt.Sprintf("create secret error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	if err := updater.CreateJob(job, c.kubeclient); err != nil {
		msg := fmt.Sprintf("create job error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}
	c.logger.Infof("succeed to create job %s", c.job.K8sJobName)
	return nil
}

func (c *FreestyleJobCtl) wait(ctx context.Context) {
	var err error
	taskTimeout := time.After(time.Duration(c.jobTaskSpec.Properties.Timeout) * time.Minute)
	c.job.Status, err = waitJobStart(ctx, c.jobTaskSpec.Properties.Namespace, c.job.K8sJobName, c.kubeclient, c.apiServer, taskTimeout, c.logger)
	if err != nil {
		c.job.Error = err.Error()
	}
	if c.job.Status == config.StatusRunning {
		c.ack()
	} else {
		return
	}
	c.job.Status, c.job.Error = waitJobEndWithFile(ctx, taskTimeout, c.jobTaskSpec.Properties.Namespace, c.job.K8sJobName, true, c.kubeclient, c.clientset, c.restConfig, c.logger)
}

func (c *FreestyleJobCtl) complete(ctx context.Context) {
	jobLabel := &JobLabel{
		JobType: string(c.job.JobType),
		JobName: c.job.K8sJobName,
	}

	// 清理用户取消和超时的任务
	defer func() {
		go func() {
			if err := ensureDeleteJob(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
				c.logger.Error(err)
			}
			if err := ensureDeleteConfigMap(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
				c.logger.Error(err)
			}
		}()
	}()

	// get job outputs info from pod terminate message.
	if err := getJobOutputFromRunningPod(c.jobTaskSpec.Properties.Namespace, c.job.Name, c.job, c.workflowCtx, c.kubeclient, c.clientset, c.restConfig); err != nil {
		c.logger.Error(err)
	}

	if err := saveContainerLog(c.jobTaskSpec.Properties.Namespace, c.jobTaskSpec.Properties.ClusterID, c.workflowCtx.WorkflowName, c.job.Name, c.workflowCtx.TaskID, jobLabel, c.kubeclient); err != nil {
		c.logger.Error(err)
		if c.job.Error == "" {
			c.job.Error = err.Error()
		}
		return
	}
	if err := stepcontroller.SummarizeSteps(ctx, c.workflowCtx, &c.jobTaskSpec.Properties.Paths, c.job.Name, c.jobTaskSpec.Steps, c.logger); err != nil {
		c.logger.Error(err)
		c.job.Error = err.Error()
		return
	}
}

func BuildJobExcutorContext(jobTaskSpec *commonmodels.JobTaskFreestyleSpec, job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger) *JobContext {
	var envVars, secretEnvVars []string
	for _, env := range jobTaskSpec.Properties.Envs {
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
		Workspace:    workflowCtx.Workspace,
		TaskID:       workflowCtx.TaskID,
		Outputs:      outputs,
		Steps:        jobTaskSpec.Steps,
		Paths:        jobTaskSpec.Properties.Paths,
	}
}
