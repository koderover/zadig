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

package step

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/microservice/jobexecutor/core/service/configmap"
	jobtypes "github.com/koderover/zadig/v2/pkg/types"
	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

type AIReviewReportStep struct {
	spec       *stepspec.StepAIReviewReportSpec
	workspace  string
	envs       []string
	secretEnvs []string
	updater    configmap.Updater
}

func NewAIReviewReportStep(spec interface{}, workspace string, envs, secretEnvs []string, updater configmap.Updater) (*AIReviewReportStep, error) {
	reportStep := &AIReviewReportStep{workspace: workspace, envs: envs, secretEnvs: secretEnvs, updater: updater}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("marshal AI review report spec: %w", err)
	}
	if err := yaml.Unmarshal(yamlBytes, &reportStep.spec); err != nil {
		return nil, fmt.Errorf("unmarshal AI review report spec: %w", err)
	}
	return reportStep, nil
}

func (s *AIReviewReportStep) Run(ctx context.Context) error {
	if s.updater == nil {
		return fmt.Errorf("AI review report ConfigMap updater is not configured")
	}
	envMap := util.MakeEnvMap(s.envs, s.secretEnvs)
	reportPath := util.ReplaceEnvWithValue(s.spec.ReportPath, envMap)
	if !filepath.IsAbs(reportPath) {
		reportPath = filepath.Join(s.workspace, reportPath)
	}
	reportPath = filepath.Clean(reportPath)
	workspace := filepath.Clean(s.workspace)
	if reportPath != workspace && !strings.HasPrefix(reportPath, workspace+string(os.PathSeparator)) {
		return fmt.Errorf("AI review report path %q is outside workspace", reportPath)
	}
	reportBytes, err := os.ReadFile(reportPath)
	if err != nil {
		return fmt.Errorf("read AI review report %q: %w", reportPath, err)
	}
	var report *stepspec.AIReviewReport
	if err := json.Unmarshal(reportBytes, &report); err != nil {
		return fmt.Errorf("AI review report is invalid JSON: %w", err)
	}
	if report == nil {
		return fmt.Errorf("AI review report cannot be null")
	}
	reportBytes, err = json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal AI review report: %w", err)
	}
	cm, err := s.updater.Get()
	if err != nil {
		return fmt.Errorf("get job ConfigMap for AI review report: %w", err)
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[jobtypes.JobAIReviewReportKey] = string(reportBytes)
	if err := s.updater.UpdateWithRetry(cm, 3, 3*time.Second); err != nil {
		return fmt.Errorf("write AI review report to job ConfigMap: %w", err)
	}
	return nil
}
