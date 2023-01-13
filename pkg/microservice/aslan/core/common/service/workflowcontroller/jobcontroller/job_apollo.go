/*
 * Copyright 2023 The KodeRover Authors.
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
	"net/url"
	"time"

	"go.uber.org/zap"

	config2 "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/apollo"
)

type ApolloJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskApolloSpec
	ack         func()
}

func NewApolloJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *ApolloJobCtl {
	jobTaskSpec := &commonmodels.JobTaskApolloSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &ApolloJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *ApolloJobCtl) Clean(ctx context.Context) {}

func (c *ApolloJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	info, err := mongodb.NewConfigurationManagementColl().GetApolloByID(context.Background(), c.jobTaskSpec.ApolloID)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}
	link := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		config2.SystemAddress(),
		c.workflowCtx.ProjectName,
		c.workflowCtx.WorkflowName,
		c.workflowCtx.TaskID,
		url.QueryEscape(c.workflowCtx.WorkflowDisplayName))

	var fail bool
	client := apollo.NewClient(info.ServerAddress, info.Token)
	for _, namespace := range c.jobTaskSpec.NamespaceList {
		for _, kv := range namespace.KeyValList {
			err := client.UpdateKeyVal(namespace.AppID, namespace.Env, namespace.ClusterID, namespace.Namespace, kv.Key, kv.Val, "zadig")
			if err != nil {
				fail = true
				namespace.Error = fmt.Sprintf("update error: %v", err)
				continue
			}
		}
		err := client.Release(namespace.AppID, namespace.Env, namespace.ClusterID, namespace.Namespace,
			&apollo.ReleaseArgs{
				ReleaseTitle:   time.Now().Format("20060102150405") + "-zadig",
				ReleaseComment: fmt.Sprintf("工作流 %s\n详情: %s", c.workflowCtx.WorkflowDisplayName, link),
				ReleasedBy:     "zadig",
			})
		if err != nil {
			fail = true
			namespace.Error = fmt.Sprintf("release error: %v", err)
		}
	}
	if fail {
		logError(c.job, "some errors occurred in apollo job", c.logger)
		return
	}
	c.job.Status = config.StatusPassed
	return
}
