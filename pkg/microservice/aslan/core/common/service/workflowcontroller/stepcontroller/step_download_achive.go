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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type downloadArchiveCtl struct {
	step             *commonmodels.StepTask
	downloadArtifact *step.StepDownloadArchiveSpec
	log              *zap.SugaredLogger
}

func NewDownloadArchiveCtl(stepTask *commonmodels.StepTask, log *zap.SugaredLogger) (*downloadArchiveCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal archive spec error: %v", err)
	}
	downloadArchiveSpec := &step.StepDownloadArchiveSpec{}
	if err := yaml.Unmarshal(yamlString, &downloadArchiveSpec); err != nil {
		return nil, fmt.Errorf("unmarshal archive spec error: %v", err)
	}
	stepTask.Spec = downloadArchiveSpec
	return &downloadArchiveCtl{downloadArtifact: downloadArchiveSpec, log: log, step: stepTask}, nil
}

func (s *downloadArchiveCtl) PreRun(ctx context.Context) error {
	if s.downloadArtifact.S3 != nil {
		return nil
	}
	s.step.Spec = s.downloadArtifact
	return nil
}

func (s *downloadArchiveCtl) AfterRun(ctx context.Context) error {
	return nil
}
