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
	"sort"
	"strings"

	"github.com/koderover/zadig/v2/pkg/setting"

	"helm.sh/helm/v3/pkg/releaseutil"
)

const separator = "\n---\n"

func CombineManifests(yamls []string) string {
	return strings.Join(yamls, separator)
}

// SplitManifests 按原始文件顺序返回 manifest 列表。
// Helm 的 SplitManifests 返回 map，这里显式排序 key，避免顺序不稳定。
func SplitManifests(content string) []string {
	res := make([]string, 0)
	manifests := releaseutil.SplitManifests(content)
	manifestKeys := make([]string, 0, len(manifests))
	for key := range manifests {
		manifestKeys = append(manifestKeys, key)
	}
	sort.Sort(releaseutil.BySplitManifestsOrder(manifestKeys))
	for _, key := range manifestKeys {
		res = append(res, manifests[key])
	}

	return res
}

// value may only contain underscores or underscores
func ReturnValidLabelValue(value string) string {
	value = strings.Replace(value, "-", "", -1)
	value = strings.Replace(value, "_", "", -1)
	if len(value) > 63 {
		value = value[:63]
	}

	return value
}

func JoinYamls(files []string) string {
	return strings.Join(files, setting.YamlFileSeperator)
}

func SplitYaml(yaml string) []string {
	return strings.Split(yaml, setting.YamlFileSeperator)
}

func ConvertIntsToInt64(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			v[key] = ConvertIntsToInt64(value)
		}
		return v
	case []interface{}:
		for i, value := range v {
			v[i] = ConvertIntsToInt64(value)
		}
		return v
	case int:
		return int64(v)
	default:
		return v
	}
}
