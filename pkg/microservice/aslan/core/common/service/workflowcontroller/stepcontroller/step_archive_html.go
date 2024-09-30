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

type archiveHtmlCtl struct {
	step            *commonmodels.StepTask
	archiveHtmlSpec *step.StepArchiveHtmlSpec
	workflowCtx     *commonmodels.WorkflowTaskCtx
	log             *zap.SugaredLogger
}

func NewArchiveHtmlCtl(stepTask *commonmodels.StepTask, workflowCtx *commonmodels.WorkflowTaskCtx, log *zap.SugaredLogger) (*archiveHtmlCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal archive spec error: %v", err)
	}
	archiveHtmlSpec := &step.StepArchiveHtmlSpec{}
	if err := yaml.Unmarshal(yamlString, &archiveHtmlSpec); err != nil {
		return nil, fmt.Errorf("unmarshal archive spec error: %v", err)
	}
	stepTask.Spec = archiveHtmlSpec
	return &archiveHtmlCtl{archiveHtmlSpec: archiveHtmlSpec, workflowCtx: workflowCtx, log: log, step: stepTask}, nil
}

func (s *archiveHtmlCtl) PreRun(ctx context.Context) error {
	if len(s.archiveHtmlSpec.HtmlPath) > 0 {
		return nil
	}
	if len(s.archiveHtmlSpec.OutputPath) > 0 {
		return fmt.Errorf("output path can't be empty")
	}
	s.step.Spec = s.archiveHtmlSpec
	return nil
}

func (s *archiveHtmlCtl) AfterRun(ctx context.Context) error {
	return nil
}
