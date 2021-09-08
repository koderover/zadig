/*
Copyright 2021 The KodeRover Authors.

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

import "sigs.k8s.io/yaml"

// MergeAndUnmarshal merges a couple of yaml files into one yaml file, and return a map[string]interface{},
// a field in latter file will override the same field in former file.
func MergeAndUnmarshal(yamls [][]byte) (map[string]interface{}, error) {
	base := map[string]interface{}{}

	for _, y := range yamls {
		currentMap := map[string]interface{}{}

		if err := yaml.Unmarshal(y, &currentMap); err != nil {
			return nil, err
		}
		// Merge with the previous map
		base = mergeMaps(base, currentMap)
	}

	return base, nil
}

// Merge merges a couple of yaml files into one yaml file,
// a field in latter file will override the same field in former file.
func Merge(yamls [][]byte) ([]byte, error) {
	m, err := MergeAndUnmarshal(yamls)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(m)
}

func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}

	return out
}
