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
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/pkg/errors"
)

type CustomDeployJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.JobTaskCustomDeploySpec
	ack         func()
}

func NewCustomDeployJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *CustomDeployJobCtl {
	jobTaskSpec := &commonmodels.JobTaskCustomDeploySpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	return &CustomDeployJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *CustomDeployJobCtl) Clean(ctx context.Context) {}

func (c *CustomDeployJobCtl) Run(ctx context.Context) {
	if err := c.run(ctx); err != nil {
		return
	}
	if c.jobTaskSpec.SkipCheckRunStatus {
		c.job.Status = config.StatusPassed
		return
	}
	c.wait(ctx)
}

func (c *CustomDeployJobCtl) run(ctx context.Context) error {
	var err error
	if c.jobTaskSpec.ClusterID != "" {
		c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
		if err != nil {
			msg := fmt.Sprintf("can't init k8s client: %v", err)
			logError(c.job, msg, c.logger)
			return errors.New(msg)
		}
	} else {
		c.kubeClient = krkubeclient.Client()
	}
	replaced := false

	switch c.jobTaskSpec.WorkloadType {
	case setting.Deployment:
		deployment, _, err := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.WorkloadName, c.kubeClient)
		if err != nil {
			logError(c.job, err.Error(), c.logger)
			return err
		}
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name == c.jobTaskSpec.ContainerName {
				err = updater.UpdateDeploymentImage(deployment.Namespace, deployment.Name, container.Name, c.jobTaskSpec.Image, c.kubeClient)
				if err != nil {
					err = errors.WithMessagef(
						err,
						"failed to update container image in %s/deployments/%s/%s",
						deployment.Namespace, deployment.Name, container.Name)
					c.logger.Error(err)
					c.job.Status = config.StatusFailed
					c.job.Error = err.Error()
					return err
				}
				c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, commonmodels.Resource{
					Kind:      setting.Deployment,
					Container: container.Name,
					Origin:    container.Image,
					Name:      deployment.Name,
				})
				replaced = true
				break
			}
		}
	case setting.StatefulSet:
		statefulSet, _, err := getter.GetStatefulSet(c.jobTaskSpec.Namespace, c.jobTaskSpec.WorkloadName, c.kubeClient)
		if err != nil {
			logError(c.job, err.Error(), c.logger)
			return err
		}
		for _, container := range statefulSet.Spec.Template.Spec.Containers {
			if container.Name == c.jobTaskSpec.ContainerName {
				err = updater.UpdateDeploymentImage(statefulSet.Namespace, statefulSet.Name, container.Name, c.jobTaskSpec.Image, c.kubeClient)
				if err != nil {
					err = errors.WithMessagef(
						err,
						"failed to update container image in %s/statefulset/%s/%s",
						statefulSet.Namespace, statefulSet.Name, container.Name)
					logError(c.job, err.Error(), c.logger)
					return err
				}
				c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, commonmodels.Resource{
					Kind:      setting.StatefulSet,
					Container: container.Name,
					Origin:    container.Image,
					Name:      statefulSet.Name,
				})
				replaced = true
				break
			}
		}
	default:
		msg := fmt.Sprintf("workfload type: %s not supported", c.jobTaskSpec.WorkloadType)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}
	if !replaced {
		msg := fmt.Sprintf("workload type: %s,name: %s, container %s is not found in namespace %s", c.jobTaskSpec.WorkloadType, c.jobTaskSpec.WorkloadName, c.jobTaskSpec.ContainerName, c.jobTaskSpec.Namespace)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}
	c.job.Spec = c.jobTaskSpec
	return nil
}

func (c *CustomDeployJobCtl) wait(ctx context.Context) {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	for {
		select {
		case <-ctx.Done():
			c.job.Status = config.StatusCancelled
			return

		case <-timeout:
			c.job.Status = config.StatusTimeout
			return

		default:
			time.Sleep(time.Second * 2)
			ready := true
			var err error
			switch c.jobTaskSpec.WorkloadType {
			case setting.Deployment:
				d, found, e := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.WorkloadName, c.kubeClient)
				if e != nil {
					err = e
				}
				if e != nil || !found {
					c.logger.Errorf(
						"failed to check deployment ready status %s/%s/%s - %v",
						c.jobTaskSpec.Namespace,
						c.jobTaskSpec.WorkloadType,
						c.jobTaskSpec.WorkloadName,
						e,
					)
					ready = false
				} else {
					ready = wrapper.Deployment(d).Ready()
				}
			case setting.StatefulSet:
				st, found, e := getter.GetStatefulSet(c.jobTaskSpec.Namespace, c.jobTaskSpec.WorkloadName, c.kubeClient)
				if e != nil {
					err = e
				}
				if err != nil || !found {
					c.logger.Errorf(
						"failed to check statefulSet ready status %s/%s/%s",
						c.jobTaskSpec.Namespace,
						c.jobTaskSpec.WorkloadType,
						c.jobTaskSpec.WorkloadName,
						e,
					)
					ready = false
				} else {
					ready = wrapper.StatefulSet(st).Ready()
				}
			default:
				msg := fmt.Sprintf("workfload type: %s not supported", c.jobTaskSpec.WorkloadType)
				logError(c.job, msg, c.logger)
				return
			}
			if ready {
				c.job.Status = config.StatusPassed
				return
			}
		}
	}
}

func (c *CustomDeployJobCtl) timeout() int64 {
	if c.jobTaskSpec.Timeout == 0 {
		c.jobTaskSpec.Timeout = setting.DeployTimeout
	} else {
		c.jobTaskSpec.Timeout = c.jobTaskSpec.Timeout * 60
	}
	return c.jobTaskSpec.Timeout
}
