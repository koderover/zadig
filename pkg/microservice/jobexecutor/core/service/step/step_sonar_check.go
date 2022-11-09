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

package step

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/sonar"
	"github.com/koderover/zadig/pkg/types/step"
)

type SonarCheckStep struct {
	spec       *step.StepSonarCheckSpec
	envs       []string
	secretEnvs []string
	workspace  string
}

func NewSonarCheckStep(spec interface{}, workspace string, envs, secretEnvs []string) (*SonarCheckStep, error) {
	sonarCheckStep := &SonarCheckStep{workspace: workspace, envs: envs, secretEnvs: secretEnvs}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return sonarCheckStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &sonarCheckStep.spec); err != nil {
		return sonarCheckStep, fmt.Errorf("unmarshal spec %s to shell spec failed", yamlBytes)
	}
	return sonarCheckStep, nil
}

func (s *SonarCheckStep) Run(ctx context.Context) error {
	log.Info("Start check Sonar scanning quality gate status.")
	client := sonar.NewSonarClient(s.spec.SonarServer, s.spec.SonarToken)
	sonarWorkDir := sonar.GetSonarWorkDir(s.spec.Parameter)
	if sonarWorkDir == "" {
		sonarWorkDir = ".scannerwork"
	}
	if !filepath.IsAbs(sonarWorkDir) {
		sonarWorkDir = filepath.Join(s.workspace, s.spec.CheckDir, sonarWorkDir)
	}
	taskReportDir := filepath.Join(sonarWorkDir, "report-task.txt")
	bytes, err := ioutil.ReadFile(taskReportDir)
	if err != nil {
		log.Errorf("read sonar task report file: %s error :%v", taskReportDir, err)
		return err
	}
	taskReportContent := string(bytes)
	ceTaskID := sonar.GetSonarCETaskID(taskReportContent)
	if ceTaskID == "" {
		log.Error("can not get sonar ce task ID")
		return errors.New("can not get sonar ce task ID")
	}
	analysisID, err := client.WaitForCETaskTobeDone(ceTaskID, time.Minute*10)
	if err != nil {
		log.Error(err)
		return err
	}
	gateInfo, err := client.GetQualityGateInfo(analysisID)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Infof("Sonar quality gate status: %s", gateInfo.ProjectStatus.Status)
	sonar.PrintSonarConditionTables(gateInfo.ProjectStatus.Conditions)
	if gateInfo.ProjectStatus.Status != sonar.QualityGateOK && gateInfo.ProjectStatus.Status != sonar.QualityGateNone {
		return fmt.Errorf("sonar quality gate status was: %s", gateInfo.ProjectStatus.Status)
	}
	return nil
}
