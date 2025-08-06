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

package util

import (
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

// TODO: LOU: drop all methods in this file

func JSONToYaml(json map[string]interface{}) (string, error) {
	jsonBytes, err := yaml.Marshal(json)
	if err != nil {
		return "", err
	}
	yamlValuesByte, err := yaml.JSONToYAML(jsonBytes)
	return string(yamlValuesByte), err
}

// ReplaceMapValue 拿匹配到的key和value去替换原始的map中的key和value
func ReplaceMapValue(jsonValues map[string]interface{}, replaceMap map[string]interface{}) map[string]interface{} {
	for replaceKey, replaceValue := range replaceMap {
		replaceKeyArr := strings.Split(replaceKey, ".")
		RecursionReplaceValue(jsonValues, replaceKeyArr, replaceValue)
	}

	return jsonValues
}

// RecursionReplaceValue 递归替换map中的value
func RecursionReplaceValue(jsonValues map[string]interface{}, replaceKeyArr []string, replaceValue interface{}) {
	if _, isExist := jsonValues[replaceKeyArr[0]]; !isExist {
		return
	}
	if len(replaceKeyArr) == 1 {
		jsonValues[replaceKeyArr[0]] = replaceValue
		return
	}
	if isMap(jsonValues[replaceKeyArr[0]]) != nil {
		RecursionReplaceValue(jsonValues[replaceKeyArr[0]].(map[string]interface{}), replaceKeyArr[1:], replaceValue)
	}
}
func isMap(yamlMap interface{}) map[string]interface{} {
	switch value := yamlMap.(type) {
	case map[string]interface{}:
		return value
	default:
		return nil
	}
	return nil
}

var jsonMap map[string]string

func GetJSONData(jsonValues map[string]interface{}) map[string]string {
	jsonMap = make(map[string]string)
	RecursionGetKeyAndValue(jsonValues, "")
	return jsonMap
}

// RecursionGetKeyAndValue 递归获取map中的key和value
func RecursionGetKeyAndValue(jsonValues map[string]interface{}, mapKey string) {
	for jsonKey, jsonValue := range jsonValues {
		levelMap := isMap(jsonValue)
		if levelMap != nil {
			tmpKey := jsonKey
			if mapKey != "" {
				tmpKey = fmt.Sprintf("%s.%s", mapKey, jsonKey)
			}
			RecursionGetKeyAndValue(levelMap, tmpKey)
		} else {
			key := jsonKey
			if mapKey != "" {
				key = fmt.Sprintf("%s.%s", mapKey, jsonKey)
			}
			jsonMap[key] = fmt.Sprintf("%v", jsonValue)
		}
	}
}

// jsonEscapeString 接受一个字符串并返回JSON转义后的字符串
func JsonEscapeString(str string) (string, error) {
	// 将字符串编码为JSON格式
	escapedBytes, err := json.Marshal(str)
	if err != nil {
		return "", err
	}

	// JSON编码后的结果是一个包含双引号的字符串，去掉两边的双引号
	escapedStr := string(escapedBytes)
	return escapedStr[1 : len(escapedStr)-1], nil
}

func IToi(before interface{}, after interface{}) error {
	b, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("marshal task error: %v", err)
	}

	if err := json.Unmarshal(b, &after); err != nil {
		return fmt.Errorf("unmarshal task error: %v", err)
	}

	return nil
}
