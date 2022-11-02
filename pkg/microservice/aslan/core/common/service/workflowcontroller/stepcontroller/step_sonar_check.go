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
	"gopkg.in/yaml.v3"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/types/step"
)

type sonarCheckCtl struct {
	step           *commonmodels.StepTask
	tarArchiveSpec *step.StepSonarCheckSpec
	log            *zap.SugaredLogger
}

func NewSonarCheckCtl(stepTask *commonmodels.StepTask, log *zap.SugaredLogger) (*sonarCheckCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal sonar check spec error: %v", err)
	}
	sonarCheckSpec := &step.StepSonarCheckSpec{}
	if err := yaml.Unmarshal(yamlString, &sonarCheckSpec); err != nil {
		return nil, fmt.Errorf("unmarshal sonar check error: %v", err)
	}
	stepTask.Spec = sonarCheckSpec
	return &sonarCheckCtl{tarArchiveSpec: sonarCheckSpec, log: log, step: stepTask}, nil
}

func (s *sonarCheckCtl) PreRun(ctx context.Context) error {
	return nil
}

func (s *sonarCheckCtl) AfterRun(ctx context.Context) error {
	return nil
}
