/*
Copyright 2026 The KodeRover Authors.

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
	"strings"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/util"
	"sigs.k8s.io/yaml"
)

func MaskSensitiveValuesYAML(content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return content, nil
	}

	values := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(content), &values); err != nil {
		return "", err
	}
	maskSensitiveValues(values)

	masked, err := yaml.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(masked), nil
}

func RestoreMaskedSensitiveValuesYAML(current, updated string) (string, error) {
	if strings.TrimSpace(updated) == "" {
		return updated, nil
	}

	currentValues := make(map[string]interface{})
	if strings.TrimSpace(current) != "" {
		if err := yaml.Unmarshal([]byte(current), &currentValues); err != nil {
			return "", fmt.Errorf("failed to parse current Values YAML: %w", err)
		}
	}
	updatedValues := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(updated), &updatedValues); err != nil {
		return "", fmt.Errorf("failed to parse updated Values YAML: %w", err)
	}
	changed, err := restoreMaskedSensitiveValues(currentValues, updatedValues, "")
	if err != nil {
		return "", err
	}
	if !changed {
		return updated, nil
	}

	restored, err := yaml.Marshal(updatedValues)
	if err != nil {
		return "", err
	}
	return string(restored), nil
}

func IsSensitiveValuesKey(key string) bool {
	normalized := util.NormalizeSensitiveKey(key)
	// These Helm fields refer to Kubernetes Secrets rather than containing credentials.
	for _, suffix := range []string{"existing_secret", "pull_secret", "existingsecret", "pullsecret"} {
		if normalized == suffix || strings.HasSuffix(normalized, "_"+suffix) {
			return false
		}
	}
	for _, suffix := range []string{"credentials", "encryption_key"} {
		if normalized == suffix || strings.HasSuffix(normalized, "_"+suffix) {
			return true
		}
	}
	return util.IsSensitiveKey(normalized)
}

func maskSensitiveValues(value interface{}) {
	switch value := value.(type) {
	case map[string]interface{}:
		for key, item := range value {
			if IsSensitiveValuesKey(key) {
				if item != nil {
					value[key] = setting.MaskValue
				}
				continue
			}
			maskSensitiveValues(item)
		}
	case []interface{}:
		for _, item := range value {
			maskSensitiveValues(item)
		}
	}
}

func restoreMaskedSensitiveValues(current, updated interface{}, path string) (bool, error) {
	changed := false
	switch updated := updated.(type) {
	case map[string]interface{}:
		currentMap, _ := current.(map[string]interface{})
		for key, item := range updated {
			itemPath := key
			if path != "" {
				itemPath = path + "." + key
			}
			currentItem, exists := currentMap[key]
			if IsSensitiveValuesKey(key) && item == setting.MaskValue {
				if !exists {
					return false, fmt.Errorf("masked sensitive value %q does not exist in current Values", itemPath)
				}
				updated[key] = currentItem
				changed = true
				continue
			}
			itemChanged, err := restoreMaskedSensitiveValues(currentItem, item, itemPath)
			if err != nil {
				return false, err
			}
			changed = changed || itemChanged
		}
	case []interface{}:
		currentSlice, _ := current.([]interface{})
		for index, item := range updated {
			var currentItem interface{}
			if index < len(currentSlice) {
				currentItem = currentSlice[index]
			}
			itemChanged, err := restoreMaskedSensitiveValues(currentItem, item, fmt.Sprintf("%s[%d]", path, index))
			if err != nil {
				return false, err
			}
			changed = changed || itemChanged
		}
	}
	return changed, nil
}
