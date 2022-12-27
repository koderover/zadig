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

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/jira"
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
	info, err := mongodb.NewProjectManagementColl().GetJira()
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}
	client := jira.NewJiraClient(info.JiraUser, info.JiraToken, info.JiraHost)
	for _, issue := range c.jobTaskSpec.Issues {
		err = client.Issue.UpdateStatus(issue.Key, c.jobTaskSpec.TargetStatusID)
		if err != nil {
			logError(c.job, fmt.Sprintf("Update issue %s status error: %v", issue.Key, err), c.logger)
			return
		}
	}
	return
}
