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

package workflowcontroller

import (
	"context"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/jobcontroller"
	"go.uber.org/zap"
)

type CustomStageCtl struct {
	stage       *commonmodels.StageTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	ack         func()
}

func NewCustomStageCtl(stage *commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) *CustomStageCtl {
	return &CustomStageCtl{
		stage:       stage,
		logger:      logger,
		workflowCtx: workflowCtx,
		ack:         ack,
	}
}

func (c *CustomStageCtl) Run(ctx context.Context, concurrency int) {
	var workerConcurrency = 1
	if c.stage.Parallel {
		if len(c.stage.Jobs) > int(concurrency) {
			workerConcurrency = int(concurrency)
		} else {
			workerConcurrency = len(c.stage.Jobs)
		}

	}
	jobcontroller.RunJobs(ctx, c.stage.Jobs, c.workflowCtx, workerConcurrency, c.logger, c.ack)
}
