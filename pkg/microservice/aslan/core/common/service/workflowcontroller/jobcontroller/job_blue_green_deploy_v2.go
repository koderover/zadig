/*
Copyright 2023 The KodeRover Authors.

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
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

type BlueGreenDeployV2JobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	namespace   string
	jobTaskSpec *commonmodels.JobTaskBlueGreenDeployV2Spec
	ack         func()
}

func NewBlueGreenDeployV2JobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *BlueGreenDeployV2JobCtl {
	jobTaskSpec := &commonmodels.JobTaskBlueGreenDeployV2Spec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	if jobTaskSpec.Events == nil {
		jobTaskSpec.Events = &commonmodels.Events{}
	}
	job.Spec = jobTaskSpec
	return &BlueGreenDeployV2JobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *BlueGreenDeployV2JobCtl) Clean(ctx context.Context) {}

func (c *BlueGreenDeployV2JobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()
	if err := c.run(ctx); err != nil {
		return
	}
	c.wait(ctx)
}

func (c *BlueGreenDeployV2JobCtl) run(ctx context.Context) error {
	var err error

	env, err := mongodb.NewProductColl().Find(&mongodb.ProductFindOptions{
		Name:    c.workflowCtx.ProjectName,
		EnvName: c.jobTaskSpec.Env,
	})
	if err != nil {
		msg := fmt.Sprintf("find project error: %v", err)
		logError(c.job, msg, c.logger)
		return
	}
	c.namespace = env.Namespace
	clusterID := env.ClusterID

	c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s client: %v", err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}

	blueService := service.DeepCopy()
	blueService.Name = c.jobTaskSpec.BlueK8sServiceName
	c.jobTaskSpec.BlueK8sServiceName = blueService.Name
	blueService.Spec.Selector[config.BlueGreenVerionLabelName] = c.jobTaskSpec.Version
	// clean service extra infos may confilict.
	blueService.Spec.ClusterIPs = []string{}
	if blueService.Spec.ClusterIP != "None" {
		blueService.Spec.ClusterIP = ""
	}
	for i := range blueService.Spec.Ports {
		blueService.Spec.Ports[i].NodePort = 0
	}
	blueService.ObjectMeta.ResourceVersion = ""

	if err := updater.CreateOrPatchService(blueService, c.kubeClient); err != nil {
		msg := fmt.Sprintf("create blue serivce: %s error: %v", c.jobTaskSpec.BlueK8sServiceName, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}
	c.jobTaskSpec.Events.Info(fmt.Sprintf("blue service: %s created", c.jobTaskSpec.BlueK8sServiceName))
	c.ack()
	blueDeployment := deployment.DeepCopy()
	blueDeployment.Name = c.jobTaskSpec.BlueWorkloadName
	for i, container := range blueDeployment.Spec.Template.Spec.Containers {
		if container.Name != c.jobTaskSpec.ContainerName {
			continue
		}
		blueDeployment.Spec.Template.Spec.Containers[i].Image = c.jobTaskSpec.Image
	}
	blueDeployment.Labels[config.BlueGreenVerionLabelName] = c.jobTaskSpec.Version
	blueDeployment.Spec.Selector.MatchLabels[config.BlueGreenVerionLabelName] = c.jobTaskSpec.Version
	blueDeployment.Spec.Template.Labels[config.BlueGreenVerionLabelName] = c.jobTaskSpec.Version
	blueDeployment.ObjectMeta.ResourceVersion = ""
	if err := updater.CreateOrPatchDeployment(blueDeployment, c.kubeClient); err != nil {
		msg := fmt.Sprintf("create blue deployment: %s error: %v", c.jobTaskSpec.BlueWorkloadName, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}
	c.jobTaskSpec.Events.Info(fmt.Sprintf("blue deployment: %s created", c.jobTaskSpec.BlueWorkloadName))
	c.ack()
	return nil
}

func (c *BlueGreenDeployV2JobCtl) wait(ctx context.Context) {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	for {
		select {
		case <-ctx.Done():
			c.job.Status = config.StatusCancelled
			return

		case <-timeout:
			c.job.Status = config.StatusTimeout
			msg := fmt.Sprintf("timeout waiting for the blue deployment: %s to run", c.jobTaskSpec.BlueWorkloadName)
			c.jobTaskSpec.Events.Info(msg)
			return

		default:
			time.Sleep(time.Second * 2)
			d, found, err := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.BlueWorkloadName, c.kubeClient)
			if err != nil || !found {
				c.logger.Errorf(
					"failed to check deployment ready status %s/%s - %v",
					c.jobTaskSpec.Namespace,
					c.jobTaskSpec.BlueWorkloadName,
					err,
				)
			} else {
				if wrapper.Deployment(d).Ready() {
					c.job.Status = config.StatusPassed
					msg := fmt.Sprintf("blue-green deployment: %s create successfully", c.jobTaskSpec.BlueWorkloadName)
					c.jobTaskSpec.Events.Info(msg)
					return
				}
			}
		}
	}
}

func (c *BlueGreenDeployV2JobCtl) timeout() int64 {
	if c.jobTaskSpec.DeployTimeout == 0 {
		c.jobTaskSpec.DeployTimeout = setting.DeployTimeout
	} else {
		c.jobTaskSpec.DeployTimeout = c.jobTaskSpec.DeployTimeout * 60
	}
	return c.jobTaskSpec.DeployTimeout
}

func (c *BlueGreenDeployV2JobCtl) SaveInfo(ctx context.Context) error {
	return mongodb.NewJobInfoColl().Create(context.TODO(), &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),
	})
}
