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
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/types/step"
)

type junitReportCtl struct {
	step            *commonmodels.StepTask
	junitReportSpec *step.StepJunitReportSpec
	log             *zap.SugaredLogger
}

func NewJunitReportCtl(stepTask *commonmodels.StepTask, log *zap.SugaredLogger) (*junitReportCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal git spec error: %v", err)
	}
	junitReportSpec := &step.StepJunitReportSpec{}
	if err := yaml.Unmarshal(yamlString, &junitReportSpec); err != nil {
		return nil, fmt.Errorf("unmarshal git spec error: %v", err)
	}
	stepTask.Spec = junitReportSpec
	return &junitReportCtl{junitReportSpec: junitReportSpec, log: log, step: stepTask}, nil
}

func (s *junitReportCtl) PreRun(ctx context.Context) error {
	if s.junitReportSpec.S3Storage == nil {
		modelS3, err := commonrepo.NewS3StorageColl().FindDefault()
		if err != nil {
			return err
		}
		s.junitReportSpec.S3Storage = modelS3toS3(modelS3)
	}
	s.step.Spec = s.junitReportSpec
	return nil
}

func (s *junitReportCtl) AfterRun(ctx context.Context) error {
	return nil
}
