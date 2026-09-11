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

import (
	"encoding/json"

	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"gopkg.in/yaml.v3"
)

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

// MergeOverrideKVStrings merges Helm --set values by key. Values in overlay
// take precedence over values in base. This is used when a request only
// contains user-editable values: environment-persisted values must remain in
// the result when they are omitted from the request.
func MergeOverrideKVStrings(base, overlay string) (string, error) {
	merged := make([]*helmtool.KV, 0)
	upsert := func(kv *helmtool.KV) {
		if kv == nil || kv.Key == "" {
			return
		}
		for i := len(merged) - 1; i >= 0; i-- {
			if merged[i].Key == kv.Key {
				merged[i].Value = kv.Value
				return
			}
		}
		merged = append(merged, kv)
	}
	decode := func(values string) error {
		kvs := make([]*helmtool.KV, 0)
		if err := json.Unmarshal([]byte(values), &kvs); err != nil {
			return err
		}
		for _, kv := range kvs {
			upsert(kv)
		}
		return nil
	}

	if base != "" {
		if err := decode(base); err != nil {
			return "", err
		}
	}
	if overlay != "" {
		if err := decode(overlay); err != nil {
			return "", err
		}
	}
	if len(merged) == 0 {
		if overlay != "" {
			return overlay, nil
		}
		return base, nil
	}

	result, err := json.Marshal(merged)
	if err != nil {
		return "", err
	}
	return string(result), nil
}
