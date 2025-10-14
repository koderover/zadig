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

package stepcontroller

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

type StepCtl interface {
	// get required info from aslan before job run.
	PreRun(ctx context.Context) error
	// collect required info after job run.(like test report)
	AfterRun(ctx context.Context) error
}

func PrepareSteps(ctx context.Context, workflowCtx *commonmodels.WorkflowTaskCtx, jobPath *string, jobKey string, steps []*commonmodels.StepTask, logger *zap.SugaredLogger) error {
	stepCtls := []StepCtl{}
	for _, step := range steps {
		stepCtl, err := instantiateStepCtl(step, workflowCtx, jobPath, jobKey, logger)
		if err != nil {
			return err
		}
		stepCtls = append(stepCtls, stepCtl)
	}
	for _, stepCtl := range stepCtls {
		if err := stepCtl.PreRun(ctx); err != nil {
			return err
		}
	}
	return nil
}

func SummarizeSteps(ctx context.Context, workflowCtx *commonmodels.WorkflowTaskCtx, jobPath *string, jobKey string, steps []*commonmodels.StepTask, logger *zap.SugaredLogger) error {
	stepCtls := []StepCtl{}
	for _, step := range steps {
		stepCtl, err := instantiateStepCtl(step, workflowCtx, jobPath, jobKey, logger)
		if err != nil {
			return err
		}
		stepCtls = append(stepCtls, stepCtl)
	}
	for _, stepCtl := range stepCtls {
		if err := stepCtl.AfterRun(ctx); err != nil {
			return nil
		}
	}
	return nil
}

func instantiateStepCtl(step *commonmodels.StepTask, workflowCtx *commonmodels.WorkflowTaskCtx, jobPath *string, jobKey string, logger *zap.SugaredLogger) (StepCtl, error) {
	var stepCtl StepCtl
	var err error
	switch step.StepType {
	case config.StepGit:
		stepCtl, err = NewGitCtl(step, logger)
	case config.StepPerforce:
		stepCtl, err = NewP4Ctl(jobKey, step, workflowCtx, logger)
	case config.StepShell:
		stepCtl, err = NewShellCtl(step, logger)
	case config.StepPowerShell:
		stepCtl, err = NewPowerShellCtl(step, logger)
	case config.StepBatchFile:
		stepCtl, err = NewBatchFileCtl(step, logger)
	case config.StepDockerBuild:
		stepCtl, err = NewDockerBuildCtl(step, workflowCtx, logger)
	case config.StepTools:
		stepCtl, err = NewToolInstallCtl(step, jobPath, logger)
	case config.StepArchive:
		stepCtl, err = NewArchiveCtl(step, workflowCtx, logger)
	case config.StepArchiveHtml:
		stepCtl, err = NewArchiveHtmlCtl(step, workflowCtx, logger)
	case config.StepDownloadArchive:
		stepCtl, err = NewDownloadArchiveCtl(step, logger)
	case config.StepJunitReport:
		stepCtl, err = NewJunitReportCtl(step, workflowCtx, logger)
	case config.StepTarArchive:
		stepCtl, err = NewTarArchiveCtl(step, logger)
	case config.StepSonarCheck:
		stepCtl, err = NewSonarCheckCtl(step, workflowCtx, logger)
	case config.StepSonarGetMetrics:
		stepCtl, err = NewSonarGetMetricsCtl(step, workflowCtx, logger)
	case config.StepDistributeImage:
		stepCtl, err = NewDistributeCtl(step, workflowCtx, jobKey, logger)
	case config.StepDebugBefore, config.StepDebugAfter:
		stepCtl, err = NewDebugCtl()
	default:
		logger.Errorf("unknown step type: %s", step.StepType)
		return stepCtl, fmt.Errorf("unknown step type: %s", step.StepType)
	}
	return stepCtl, err
}
