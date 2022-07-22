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
	jobName     string
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeclient  crClient.Client
	clientset   kubernetes.Interface
	restConfig  *rest.Config
	ack         func()
}

func NewPluginsJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *PluginJobCtl {
	return &PluginJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobName:     getJobName(workflowCtx.WorkflowName, workflowCtx.TaskID),
	}
}

func (c *PluginJobCtl) Run(ctx context.Context) {
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

	jobLabel := &JobLabel{
		WorkflowName: c.workflowCtx.WorkflowName,
		TaskID:       c.workflowCtx.TaskID,
		JobType:      string(c.job.JobType),
		JobName:      c.job.Name,
	}
	job, err := buildPlainJob(c.jobName, c.job.Properties.ResourceRequest, c.job.Properties.ResReqSpec, c.job, c.workflowCtx)
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
	if err := updater.CreateJob(job, c.kubeclient); err != nil {
		msg := fmt.Sprintf("create job error: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}
	c.logger.Infof("succeed to create job %s", c.jobName)

}

func (c *PluginJobCtl) Wait(ctx context.Context) {
	status := waitPlainJobEnd(ctx, int(c.job.Properties.Timeout), c.job.Properties.Namespace, c.jobName, c.kubeclient, c.logger)
	c.job.Status = status

}

func (c *PluginJobCtl) Complete(ctx context.Context) {
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
