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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"go.uber.org/zap"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ZadigIstioCopySuffix     = "zadig-copy"
	ZadigIstioLabelOriginal  = "original"
	ZadigIstioLabelDuplicate = "duplicate"
)

const (
	// label definition
	ZadigIstioIdentifierLabel = "zadig-istio-release-version"
	ZadigIstioOriginalVSLabel = "last-applied-virtual-service"
)

type IstioReleaseJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.JobIstioReleaseSpec
	ack         func()
}

func NewIstioReleaseJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *IstioReleaseJobCtl {
	jobTaskSpec := &commonmodels.JobIstioReleaseSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	if jobTaskSpec.Event == nil {
		jobTaskSpec.Event = make([]*commonmodels.Event, 0)
	}
	job.Spec = jobTaskSpec
	return &IstioReleaseJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *IstioReleaseJobCtl) Clean(ctx context.Context) {
}

func (c *IstioReleaseJobCtl) Run(ctx context.Context) {
	var err error
	c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		c.Errorf("can't init k8s client: %v", err)
		return
	}
	deployment, found, err := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.Service.WorkloadName, c.kubeClient)
	if err != nil || !found {
		c.Errorf("deployment: %s not found: %v", c.jobTaskSpec.Service.WorkloadName, err)
		return
	}

	// ==================================================================
	//                     Deployment modification
	// ==================================================================

	newDeployment := deployment.DeepCopy()

	// if an original VS is provided, put a label into the label, so we could find it during the rollback
	if c.jobTaskSpec.Service.VirtualServiceName != "" {
		deployment.Labels[ZadigIstioOriginalVSLabel] = c.jobTaskSpec.Service.VirtualServiceName
	}

	if value, ok := deployment.Spec.Template.Labels[ZadigIstioIdentifierLabel]; ok {
		if value != ZadigIstioLabelOriginal {
			// the given label has to have value: original for the thing to work
			c.Errorf("the deployment %s's label: %s need to have the value: %s to proceed.", deployment.Name, ZadigIstioIdentifierLabel, ZadigIstioLabelOriginal)
			return
		}
	} else {
		// if the specified key does not exist, we simply return error
		c.Errorf("the deployment %s need to have label: %s on its metadata", deployment.Name, ZadigIstioIdentifierLabel)
		return
	}

	c.Infof("Adding annotation to original deployment: %s", c.jobTaskSpec.Service.WorkloadName)
	if err := updater.CreateOrPatchDeployment(deployment, c.kubeClient); err != nil {
		c.Errorf("add annotations to origin deployment: %s failed: %v", c.jobTaskSpec.Service.WorkloadName, err)
		return
	}

	// create a new deployment called <deployment-name>-zadig-copy
	newDeployment.Name = fmt.Sprintf("%s-%s", deployment.Name, ZadigIstioCopySuffix)
	// edit the label of the new deployment so we could find it
	newDeployment.Labels[ZadigIstioIdentifierLabel] = ZadigIstioLabelDuplicate
	newDeployment.Spec.Selector.MatchLabels[ZadigIstioIdentifierLabel] = ZadigIstioLabelDuplicate
	newDeployment.Spec.Template.Labels[ZadigIstioIdentifierLabel] = ZadigIstioLabelDuplicate
	// edit the image of the new deployment
	for _, container := range newDeployment.Spec.Template.Spec.Containers {
		if container.Name == c.jobTaskSpec.Service.ContainerName {
			container.Image = c.jobTaskSpec.Service.Image
		}
	}

	c.Infof("Creating deployment copy for deployment: %s", c.jobTaskSpec.Service.WorkloadName)
	if err := updater.CreateOrPatchDeployment(newDeployment, c.kubeClient); err != nil {
		c.Errorf("creating deployment copy: %s failed: %v", fmt.Sprintf("%s-%s", deployment.Name, ZadigIstioCopySuffix), err)
		return
	}

	// waiting for original deployment to run
	c.Infof("Waiting for deployment: %s to start", c.jobTaskSpec.Service.WorkloadName)
	if status, err := waitDeploymentReady(ctx, c.jobTaskSpec.Service.WorkloadName, c.jobTaskSpec.Namespace, c.timeout(), c.kubeClient, c.logger); err != nil {
		c.Errorf("Timout waiting for deployment: %s", c.jobTaskSpec.Service.WorkloadName)
		c.job.Status = status
		return
	}

	// waiting for the deployment copy to run
	c.Infof("Waiting for the duplicate deployment: %s to start", c.jobTaskSpec.Service.WorkloadName)
	if status, err := waitDeploymentReady(ctx, fmt.Sprintf("%s-%s", deployment.Name, ZadigIstioCopySuffix), c.jobTaskSpec.Namespace, c.timeout(), c.kubeClient, c.logger); err != nil {
		c.Errorf("Timout waiting for deployment: %s", fmt.Sprintf("%s-%s", deployment.Name, ZadigIstioCopySuffix))
		c.job.Status = status
		return
	}

	// ==================================================================
	//                     Deployment modification
	// ==================================================================

	c.Infof("job started, this is a test")

	c.job.Status = config.StatusPassed
}

func (c *IstioReleaseJobCtl) Errorf(format string, a ...any) {
	errMsg := fmt.Sprintf(format, a...)
	logError(c.job, errMsg, c.logger)
	c.jobTaskSpec.Event = append(c.jobTaskSpec.Event, &commonmodels.Event{
		EventType: "error",
		Time:      time.Now().Format("2006-01-02 15:04:05"),
		Message:   errMsg,
	})
}

func (c *IstioReleaseJobCtl) Infof(format string, a ...any) {
	InfoMsg := fmt.Sprintf(format, a...)
	c.jobTaskSpec.Event = append(c.jobTaskSpec.Event, &commonmodels.Event{
		EventType: "info",
		Time:      time.Now().Format("2006-01-02 15:04:05"),
		Message:   InfoMsg,
	})
}

func (c *IstioReleaseJobCtl) timeout() int64 {
	if c.jobTaskSpec.Timeout == 0 {
		c.jobTaskSpec.Timeout = setting.DeployTimeout
	} else {
		c.jobTaskSpec.Timeout = c.jobTaskSpec.Timeout * 60
	}
	return c.jobTaskSpec.Timeout
}
