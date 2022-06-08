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

	"sigs.k8s.io/yaml"
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
