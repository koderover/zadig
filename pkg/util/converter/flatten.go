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

package converter

import (
	"fmt"
	"reflect"

	"sigs.k8s.io/yaml"
)

const (
	delimiter = "."
	// maxDepth = 0
)

func YamlToFlatMap(data []byte) (map[string]interface{}, error) {
	nested := make(map[string]interface{})
	if err := yaml.Unmarshal(data, &nested); err != nil {
		return nil, err
	}

	return Flatten(nested)
}

// Flatten flattens the nested map
// examples:
// {a: {b: c}} => {a.b: c}
// {a: [{b: c}, [b: d]} => {a[0].b: c, a[1].b: d}
// {a: [b, c]} => {a[0]: b, a[1]: c}
func Flatten(nested map[string]interface{}) (map[string]interface{}, error) {
	return flatten("", 0, nested)
}

func flatten(prefix string, depth int, nested interface{}) (flat map[string]interface{}, err error) {
	flat = make(map[string]interface{})

	switch nested := nested.(type) {
	case map[string]interface{}:
		//if maxDepth != 0 && depth >= maxDepth {
		//	flat[prefix] = nested
		//	return
		//}
		if reflect.DeepEqual(nested, map[string]interface{}{}) {
			flat[prefix] = nested
			return
		}
		for k, v := range nested {
			newKey := k
			if prefix != "" {
				newKey = prefix + delimiter + newKey
			}
			fm1, fe := flatten(newKey, depth+1, v)
			if fe != nil {
				err = fe
				return
			}
			mergeMap(flat, fm1)
		}
	case []interface{}:
		if reflect.DeepEqual(nested, []interface{}{}) {
			flat[prefix] = nested
			return
		}
		for i, v := range nested {
			newKey := fmt.Sprintf("%s[%d]", prefix, i)
			fm1, fe := flatten(newKey, depth+1, v)
			if fe != nil {
				err = fe
				return
			}
			mergeMap(flat, fm1)
		}
	default:
		flat[prefix] = nested
	}
	return
}

func mergeMap(to map[string]interface{}, from map[string]interface{}) {
	for kt, vt := range from {
		to[kt] = vt
	}
}
