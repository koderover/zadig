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
	ServiceVariableKV `bson:",inline" yaml:",inline" json:",inline"`
	UseGlobalVariable bool `bson:"use_global_variable"     yaml:"use_global_variable"    json:"use_global_variable"`
}

type GlobalVariableKV struct {
	ServiceVariableKV `bson:",inline" yaml:",inline" json:",inline"`
	RelatedServices   []string `bson:"related_services"     yaml:"related_services"     json:"related_services"`
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
				return "", fmt.Errorf("failed to convert value to yaml, key: %v, value: %v", kv.Key, kv.Value)
			}

			err := yaml.Unmarshal([]byte(value), intf)
			if err != nil {
				return "", fmt.Errorf("failed to unmarshal yaml, key: %v, err: %w", kv.Key, err)
			}
			kvMap[kv.Key] = intf
		case ServiceVariableKVTypeBoolean:
			v, ok := kv.Value.(bool)
			if ok {
				kvMap[kv.Key] = v
			} else if kv.Value == "true" {
				kvMap[kv.Key] = true
			} else if kv.Value == "false" {
				kvMap[kv.Key] = false
			} else {
				return "", fmt.Errorf("invaild value for boolean, key: %v, value: %v", kv.Key, kv.Value)
			}
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
				return nil, fmt.Errorf("key %s: failed to marshal snippet, err: %w", retKV.Key, err)
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

		// validate key
		if key != retKV.Key {
			return nil, fmt.Errorf("key not match, key: %s, orig key: %s", key, origKV.Key)
		}

		// validate value match type
		switch retKV.Type {
		case ServiceVariableKVTypeBoolean:
			// check if value valid for boolean
			value, ok := snippet.(string)
			if ok {
				if value != "true" && value != "false" {
					return nil, fmt.Errorf("key %s: invalid value: %s for boolean", retKV.Key, snippet.(string))
				}
			} else {
				_, ok = snippet.(bool)
				if !ok {
					return nil, fmt.Errorf("key %s: invalid value: %v for boolean", retKV.Key, snippet)
				}
			}
		case ServiceVariableKVTypeEnum:
			// check if value exist in options
			optionSet := sets.NewString(origKV.Options...)
			valueStr := fmt.Sprintf("%v", snippet)
			if !optionSet.Has(valueStr) {
				return nil, fmt.Errorf("key %s: invalid value: %v, valid options: %v", retKV.Key, snippet, origKV.Options)
			}
		}

		// convert snippet to value
		if retKV.Type == ServiceVariableKVTypeYaml {
			snippetBytes, err := yaml.Marshal(snippet)
			if err != nil {
				return nil, fmt.Errorf("key %s: failed to marshal snippet, err: %w", retKV.Key, err)
			}
			retKV.Value = string(snippetBytes)
		} else {
			retKV.Value = snippet
		}
	}

	return retKV, nil
}

func transferServiceVariable(render, template *ServiceVariableKV) *ServiceVariableKV {
	if render == nil {
		return template
	}
	if template == nil {
		return render
	}

	ret := &ServiceVariableKV{}
	if render.Type != template.Type {
		ret = template
	} else {
		ret = template
		ret.Value = render.Value
	}

	return ret
}

func MergeServiceVariableKVsIfNotExist(kvsList ...[]*ServiceVariableKV) (string, []*ServiceVariableKV, error) {
	kvSet := sets.NewString()
	ret := make([]*ServiceVariableKV, 0)
	for _, kvs := range kvsList {
		for _, kv := range kvs {
			if !kvSet.Has(kv.Key) {
				kvSet.Insert(kv.Key)
				ret = append(ret, kv)
			}
		}
	}

	yaml, err := ServiceVariableKVToYaml(ret)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert service variable kv to yaml, err: %w", err)
	}

	return yaml, ret, nil
}

func MergeServiceVariableKVs(kvsList ...[]*ServiceVariableKV) (string, []*ServiceVariableKV, error) {
	kvMap := map[string]*ServiceVariableKV{}

	for _, kvs := range kvsList {
		for _, kv := range kvs {
			kvMap[kv.Key] = kv
		}
	}

	ret := []*ServiceVariableKV{}
	for _, kv := range kvMap {
		ret = append(ret, kv)
	}

	yaml, err := ServiceVariableKVToYaml(ret)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert service variable kv to yaml, err: %w", err)
	}

	return yaml, ret, nil
}

func MergeRenderVariableKVs(kvsList ...[]*RenderVariableKV) (string, []*RenderVariableKV, error) {
	kvMap := map[string]*RenderVariableKV{}

	for _, kvs := range kvsList {
		for _, kv := range kvs {
			kvMap[kv.Key] = kv
		}
	}

	ret := []*RenderVariableKV{}
	for _, kv := range kvMap {
		ret = append(ret, kv)
	}

	yaml, err := RenderVariableKVToYaml(ret)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert render variable kv to yaml, err: %w", err)
	}

	return yaml, ret, nil
}

func MergeRenderAndServiceTemplateVariableKVs(render []*RenderVariableKV, serivceTemplate []*ServiceVariableKV) (string, []*RenderVariableKV, error) {
	svcTemplMap := map[string]*ServiceVariableKV{}
	for _, kv := range serivceTemplate {
		svcTemplMap[kv.Key] = kv
	}

	renderMap := map[string]*RenderVariableKV{}
	for _, kv := range render {
		renderMap[kv.Key] = kv
	}

	ret := []*RenderVariableKV{}
	for _, kv := range svcTemplMap {
		if renderKV, ok := renderMap[kv.Key]; !ok {
			ret = append(ret, &RenderVariableKV{
				ServiceVariableKV: *kv,
				UseGlobalVariable: false,
			})
		} else {
			if renderKV.UseGlobalVariable {
				ret = append(ret, renderKV)
			} else {
				newKV := transferServiceVariable(&renderKV.ServiceVariableKV, kv)
				transferedKV := &RenderVariableKV{
					ServiceVariableKV: *newKV,
					UseGlobalVariable: false,
				}
				ret = append(ret, transferedKV)
			}
		}
	}

	yaml, err := RenderVariableKVToYaml(ret)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert render variable kv to yaml, err: %w", err)
	}

	return yaml, ret, nil
}

func RenderVariableKVToYaml(kvs []*RenderVariableKV) (string, error) {
	serviceVariableKVs := make([]*ServiceVariableKV, 0)
	for _, kv := range kvs {
		serviceVariableKVs = append(serviceVariableKVs, &kv.ServiceVariableKV)
	}

	return ServiceVariableKVToYaml(serviceVariableKVs)
}

func GlobalVariableKVToYaml(kvs []*GlobalVariableKV) (string, error) {
	serviceVariableKVs := make([]*ServiceVariableKV, 0)
	for _, kv := range kvs {
		serviceVariableKVs = append(serviceVariableKVs, &kv.ServiceVariableKV)
	}

	return ServiceVariableKVToYaml(serviceVariableKVs)
}

func ValidateGlobalVariables(globalVariablesDefine []*ServiceVariableKV, globalVariables []*GlobalVariableKV) bool {
	globalVariableMap := map[string]*ServiceVariableKV{}
	for _, kv := range globalVariablesDefine {
		globalVariableMap[kv.Key] = kv
	}

	for _, kv := range globalVariables {
		if _, ok := globalVariableMap[kv.Key]; !ok {
			return false
		}
	}

	return true
}

func ValidateRenderVariables(globalVariables []*GlobalVariableKV, renderVariables []*RenderVariableKV) error {
	globalVariableMap := map[string]*GlobalVariableKV{}
	for _, kv := range globalVariables {
		globalVariableMap[kv.Key] = kv
	}

	for _, kv := range renderVariables {
		if kv.UseGlobalVariable {
			globalVariable, ok := globalVariableMap[kv.Key]
			if !ok {
				return fmt.Errorf("referenced global variable not exist, key: %s", kv.Key)
			}
			kv.Value = globalVariable.Value
			kv.Type = globalVariable.Type
			kv.Options = globalVariable.Options
			kv.Desc = globalVariable.Desc
		}
	}

	return nil
}

func ServiceToRenderVariableKVs(ServiceVariables []*ServiceVariableKV) []*RenderVariableKV {
	ret := []*RenderVariableKV{}
	for _, kv := range ServiceVariables {
		ret = append(ret, &RenderVariableKV{
			ServiceVariableKV: *kv,
			UseGlobalVariable: false,
		})
	}

	return ret
}

func RemoveGlobalVariableRelatedService(globalVariableKVs []*GlobalVariableKV, serviceNames ...string) []*GlobalVariableKV {
	for _, kv := range globalVariableKVs {
		relatedServiceSet := sets.NewString(kv.RelatedServices...)
		relatedServiceSet = relatedServiceSet.Delete(serviceNames...)
		kv.RelatedServices = relatedServiceSet.List()
	}

	return globalVariableKVs
}

func UpdateGlobalVariableKVs(serviceName string, globalVariables []*GlobalVariableKV, argVariables, currentVariables []*RenderVariableKV) ([]*GlobalVariableKV, []*RenderVariableKV, error) {
	globalVariableMap := map[string]*GlobalVariableKV{}
	for _, kv := range globalVariables {
		globalVariableMap[kv.Key] = kv
	}
	argVariableMap := map[string]*RenderVariableKV{}
	for _, kv := range argVariables {
		argVariableMap[kv.Key] = kv
	}
	renderVariableMap := map[string]*RenderVariableKV{}
	for _, kv := range currentVariables {
		renderVariableMap[kv.Key] = kv
	}

	addRelatedServices := func(globalVariableKV *GlobalVariableKV, serviceName string) {
		relatedSvcSet := sets.NewString(globalVariableKV.RelatedServices...)
		relatedSvcSet.Insert(serviceName)
		globalVariableKV.RelatedServices = relatedSvcSet.List()
	}

	delRelatedServices := func(globalVariableKV *GlobalVariableKV, serviceName string) {
		relatedSvcSet := sets.NewString(globalVariableKV.RelatedServices...)
		relatedSvcSet.Delete(serviceName)
		globalVariableKV.RelatedServices = relatedSvcSet.List()
	}

	updateRenderVariable := func(globalVariableKV *GlobalVariableKV, renderVariableKV *RenderVariableKV) {
		renderVariableKV.Value = globalVariableKV.Value
		renderVariableKV.Type = globalVariableKV.Type
		renderVariableKV.Options = globalVariableKV.Options
		renderVariableKV.Desc = globalVariableKV.Desc
	}

	// check render vairaible diff
	for _, argKV := range argVariableMap {
		globalVariableKV, ok := globalVariableMap[argKV.Key]
		if !ok && argKV.UseGlobalVariable {
			// global variable not exist, but arg use global variable
			return nil, nil, fmt.Errorf("global variable not exist, but used by serviceName: %s, key: %s", serviceName, argKV.Key)
		}

		renderKV, ok := renderVariableMap[argKV.Key]
		if !ok {
			// render variable not exist, newly added variable
			if argKV.UseGlobalVariable {
				// arg use global variable
				addRelatedServices(globalVariableKV, serviceName)
				updateRenderVariable(globalVariableKV, argKV)
			}
		} else {
			// render variable exist
			if argKV.UseGlobalVariable && !renderKV.UseGlobalVariable {
				addRelatedServices(globalVariableKV, serviceName)
				updateRenderVariable(globalVariableKV, argKV)
			} else if !argKV.UseGlobalVariable && renderKV.UseGlobalVariable {
				delRelatedServices(globalVariableKV, serviceName)
			}

			// delete from render when this kv is checked
			delete(renderVariableMap, argKV.Key)
		}
	}

	// deleted variable
	for _, renderKV := range renderVariableMap {
		globalVariableKV, ok := globalVariableMap[renderKV.Key]
		if !ok && renderKV.UseGlobalVariable {
			// global variable not exist, but render use global variable
			return nil, nil, fmt.Errorf("global variable not exist, but used by serviceName: %s, key: %s", serviceName, renderKV.Key)
		}

		if renderKV.UseGlobalVariable {
			delRelatedServices(globalVariableKV, serviceName)
		}
	}

	retGlobalVariables := []*GlobalVariableKV{}
	for _, kv := range globalVariableMap {
		retGlobalVariables = append(retGlobalVariables, kv)
	}

	return retGlobalVariables, argVariables, nil
}

func UpdateRenderVariable(global []*GlobalVariableKV, render []*RenderVariableKV) []*RenderVariableKV {
	globalMap := map[string]*GlobalVariableKV{}
	for _, kv := range global {
		globalMap[kv.Key] = kv
	}

	ret := []*RenderVariableKV{}
	for _, kv := range render {
		if globalKV, ok := globalMap[kv.Key]; ok {
			if kv.UseGlobalVariable {
				retKV := &RenderVariableKV{
					ServiceVariableKV: ServiceVariableKV{
						Key:     globalKV.Key,
						Value:   globalKV.Value,
						Type:    globalKV.Type,
						Options: globalKV.Options,
						Desc:    globalKV.Desc,
					},
					UseGlobalVariable: true,
				}
				ret = append(ret, retKV)
			} else {
				ret = append(ret, kv)
			}
		} else {
			ret = append(ret, kv)
		}
	}

	return ret
}

func ClipRenderVariableKVs(template []*ServiceVariableKV, render []*RenderVariableKV) (yaml string, kvs []*RenderVariableKV, err error) {
	templateSet := sets.NewString()
	for _, kv := range template {
		templateSet.Insert(kv.Key)
	}

	ret := []*RenderVariableKV{}
	for _, kv := range render {
		if templateSet.Has(kv.Key) {
			ret = append(ret, kv)
		}
	}

	yaml, err = RenderVariableKVToYaml(ret)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert render variable kv to yaml, err: %w", err)
	}

	return yaml, ret, nil
}
