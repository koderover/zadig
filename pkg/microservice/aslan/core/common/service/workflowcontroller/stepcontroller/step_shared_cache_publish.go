/*
Copyright 2026 The KodeRover Authors.

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

type sharedCachePublishCtl struct {
	step *commonmodels.StepTask
	spec *step.StepSharedCachePublishSpec
}

func NewSharedCachePublishCtl(stepTask *commonmodels.StepTask, _ *zap.SugaredLogger) (*sharedCachePublishCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal shared cache publish spec error: %v", err)
	}
	publishSpec := &step.StepSharedCachePublishSpec{}
	if err := yaml.Unmarshal(yamlString, publishSpec); err != nil {
		return nil, fmt.Errorf("unmarshal shared cache publish spec error: %v", err)
	}
	stepTask.Spec = publishSpec
	return &sharedCachePublishCtl{step: stepTask, spec: publishSpec}, nil
}

func (s *sharedCachePublishCtl) PreRun(ctx context.Context) error {
	s.step.Spec = s.spec
	return nil
}

func (s *sharedCachePublishCtl) AfterRun(ctx context.Context) error {
	return nil
}
