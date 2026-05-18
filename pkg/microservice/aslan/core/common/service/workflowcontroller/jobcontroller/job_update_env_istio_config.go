/*
Copyright 2024 The KodeRover Authors.
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
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
)

type UpdateEnvIstioConfigJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.UpdateEnvIstioConfigJobSpec
	ack         func()
}

func NewUpdateEnvIstioConfigJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *UpdateEnvIstioConfigJobCtl {
	jobTaskSpec := &commonmodels.UpdateEnvIstioConfigJobSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &UpdateEnvIstioConfigJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *UpdateEnvIstioConfigJobCtl) Clean(ctx context.Context) {
}

func (c *UpdateEnvIstioConfigJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	req := kube.SetIstioGrayscaleConfigRequest{
		GrayscaleStrategy:  c.jobTaskSpec.GrayscaleStrategy,
		WeightConfigs:      c.jobTaskSpec.WeightConfigs,
		HeaderMatchConfigs: c.jobTaskSpec.HeaderMatchConfigs,
	}
	err := kube.SetIstioGrayscaleConfig(context.TODO(), c.jobTaskSpec.BaseEnv, c.workflowCtx.ProjectName, req)
	if err != nil {
		c.Errorf("Failed to set istio config, error: %v", err)
		return
	}

	c.job.Status = config.StatusPassed
}

func (c *UpdateEnvIstioConfigJobCtl) Errorf(format string, a ...any) {
	errMsg := fmt.Sprintf(format, a...)
	logError(c.job, errMsg, c.logger)
}

func (c *UpdateEnvIstioConfigJobCtl) SaveInfo(ctx context.Context) error {
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
