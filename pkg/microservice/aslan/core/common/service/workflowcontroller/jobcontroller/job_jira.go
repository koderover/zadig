/*
 * Copyright 2022 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package jobcontroller

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/jira"
)

type JiraJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskJiraSpec
	ack         func()
}

func NewJiraJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *JiraJobCtl {
	jobTaskSpec := &commonmodels.JobTaskJiraSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &JiraJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *JiraJobCtl) Clean(ctx context.Context) {}

func (c *JiraJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	info, err := mongodb.NewProjectManagementColl().GetJiraByID(c.jobTaskSpec.JiraID)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}
	if len(c.jobTaskSpec.Issues) == 0 {
		logError(c.job, "issues not found in job spec", c.logger)
		return
	}

	spec := &commonmodels.ProjectManagementJiraSpec{}
	err = commonmodels.IToi(info.Spec, spec)
	if err != nil {
		logError(c.job, fmt.Sprintf("failed to convert job spec to jira spec: %v", err), c.logger)
		return
	}

	client := jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType)
	for _, issue := range c.jobTaskSpec.Issues {
		list, err := client.Issue.GetTransitions(issue.Key)
		if err != nil {
			logError(c.job, fmt.Sprintf("GetTransitions issue %s error: %v", issue.Key, err), c.logger)
			issue.Status = string(config.StatusFailed)
			return
		}
		var id string
		for _, transition := range list {
			if transition.To.Name == c.jobTaskSpec.TargetStatus {
				id = transition.ID
				break
			}
		}
		if id == "" {
			logError(c.job, fmt.Sprintf("Issue %s failed to find status %s transition id", issue.Key, c.jobTaskSpec.TargetStatus), c.logger)
			issue.Status = string(config.StatusFailed)
			return
		}
		err = client.Issue.UpdateStatus(issue.Key, id)
		if err != nil {
			logError(c.job, fmt.Sprintf("Update issue %s status error: %v", issue.Key, err), c.logger)
			issue.Status = string(config.StatusFailed)
		} else {
			issue.Status = string(config.StatusPassed)
		}
	}
	c.job.Status = config.StatusPassed
	return
}

func (c *JiraJobCtl) SaveInfo(ctx context.Context) error {
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
