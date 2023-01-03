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
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
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

// SafeMergeVariableYaml merge yamls
// support go template grammar, you can put {{.xxx}} as value in variable yaml
func SafeMergeVariableYaml(variableYamls ...string) (string, map[string]string, error) {
	templateKv := make(map[string]string)
	yamlsToMerge := make([][]byte, 0)

	for _, vYaml := range variableYamls {
		if len(vYaml) == 0 {
			continue
		}
		kvs, err := GetYamlVariables(vYaml, log.SugaredLogger())
		if err != nil {
			return "", nil, fmt.Errorf("failed to get variable from yaml: %s, err: %s", vYaml, err)
		}
		// parse go template variables
		for _, kv := range kvs {
			vYaml = strings.ReplaceAll(vYaml, fmt.Sprintf("{{.%s}}", kv.Key), fmt.Sprintf("$%s$", kv.Key))
			templateKv[fmt.Sprintf("$%s$", kv.Key)] = fmt.Sprintf("{{.%s}}", kv.Key)
		}
		yamlsToMerge = append(yamlsToMerge, []byte(vYaml))
	}

	mergedYaml, err := yamlutil.Merge(yamlsToMerge)
	if err != nil {
		return "", nil, fmt.Errorf("failed to merge variable yamls, err: %s", err)
	}
	return string(mergedYaml), templateKv, nil
}

func getParameterKey(parameter string) string {
	a := strings.TrimPrefix(parameter, "{{.")
	return strings.TrimSuffix(a, "}}")
}

// GetYamlVariables extract variables from go template yaml to Zadig-defined vars
// use regex to search {{.rar}} and turn to {key: var}
// NOTE this function DOES NOT support full go-template grammar, like {{- range }} / {{- eq }} / {{- if }} / nested data
// technically this function should be deprecated
func GetYamlVariables(s string, logger *zap.SugaredLogger) ([]*models.ChartVariable, error) {
	resp := make([]*models.ChartVariable, 0)
	regex, err := regexp.Compile(setting.RegExpParameter)
	if err != nil {
		logger.Errorf("Cannot get regexp from the expression: %s, the error is: %s", setting.RegExpParameter, err)
		return []*models.ChartVariable{}, err
	}
	params := regex.FindAllString(s, -1)
	keyMap := make(map[string]int)
	for _, param := range params {
		key := getParameterKey(param)
		if keyMap[key] == 0 {
			resp = append(resp, &models.ChartVariable{
				Key: key,
			})
			keyMap[key] = 1
		}
	}
	return resp, nil
}

// FetchTemplateVariableYaml extract variables from go template yaml and package them into yaml
// supports most standard go-template grammar, but can't deal with complex situation like range in range / local vars using $
// NOTE the return value should not be fully trusted
func FetchTemplateVariableYaml(source string, logger *zap.SugaredLogger) (string, error) {
	return "", nil
}
