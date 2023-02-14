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
	"time"

	"go.uber.org/zap"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

type GrayRollbackJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.JobTaskGrayRollbackSpec
	ack         func()
}

func NewGrayRollbackJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *GrayRollbackJobCtl {
	jobTaskSpec := &commonmodels.JobTaskGrayRollbackSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	if jobTaskSpec.Events == nil {
		jobTaskSpec.Events = &commonmodels.Events{}
	}
	job.Spec = jobTaskSpec
	return &GrayRollbackJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *GrayRollbackJobCtl) Clean(ctx context.Context) {
}

func (c *GrayRollbackJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	var err error
	c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		c.Errorf("can't init k8s client: %v", err)
		return
	}
	deployment, found, err := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.WorkloadName, c.kubeClient)
	if err != nil || !found {
		c.Errorf("deployment: %s not found: %v", c.jobTaskSpec.WorkloadName, err)
		return
	}
	deployment.Spec.Replicas = int32Ptr(int32(c.jobTaskSpec.TotalReplica))
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == c.jobTaskSpec.ContainerName {
			deployment.Spec.Template.Spec.Containers[i].Image = c.jobTaskSpec.Image
			break
		}
	}
	if err := updater.CreateOrPatchDeployment(deployment, c.kubeClient); err != nil {
		c.Errorf("update origin deployment: %s failed: %v", c.jobTaskSpec.WorkloadName, err)
		return
	}
	if status, err := waitDeploymentReady(ctx, c.jobTaskSpec.WorkloadName, c.jobTaskSpec.Namespace, c.timeout(), c.kubeClient, c.logger); err != nil {
		c.logger.Error(err)
		c.job.Status = status
		c.job.Error = err.Error()
		c.jobTaskSpec.Events.Error(err.Error())
		return
	}
	c.jobTaskSpec.Events.Info(fmt.Sprintf("deployment: %s replica set to %d", c.jobTaskSpec.WorkloadName, c.jobTaskSpec.TotalReplica))
	c.jobTaskSpec.Events.Info(fmt.Sprintf("deployment: %s image set to %s", c.jobTaskSpec.WorkloadName, c.jobTaskSpec.Image))
	c.ack()

	_, found, err = getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.GrayWorkloadName, c.kubeClient)
	if err != nil {
		c.Errorf("get gray release deployment: %s error: %v", c.jobTaskSpec.GrayWorkloadName, err)
		return
	}
	if found {
		if err := updater.DeleteDeploymentAndWaitWithTimeout(c.jobTaskSpec.Namespace, c.jobTaskSpec.GrayWorkloadName, time.Duration(c.timeout())*time.Second, c.kubeClient); err != nil {
			msg := fmt.Sprintf("delete gray deployment %s error: %v", c.jobTaskSpec.GrayWorkloadName, err)
			logError(c.job, msg, c.logger)
			c.jobTaskSpec.Events.Error(msg)
			return
		}
		c.jobTaskSpec.Events.Info(fmt.Sprintf("gray release deployment: %s was deleted", c.jobTaskSpec.GrayWorkloadName))
	}
	c.job.Status = config.StatusPassed
}

func (c *GrayRollbackJobCtl) Errorf(format string, a ...any) {
	errMsg := fmt.Sprintf(format, a...)
	logError(c.job, errMsg, c.logger)
	c.jobTaskSpec.Events.Error(errMsg)
}

func (c *GrayRollbackJobCtl) timeout() int64 {
	if c.jobTaskSpec.RollbackTimeout == 0 {
		c.jobTaskSpec.RollbackTimeout = setting.DeployTimeout
	} else {
		c.jobTaskSpec.RollbackTimeout = c.jobTaskSpec.RollbackTimeout * 60
	}
	return c.jobTaskSpec.RollbackTimeout
}
