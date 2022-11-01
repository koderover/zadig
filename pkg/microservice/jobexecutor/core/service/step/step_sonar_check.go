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
	"strings"
	"time"

	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/sonar"
	"github.com/koderover/zadig/pkg/types/step"
	"gopkg.in/yaml.v3"
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
	sonarWorkDir := getKeyValue(s.spec.Parameter, "sonar.working.directory")
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
	ceTaskID := getKeyValue(taskReportContent, "ceTaskId")
	if ceTaskID == "" {
		log.Error("can not get sonar ce task ID")
		return errors.New("can not get sonar ce task ID")
	}
	analysisID, err := s.waitForCETaskTobeDone(ceTaskID)
	if err != nil {
		log.Error(err)
		return err
	}
	client := sonar.NewSonarClient(s.spec.SonarServer, s.spec.SonarToken)
	gateInfo, err := client.GetQualityGateInfo(analysisID)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Infof("Sonar quality gate status: %s", gateInfo.ProjectStatus.Status)
	printSonarConditionTables(gateInfo.ProjectStatus.Conditions)
	if gateInfo.ProjectStatus.Status != sonar.QualityGateOK && gateInfo.ProjectStatus.Status != sonar.QualityGateNone {
		return fmt.Errorf("sonar quality gate status was: %s", gateInfo.ProjectStatus.Status)
	}
	return nil
}

func (s *SonarCheckStep) waitForCETaskTobeDone(taskID string) (string, error) {
	client := sonar.NewSonarClient(s.spec.SonarServer, s.spec.SonarToken)
	timeout := time.After(10 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-timeout:
			return "", errors.New("sonar ce task excution timeout 10m")
		case <-ticker.C:
			taskInfo, err := client.GetCETaskInfo(taskID)
			if err != nil {
				return "", fmt.Errorf("get sonar ce task info error: %v", err)
			}
			if taskInfo.Task.Status == sonar.CETaskSuccess {
				return taskInfo.Task.AnalysisID, nil
			}
			if taskInfo.Task.Status == sonar.CETaskCanceled || taskInfo.Task.Status == sonar.CETaskFailed {
				return "", fmt.Errorf("sonar ce task status was %s", taskInfo.Task.Status)
			}
		}
	}
}

func printSonarConditionTables(conditions []sonar.Condition) {
	fmt.Printf("%-40s|%-10s|%-10s|%-10s|%-20s|\n", "Metric", "Status", "Operator", "Threshold", "Actualvalue")
	for _, condition := range conditions {
		fmt.Printf("%-40s|%-10s|%-10s|%-10s|%-20s|\n", condition.MetricKey, condition.Status, condition.Comparator, condition.ErrorThreshold, condition.ActualValue)
	}
	fmt.Printf("\n")
}

func getKeyValue(content, inputKey string) string {
	kvStrs := strings.Split(content, "\n")
	for _, kvStr := range kvStrs {
		kvStr = strings.TrimSpace(string(kvStr))
		index := strings.Index(kvStr, "=")
		if index < 0 {
			continue
		}
		key := strings.TrimSpace(kvStr[:index])
		if len(key) == 0 {
			continue
		}
		if key != inputKey {
			continue
		}
		return strings.TrimSpace(kvStr[index+1:])
	}
	return ""
}
