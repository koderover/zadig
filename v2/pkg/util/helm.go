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

package util

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"k8s.io/helm/pkg/strvals"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/util/converter"
)

func GeneReleaseName(namingRule, projectName, namespace, envName, service string) string {
	ret := strings.ReplaceAll(namingRule, "$Product$", projectName)
	ret = strings.ReplaceAll(ret, "$Namespace$", namespace)
	ret = strings.ReplaceAll(ret, "$EnvName$", envName)
	ret = strings.ReplaceAll(ret, "$Service$", service)
	return ret
}

// for keys exist in both yaml, current values will override the latest values
func OverrideValues(currentValuesYaml, latestValuesYaml []byte) ([]byte, error) {
	currentValuesMap := map[string]interface{}{}
	if err := yaml.Unmarshal(currentValuesYaml, &currentValuesMap); err != nil {
		return nil, err
	}

	currentValuesFlatMap, err := converter.Flatten(currentValuesMap)
	if err != nil {
		return nil, err
	}

	latestValuesMap := map[string]interface{}{}
	if err := yaml.Unmarshal(latestValuesYaml, &latestValuesMap); err != nil {
		return nil, err
	}

	latestValuesFlatMap, err := converter.Flatten(latestValuesMap)
	if err != nil {
		return nil, err
	}

	replaceMap := make(map[string]interface{})
	for key := range latestValuesFlatMap {
		if currentValue, ok := currentValuesFlatMap[key]; ok {
			replaceMap[key] = currentValue
		}
	}

	if len(replaceMap) == 0 {
		return latestValuesYaml, nil
	}

	var replaceKV []string
	for k, v := range replaceMap {
		replaceKV = append(replaceKV, fmt.Sprintf("%s=%v", k, v))
	}

	if err := strvals.ParseInto(strings.Join(replaceKV, ","), latestValuesMap); err != nil {
		return nil, err
	}

	return yaml.Marshal(latestValuesMap)
}

func ReadValuesYAML(chartTree fs.FS, base string, logger *zap.SugaredLogger) ([]byte, error) {
	content, err := fs.ReadFile(chartTree, filepath.Join(base, setting.ValuesYaml))
	if err != nil {
		logger.Errorf("Failed to read %s, err: %s", setting.ValuesYaml, err)
		return nil, err
	}
	return content, nil
}

func ReadValuesYAMLFromLocal(base string, logger *zap.SugaredLogger) ([]byte, error) {
	content, err := ReadFile(filepath.Join(base, setting.ValuesYaml))
	if err != nil {
		logger.Errorf("Failed to read %s, err: %s", setting.ValuesYaml, err)
		return nil, err
	}
	return content, nil
}
