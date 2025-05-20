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

package helm

import "gopkg.in/yaml.v3"

func GetValuesMapFromString(values string) (map[string]interface{}, error) {
	ret := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(values), &ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func MergeHelmValues(oldVals, newVals map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range oldVals {
		result[k] = v
	}

	for k, newVal := range newVals {
		if existingVal, exists := result[k]; exists {
			if existingMap, ok := existingVal.(map[string]interface{}); ok {
				if newMap, ok := newVal.(map[string]interface{}); ok {
					result[k] = MergeHelmValues(existingMap, newMap)
					continue
				}
			}
		}
		result[k] = newVal
	}
	return result
}
