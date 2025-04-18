/*
Copyright 2023 The KodeRover Authors.

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

package job

import (
	"fmt"
	"strings"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

type KeyValMap struct {
	keyValMap map[string]*commonmodels.KeyVal
}

func NewKeyValMap() *KeyValMap {
	return &KeyValMap{}
}

func (m *KeyValMap) Insert(keyVals ...*commonmodels.KeyVal) {
	if m.keyValMap == nil {
		m.keyValMap = make(map[string]*commonmodels.KeyVal)
	}
	for _, keyVal := range keyVals {
		if _, ok := m.keyValMap[keyVal.Key]; ok {
			continue
		}
		m.keyValMap[keyVal.Key] = keyVal
	}
}

func (m *KeyValMap) List() []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	for _, kv := range m.keyValMap {
		ret = append(ret, kv)
	}
	return ret
}
func genJobDisplayName(jobName string, options ...string) string {
	parts := append([]string{jobName}, options...)
	return strings.Join(parts, "-")
}

func genJobKey(jobName string, options ...string) string {
	parts := append([]string{jobName}, options...)
	return strings.Join(parts, ".")
}

func GenJobName(workflow *commonmodels.WorkflowV4, jobName string, subTaskID int) string {
	stageName := ""
	stageIndex := 0
	jobIndex := 0
	for i, stage := range workflow.Stages {
		for j, job := range stage.Jobs {
			if job.Name == jobName {
				stageName = stage.Name
				stageIndex = i
				jobIndex = j
				break
			}
		}
	}

	_ = stageName

	return fmt.Sprintf("job-%d-%d-%d-%s", stageIndex, jobIndex, subTaskID, jobName)
}
