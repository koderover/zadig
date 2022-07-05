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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/types/step"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type archiveCtl struct {
	step        *commonmodels.StepTask
	archiveSpec *step.StepArchiveSpec
	log         *zap.SugaredLogger
}

func NewArchiveCtl(stepTask *commonmodels.StepTask, log *zap.SugaredLogger) (*archiveCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal tool install spec error: %v", err)
	}
	archiveSpec := &step.StepArchiveSpec{}
	if err := yaml.Unmarshal(yamlString, &archiveSpec); err != nil {
		return nil, fmt.Errorf("unmarshal git spec error: %v", err)
	}
	return &archiveCtl{archiveSpec: archiveSpec, log: log, step: stepTask}, nil
}

func (s *archiveCtl) PreRun(ctx context.Context) error {
	return nil
}

func (s *archiveCtl) Run(ctx context.Context) (config.Status, error) {
	return config.StatusPassed, nil
}

func (s *archiveCtl) AfterRun(ctx context.Context) error {
	return nil
}
