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
	"strings"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type StepCtl interface {
	// get required info from aslan before job run.
	PreRun(ctx context.Context) error
	// run specific step, now only use for deploy job (freestyle job merge step together)
	Run(ctx context.Context) (config.Status, error)

	// collect required info after job run.(like test report)
	AfterRun(ctx context.Context) error
}

func PrepareSteps(ctx context.Context, workflowCtx *commonmodels.WorkflowTaskCtx, jobPath *string, steps []*commonmodels.StepTask, logger *zap.SugaredLogger) error {
	// inject global context into step spec.
	workflowCtx.GlobalContextEach(func(k, v string) bool {
		for _, step := range steps {
			yamlString, err := yaml.Marshal(step.Spec)
			if err != nil {
				logger.Errorf("marshal step spec error: %v", err)
				return false
			}
			replacedString := strings.ReplaceAll(string(yamlString), fmt.Sprintf("$(%s)", k), v)
			if err := yaml.Unmarshal([]byte(replacedString), &step.Spec); err != nil {
				logger.Errorf("unmarshal step spec error: %v", err)
				return false
			}
		}
		return true
	})

	stepCtls := []StepCtl{}
	for _, step := range steps {
		stepCtl, err := instantiateStepCtl(step, workflowCtx, jobPath, logger)
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

func RunSteps(ctx context.Context, workflowCtx *commonmodels.WorkflowTaskCtx, jobPath *string, steps []*commonmodels.StepTask, logger *zap.SugaredLogger) (config.Status, error) {
	stepCtls := []StepCtl{}
	for _, step := range steps {
		stepCtl, err := instantiateStepCtl(step, workflowCtx, jobPath, logger)
		if err != nil {
			return config.StatusFailed, err
		}
		stepCtls = append(stepCtls, stepCtl)
	}
	for _, stepCtl := range stepCtls {
		status, err := stepCtl.Run(ctx)
		if err != nil || status != config.StatusPassed {
			return status, err
		}
	}
	return config.StatusPassed, nil
}

func SummarizeSteps(ctx context.Context, workflowCtx *commonmodels.WorkflowTaskCtx, jobPath *string, steps []*commonmodels.StepTask, logger *zap.SugaredLogger) error {
	stepCtls := []StepCtl{}
	for _, step := range steps {
		stepCtl, err := instantiateStepCtl(step, workflowCtx, jobPath, logger)
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

func instantiateStepCtl(step *commonmodels.StepTask, workflowCtx *commonmodels.WorkflowTaskCtx, jobPath *string, logger *zap.SugaredLogger) (StepCtl, error) {
	var stepCtl StepCtl
	var err error
	switch step.StepType {
	case config.StepGit:
		stepCtl, err = NewGitCtl(step, logger)
	case config.StepShell:
		stepCtl, err = NewShellCtl(step, logger)
	case config.StepDockerBuild:
		stepCtl, err = NewDockerBuildCtl(step, logger)
	case config.StepTools:
		stepCtl, err = NewToolInstallCtl(step, jobPath, logger)
	case config.StepArchive:
		stepCtl, err = NewArchiveCtl(step, logger)
	case config.StepDeploy:
		stepCtl, err = NewDeployCtl(step, workflowCtx, logger)
	case config.StepHelmDeploy:
		stepCtl, err = NewHelmDeployCtl(step, workflowCtx, logger)
	default:
		logger.Infof("unknown step type: %s", step.StepType)
	}
	return stepCtl, err
}
