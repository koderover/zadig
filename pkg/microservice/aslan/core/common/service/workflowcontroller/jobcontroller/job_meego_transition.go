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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/meego"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type MeegoTransitionJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.MeegoTransitionSpec
	ack         func()
}

func NewMeegoTransitionJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *MeegoTransitionJobCtl {
	jobTaskSpec := &commonmodels.MeegoTransitionSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &MeegoTransitionJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *MeegoTransitionJobCtl) Clean(ctx context.Context) {
}

func (c *MeegoTransitionJobCtl) Run(ctx context.Context) {
	// since this runs in aslan, we will connect directly to the database to retrieve meego integration info
	meegoInfo, err := commonrepo.NewProjectManagementColl().GetMeegoByID(c.jobTaskSpec.MeegoID)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		c.job.Status = config.StatusFailed
		return
	}

	spec := &commonmodels.ProjectManagementMeegoSpec{}
	err = commonmodels.IToi(meegoInfo.Spec, spec)
	if err != nil {
		logError(c.job, fmt.Sprintf("failed to convert job spec to meego spec: %v", err), c.logger)
		return
	}

	client, err := meego.NewClient(spec.MeegoHost, spec.MeegoPluginID, spec.MeegoPluginSecret, spec.MeegoUserKey)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		c.job.Status = config.StatusFailed
		return
	}
	for _, task := range c.jobTaskSpec.WorkItems {
		err := client.StatusTransition(c.jobTaskSpec.ProjectKey, c.jobTaskSpec.WorkItemTypeKey, task.ID, task.TransitionID)
		if err != nil {
			errMsg := fmt.Sprintf("work item: [%d] failed to do transition, err: %s", task.ID, err)
			logError(c.job, errMsg, c.logger)
			task.Status = string(config.StatusFailed)
			c.job.Status = config.StatusFailed
			return
		}
		task.Status = string(config.StatusPassed)
	}
	c.job.Status = config.StatusPassed
}

func (c *MeegoTransitionJobCtl) SaveInfo(ctx context.Context) error {
	return commonrepo.NewJobInfoColl().Create(context.TODO(), &commonmodels.JobInfo{
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
