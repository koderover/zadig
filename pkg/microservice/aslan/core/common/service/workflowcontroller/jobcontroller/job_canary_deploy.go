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

	"github.com/pkg/errors"
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
	CanaryDeploymentSuffix = "-zadig-canary"
)

type CanaryDeployJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.JobTaskCanaryDeploySpec
	ack         func()
}

func NewCanaryDeployJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *CanaryDeployJobCtl {
	jobTaskSpec := &commonmodels.JobTaskCanaryDeploySpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	if jobTaskSpec.Events == nil {
		jobTaskSpec.Events = &commonmodels.Events{}
	}
	job.Spec = jobTaskSpec
	return &CanaryDeployJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *CanaryDeployJobCtl) Clean(ctx context.Context) {}

func (c *CanaryDeployJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()
	if err := c.run(ctx); err != nil {
		return
	}
	c.wait(ctx)
}

func (c *CanaryDeployJobCtl) run(ctx context.Context) error {
	var err error
	c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s client: %v", err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}

	_, exist, err := getter.GetService(c.jobTaskSpec.Namespace, c.jobTaskSpec.K8sServiceName, c.kubeClient)
	if err != nil || !exist {
		msg := fmt.Sprintf("service: %s not found: %v", c.jobTaskSpec.K8sServiceName, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}

	deployment, exist, err := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.WorkloadName, c.kubeClient)
	if err != nil || !exist {
		msg := fmt.Sprintf("deployment: %s not found: %v", c.jobTaskSpec.WorkloadName, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name != c.jobTaskSpec.ContainerName {
			continue
		}
		c.jobTaskSpec.Events.Info(fmt.Sprintf("the original image is: %s", container.Image))
		c.ack()
		break
	}

	if strings.HasSuffix(deployment.Name, CanaryDeploymentSuffix) {
		msg := "canary deployment already exists"
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}
	deployment.Name = deployment.Name + CanaryDeploymentSuffix
	c.jobTaskSpec.CanaryWorkloadName = deployment.Name
	deployment.Spec.Replicas = int32Ptr(int32(c.jobTaskSpec.CanaryReplica))
	deployment.ObjectMeta.ResourceVersion = ""
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == c.jobTaskSpec.ContainerName {
			deployment.Spec.Template.Spec.Containers[i].Image = c.jobTaskSpec.Image
			break
		}
	}
	if err := updater.CreateOrPatchDeployment(deployment, c.kubeClient); err != nil {
		msg := fmt.Sprintf("create canary deployment: %s failed: %v", c.jobTaskSpec.CanaryWorkloadName, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}
	msg := fmt.Sprintf("canary deployment: %s created", deployment.Name)
	c.jobTaskSpec.Events.Info(msg)
	c.ack()
	return nil
}

func (c *CanaryDeployJobCtl) wait(ctx context.Context) {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	for {
		select {
		case <-ctx.Done():
			c.job.Status = config.StatusCancelled
			return

		case <-timeout:
			c.job.Status = config.StatusTimeout
			msg := fmt.Sprintf("timeout waiting for the canary deployment: %s to run", c.jobTaskSpec.CanaryWorkloadName)
			c.jobTaskSpec.Events.Info(msg)
			return

		default:
			time.Sleep(time.Second * 2)
			d, found, err := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.CanaryWorkloadName, c.kubeClient)
			if err != nil || !found {
				c.logger.Errorf(
					"failed to check deployment ready status %s/%s - %v",
					c.jobTaskSpec.Namespace,
					c.jobTaskSpec.CanaryWorkloadName,
					err,
				)
			} else {
				if wrapper.Deployment(d).Ready() {
					c.job.Status = config.StatusPassed
					msg := fmt.Sprintf("canary deployment: %s create successfully", c.jobTaskSpec.CanaryWorkloadName)
					c.jobTaskSpec.Events.Info(msg)
					return
				}
			}
		}
	}
}

func (c *CanaryDeployJobCtl) timeout() int64 {
	if c.jobTaskSpec.DeployTimeout == 0 {
		c.jobTaskSpec.DeployTimeout = setting.DeployTimeout
	} else {
		c.jobTaskSpec.DeployTimeout = c.jobTaskSpec.DeployTimeout * 60
	}
	return c.jobTaskSpec.DeployTimeout
}
