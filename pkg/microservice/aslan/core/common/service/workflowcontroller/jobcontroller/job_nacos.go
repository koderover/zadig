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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/tool/nacos"
	"go.uber.org/zap"
)

type NacosJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskNacosSpec
	ack         func()
}

func NewNacosJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *NacosJobCtl {
	jobTaskSpec := &commonmodels.JobTaskNacosSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &NacosJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *NacosJobCtl) Clean(ctx context.Context) {}

func (c *NacosJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	client, err := nacos.NewNacosClient(c.jobTaskSpec.NacosAddr, c.jobTaskSpec.UserName, c.jobTaskSpec.Password)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}
	for _, data := range c.jobTaskSpec.NacosDatas {
		if err := client.UpdateConfig(data.DataID, data.Group, c.jobTaskSpec.NamespaceID, data.Content, data.Format); err != nil {
			data.Error = err.Error()
			logError(c.job, err.Error(), c.logger)
			return
		}
	}
	c.job.Status = config.StatusPassed
}
