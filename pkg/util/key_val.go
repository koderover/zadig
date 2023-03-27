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

import "gopkg.in/yaml.v3"

type KVInput []*KeyValue

// TODO: the key field of this struct only support non-nested value
// e.g. the key "a.b" will not be parse as a nested value
// this is different from the behavior of our normal service KV parsing.
// How we handle the KV should be decided after further discussion.
type KeyValue struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

func (i KVInput) FormYamlString() (string, error) {
	kvMap := make(map[string]interface{})
	for _, kv := range i {
		kvMap[kv.Key] = kv.Value
	}

	out, err := yaml.Marshal(kvMap)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
