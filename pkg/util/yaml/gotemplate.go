/*
Copyright 2022 The KodeRover Authors.

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
	"reflect"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/sets"
)

// guessPreciseType guess the type of variable for situation: {{ eq }}
func guessPreciseType(nodes []parse.Node) string {
	if len(nodes) == 3 && nodes[0].Type() == parse.NodeIdentifier && nodes[0].String() == "eq" {
		switch nodes[2].Type() {
		case parse.NodeString:
			return "string"
		case parse.NodeNumber:
			return "number"
		case parse.NodeNil:
			return "nil"
		}
	}
	return "default"
}

// ExtractVariableYaml extracts variables from go-template and packages into yaml
func ExtractVariableYaml(sourceTmpl string) (string, error) {
	defer func() {
		recover()
	}()

	ot := template.New("sourceTmpl")
	tpl, err := ot.Parse(sourceTmpl)
	if err != nil {
		return "", errors.Wrap(err, "invalid template file")
	}

	keys := parseTemplateVariables(tpl)
	placeholders := buildPlaceholderData(keys)

	variableYaml, err := yaml.Marshal(placeholders)
	if err != nil {
		return "", errors.Wrap(err, "failed to package variables")
	}

	return string(variableYaml), nil
}

// parseTemplateVariables extracts the variable used in go-template file
// complex grammar is not supported, eg: custom functions / range in range / local variables using $ ...
// response data format: []string{var-name:var-type...}
// the resp data is only advisory
func parseTemplateVariables(parsedTemplate *template.Template) []string {
	ret := sets.NewString()
	visited := make(map[interface{}]bool)
	queue := []reflect.Value{reflect.ValueOf(parsedTemplate.Root)}
	for len(queue) != 0 {
		node := queue[0]
		queue = queue[1:]

		for node.Kind() != reflect.Struct && node.Kind() != reflect.Array && node.Kind() != reflect.Slice {
			node = node.Elem()
		}

		if node.CanInterface() {
			vInterface := node.Interface()
			if listNode, ok := vInterface.([]parse.Node); ok {
				for _, itemNode := range listNode {
					if itemNode.Type() == parse.NodeField {
						ret.Insert(fmt.Sprintf("%v:%s", itemNode, guessPreciseType(listNode)))
					}
				}
			}
		}

		var n int
		if node.Kind() == reflect.Array || node.Kind() == reflect.Slice {
			n = node.Len()
		} else {
			n = node.NumField()
		}
		for i := 0; i != n; i++ {
			var itemNode reflect.Value
			if node.Kind() == reflect.Array || node.Kind() == reflect.Slice {
				itemNode = node.Index(i)
			} else {
				itemNode = node.Field(i)
			}

			if !itemNode.IsValid() || itemNode.IsZero() {
				continue
			}

			if itemNode.Kind() == reflect.Bool ||
				itemNode.Kind() == reflect.String ||
				itemNode.Kind() == reflect.Int || itemNode.Kind() == reflect.Int8 || itemNode.Kind() == reflect.Int16 || itemNode.Kind() == reflect.Int32 || itemNode.Kind() == reflect.Int64 ||
				itemNode.Kind() == reflect.Uint || itemNode.Kind() == reflect.Uint8 || itemNode.Kind() == reflect.Uint16 || itemNode.Kind() == reflect.Uint32 || itemNode.Kind() == reflect.Uint64 || itemNode.Kind() == reflect.Uintptr ||
				itemNode.Kind() == reflect.Float32 || itemNode.Kind() == reflect.Float64 ||
				itemNode.Kind() == reflect.Complex64 || itemNode.Kind() == reflect.Complex128 {
				continue
			}

			itemNodePtr := itemNode.Addr()
			if _, ok := visited[itemNodePtr]; ok {
				continue
			}
			visited[itemNodePtr] = true

			queue = append(queue, itemNode)
		}
	}
	return ret.List()
}

func buildPlaceholderData(keys []string) interface{} {
	ret := &ComplexStruct{}
	for _, key := range keys {
		key = strings.TrimLeft(key, ".")
		keyList := strings.Split(key, ".")
		parent := ret
		ok := false
		for i, singleKey := range keyList {
			parent.Insert(singleKey, i == len(keyList)-1)
			parentInterface := parent.Get(singleKey)
			if parent, ok = parentInterface.(*ComplexStruct); !ok {
				break
			}
		}
	}
	return ret
}

// ComplexStruct is the alias of map, so operator 'len' is supported in go-template
type ComplexStruct map[string]interface{}

func (cs *ComplexStruct) String() string {
	return ""
}

func getKeyAndType(originKey string) (string, string) {
	keys := strings.Split(originKey, ":")
	if len(keys) != 2 {
		return originKey, "default"
	}
	return keys[0], keys[1]
}

func (cs *ComplexStruct) Insert(key string, notParent bool) {
	pureKey, keyType := getKeyAndType(key)
	if _, ok := (map[string]interface{}(*cs))[pureKey]; ok {
		return
	}
	if notParent {
		if keyType == "default" {
			keyType = "nil"
		}
	} else {
		keyType = "default"
	}
	switch keyType {
	case "string":
		(map[string]interface{}(*cs))[pureKey] = ""
	case "number":
		(map[string]interface{}(*cs))[pureKey] = 0
	case "nil":
		(map[string]interface{}(*cs))[pureKey] = nil
	default:
		(map[string]interface{}(*cs))[pureKey] = &ComplexStruct{}
	}
}

func (cs *ComplexStruct) Get(key string) interface{} {
	pureKey, _ := getKeyAndType(key)
	return (map[string]interface{}(*cs))[pureKey]
}
