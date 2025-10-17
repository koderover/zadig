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
	"regexp"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"gopkg.in/yaml.v3"
)

const GoTemplateKeyRegExp = `{{\.(job(\.[^{}]+){2}|job(\.[^{}]+){4}|workflow\.params(\.[^{}]+){1}|workflow(\.[^{}]+){1}|project(\.[^{}]+){1}|workflow(\.[^{}]+){3})}}`

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

// FindVariableKeyRef finds all the key used in the input param and return them as a string array
func FindVariableKeyRef(input string) []string {
	re := regexp.MustCompile(GoTemplateKeyRegExp)

	// Find all matches
	matches := re.FindAllStringSubmatch(input, -1)

	// Extract the keys
	keys := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			keys = append(keys, match[1]) // match[1] contains the key
		}
	}

	return keys
}

// RunScriptWithCallFunc runs the given script with a caller function, the response is now forced to be string array.
func RunScriptWithCallFunc(script, callFunc string) ([]string, error) {
	i := interp.New(interp.Options{})
	i.Use(stdlib.Symbols)

	// Evaluate the code snippet
	_, err := i.Eval(script)
	if err != nil {
		return nil, fmt.Errorf("检查脚本失败, 错误: %w", err)
	}
	v, err := i.Eval(callFunc)
	if err != nil {
		return nil, fmt.Errorf("执行调用函数失败，错误: %w", err)
	}

	if !v.IsValid() {
		return nil, fmt.Errorf("unexpected invalid return value")
	}
	if v.IsNil() {
		return nil, fmt.Errorf("unexpected nil return value")
	}

	if v.IsZero() {
		return []string{}, nil
	}

	if !v.CanInterface() {
		return nil, fmt.Errorf("unexpected non-interface return value")
	}

	// Assert the returned value is of the expected type
	result, ok := v.Interface().([]string)
	if !ok {
		return nil, fmt.Errorf("unexpected return type")
	}

	return result, nil
}
