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

package types

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"
)

type ServiceWithVariable struct {
	ServiceName  string              `json:"service_name"`
	VariableYaml string              `json:"variable_yaml"`
	VariableKVs  []*RenderVariableKV `json:"variable_kvs"`
}

type ServiceVariableKVType string

const (
	ServiceVariableKVTypeBoolean ServiceVariableKVType = "bool"
	ServiceVariableKVTypeString  ServiceVariableKVType = "string"
	ServiceVariableKVTypeEnum    ServiceVariableKVType = "enum"
	ServiceVariableKVTypeYaml    ServiceVariableKVType = "yaml"
)

// This kv will aggregate complicated struct to the first layer of key
type ServiceVariableKV struct {
	Key     string                `bson:"key"      yaml:"key"      json:"key"`
	Value   interface{}           `bson:"value"    yaml:"value"    json:"value"`
	Type    ServiceVariableKVType `bson:"type"     yaml:"type"     json:"type"`
	Options []string              `bson:"options"  yaml:"options"  json:"options"`
	Desc    string                `bson:"desc"     yaml:"desc"     json:"desc"`
}

type RenderVariableKV struct {
	ServiceVariableKV
	UseGlobalVariable bool `bson:"use_global_variable"    json:"use_global_variable"`
}

type GlobalVariableKV struct {
	ServiceVariableKV
	RelatedServices []string `bson:"related_services" json:"related_services"`
}

// not suitable for flatten kv
// ServiceVariableKV is a kv which aggregate complicated struct to the first layer
func ServiceVariableKVToYaml(kvs []*ServiceVariableKV) (string, error) {
	kvMap := make(map[string]interface{}, 0)
	for _, kv := range kvs {
		if kv == nil {
			continue
		}

		switch kv.Type {
		case ServiceVariableKVTypeYaml:
			intf := new(interface{})
			value, ok := kv.Value.(string)
			if !ok {
				return "", fmt.Errorf("failed to convert value to string, value: %v", kv.Value)
			}

			err := yaml.Unmarshal([]byte(value), intf)
			if err != nil {
				return "", fmt.Errorf("failed to unmarshal yaml, err: %w", err)
			}
			kvMap[kv.Key] = intf
		case ServiceVariableKVTypeBoolean:
			_, ok := kv.Value.(bool)
			if !ok {
				return "", fmt.Errorf("invaild value for boolean")
			}
			kvMap[kv.Key] = kv.Value
		default:
			kvMap[kv.Key] = kv.Value
		}
	}

	yamlBytes, err := yaml.Marshal(kvMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal yaml, err: %w", err)
	}

	return string(yamlBytes), nil
}

// not suitable for flatten kv
// ServiceVariableKV is a kv which aggregate complicated struct to the first layer
func YamlToServiceVariableKV(yamlStr string, origKVs []*ServiceVariableKV) ([]*ServiceVariableKV, error) {
	kvMap := make(map[string]interface{}, 0)
	err := yaml.Unmarshal([]byte(yamlStr), &kvMap)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml, err: %w", err)
	}

	origKVMap := make(map[string]*ServiceVariableKV, 0)
	for _, kv := range origKVs {
		origKVMap[kv.Key] = kv
	}

	ret := make([]*ServiceVariableKV, 0)
	for k, v := range kvMap {
		kv, err := snippetToKV(k, v, origKVMap[k])
		if err != nil {
			return nil, fmt.Errorf("failed to convert snippet to kv, err: %w", err)
		}

		ret = append(ret, kv)
	}

	return ret, nil
}

// FIXME: ServiceVariableKVType not equal with golang runtime type
func snippetToKV(key string, snippet interface{}, origKV *ServiceVariableKV) (*ServiceVariableKV, error) {
	var retKV *ServiceVariableKV
	if origKV == nil {
		// origKV is nil, create a new kv
		retKV = &ServiceVariableKV{
			Key: key,
		}

		switch snippet.(type) {
		case bool:
			retKV.Type = ServiceVariableKVTypeBoolean
			retKV.Value = snippet.(bool)
		case string:
			retKV.Type = ServiceVariableKVTypeString
			retKV.Value = snippet.(string)
		case map[string]interface{}, []interface{}:
			snippetBytes, err := yaml.Marshal(snippet)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal snippet, err: %w", err)
			}
			retKV.Type = ServiceVariableKVTypeYaml
			retKV.Value = string(snippetBytes)
		default:
			retKV.Type = ServiceVariableKVTypeString
			retKV.Value = snippet
		}
	} else {
		// origKV is not nil, inherit type, options, and desc from it
		retKV = &ServiceVariableKV{
			Key:     origKV.Key,
			Value:   origKV.Value,
			Type:    origKV.Type,
			Options: origKV.Options,
			Desc:    origKV.Desc,
		}

		// check key
		if key != retKV.Key {
			return nil, fmt.Errorf("key not match, key: %s, orig key: %s", key, origKV.Key)
		}

		switch retKV.Type {
		case ServiceVariableKVTypeBoolean:
			// check if value valid for boolean
			value, ok := snippet.(string)
			if ok && value != "true" && value != "false" {
				return nil, fmt.Errorf("invalid value: %s for boolean", snippet.(string))
			}

		case ServiceVariableKVTypeEnum:
			// check if value exist in options
			optionSet := sets.NewString(origKV.Options...)
			valueStr := fmt.Sprintf("%v", snippet)
			if !optionSet.Has(valueStr) {
				return nil, fmt.Errorf("invalid value: %v, valid options: %v", snippet, origKV.Options)
			}
		}

		if retKV.Type == ServiceVariableKVTypeYaml {
			snippetBytes, err := yaml.Marshal(snippet)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal snippet, err: %w", err)
			}
			retKV.Value = string(snippetBytes)
		} else {
			retKV.Value = snippet
		}
	}

	return retKV, nil
}

func MergeServiceVariableKVsIfNotExist(origin, new []*ServiceVariableKV) (yaml string, kvs []*ServiceVariableKV, err error) {
	KVSet := sets.NewString()
	for _, kv := range origin {
		KVSet.Insert(kv.Key)
	}

	for _, kv := range new {
		if !KVSet.Has(kv.Key) {
			origin = append(origin, kv)
		}
	}

	yaml, err = ServiceVariableKVToYaml(origin)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert service variable kv to yaml, err: %w", err)
	}

	return yaml, origin, nil
}

func MergeServiceVariableKVs(base, override []*ServiceVariableKV) (yaml string, kvs []*ServiceVariableKV, err error) {
	baseMap := map[string]*ServiceVariableKV{}
	for _, kv := range base {
		baseMap[kv.Key] = kv
	}

	for _, kv := range override {
		baseMap[kv.Key] = kv
	}

	ret := []*ServiceVariableKV{}
	for _, kv := range baseMap {
		ret = append(ret, kv)
	}

	yaml, err = ServiceVariableKVToYaml(ret)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert service variable kv to yaml, err: %w", err)
	}

	return yaml, ret, nil
}

func MergeRenderAndServiceVariableKVs(render []*RenderVariableKV, serivce []*ServiceVariableKV) []*RenderVariableKV {
	svcMap := map[string]*ServiceVariableKV{}
	for _, kv := range serivce {
		svcMap[kv.Key] = kv
	}

	renderMap := map[string]*RenderVariableKV{}
	for _, kv := range render {
		renderMap[kv.Key] = kv
	}

	ret := []*RenderVariableKV{}
	for _, kv := range svcMap {
		if renderKV, ok := renderMap[kv.Key]; !ok {
			ret = append(ret, &RenderVariableKV{
				ServiceVariableKV: *kv,
				UseGlobalVariable: false,
			})
		} else {
			ret = append(ret, renderKV)
		}
	}

	return ret
}

func RenderVariableKVToYaml(kvs []*RenderVariableKV) (string, error) {
	serviceVariableKVs := make([]*ServiceVariableKV, 0)
	for _, kv := range kvs {
		serviceVariableKVs = append(serviceVariableKVs, &kv.ServiceVariableKV)
	}

	return ServiceVariableKVToYaml(serviceVariableKVs)
}

// update the global variable kvs base on the render variable kvs
// if the key is not exist, create a new one
// if the key exist, update the related services
func UpdateGlobalVariableKVs(serviceName string, globalVariables []*GlobalVariableKV, renderVariables []*RenderVariableKV) ([]*GlobalVariableKV, error) {
	globalVariableMap := map[string]*GlobalVariableKV{}
	for _, kv := range globalVariables {
		globalVariableMap[kv.Key] = kv
	}

	for _, kv := range renderVariables {
		if kv.UseGlobalVariable {
			globalVariableKV, ok := globalVariableMap[kv.Key]
			if !ok {
				globalVariableKV = &GlobalVariableKV{
					ServiceVariableKV: ServiceVariableKV{
						Key:     kv.Key,
						Value:   kv.Value,
						Type:    kv.Type,
						Options: kv.Options,
						Desc:    kv.Desc,
					},
					RelatedServices: []string{serviceName},
				}
			} else {
				relatedServiceSet := sets.NewString(globalVariableKV.RelatedServices...)
				if !relatedServiceSet.Has(serviceName) {
					globalVariableKV.RelatedServices = append(globalVariableKV.RelatedServices, serviceName)
				}
			}
		}
	}

	retGlobalVariables := []*GlobalVariableKV{}
	for _, kv := range globalVariableMap {
		retGlobalVariables = append(retGlobalVariables, kv)
	}

	return retGlobalVariables, nil
}
