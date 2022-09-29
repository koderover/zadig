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

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	zadigconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

type PluginJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeclient  crClient.Client
	clientset   kubernetes.Interface
	restConfig  *rest.Config
	jobTaskSpec *commonmodels.JobTaskPluginSpec
	ack         func()
}

func NewPluginsJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *PluginJobCtl {
	jobTaskSpec := &commonmodels.JobTaskPluginSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	return &PluginJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *PluginJobCtl) prepare(ctx context.Context) {
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
}

func (c *PluginJobCtl) Clean(ctx context.Context) {}

func (c *PluginJobCtl) Run(ctx context.Context) {
	c.prepare(ctx)
	if err := c.run(ctx); err != nil {
		return
	}
	c.wait(ctx)
	c.complete(ctx)
}

func (c *PluginJobCtl) run(ctx context.Context) error {
	// get kube client
	hubServerAddr := config.HubServerAddress()
	switch c.jobTaskSpec.Properties.ClusterID {
	case setting.LocalClusterID:
		c.jobTaskSpec.Properties.Namespace = zadigconfig.Namespace()
		c.kubeclient = krkubeclient.Client()
		c.clientset = krkubeclient.Clientset()
		c.restConfig = krkubeclient.RESTConfig()
	default:
		c.jobTaskSpec.Properties.Namespace = setting.AttachedClusterNamespace

		crClient, clientset, restConfig, err := GetK8sClients(hubServerAddr, c.jobTaskSpec.Properties.ClusterID)
		if err != nil {
			logError(c.job, err.Error(), c.logger)
			return err
		}
		c.kubeclient = crClient
		c.clientset = clientset
		c.restConfig = restConfig
	}

	jobLabel := &JobLabel{
		JobType: string(c.job.JobType),
		JobName: c.job.K8sJobName,
	}
	job, err := buildPlainJob(c.job.K8sJobName, c.jobTaskSpec.Properties.ResourceRequest, c.jobTaskSpec.Properties.ResReqSpec, c.job, c.jobTaskSpec, c.workflowCtx)
	if err != nil {
		msg := fmt.Sprintf("create job context error: %v", err)
		logError(c.job, msg, c.logger)
		return err
	}

	job.Namespace = c.jobTaskSpec.Properties.Namespace

	if err := ensureDeleteJob(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
		msg := fmt.Sprintf("delete job error: %v", err)
		logError(c.job, msg, c.logger)
		return err
	}
	if err := updater.CreateJob(job, c.kubeclient); err != nil {
		msg := fmt.Sprintf("create job error: %v", err)
		logError(c.job, msg, c.logger)
		return err
	}
	c.logger.Infof("succeed to create job %s", c.job.K8sJobName)
	return nil
}

func (c *PluginJobCtl) wait(ctx context.Context) {
	status := waitPlainJobEnd(ctx, int(c.jobTaskSpec.Properties.Timeout), c.jobTaskSpec.Properties.Namespace, c.job.K8sJobName, c.kubeclient, c.logger)
	c.job.Status = status

}

func (c *PluginJobCtl) complete(ctx context.Context) {
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
		}()
	}()

	// get job outputs info from pod terminate message.
	outputs, err := getJobOutput(c.jobTaskSpec.Properties.Namespace, c.job.Name, jobLabel, c.kubeclient)
	if err != nil {
		c.logger.Error(err)
		c.job.Error = err.Error()
	}

	// write jobs output info to globalcontext so other job can use like this $(workflow.jobName.outputName)
	for _, output := range outputs {
		c.workflowCtx.GlobalContextSet(strings.Join([]string{"workflow", c.job.Name, output.Name}, "."), output.Value)
	}

	if err := saveContainerLog(c.jobTaskSpec.Properties.Namespace, c.jobTaskSpec.Properties.ClusterID, c.workflowCtx.WorkflowName, c.job.Name, c.workflowCtx.TaskID, jobLabel, c.kubeclient); err != nil {
		c.logger.Error(err)
		c.job.Error = err.Error()
		return
	}
}
