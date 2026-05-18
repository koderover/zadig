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
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/larkplugin"
	sdkcore "github.com/larksuite/project-oapi-sdk-golang/core"
	"github.com/larksuite/project-oapi-sdk-golang/service/project"
	"github.com/larksuite/project-oapi-sdk-golang/service/workitem"
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

	client := larkplugin.NewClient(spec.MeegoPluginID, spec.MeegoPluginSecret, setting.IMLark)
	resp, err := client.Client.Plugin.GetPluginToken(ctx, 0)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		c.job.Status = config.StatusFailed
		return
	}
	if resp.Error != nil && resp.Error.Code != 0 {
		logError(c.job, fmt.Sprintf("failed to get lark plugin token, error code: %d, message: %s", resp.Error.Code, resp.Error.Msg), c.logger)
		c.job.Status = config.StatusFailed
		return
	}

	workitemTypes, err := client.Client.Project.ListProjectWorkItemType(ctx, project.NewListProjectWorkItemTypeReqBuilder().
		ProjectKey(c.jobTaskSpec.ProjectKey).
		Build(),
		sdkcore.WithAccessToken(resp.Data.Token),
		sdkcore.WithUserKey(spec.MeegoUserKey),
	)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		c.job.Status = config.StatusFailed
		return
	}
	if !workitemTypes.Success() {
		logError(c.job, fmt.Sprintf("failed to get project workitem type, code: %d, message: %s", workitemTypes.Code(), workitemTypes.Error()), c.logger)
		c.job.Status = config.StatusFailed
		return
	}

	workitemTypeAPIName := c.jobTaskSpec.WorkItemTypeKey
	for _, workitemType := range workitemTypes.Data {
		if workitemType.TypeKey == c.jobTaskSpec.WorkItemTypeKey {
			workitemTypeAPIName = workitemType.APIName
			break
		}
	}
	c.jobTaskSpec.WorkItemTypeAPIName = workitemTypeAPIName

	projectDetail, err := client.Client.Project.GetProjectDetail(ctx, project.NewGetProjectDetailReqBuilder().
		ProjectKeys([]string{c.jobTaskSpec.ProjectKey}).
		Build(),
		sdkcore.WithAccessToken(resp.Data.Token),
		sdkcore.WithUserKey(spec.MeegoUserKey),
	)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		c.job.Status = config.StatusFailed
		return
	}
	if !projectDetail.Success() {
		logError(c.job, fmt.Sprintf("failed to get project detail, code: %d, message: %s", projectDetail.Code(), projectDetail.Error()), c.logger)
		c.job.Status = config.StatusFailed
		return
	}

	if projectDetail.Data[c.jobTaskSpec.ProjectKey] == nil {
		logError(c.job, fmt.Sprintf("project %s not found", c.jobTaskSpec.ProjectKey), c.logger)
		c.job.Status = config.StatusFailed
		return
	}
	c.jobTaskSpec.SimpleName = projectDetail.Data[c.jobTaskSpec.ProjectKey].SimpleName

	for _, task := range c.jobTaskSpec.StatusWorkItems {
		resp, err := client.Client.WorkItem.NodeStateChange(ctx, workitem.NewNodeStateChangeReqBuilder().
			ProjectKey(c.jobTaskSpec.ProjectKey).
			WorkItemTypeKey(c.jobTaskSpec.WorkItemTypeKey).
			WorkItemID(int64(task.ID)).
			TransitionID(task.TransitionID).
			Build(),
			sdkcore.WithAccessToken(resp.Data.Token),
			sdkcore.WithUserKey(spec.MeegoUserKey),
		)
		if err != nil {
			errMsg := fmt.Sprintf("work item: [%d] failed to do transition, err: %s", task.ID, err)
			logError(c.job, errMsg, c.logger)
			task.Status = string(config.StatusFailed)
			c.job.Status = config.StatusFailed
			return
		}
		if !resp.Success() {
			logError(c.job, fmt.Sprintf("failed to do transition, code: %d, message: %s", resp.Code(), resp.Error()), c.logger)
			task.Status = string(config.StatusFailed)
			c.job.Status = config.StatusFailed
			return
		}
		task.Status = string(config.StatusPassed)
	}

	for _, task := range c.jobTaskSpec.NodeWorkItems {
		resp, err := client.Client.WorkItem.NodeOperate(ctx, workitem.NewNodeOperateReqBuilder().
			ProjectKey(c.jobTaskSpec.ProjectKey).
			WorkItemTypeKey(c.jobTaskSpec.WorkItemTypeKey).
			WorkItemID(int64(task.ID)).
			NodeID(task.NodeID).
			Action("confirm").
			Build(),
			sdkcore.WithAccessToken(resp.Data.Token),
			sdkcore.WithUserKey(spec.MeegoUserKey),
		)
		if err != nil {
			errMsg := fmt.Sprintf("work item: [%d] failed to operate, err: %s", task.ID, err)
			logError(c.job, errMsg, c.logger)
			task.Status = string(config.StatusFailed)
			c.job.Status = config.StatusFailed
			return
		}
		if !resp.Success() {
			logError(c.job, fmt.Sprintf("failed to operate node, code: %d, message: %s", resp.Code(), resp.Error()), c.logger)
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
