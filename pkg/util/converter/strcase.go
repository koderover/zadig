/*
Copyright 2024 The KodeRover Authors.

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
	"encoding/json"

	"github.com/iancoleman/strcase"
)

func ConvertToSnakeCase(v interface{}) (map[string]interface{}, error) {
	// Marshal the struct to JSON
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// Unmarshal into a generic map
	var intermediate map[string]interface{}
	err = json.Unmarshal(data, &intermediate)
	if err != nil {
		return nil, err
	}

	// Convert keys to snake case recursively
	return convertMapKeysToSnakeCase(intermediate), nil
}

func ConvertToLowerCamelCase(v interface{}) (map[string]interface{}, error) {
	// Marshal the struct to JSON
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// Unmarshal into a generic map
	var intermediate map[string]interface{}
	err = json.Unmarshal(data, &intermediate)
	if err != nil {
		return nil, err
	}

	// Convert keys to snake case recursively
	return convertMapKeysToLowerCamelCase(intermediate), nil
}

func convertMapKeysToSnakeCase(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range data {
		newKey := strcase.ToSnake(key)
		switch v := value.(type) {
		case map[string]interface{}:
			result[newKey] = convertMapKeysToSnakeCase(v)
		case []interface{}:
			result[newKey] = convertSliceToSnakeCase(v)
		default:
			result[newKey] = v
		}
	}
	return result
}

func convertMapKeysToLowerCamelCase(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range data {
		newKey := strcase.ToLowerCamel(key)
		switch v := value.(type) {
		case map[string]interface{}:
			result[newKey] = convertMapKeysToLowerCamelCase(v)
		case []interface{}:
			result[newKey] = convertSliceToSnakeCase(v)
		default:
			result[newKey] = v
		}
	}
	return result
}

func convertSliceToSnakeCase(data []interface{}) []interface{} {
	for i, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			data[i] = convertMapKeysToSnakeCase(m)
		}
	}
	return data
}

func convertSliceToLowerCamelCase(data []interface{}) []interface{} {
	for i, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			data[i] = convertMapKeysToLowerCamelCase(m)
		}
	}
	return data
}
