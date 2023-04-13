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

package yaml

import (
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/util/converter"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

func MergeYamls(yamls []string) (string, error) {
	if len(yamls) == 0 {
		return "", nil
	}

	if len(yamls) == 1 {
		return yamls[0], nil
	}

	defaultKVs, err := GeneKVFromYaml(yamls[0])
	if err != nil {
		return "", err
	}
	KVMap := map[string]*commonmodels.VariableKV{}
	for _, kv := range defaultKVs {
		KVMap[kv.Key] = kv
	}

	for i := 1; i < len(yamls); i++ {
		overrideKVs, err := GeneKVFromYaml(yamls[i])
		if err != nil {
			return "", err
		}

		for _, kv := range overrideKVs {
			KVMap[kv.Key] = kv
		}
	}

	outKVs := []*commonmodels.VariableKV{}
	for _, kv := range KVMap {
		outKVs = append(outKVs, kv)
	}

	outYaml, err := GenerateYamlFromKV(outKVs)
	if err != nil {
		return "", fmt.Errorf("failed to generate yaml from kvs, err: %v, kvs %+v", err, outKVs)
	}

	return outYaml, nil
}

func GeneKVFromYaml(yamlContent string) ([]*commonmodels.VariableKV, error) {
	if len(yamlContent) == 0 {
		return nil, nil
	}
	flatMap, err := converter.YamlToFlatMap([]byte(yamlContent))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert yaml to flat map")
	} else {
		kvs := make([]*commonmodels.VariableKV, 0)
		for k, v := range flatMap {
			if len(k) == 0 {
				continue
			}
			kvs = append(kvs, &models.VariableKV{
				Key:   k,
				Value: v,
			})
		}
		return kvs, nil
	}
}

func GenerateYamlFromKV(kvs []*commonmodels.VariableKV) (string, error) {
	if len(kvs) == 0 {
		return "", nil
	}
	flatMap := make(map[string]interface{})
	for _, kv := range kvs {
		flatMap[kv.Key] = kv.Value
	}

	validKvMap, err := converter.Expand(flatMap)
	if err != nil {
		return "", errors.Wrapf(err, "failed to expand flat map")
	}

	bs, err := yaml.Marshal(validKvMap)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal map to yaml")
	}
	return string(bs), nil
}
