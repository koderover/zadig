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

package yaml

import (
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/v2/pkg/util/converter"
)

func Equal(source, target string) (bool, error) {
	if source == target {
		return true, nil
	}

	sourceYamlObj, targetYamlObj := make(map[string]interface{}), make(map[string]interface{})

	err := yaml.Unmarshal([]byte(source), &sourceYamlObj)
	if err != nil {
		return false, err
	}

	err = yaml.Unmarshal([]byte(target), &targetYamlObj)
	if err != nil {
		return false, err
	}

	return reflect.DeepEqual(sourceYamlObj, targetYamlObj), nil
}

// DiffFlatKeys finds flat keys with different values from two yamls
func DiffFlatKeys(source, target string) ([]string, error) {
	equal, err := Equal(source, target)
	if err != nil || equal {
		return nil, err
	}

	sourceFlatMap, err := converter.YamlToFlatMap([]byte(source))
	if err != nil {
		return nil, err
	}

	targetFloatMap, err := converter.YamlToFlatMap([]byte(target))
	if err != nil {
		return nil, err
	}

	diffFlatKeys := sets.NewString()

	for k, v := range sourceFlatMap {
		if v != targetFloatMap[k] {
			diffFlatKeys.Insert(k)
		}
	}

	for k, v := range targetFloatMap {
		if v != sourceFlatMap[k] {
			diffFlatKeys.Insert(k)
		}
	}

	return diffFlatKeys.List(), nil
}

func ContainsFlatKey(source string, excludedKeys []string, keys []string) (bool, error) {
	sourceFlatMap, err := converter.YamlToFlatMap([]byte(source))
	if err != nil {
		return false, err
	}

	keySet := sets.NewString(keys...)
	excludedKeySet := sets.NewString(excludedKeys...)
	for k := range sourceFlatMap {
		if excludedKeySet.Has(k) {
			continue
		}
		if keySet.Has(k) {
			return true, nil
		}
	}
	return false, nil
}

func Diff(a, b string) string {
	aLines := strings.Split(a, "\n")
	bLines := strings.Split(b, "\n")
	var result []string
	i, j := 0, 0

	for i < len(aLines) && j < len(bLines) {
		if aLines[i] == bLines[j] {
			// 相同时直接显示
			result = append(result, " "+aLines[i])
			i++
			j++
		} else {
			// 差异时分别用“-”和“+”标记
			result = append(result, "-"+aLines[i])
			result = append(result, "+"+bLines[j])
			i++
			j++
		}
	}
	// 处理a剩余
	for i < len(aLines) {
		result = append(result, "-"+aLines[i])
		i++
	}
	// 处理b剩余
	for j < len(bLines) {
		result = append(result, "+"+bLines[j])
		j++
	}
	return strings.Join(result, "\n")
}
