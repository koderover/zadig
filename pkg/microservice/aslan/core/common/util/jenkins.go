/*
Copyright 2025 The KodeRover Authors.

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

import commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"

func ApplyJenkinsParameter(origin, input []*commonmodels.JenkinsJobParameter) []*commonmodels.JenkinsJobParameter {
	resp := make([]*commonmodels.JenkinsJobParameter, 0)
	inputMap := make(map[string]*commonmodels.JenkinsJobParameter)
	for _, kv := range input {
		inputMap[kv.Name] = kv
	}

	for _, kv := range origin {
		item := &commonmodels.JenkinsJobParameter{
			Name:    kv.Name,
			Value:   kv.Value,
			Type:    kv.Type,
			Choices: kv.Choices,
			Source:  kv.Source,
		}

		if i, ok := inputMap[kv.Name]; ok {
			item.Value = i.Value
		}

		resp = append(resp, item)
	}
	return resp
}
