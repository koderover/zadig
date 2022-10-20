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
	"time"

	"go.uber.org/zap"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

const (
	GrayLabelKey               = "zadig-gray-release"
	GrayLabelValue             = "released-by-zadig"
	GrayImageAnnotationKey     = "zadig-gray-release-image"
	GrayContainerAnnotationKey = "zadig-gray-release-container"
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
		deployment.Name = c.jobTaskSpec.GrayWorkloadName
		deployment.Spec.Replicas = int32Ptr(int32(c.jobTaskSpec.GrayReplica))
		deployment.ObjectMeta.ResourceVersion = ""
		deployment.Spec.Template.Labels[GrayLabelKey] = GrayLabelValue

		for i := range deployment.Spec.Template.Spec.Containers {
			if deployment.Spec.Template.Spec.Containers[i].Name == c.jobTaskSpec.ContainerName {
				deployment.ObjectMeta.Annotations[GrayImageAnnotationKey] = deployment.Spec.Template.Spec.Containers[i].Image
				deployment.ObjectMeta.Annotations[GrayContainerAnnotationKey] = c.jobTaskSpec.ContainerName
				deployment.Spec.Template.Spec.Containers[i].Image = c.jobTaskSpec.Image
				break
			}
		}
		if err := updater.CreateOrPatchDeployment(deployment, c.kubeClient); err != nil {
			c.Errorf("create gray release deployment: %s failed: %v", c.jobTaskSpec.GrayWorkloadName, err)
			return
		}
		c.jobTaskSpec.Events.Info(fmt.Sprintf("gray release deployment: %s created", c.jobTaskSpec.GrayWorkloadName))
		if status, err := c.waitDeploymentReady(ctx, c.jobTaskSpec.GrayWorkloadName); err != nil {
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
		if status, err := c.waitDeploymentReady(ctx, c.jobTaskSpec.WorkloadName); err != nil {
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

	if c.jobTaskSpec.GrayScale >= 100 {
		if err := updater.ScaleDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.WorkloadName, c.jobTaskSpec.TotalReplica, c.kubeClient); err != nil {
			c.Errorf("update origin deployment: %s failed: %v", c.jobTaskSpec.WorkloadName, err)
			return
		}
		if status, err := c.waitDeploymentReady(ctx, c.jobTaskSpec.WorkloadName); err != nil {
			c.logger.Error(err)
			c.job.Status = status
			c.job.Error = err.Error()
			c.jobTaskSpec.Events.Error(err.Error())
			return
		}
		c.jobTaskSpec.Events.Info(fmt.Sprintf("deployment: %s replica set to %d", c.jobTaskSpec.WorkloadName, c.jobTaskSpec.TotalReplica))

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
	if status, err := c.waitDeploymentReady(ctx, c.jobTaskSpec.WorkloadName); err != nil {
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
	if status, err := c.waitDeploymentReady(ctx, c.jobTaskSpec.WorkloadName); err != nil {
		c.logger.Error(err)
		c.job.Status = status
		c.job.Error = err.Error()
		c.jobTaskSpec.Events.Error(err.Error())
		return
	}
	c.jobTaskSpec.Events.Info(fmt.Sprintf("deployment: %s replica set to %d", c.jobTaskSpec.WorkloadName, leftReplica))
	c.job.Status = config.StatusPassed
}

func (c *GrayReleaseJobCtl) waitDeploymentReady(ctx context.Context, deploymentName string) (config.Status, error) {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return config.StatusCancelled, errors.New("job was cancelled")

		case <-timeout:
			msg := fmt.Sprintf("timeout waiting for the deployment: %s to run", deploymentName)
			return config.StatusTimeout, errors.New(msg)

		default:
			time.Sleep(time.Second * 2)
			d, found, err := getter.GetDeployment(c.jobTaskSpec.Namespace, deploymentName, c.kubeClient)
			if err != nil || !found {
				c.logger.Errorf(
					"failed to check deployment ready status %s/%s - %v",
					c.jobTaskSpec.Namespace,
					deploymentName,
					err,
				)
			} else {
				if wrapper.Deployment(d).Ready() {
					return config.StatusRunning, nil
				}
			}
		}
	}
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
