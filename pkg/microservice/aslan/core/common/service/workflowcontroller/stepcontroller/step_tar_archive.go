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
	"gopkg.in/yaml.v2"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/types/step"
)

type tarArchiveCtl struct {
	step           *commonmodels.StepTask
	tarArchiveSpec *step.StepTarArchiveSpec
	log            *zap.SugaredLogger
}

func NewTarArchiveCtl(stepTask *commonmodels.StepTask, log *zap.SugaredLogger) (*tarArchiveCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal tar archive spec error: %v", err)
	}
	tarArchiveSpec := &step.StepTarArchiveSpec{}
	if err := yaml.Unmarshal(yamlString, &tarArchiveSpec); err != nil {
		return nil, fmt.Errorf("unmarshal tar archive spec error: %v", err)
	}
	stepTask.Spec = tarArchiveSpec
	return &tarArchiveCtl{tarArchiveSpec: tarArchiveSpec, log: log, step: stepTask}, nil
}

func (s *tarArchiveCtl) PreRun(ctx context.Context) error {
	if s.tarArchiveSpec.S3Storage == nil {
		modelS3, err := commonrepo.NewS3StorageColl().FindDefault()
		if err != nil {
			return err
		}
		s.tarArchiveSpec.S3Storage = modelS3toS3(modelS3)
	}
	s.step.Spec = s.tarArchiveSpec
	return nil
}

func (s *tarArchiveCtl) AfterRun(ctx context.Context) error {
	return nil
}
