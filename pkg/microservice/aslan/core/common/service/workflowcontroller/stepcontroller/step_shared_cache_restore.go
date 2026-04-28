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
	typesstep "github.com/koderover/zadig/v2/pkg/types/step"
)

type sharedCacheRestoreCtl struct {
	step *commonmodels.StepTask
	spec *typesstep.StepSharedCacheRestoreSpec
}

func NewSharedCacheRestoreCtl(stepTask *commonmodels.StepTask, _ *zap.SugaredLogger) (*sharedCacheRestoreCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal shared cache restore spec error: %v", err)
	}
	restoreSpec := &typesstep.StepSharedCacheRestoreSpec{}
	if err := yaml.Unmarshal(yamlString, restoreSpec); err != nil {
		return nil, fmt.Errorf("unmarshal shared cache restore spec error: %v", err)
	}
	stepTask.Spec = restoreSpec
	return &sharedCacheRestoreCtl{step: stepTask, spec: restoreSpec}, nil
}

func (s *sharedCacheRestoreCtl) PreRun(ctx context.Context) error {
	s.step.Spec = s.spec
	return nil
}

func (s *sharedCacheRestoreCtl) AfterRun(ctx context.Context) error {
	return nil
}
