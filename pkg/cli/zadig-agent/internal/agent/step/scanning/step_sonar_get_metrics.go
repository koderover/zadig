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

package scanning

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/sonar"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

type SonarGetMetrics struct {
	spec       *step.StepSonarGetMetricsSpec
	envs       []string
	secretEnvs []string
	workspace  string
	dirs       *types.AgentWorkDirs
	Logger     *log.JobLogger
}

func NewSonarGetMetricsStep(spec interface{}, dirs *types.AgentWorkDirs, envs, secretEnvs []string, logger *log.JobLogger) (*SonarGetMetrics, error) {
	sonarCheckStep := &SonarGetMetrics{dirs: dirs, workspace: dirs.Workspace, envs: envs, secretEnvs: secretEnvs}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return sonarCheckStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &sonarCheckStep.spec); err != nil {
		return sonarCheckStep, fmt.Errorf("unmarshal spec %s to sonar get metrics spec failed", yamlBytes)
	}
	return sonarCheckStep, nil
}

func (s *SonarGetMetrics) Run(ctx context.Context) error {
	s.Logger.Infof("Start get Sonar scanning metrics.")
	sonarWorkDir := sonar.GetSonarWorkDir(s.spec.Parameter)
	if sonarWorkDir == "" {
		sonarWorkDir = ".scannerwork"
	}
	if !filepath.IsAbs(sonarWorkDir) {
		sonarWorkDir = filepath.Join(s.workspace, s.spec.CheckDir, sonarWorkDir)
	}
	taskReportDir := filepath.Join(sonarWorkDir, "report-task.txt")
	bytes, err := os.ReadFile(taskReportDir)
	if err != nil {
		s.Logger.Errorf("read sonar task report file: %s error :%v", time.Now().Format(setting.WorkflowTimeFormat), taskReportDir, err)
		return nil
	}
	taskReportContent := string(bytes)
	ceTaskID := sonar.GetSonarCETaskID(taskReportContent)
	if ceTaskID == "" {
		s.Logger.Errorf("can not get sonar ce task ID")
		return nil
	}

	outputFileName := filepath.Join(s.dirs.JobOutputsDir, setting.WorkflowScanningJobOutputKey)
	err = util.AppendToFile(outputFileName, ceTaskID)
	if err != nil {
		err = fmt.Errorf("append sonar ce task ID %s to output file %s error: %v", ceTaskID, outputFileName, err)
		s.Logger.Errorf(err.Error())
		return nil
	}

	if s.spec.ProjectKey == "" {
		projectKey := sonar.GetProjectKey(taskReportContent)
		if projectKey == "" {
			log.Error("can not get sonar project key")
			return nil
		}
		outputFileName = filepath.Join(s.dirs.JobOutputsDir, setting.WorkflowScanningJobOutputKeyProject)
		err = util.AppendToFile(outputFileName, projectKey)
		if err != nil {
			err = fmt.Errorf("append sonar project key %s to output file %s error: %v", ceTaskID, outputFileName, err)
			log.Error(err)
			return nil
		}
	}

	return nil
}
