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

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/log"
)

type OfflineServiceJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskOfflineServiceSpec
	ack         func()
}

func NewOfflineServiceJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *OfflineServiceJobCtl {
	jobTaskSpec := &commonmodels.JobTaskOfflineServiceSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &OfflineServiceJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *OfflineServiceJobCtl) Clean(ctx context.Context) {}

func (c *OfflineServiceJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	env, err := mongodb.NewProductColl().Find(&mongodb.ProductFindOptions{
		Name:    c.workflowCtx.ProjectName,
		EnvName: c.jobTaskSpec.EnvName,
	})
	if err != nil {
		log.Errorf("OfflineServiceJobCtl: find product env error: %v", err)
		c.job.Error = fmt.Sprintf("find product env error: %v", err)
		c.job.Status = config.StatusFailed
		return
	}
	c.jobTaskSpec.Namespace = env.Namespace

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		c.job.Error = fmt.Sprintf("get kube client error: %v", err)
		c.job.Status = config.StatusFailed
		return
	}

	var fail bool
	for _, event := range c.jobTaskSpec.ServiceEvents {
		logger := c.logger.With("service", event.ServiceName)

		yaml, _, err := kube.FetchCurrentAppliedYaml(&kube.GeneSvcYamlOption{
			ProductName: c.workflowCtx.ProjectName,
			EnvName:     c.jobTaskSpec.EnvName,
			ServiceName: event.ServiceName,
			UnInstall:   true,
		})
		if err != nil {
			event.Error = fmt.Sprintf("fetch current applied yaml error: %v", err)
			event.Status = config.StatusFailed
			logger.Errorf("OfflineServiceJobCtl: fetch current applied yaml error: %v", err)
			fail = true
			continue
		}
		//todo debug
		logger.Debugf("OfflineServiceJobCtl: %s", yaml)

		err = UpdateProductServiceDeployInfo(&ProductServiceDeployInfo{
			ProductName: c.workflowCtx.ProjectName,
			EnvName:     c.jobTaskSpec.EnvName,
			ServiceName: event.ServiceName,
			Uninstall:   true,
		})
		if err != nil {
			event.Error = fmt.Sprintf("update product service deploy info error: %v", err)
			event.Status = config.StatusFailed
			logger.Errorf("OfflineServiceJobCtl: update product service deploy info error: %v", err)
			fail = true
			continue
		}

		_, err = kube.CreateOrPatchResource(&kube.ResourceApplyParam{
			ProductInfo:         env,
			ServiceName:         event.ServiceName,
			CurrentResourceYaml: yaml,
			KubeClient:          kubeClient,
			Uninstall:           true,
		}, c.logger.With("caller", "OfflineServiceJobCtl.Run"))
		if err != nil {
			event.Error = fmt.Sprintf("remove resource error: %v", err)
			event.Status = config.StatusFailed
			logger.Errorf("OfflineServiceJobCtl: create or patch resource error: %v", err)
			fail = true
			continue
		}
		event.Status = config.StatusPassed
	}
	if fail {
		c.job.Error = "offline some services failed"
		c.job.Status = config.StatusFailed
		return
	}

	c.job.Status = config.StatusPassed
	return
}

func (c *OfflineServiceJobCtl) SaveInfo(ctx context.Context) error {
	return mongodb.NewJobInfoColl().Create(ctx, &commonmodels.JobInfo{
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
