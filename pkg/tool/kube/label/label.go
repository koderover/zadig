/*
 * Copyright 2022 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package label

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/crypto"
	"github.com/koderover/zadig/pkg/util"
)

// JobLabel is to describe labels that specify job identity
type JobLabel struct {
	PipelineName string
	TaskID       int64
	TaskType     string
	ServiceName  string
	PipelineType string
}

// GetJobLabels get labels k-v map from JobLabel struct
func GetJobLabels(jobLabel *JobLabel) map[string]string {
	name := jobLabel.PipelineName
	// limit length of TaskKey with taskID less than 64
	if len(name) >= 57 {
		name = name[:57]
	}
	retMap := map[string]string{
		setting.TaskLabel:         fmt.Sprintf("%s-%d", strings.ToLower(name), jobLabel.TaskID),
		setting.ServiceLabel:      strings.ToLower(util.ReturnValidLabelValue(jobLabel.ServiceName)),
		setting.TypeLabel:         strings.Replace(jobLabel.TaskType, "_", "-", -1),
		setting.PipelineTypeLable: jobLabel.PipelineType,
		setting.LabelHashKey:      crypto.Sha1([]byte(fmt.Sprintf("%s-%d", jobLabel.PipelineName, jobLabel.TaskID))),
	}
	// no need to add labels with empty value to a job
	for k, v := range retMap {
		if v == "" {
			delete(retMap, k)
		}
	}
	return retMap
}
