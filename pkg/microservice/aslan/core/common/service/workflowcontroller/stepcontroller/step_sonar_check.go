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
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/sonar"
	"github.com/koderover/zadig/v2/pkg/types/job"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type sonarCheckCtl struct {
	step           *commonmodels.StepTask
	sonarCheckSpec *step.StepSonarCheckSpec
	log            *zap.SugaredLogger
	workflowCtx    *commonmodels.WorkflowTaskCtx
}

func NewSonarCheckCtl(stepTask *commonmodels.StepTask, workflowCtx *commonmodels.WorkflowTaskCtx, log *zap.SugaredLogger) (*sonarCheckCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal sonar check spec error: %v", err)
	}
	sonarCheckSpec := &step.StepSonarCheckSpec{}
	if err := yaml.Unmarshal(yamlString, &sonarCheckSpec); err != nil {
		return nil, fmt.Errorf("unmarshal sonar check error: %v", err)
	}
	stepTask.Spec = sonarCheckSpec
	return &sonarCheckCtl{sonarCheckSpec: sonarCheckSpec, log: log, step: stepTask, workflowCtx: workflowCtx}, nil
}

func (s *sonarCheckCtl) PreRun(ctx context.Context) error {
	return nil
}

func (s *sonarCheckCtl) AfterRun(ctx context.Context) error {
	key := job.GetJobOutputKey(s.step.JobKey, setting.WorkflowScanningJobOutputKey)
	id, ok := s.workflowCtx.GlobalContextGet(key)
	if !ok {
		err := fmt.Errorf("sonar check job output %s not found", key)
		log.Error(err)
		return err
	}

	client := sonar.NewSonarClient(s.sonarCheckSpec.SonarServer, s.sonarCheckSpec.SonarToken)
	taskInfo, err := client.GetCETaskInfo(id)
	if err != nil {
		err = fmt.Errorf("get ce task %s info error: %v", id, err)
		log.Error(err)
		return err
	}
	resp, err := client.GetComponentMeasures(taskInfo.Task.ComponentKey)
	if err != nil {
		err = fmt.Errorf("get component measures error: %v", err)
		log.Error(err)
		return err
	}
	for _, mesures := range resp.Component.Measures {
		_ = mesures
	}

	return nil
}
