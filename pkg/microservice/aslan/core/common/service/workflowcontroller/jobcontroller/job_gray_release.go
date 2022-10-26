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
	"strconv"

	"go.uber.org/zap"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

type GrayReleaseJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.JobTaskGrayReleaseSpec
	ack         func()
}

func NewGrayReleaseJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *GrayReleaseJobCtl {
	jobTaskSpec := &commonmodels.JobTaskGrayReleaseSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	if jobTaskSpec.Events == nil {
		jobTaskSpec.Events = &commonmodels.Events{}
	}
	job.Spec = jobTaskSpec
	return &GrayReleaseJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *GrayReleaseJobCtl) Clean(ctx context.Context) {
}

func (c *GrayReleaseJobCtl) Run(ctx context.Context) {
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
	leftReplica := c.jobTaskSpec.TotalReplica - c.jobTaskSpec.GrayReplica

	if c.jobTaskSpec.FirstJob {
		_, found, err := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.GrayWorkloadName, c.kubeClient)
		if found {
			c.Errorf("gray deployment: %s already exists", c.jobTaskSpec.GrayWorkloadName)
			return
		}
		if err != nil {
			c.Errorf("get deployment: %s error", c.jobTaskSpec.GrayWorkloadName)
			return
		}
		grayDeployment := deployment.DeepCopy()
		grayDeployment.Name = c.jobTaskSpec.GrayWorkloadName
		grayDeployment.Spec.Replicas = int32Ptr(int32(c.jobTaskSpec.GrayReplica))
		grayDeployment.ObjectMeta.ResourceVersion = ""
		grayDeployment.Spec.Template.Labels[config.GrayLabelKey] = config.GrayLabelValue

		for i := range grayDeployment.Spec.Template.Spec.Containers {
			if grayDeployment.Spec.Template.Spec.Containers[i].Name == c.jobTaskSpec.ContainerName {
				deployment.ObjectMeta.Annotations[config.GrayImageAnnotationKey] = grayDeployment.Spec.Template.Spec.Containers[i].Image
				deployment.ObjectMeta.Annotations[config.GrayContainerAnnotationKey] = c.jobTaskSpec.ContainerName
				deployment.ObjectMeta.Annotations[config.GrayReplicaAnnotationKey] = strconv.Itoa(c.jobTaskSpec.TotalReplica)
				grayDeployment.Spec.Template.Spec.Containers[i].Image = c.jobTaskSpec.Image
				break
			}
		}

		if err := updater.CreateOrPatchDeployment(deployment, c.kubeClient); err != nil {
			c.Errorf("add annotations to origin deployment: %s failed: %v", c.jobTaskSpec.WorkloadName, err)
			return
		}
		c.jobTaskSpec.Events.Info(fmt.Sprintf("add annotations to origin deployment: %s", c.jobTaskSpec.WorkloadName))
		if err := updater.CreateOrPatchDeployment(grayDeployment, c.kubeClient); err != nil {
			c.Errorf("create gray release deployment: %s failed: %v", c.jobTaskSpec.GrayWorkloadName, err)
			return
		}
		c.jobTaskSpec.Events.Info(fmt.Sprintf("gray release deployment: %s created", c.jobTaskSpec.GrayWorkloadName))
		if status, err := waitDeploymentReady(ctx, c.jobTaskSpec.GrayWorkloadName, c.jobTaskSpec.Namespace, c.timeout(), c.kubeClient, c.logger); err != nil {
			c.logger.Error(err)
			c.job.Status = status
			c.job.Error = err.Error()
			c.jobTaskSpec.Events.Error(err.Error())
			return
		}
		c.jobTaskSpec.Events.Info(fmt.Sprintf("gray release deployment: %s ready", c.jobTaskSpec.GrayWorkloadName))
		if err := updater.ScaleDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.WorkloadName, leftReplica, c.kubeClient); err != nil {
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
		c.jobTaskSpec.Events.Info(fmt.Sprintf("origin deployment: %s replica set to %d", c.jobTaskSpec.WorkloadName, leftReplica))
		c.job.Status = config.StatusPassed
		return
	}

	_, found, err = getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.GrayWorkloadName, c.kubeClient)
	if err != nil || !found {
		c.Errorf("gray deployment: %s not found: %v", c.jobTaskSpec.GrayWorkloadName, err)
		return
	}

	// gray scale equals 100 means it was a full release
	if c.jobTaskSpec.GrayScale >= 100 {
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

		if err := updater.DeleteDeploymentAndWait(c.jobTaskSpec.Namespace, c.jobTaskSpec.GrayWorkloadName, c.kubeClient); err != nil {
			msg := fmt.Sprintf("delete gray deployment %s error: %v", c.jobTaskSpec.GrayWorkloadName, err)
			logError(c.job, msg, c.logger)
			c.jobTaskSpec.Events.Error(msg)
			return
		}
		c.jobTaskSpec.Events.Info(fmt.Sprintf("gray release deployment: %s was deleted", c.jobTaskSpec.GrayWorkloadName))
		c.job.Status = config.StatusPassed
		return
	}
	if err := updater.ScaleDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.GrayWorkloadName, c.jobTaskSpec.GrayReplica, c.kubeClient); err != nil {
		c.Errorf("update gray release deployment: %s failed: %v", c.jobTaskSpec.WorkloadName, err)
		return
	}
	if status, err := waitDeploymentReady(ctx, c.jobTaskSpec.WorkloadName, c.jobTaskSpec.Namespace, c.timeout(), c.kubeClient, c.logger); err != nil {
		c.logger.Error(err)
		c.job.Status = status
		c.job.Error = err.Error()
		c.jobTaskSpec.Events.Error(err.Error())
		return
	}
	c.jobTaskSpec.Events.Info(fmt.Sprintf("gray release deployment: %s replica set to %d", c.jobTaskSpec.GrayWorkloadName, c.jobTaskSpec.GrayReplica))
	if err := updater.ScaleDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.WorkloadName, leftReplica, c.kubeClient); err != nil {
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
	c.jobTaskSpec.Events.Info(fmt.Sprintf("deployment: %s replica set to %d", c.jobTaskSpec.WorkloadName, leftReplica))
	c.job.Status = config.StatusPassed
}

func (c *GrayReleaseJobCtl) Errorf(format string, a ...any) {
	errMsg := fmt.Sprintf(format, a...)
	logError(c.job, errMsg, c.logger)
	c.jobTaskSpec.Events.Error(errMsg)
}

func (c *GrayReleaseJobCtl) timeout() int64 {
	if c.jobTaskSpec.DeployTimeout == 0 {
		c.jobTaskSpec.DeployTimeout = setting.DeployTimeout
	} else {
		c.jobTaskSpec.DeployTimeout = c.jobTaskSpec.DeployTimeout * 60
	}
	return c.jobTaskSpec.DeployTimeout
}
