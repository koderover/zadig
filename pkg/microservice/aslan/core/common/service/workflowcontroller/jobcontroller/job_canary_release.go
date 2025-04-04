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

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
)

type CanaryReleaseJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.JobTaskCanaryReleaseSpec
	ack         func()
}

func NewCanaryReleaseJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *CanaryReleaseJobCtl {
	jobTaskSpec := &commonmodels.JobTaskCanaryReleaseSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	if jobTaskSpec.Events == nil {
		jobTaskSpec.Events = &commonmodels.Events{}
	}
	job.Spec = jobTaskSpec
	return &CanaryReleaseJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *CanaryReleaseJobCtl) Clean(ctx context.Context) {
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(c.jobTaskSpec.ClusterID)
	if err != nil {
		c.logger.Errorf("can't init k8s client: %v", err)
		return
	}

	canarydeploymentName := c.jobTaskSpec.WorkloadName + CanaryDeploymentSuffix
	if err := updater.DeleteDeploymentAndWaitWithTimeout(c.jobTaskSpec.Namespace, canarydeploymentName, time.Duration(c.timeout())*time.Second, kubeClient); err != nil {
		c.logger.Errorf("delete canary deployment %s error: %v", canarydeploymentName, err)
	}
}

func (c *CanaryReleaseJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()
	if err := c.run(ctx); err != nil {
		return
	}
	c.wait(ctx)
}

func (c *CanaryReleaseJobCtl) run(ctx context.Context) error {
	var err error
	c.kubeClient, err = clientmanager.NewKubeClientManager().GetControllerRuntimeClient(c.jobTaskSpec.ClusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s client: %v", err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}

	canarydeploymentName := c.jobTaskSpec.WorkloadName + CanaryDeploymentSuffix
	if err := updater.DeleteDeploymentAndWaitWithTimeout(c.jobTaskSpec.Namespace, canarydeploymentName, time.Duration(c.timeout())*time.Second, c.kubeClient); err != nil {
		msg := fmt.Sprintf("delete canary deployment %s error: %v", canarydeploymentName, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}
	msg := fmt.Sprintf("canary deployment: %s deleted", canarydeploymentName)
	c.jobTaskSpec.Events.Info(msg)
	c.ack()
	if err := updater.UpdateDeploymentImage(c.jobTaskSpec.Namespace, c.jobTaskSpec.WorkloadName, c.jobTaskSpec.ContainerName, c.jobTaskSpec.Image, c.kubeClient); err != nil {
		msg := fmt.Sprintf("update deployment: %s image error: %v", c.jobTaskSpec.WorkloadName, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}
	msg = fmt.Sprintf("updating deployment: %s image", c.jobTaskSpec.WorkloadName)
	c.jobTaskSpec.Events.Info(msg)
	c.ack()
	return nil
}

func (c *CanaryReleaseJobCtl) wait(ctx context.Context) {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	for {
		select {
		case <-ctx.Done():
			c.job.Status = config.StatusCancelled
			return

		case <-timeout:
			c.job.Status = config.StatusTimeout
			msg := fmt.Sprintf("timeout waiting for the deployment: %s to run", c.jobTaskSpec.WorkloadName)
			c.jobTaskSpec.Events.Info(msg)
			return

		default:
			time.Sleep(time.Second * 2)
			d, found, err := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.WorkloadName, c.kubeClient)
			if err != nil || !found {
				c.logger.Errorf(
					"failed to check deployment ready status %s/%s - %v",
					c.jobTaskSpec.Namespace,
					c.jobTaskSpec.WorkloadName,
					err,
				)
			} else {
				if wrapper.Deployment(d).Ready() {
					c.job.Status = config.StatusPassed
					msg := fmt.Sprintf("deployment: %s image updateed successfully", c.jobTaskSpec.WorkloadName)
					c.jobTaskSpec.Events.Info(msg)
					return
				}
			}
		}
	}
}

func (c *CanaryReleaseJobCtl) timeout() int64 {
	if c.jobTaskSpec.ReleaseTimeout == 0 {
		c.jobTaskSpec.ReleaseTimeout = setting.DeployTimeout
	} else {
		c.jobTaskSpec.ReleaseTimeout = c.jobTaskSpec.ReleaseTimeout * 60
	}
	return c.jobTaskSpec.ReleaseTimeout
}

func (c *CanaryReleaseJobCtl) SaveInfo(ctx context.Context) error {
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
