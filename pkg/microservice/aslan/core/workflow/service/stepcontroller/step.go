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
	PreRun(ctx context.Context)
	// collect required info after job run.(like test report)
	AfterRun(ctx context.Context)
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
		stepCtl, err := instantiateStepCtl(step, jobPath, logger)
		if err != nil {
			return err
		}
		stepCtls = append(stepCtls, stepCtl)
	}
	for _, stepCtl := range stepCtls {
		stepCtl.PreRun(ctx)
	}
	return nil
}

func SummarizeSteps(ctx context.Context, workflowCtx *commonmodels.WorkflowTaskCtx, jobPath *string, steps []*commonmodels.StepTask, logger *zap.SugaredLogger) error {
	stepCtls := []StepCtl{}
	for _, step := range steps {
		stepCtl, err := instantiateStepCtl(step, jobPath, logger)
		if err != nil {
			return err
		}
		stepCtls = append(stepCtls, stepCtl)
	}
	for _, stepCtl := range stepCtls {
		stepCtl.AfterRun(ctx)
	}
	return nil
}

func instantiateStepCtl(step *commonmodels.StepTask, jobPath *string, logger *zap.SugaredLogger) (StepCtl, error) {
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
	default:
		logger.Infof("unknown step type: %s", step.StepType)
	}
	return stepCtl, err
}
