/*
Copyright 2024 The KodeRover Authors.

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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type perforceCtl struct {
	step   *commonmodels.StepTask
	p4Spec *step.StepP4Spec
	log    *zap.SugaredLogger
}

func NewP4Ctl(stepTask *commonmodels.StepTask, workflowCtx *commonmodels.WorkflowTaskCtx, log *zap.SugaredLogger) (*perforceCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal git spec error: %v", err)
	}
	p4Spec := &step.StepP4Spec{}
	if err := yaml.Unmarshal(yamlString, &p4Spec); err != nil {
		return nil, fmt.Errorf("unmarshal git spec error: %v", err)
	}
	p4Spec.WorkflowName = workflowCtx.WorkflowName
	p4Spec.ProjectKey = workflowCtx.ProjectName
	p4Spec.TaskID = workflowCtx.TaskID
	stepTask.Spec = p4Spec
	return &perforceCtl{p4Spec: p4Spec, log: log, step: stepTask}, nil
}

func (s *perforceCtl) PreRun(ctx context.Context) error {
	for _, repo := range s.p4Spec.Repos {
		cID := repo.CodehostID
		if cID == 0 {
			log.Error("codehostID can't be empty")
			return fmt.Errorf("codehostID can't be empty")
		}
		detail, err := systemconfig.New().GetCodeHost(cID)
		if err != nil {
			s.log.Error(err)
			return err
		}
		repo.Source = detail.Type
		repo.PerforceHost = detail.P4Host
		repo.PerforcePort = detail.P4Port
		repo.Username = detail.Username
		repo.Password = detail.Password
	}
	s.step.Spec = s.p4Spec
	return nil
}

func (s *perforceCtl) AfterRun(ctx context.Context) error {
	return nil
}
