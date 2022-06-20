package stepcontroller

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/types/step"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type shellCtl struct {
	step      *commonmodels.StepTask
	shellSpec *step.StepShellSpec
	log       *zap.SugaredLogger
}

func NewShellCtl(stepTask *commonmodels.StepTask, log *zap.SugaredLogger) (*shellCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal shell spec error: %v", err)
	}
	shellSpec := &step.StepShellSpec{}
	if err := yaml.Unmarshal(yamlString, &shellSpec); err != nil {
		return nil, fmt.Errorf("unmarshal shell spec error: %v", err)
	}
	return &shellCtl{shellSpec: shellSpec, log: log, step: stepTask}, nil
}

func (s *shellCtl) PreRun(ctx context.Context) error {
	return nil
}

func (s *shellCtl) Run(ctx context.Context) (config.Status, error) {
	return config.StatusPassed, nil
}

func (s *shellCtl) AfterRun(ctx context.Context) error {
	return nil
}
