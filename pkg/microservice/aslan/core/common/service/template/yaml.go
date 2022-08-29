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

package template

import (
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
)

// GetTemplateVariableYaml returns variable yaml of yamlTemplate or templateService
func GetTemplateVariableYaml(variables []*models.Variable, variableYaml string) (string, error) {
	if len(variableYaml) > 0 {
		return variableYaml, nil
	}
	if len(variables) == 0 {
		return "", nil
	}
	valuesMap := make(map[string]interface{})
	for _, v := range variables {
		valuesMap[v.Key] = v.Value
	}
	yamlBs, err := yaml.Marshal(valuesMap)
	return string(yamlBs), err
}
