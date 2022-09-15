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

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

type BlueGreenReleaseJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.JobTaskBlueGreenReleaseSpec
	ack         func()
}

func NewBlueGreenReleaseJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *BlueGreenReleaseJobCtl {
	jobTaskSpec := &commonmodels.JobTaskBlueGreenReleaseSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	if jobTaskSpec.Events == nil {
		jobTaskSpec.Events = &commonmodels.Events{}
	}
	job.Spec = jobTaskSpec
	return &BlueGreenReleaseJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *BlueGreenReleaseJobCtl) Clean(ctx context.Context) {
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		c.logger.Errorf("can't init k8s client: %v", err)
	}
	service, exist, err := getter.GetService(c.jobTaskSpec.Namespace, c.jobTaskSpec.K8sServiceName, kubeClient)
	if err != nil || !exist {
		c.logger.Errorf("get service error: %v", err)
		return
	}
	// ensure delete blue service.
	if err := updater.DeleteService(c.jobTaskSpec.Namespace, c.jobTaskSpec.BlueK8sServiceName, kubeClient); err != nil {
		c.logger.Errorf("delete blue service error: %v", err)
	}
	// if service point to new deployment means blue green release succeed.
	if service.Spec.Selector[config.BlueGreenVerionLabelName] == c.jobTaskSpec.Version {
		return
	}
	// clear intermediate state resources
	if err := updater.DeleteDeploymentAndWait(c.jobTaskSpec.Namespace, c.jobTaskSpec.BlueWorkloadName, kubeClient); err != nil {
		c.logger.Errorf("delete old service error: %v", err)
	}
	// if it was the first time blue-green deployment, clean the origin labels.
	if service.Spec.Selector[config.BlueGreenVerionLabelName] == config.OriginVersion {
		delete(service.Spec.Selector, config.BlueGreenVerionLabelName)
		if err := updater.CreateOrPatchService(service, kubeClient); err != nil {
			c.logger.Errorf("delete origin label for service error: %v", err)
			return
		}
		selector := labels.Set(service.Spec.Selector).AsSelector()
		pods, err := getter.ListPods(c.jobTaskSpec.Namespace, selector, c.kubeClient)
		if err != nil {
			c.logger.Errorf("list pods error: %v", err)
			return
		}
		for _, pod := range pods {
			deleteLabelPatch := fmt.Sprintf(`[{"op":"remove","path":"/metadata/labels/%s","value":"%s" }]`, config.BlueGreenVerionLabelName, config.OriginVersion)
			if err := updater.PatchPod(c.jobTaskSpec.Namespace, pod.Name, []byte(deleteLabelPatch), c.kubeClient); err != nil {
				c.logger.Errorf("patch pod error: %v", err)
			}
		}
	}
}

func (c *BlueGreenReleaseJobCtl) Run(ctx context.Context) {
	var err error
	if c.jobTaskSpec.ClusterID != "" {
		c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
		if err != nil {
			msg := fmt.Sprintf("can't init k8s client: %v", err)
			c.logger.Error(msg)
			c.job.Status = config.StatusFailed
			c.job.Error = msg
			c.jobTaskSpec.Events.Error(msg)
			return
		}
	} else {
		c.kubeClient = krkubeclient.Client()
	}
	service, exist, err := getter.GetService(c.jobTaskSpec.Namespace, c.jobTaskSpec.K8sServiceName, c.kubeClient)
	if err != nil || !exist {
		msg := fmt.Sprintf("get service %s failed, err: %v", c.jobTaskSpec.K8sServiceName, err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		c.jobTaskSpec.Events.Error(msg)
		return
	}
	service.Spec.Selector[config.BlueGreenVerionLabelName] = c.jobTaskSpec.Version
	if err := updater.CreateOrPatchService(service, c.kubeClient); err != nil {
		msg := fmt.Sprintf("point service to new deployment failed: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		c.jobTaskSpec.Events.Error(msg)
		return
	}
	c.jobTaskSpec.Events.Info("point service to new deployment success")
	c.ack()
	blueServiceName := c.jobTaskSpec.BlueK8sServiceName
	if err := updater.DeleteService(c.jobTaskSpec.Namespace, blueServiceName, c.kubeClient); err != nil {
		// delete failed, but we don't care
		msg := fmt.Sprintf("delete blue service failed: %v", err)
		c.jobTaskSpec.Events.Error(msg)
		c.ack()
	}
	if err := updater.DeleteDeploymentAndWait(c.jobTaskSpec.Namespace, c.jobTaskSpec.WorkloadName, c.kubeClient); err != nil {
		msg := fmt.Sprintf("delete old deployment failed: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		c.jobTaskSpec.Events.Error(msg)
		return
	}
	c.jobTaskSpec.Events.Info(fmt.Sprintf("blue green deployment succeed, now service point to deployemt: %s" + c.jobTaskSpec.BlueWorkloadName))
}
