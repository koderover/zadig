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
	"encoding/json"
	"fmt"
	"time"

	sae "github.com/alibabacloud-go/sae-20190506/client"
	"github.com/alibabacloud-go/tea/tea"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	cloudservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/cloudservice"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/util/converter"
)

type SAEDeployJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskSAEDeploySpec
	ack         func()
}

func NewSAEDeployJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *SAEDeployJobCtl {
	jobTaskSpec := &commonmodels.JobTaskSAEDeploySpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &SAEDeployJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *SAEDeployJobCtl) Clean(ctx context.Context) {}

func (c *SAEDeployJobCtl) Run(ctx context.Context) {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: c.workflowCtx.ProjectName, EnvName: c.jobTaskSpec.Env, Production: &c.jobTaskSpec.Production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("failed to find SAE env, projectName: %s, envName: %s, production %v, error: %s", c.workflowCtx.ProjectName, c.jobTaskSpec.Env, c.jobTaskSpec.Production, err)
		c.logger.Error(err)
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(ctx, setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("failed to find default sae, err: %s", err)
		c.logger.Error(err)
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("failed to create sae client, err: %s", err)
		c.logger.Error(err)
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return
	}

	saeEnvs, err := cloudservice.ToSAEKVString(c.jobTaskSpec.Envs)
	if err != nil {
		err = fmt.Errorf("failed to serialize sae envs, err: %s", err)
		c.logger.Error(err)
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return
	}

	lowerCamelUpdateStrategy, err := converter.ConvertToLowerCamelCase(c.jobTaskSpec.UpdateStrategy)
	if err != nil {
		err = fmt.Errorf("failed to serialize sae update strategy, err: %s", err)
		c.logger.Error(err)
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return
	}

	updateStrategyBytes, err := json.Marshal(lowerCamelUpdateStrategy)
	if err != nil {
		err = fmt.Errorf("failed to serialize sae update strategy, err: %s", err)
		c.logger.Error(err)
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return
	}

	saeRequestUpdateStrategy := tea.String(string(updateStrategyBytes))
	if len(lowerCamelUpdateStrategy) == 0 {
		saeRequestUpdateStrategy = nil
	}

	saeRequest := &sae.DeployApplicationRequest{
		AppId:                 tea.String(c.jobTaskSpec.AppID),
		BatchWaitTime:         tea.Int32(c.jobTaskSpec.BatchWaitTime),
		Envs:                  saeEnvs,
		ImageUrl:              tea.String(c.jobTaskSpec.Image),
		MinReadyInstanceRatio: tea.Int32(c.jobTaskSpec.MinReadyInstanceRatio),
		MinReadyInstances:     tea.Int32(c.jobTaskSpec.MinReadyInstances),
		PackageType:           tea.String("Image"),
		UpdateStrategy:        saeRequestUpdateStrategy,
	}

	saeResp, err := saeClient.DeployApplication(saeRequest)
	if err != nil {
		err = fmt.Errorf("failed to create change order for appID: %s, err: %s", c.jobTaskSpec.AppID, err)
		c.logger.Error(err)
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return
	}

	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("failed to create change order for appID: %s, statusCode: %d, code: %s, errCode: %s, message: %s", c.jobTaskSpec.AppID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		c.logger.Error(err)
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return
	}

	c.jobTaskSpec.ChangeOrderID = tea.StringValue(saeResp.Body.Data.ChangeOrderId)
	c.ack()

	c.wait(ctx, saeClient)
}

func (c *SAEDeployJobCtl) wait(ctx context.Context, client *sae.Client) {
	//timeout := time.After(time.Duration(setting.DeployTimeout) * time.Second)

	for {
		select {
		case <-ctx.Done():
			c.job.Status = config.StatusCancelled

			saeRequest := &sae.AbortAndRollbackChangeOrderRequest{ChangeOrderId: tea.String(c.jobTaskSpec.ChangeOrderID)}
			saeResp, err := client.AbortAndRollbackChangeOrder(saeRequest)
			if err != nil {
				err = fmt.Errorf("failed to rollback change order, orderID: %s, appID: %s, err: %s", c.jobTaskSpec.ChangeOrderID, c.jobTaskSpec.AppID, err)
				c.logger.Warn(err)
			}

			if !tea.BoolValue(saeResp.Body.Success) {
				err = fmt.Errorf("failed to rollback change order, appID: %s, statusCode: %d, code: %s, errCode: %s, message: %s", c.jobTaskSpec.AppID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
				c.logger.Warn(err)
			}

			return
		//case <-timeout:
		//	// TODO: maybe add logic?
		//	c.job.Status = config.StatusTimeout
		//	return

		default:
			time.Sleep(time.Second * 2)

			saeRequest := &sae.DescribeChangeOrderRequest{ChangeOrderId: tea.String(c.jobTaskSpec.ChangeOrderID)}
			saeResp, err := client.DescribeChangeOrder(saeRequest)
			if err != nil {
				err = fmt.Errorf("failed to get change order detail, orderID: %s, appID: %s, err: %s", c.jobTaskSpec.ChangeOrderID, c.jobTaskSpec.AppID, err)
				c.logger.Warn(err)
			}

			if !tea.BoolValue(saeResp.Body.Success) {
				err = fmt.Errorf("failed to get change order detail, appID: %s, statusCode: %d, code: %s, errCode: %s, message: %s", c.jobTaskSpec.AppID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
				c.logger.Warn(err)
			}

			switch tea.Int32Value(saeResp.Body.Data.Status) {
			case 0:
				c.job.Status = config.StatusPrepare
				c.ack()
			case 1, 8, 9, 11, 12:
				c.job.Status = config.StatusRunning
				c.ack()
			case 2:
				c.job.Status = config.StatusPassed
				c.ack()
				return
			case 3, 10:
				c.job.Status = config.StatusFailed
				c.ack()
				return
			case 6:
				c.job.Status = config.StatusCancelled
				c.ack()
				return
			}
		}
	}
}

func (c *SAEDeployJobCtl) SaveInfo(ctx context.Context) error {
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

		ServiceType:   "sae",
		ServiceName:   c.jobTaskSpec.ServiceName,
		ServiceModule: c.jobTaskSpec.ServiceModule,
		TargetEnv:     c.jobTaskSpec.Env,
		Production:    c.jobTaskSpec.Production,
	})
}
