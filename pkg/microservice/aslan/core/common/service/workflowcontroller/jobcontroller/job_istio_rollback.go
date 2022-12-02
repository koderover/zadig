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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
)

type IstioRollbackJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.JobIstioRollbackSpec
	ack         func()
}

func NewIstioRollbackJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *IstioRollbackJobCtl {
	jobTaskSpec := &commonmodels.JobIstioRollbackSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}

	jobTaskSpec.Replicas = 100

	job.Spec = jobTaskSpec
	return &IstioRollbackJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *IstioRollbackJobCtl) Clean(ctx context.Context) {
}

func (c *IstioRollbackJobCtl) Run(ctx context.Context) {
	var err error

	// initialize istio client
	// NOTE that the only supported version is v1alpha3 right now
	istioClient, err := kubeclient.GetIstioClientV1Alpha3Client(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		c.logger.Errorf("failed to prepare istio client to do the resource update")
		return
	}

	c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		logError(c.job, fmt.Sprintf("can't init k8s client: %v", err), c.logger)
		return
	}
	// first we need to delete the destination rule created by zadig
	newDestinationRuleName := fmt.Sprintf(ServiceDestinationRuleTemplate, c.jobTaskSpec.Targets.WorkloadName)
	c.logger.Infof("deleting zadig's destination rule: %s", newDestinationRuleName)

	err = istioClient.DestinationRules(c.jobTaskSpec.Namespace).Delete(context.TODO(), newDestinationRuleName, v1.DeleteOptions{})
	if err != nil {
		// since this is not a fatal error, we simply print an error message and move on
		c.logger.Errorf("failed to delete destination rule: %s, error is: %s", newDestinationRuleName, err)
	}

	c.job.Status = config.StatusPassed
}
