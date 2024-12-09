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
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/sonar"
	"github.com/koderover/zadig/v2/pkg/types/job"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type sonarGetMetricsCtl struct {
	step                *commonmodels.StepTask
	sonarGetMetricsSpec *step.StepSonarGetMetricsSpec
	log                 *zap.SugaredLogger
	workflowCtx         *commonmodels.WorkflowTaskCtx
}

func NewSonarGetMetricsCtl(stepTask *commonmodels.StepTask, workflowCtx *commonmodels.WorkflowTaskCtx, log *zap.SugaredLogger) (*sonarGetMetricsCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal sonar check spec error: %v", err)
	}
	sonarGetMetricsSpec := &step.StepSonarGetMetricsSpec{}
	if err := yaml.Unmarshal(yamlString, &sonarGetMetricsSpec); err != nil {
		return nil, fmt.Errorf("unmarshal sonar check error: %v", err)
	}
	stepTask.Spec = sonarGetMetricsSpec
	return &sonarGetMetricsCtl{sonarGetMetricsSpec: sonarGetMetricsSpec, log: log, step: stepTask, workflowCtx: workflowCtx}, nil
}

func (s *sonarGetMetricsCtl) PreRun(ctx context.Context) error {
	return nil
}

func (s *sonarGetMetricsCtl) AfterRun(ctx context.Context) error {
	key := job.GetJobOutputKey(s.step.JobKey, setting.WorkflowScanningJobOutputKey)
	id, ok := s.workflowCtx.GlobalContextGet(key)
	if !ok {
		err := fmt.Errorf("sonar check job output %s not found", key)
		log.Error(err)
		return err
	}
	if s.sonarGetMetricsSpec.ProjectKey == "" {
		key := job.GetJobOutputKey(s.step.JobKey, setting.WorkflowScanningJobOutputKeyProject)
		projectKey, ok := s.workflowCtx.GlobalContextGet(key)
		if !ok {
			err := fmt.Errorf("sonar check job output %s not found", key)
			log.Error(err)
			return err
		}
		s.sonarGetMetricsSpec.ProjectKey = projectKey
	}
	if s.sonarGetMetricsSpec.Branch == "" {
		key := job.GetJobOutputKey(s.step.JobKey, setting.WorkflowScanningJobOutputKeyBranch)
		branch, ok := s.workflowCtx.GlobalContextGet(key)
		if !ok {
			err := fmt.Errorf("sonar check job output %s not found", key)
			log.Error(err)
			return err
		}
		s.sonarGetMetricsSpec.Branch = branch
	}

	client := sonar.NewSonarClient(s.sonarGetMetricsSpec.SonarServer, s.sonarGetMetricsSpec.SonarToken)
	analysisID, err := client.WaitForCETaskTobeDone(id, time.Minute*10)
	if err != nil {
		log.Error(err)
		return err
	}

	resp, err := client.GetComponentMeasures(s.sonarGetMetricsSpec.ProjectKey, s.sonarGetMetricsSpec.Branch)
	if err != nil {
		err = fmt.Errorf("get component measures error: %v", err)
		log.Error(err)
		return err
	}
	s.sonarGetMetricsSpec.SonarMetrics = &step.SonarMetrics{}
	for _, mesures := range resp.Component.Measures {
		switch mesures.Metric {
		case "coverage":
			s.sonarGetMetricsSpec.SonarMetrics.Coverage = mesures.Value
		case "bugs":
			s.sonarGetMetricsSpec.SonarMetrics.Bugs = mesures.Value
		case "vulnerabilities":
			s.sonarGetMetricsSpec.SonarMetrics.Vulnerabilities = mesures.Value
		case "code_smells":
			s.sonarGetMetricsSpec.SonarMetrics.CodeSmells = mesures.Value
		case "ncloc":
			s.sonarGetMetricsSpec.SonarMetrics.Ncloc = mesures.Value
		default:
			log.Warnf("unknown metric: %s: %s", mesures.Metric, mesures.Value)
		}
	}

	if s.sonarGetMetricsSpec.CheckQualityGate {
		gateInfo, err := client.GetQualityGateInfo(analysisID)
		if err != nil {
			log.Error(err)
			return err
		}
		s.sonarGetMetricsSpec.SonarMetrics.QualityGateStatus = gateInfo.ProjectStatus.Status
	}

	return nil
}
