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

package util

import (
	"fmt"
	"regexp"
	"strings"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
)

var workflowJobNameRegx = regexp.MustCompile(setting.JobNameRegx)

func CalcWorkflowTaskRunningTime(task *commonmodels.WorkflowTask) int64 {
	runningTime := int64(0)
	for _, stage := range task.Stages {
		if stage.StartTime == 0 {
			runningTime += 0
		} else if task.EndTime == 0 {
			runningTime += 0
			// log.Errorf("workflow task %s/%d stage %s end time is 0", task.WorkflowName, task.TaskID, stage.Name)
		} else {
			runningTime += stage.EndTime - stage.StartTime
		}
	}
	return runningTime
}
func GenScanningWorkflowName(scanningID string) string {
	return fmt.Sprintf(setting.ScanWorkflowNamingConvention, scanningID)
}

func GenTestingWorkflowName(testingName string) string {
	return fmt.Sprintf(setting.TestWorkflowNamingConvention, testingName)
}

func GenerateTestingModuleJobName(name string) string {
	return strings.ToLower(name)
}

func GenerateScanningModuleJobName(name string) string {
	if len(name) >= 32 {
		return strings.TrimSuffix(name[:31], "-")
	}
	return name
}

func ValidateGeneratedWorkflowJobName(name string, generator func(string) string) error {
	jobName := generator(name)
	if !workflowJobNameRegx.MatchString(jobName) {
		return fmt.Errorf("name [%s] cannot be used to generate a workflow job name, generated job name [%s] did not match %s", name, jobName, setting.JobNameRegx)
	}
	return nil
}
