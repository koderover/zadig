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
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/sets"
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
	UseGlobalVariable   bool `bson:"use_global_variable"   yaml:"use_global_variable"    json:"use_global_variable"`
	IsReferenceVariable bool `bson:"is_reference_variable" yaml:"is_reference_variable"  json:"is_reference_variable"`
}

type GlobalVariableKV struct {
	ServiceVariableKV `bson:",inline" yaml:",inline" json:",inline"`
	RelatedServices   []string `bson:"related_services"     yaml:"related_services"     json:"related_services"`
}

// yaml spec document: https://yaml.org/spec/1.2.2/
func convertValueToNode(data interface{}) *yaml.Node {
	node := &yaml.Node{}
	if dataMap, ok := data.(map[string]interface{}); ok {
		node.Kind = yaml.MappingNode

		for key, value := range dataMap {
			childKey := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: key,
			}
			node.Content = append(node.Content, childKey)

			childValue := convertValueToNode(value)
			node.Content = append(node.Content, childValue)
		}
	} else if dataSlice, ok := data.([]interface{}); ok {
		node.Kind = yaml.SequenceNode

		for _, value := range dataSlice {
			child := convertValueToNode(value)
			node.Content = append(node.Content, child)
		}
	} else {
		node.Kind = yaml.ScalarNode
		node.Value = fmt.Sprintf("%v", data)
	}

	return node
}

func addValueToNode(key string, data interface{}, node *yaml.Node) *yaml.Node {
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: key,
	}
	node.Content = append(node.Content, keyNode)

	switch data.(type) {
	case *yaml.Node:
		valueNode := data.(*yaml.Node)
		if valueNode.Kind == yaml.DocumentNode {
			node.Content = append(node.Content, valueNode.Content...)
		} else {
			if valueNode.Kind != yaml.DocumentNode &&
				valueNode.Kind != yaml.SequenceNode &&
				valueNode.Kind != yaml.MappingNode &&
				valueNode.Kind != yaml.ScalarNode &&
				valueNode.Kind != yaml.AliasNode {
				valueNode.Kind = yaml.ScalarNode
			}
			node.Content = append(node.Content, valueNode)
		}
	default:
		valueChild := convertValueToNode(data)
		node.Content = append(node.Content, valueChild)
	}

	return node
}

// inorder to preserve the order of the yaml, we need to use yaml.node to marshal and unmarshal yaml
func marshalYamlNode(node *yaml.Node) (string, error) {
	buf := bytes.Buffer{}
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err := enc.Encode(node)
	if err != nil {
		return "", fmt.Errorf("failed to marshal node yaml, err: %w", err)
	}
	return buf.String(), nil
}

// not suitable for flatten kv
// ServiceVariableKV is a kv which aggregate complicated struct to the first layer
func ServiceVariableKVToYaml(kvs []*ServiceVariableKV, forceCheck bool) (string, error) {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
	}

	for _, kv := range kvs {
		if kv == nil {
			continue
		}

		switch kv.Type {
		case ServiceVariableKVTypeYaml:
			value, ok := kv.Value.(string)
			if !ok {
				return "", fmt.Errorf("failed to convert value to yaml, key: %v, value: %v", kv.Key, kv.Value)
			}

			valueNode := &yaml.Node{}
			err := yaml.Unmarshal([]byte(value), valueNode)
			if err != nil {
				return "", fmt.Errorf("failed to unmarshal yaml, key: %v, err: %w", kv.Key, err)
			}
			node = addValueToNode(kv.Key, valueNode, node)
		case ServiceVariableKVTypeBoolean:
			v, ok := kv.Value.(bool)
			if ok {
				node = addValueToNode(kv.Key, v, node)
			} else if kv.Value == "true" {
				node = addValueToNode(kv.Key, true, node)
			} else if kv.Value == "false" {
				node = addValueToNode(kv.Key, false, node)
			} else {
				return "", fmt.Errorf("invaild value for boolean, key: %v, value: %v", kv.Key, kv.Value)
			}
		case ServiceVariableKVTypeEnum:
			if forceCheck {
				v := fmt.Sprintf("%v", kv.Value)
				if !sets.NewString(kv.Options...).Has(v) {
					return "", fmt.Errorf("invaild value for enum, key: %v, value: %v, options: %v", kv.Key, kv.Value, kv.Options)
				}
			}
			node = addValueToNode(kv.Key, kv.Value, node)
		default:
			node = addValueToNode(kv.Key, kv.Value, node)
		}
	}

	return marshalYamlNode(node)
}

// not suitable for flatten kv
// ServiceVariableKV is a kv which aggregate complicated struct to the first layer
func YamlToServiceVariableKV(yamlStr string, origKVs []*ServiceVariableKV) ([]*ServiceVariableKV, error) {
	if yamlStr == "null" || yamlStr == "null\n" {
		return nil, nil
	}

	node := &yaml.Node{}
	err := yaml.Unmarshal([]byte(yamlStr), node)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml, err: %w", err)
	}

	origKVMap := make(map[string]*ServiceVariableKV, 0)
	for _, kv := range origKVs {
		origKVMap[kv.Key] = kv
	}

	ret := make([]*ServiceVariableKV, 0)
	if node != nil && node.Content != nil {
		if node.Kind != yaml.DocumentNode {
			return nil, fmt.Errorf("root node is not docment node")
		}

		for _, root := range node.Content {
			if root.Kind == yaml.MappingNode {
				for i := 0; i < len(root.Content); i += 2 {
					key := root.Content[i].Value
					value := root.Content[i+1]

					kv, err := snippetToKV(key, value, origKVMap[key])
					if err != nil {
						return nil, fmt.Errorf("failed to convert snippet to kv, err: %w", err)
					}

					ret = append(ret, kv)
				}
			} else {
				return nil, fmt.Errorf("root node is not mapping node")
			}
		}
	}

	return ret, nil
}

// FIXME: ServiceVariableKVType not equal with golang runtime type
func snippetToKV(key string, snippet *yaml.Node, origKV *ServiceVariableKV) (*ServiceVariableKV, error) {
	var retKV *ServiceVariableKV
	if origKV == nil {
		// origKV is nil, create a new kv
		retKV = &ServiceVariableKV{
			Key: key,
		}

		switch snippet.Kind {
		case yaml.ScalarNode:
			retKV.Type = ServiceVariableKVTypeString
			retKV.Value = snippet.Value
		case yaml.MappingNode:
			retKV.Type = ServiceVariableKVTypeYaml
			snippetYaml, err := marshalYamlNode(snippet)
			if err != nil {
				return nil, fmt.Errorf("key %s: failed to marshal snippet, err: %w", retKV.Key, err)
			}
			retKV.Type = ServiceVariableKVTypeYaml
			retKV.Value = snippetYaml
		case yaml.SequenceNode:
			retKV.Type = ServiceVariableKVTypeYaml
			snippetYaml, err := marshalYamlNode(snippet)
			if err != nil {
				return nil, fmt.Errorf("key %s: failed to marshal snippet, err: %w", retKV.Key, err)
			}
			retKV.Type = ServiceVariableKVTypeYaml
			retKV.Value = snippetYaml
		default:
			if snippet.Value == "true" || snippet.Value == "false" {
				retKV.Type = ServiceVariableKVTypeBoolean
			} else {
				retKV.Type = ServiceVariableKVTypeString
			}
			retKV.Value = snippet.Value
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
			value := snippet.Value
			if value != "true" && value != "false" {
				return nil, fmt.Errorf("key %s: invalid value: %s for boolean", retKV.Key, snippet.Value)
			}
		case ServiceVariableKVTypeEnum:
			// check if value exist in options
			optionSet := sets.NewString(origKV.Options...)
			if !optionSet.Has(snippet.Value) {
				return nil, fmt.Errorf("key %s: invalid value: %v, valid options: %v", retKV.Key, snippet.Value, origKV.Options)
			}
		}

		// convert snippet to value
		if retKV.Type == ServiceVariableKVTypeYaml {
			snippetYaml, err := marshalYamlNode(snippet)
			if err != nil {
				return nil, fmt.Errorf("key %s: failed to marshal snippet, err: %w", retKV.Key, err)
			}
			retKV.Value = snippetYaml
		} else {
			switch snippet.Kind {
			case yaml.MappingNode, yaml.SequenceNode:
				snippetYaml, err := marshalYamlNode(snippet)
				if err != nil {
					return nil, fmt.Errorf("key %s: failed to marshal snippet, err: %w", retKV.Key, err)
				}
				retKV.Value = snippetYaml
			default:
				retKV.Value = snippet.Value
			}
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
		ret.Key = template.Key
		ret.Value = template.Value
		ret.Type = template.Type
		ret.Options = template.Options
		ret.Desc = template.Desc
	} else {
		ret.Key = template.Key
		ret.Value = render.Value
		ret.Type = template.Type
		ret.Options = template.Options
		ret.Desc = template.Desc
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

	yaml, err := ServiceVariableKVToYaml(ret, true)
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

	yaml, err := ServiceVariableKVToYaml(ret, true)
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

	yaml, err := RenderVariableKVToYaml(ret, true)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert render variable kv to yaml, err: %w", err)
	}

	return yaml, ret, nil
}

// merge service variables base on service template variables
// serivce variables has higher value priority, service template variables has higher type priority
func MergeServiceAndServiceTemplateVariableKVs(service []*ServiceVariableKV, serivceTemplate []*ServiceVariableKV) (string, []*ServiceVariableKV, error) {
	serivce := map[string]*ServiceVariableKV{}
	for _, kv := range service {
		serivce[kv.Key] = kv
	}

	ret := []*ServiceVariableKV{}
	for _, kv := range serivceTemplate {
		if serviceKV, ok := serivce[kv.Key]; !ok {
			ret = append(ret, kv)
		} else {
			newKV := transferServiceVariable(serviceKV, kv)
			ret = append(ret, newKV)
		}
	}

	yaml, err := ServiceVariableKVToYaml(ret, false)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert service variable kv to yaml, err: %w", err)
	}

	return yaml, ret, nil
}

// merge render variables base on service template variables
// render variables has higher value priority, service template variables has higher type priority
// normally used to get latest variables for a service
func MergeRenderAndServiceTemplateVariableKVs(render []*RenderVariableKV, serviceTemplate []*ServiceVariableKV) (string, []*RenderVariableKV, error) {
	renderMap := map[string]*RenderVariableKV{}
	for _, kv := range render {
		renderMap[kv.Key] = kv
	}

	ret := []*RenderVariableKV{}
	for _, kv := range serviceTemplate {
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

	yaml, err := RenderVariableKVToYaml(ret, false)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert render variable kv to yaml, err: %w", err)
	}

	return yaml, ret, nil
}

// RenderVariableKVToYaml
func RenderVariableKVToYaml(kvs []*RenderVariableKV, forceCheck bool) (string, error) {
	serviceVariableKVs := make([]*ServiceVariableKV, 0)
	for _, kv := range kvs {
		serviceVariableKVs = append(serviceVariableKVs, &kv.ServiceVariableKV)
	}

	return ServiceVariableKVToYaml(serviceVariableKVs, forceCheck)
}

func GlobalVariableKVToYaml(kvs []*GlobalVariableKV) (string, error) {
	serviceVariableKVs := make([]*ServiceVariableKV, 0)
	for _, kv := range kvs {
		serviceVariableKVs = append(serviceVariableKVs, &kv.ServiceVariableKV)
	}

	return ServiceVariableKVToYaml(serviceVariableKVs, true)
}

// validate global variables base on global variables define
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

// update global variable's related services and render variables value base on useGlobalVariable flag
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

// update render variables base on global variables and useGlobalVariable flag
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

	yaml, err = RenderVariableKVToYaml(ret, true)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert render variable kv to yaml, err: %w", err)
	}

	return yaml, ret, nil
}

func ClipServiceVariableKVs(clipRange []*ServiceVariableKV, kvs []*ServiceVariableKV) (string, []*ServiceVariableKV, error) {
	clipRangeSet := sets.NewString()
	for _, kv := range clipRange {
		clipRangeSet.Insert(kv.Key)
	}

	ret := []*ServiceVariableKV{}
	for _, kv := range kvs {
		if clipRangeSet.Has(kv.Key) {
			ret = append(ret, kv)
		}
	}

	yaml, err := ServiceVariableKVToYaml(ret, false)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert service variable kv to yaml, err: %w", err)
	}

	return yaml, ret, nil
}

type GlobalVariables struct {
	Variables           []*ServiceVariableKV `json:"variables"`
	ProductionVariables []*ServiceVariableKV `json:"production_variables"`
}
