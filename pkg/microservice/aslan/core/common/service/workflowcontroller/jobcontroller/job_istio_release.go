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
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"go.uber.org/zap"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"
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
	_, found, err := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.Service.WorkloadName, c.kubeClient)
	if err != nil || !found {
		c.Errorf("deployment: %s not found: %v", c.jobTaskSpec.Service.WorkloadName, err)
		return
	}

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
