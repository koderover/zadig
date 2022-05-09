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

func CheckEqual(source, target []byte) (bool, error) {
	if source == nil && target == nil {
		return true, nil
	}
	if source == nil && target != nil {
		return false, nil
	}
	if source != nil && target == nil {
		return false, nil
	}
	sourceMap := make(map[string]interface{})
	if err := yaml.Unmarshal(source, &sourceMap); err != nil {
		return false, err
	}
	targetMap := make(map[string]interface{})
	if err := yaml.Unmarshal(target, &targetMap); err != nil {
		return false, err
	}
	return reflect.DeepEqual(sourceMap, targetMap), nil
}
