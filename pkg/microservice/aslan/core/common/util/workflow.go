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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
)

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
