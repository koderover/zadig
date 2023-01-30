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

package kube

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	gotemplate "text/template"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commomtemplate "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util/converter"
)

func ClipVariableYaml(variableYaml string, validKeys []string) (string, error) {
	valuesMap, err := converter.YamlToFlatMap([]byte(variableYaml))
	if err != nil {
		return "", fmt.Errorf("failed to get flat map for service variable, err: %s", err)
	}

	keysSet := sets.NewString(validKeys...)
	validKvMap := make(map[string]interface{})
	for k, v := range valuesMap {
		if keysSet.Has(k) {
			validKvMap[k] = v
		}
	}

	validKvMap, err = converter.Expand(validKvMap)
	if err != nil {
		return "", err
	}

	bs, err := yaml.Marshal(validKvMap)
	return string(bs), err
}

// extract valid svc variable from service variable
// keys defined in service vars are valid
// keys not defined in service vars or default values are valid as well
func extractValidSvcVariable(serviceName string, rs *commonmodels.RenderSet, serviceVars []string, serviceDefaultValues string) (string, error) {
	serviceVariable := ""
	for _, v := range rs.ServiceVariables {
		if v.ServiceName != serviceName {
			continue
		}
		if v.OverrideYaml != nil {
			serviceVariable = v.OverrideYaml.YamlContent
		}
		break
	}

	valuesMap, err := converter.YamlToFlatMap([]byte(serviceVariable))
	if err != nil {
		return "", fmt.Errorf("failed to get flat map for service variable, err: %s", err)
	}

	serviceDefaultValuesMap, err := converter.YamlToFlatMap([]byte(serviceDefaultValues))
	if err != nil {
		return "", fmt.Errorf("failed to get flat map for service default variable, err: %s", err)
	}

	// keys defined in service vars
	keysSet := sets.NewString(serviceVars...)
	validKvMap, svcValidDefaultValueMap := make(map[string]interface{}), make(map[string]interface{})

	for k, v := range valuesMap {
		if keysSet.Has(k) {
			validKvMap[k] = v
		}
	}

	for k, v := range serviceDefaultValuesMap {
		if _, ok := validKvMap[k]; !ok {
			svcValidDefaultValueMap[k] = v
		}
	}

	defaultValuesMap, err := converter.YamlToFlatMap([]byte(rs.DefaultValues))
	if err != nil {
		return "", fmt.Errorf("failed to get flat map for default variable, err: %s", err)
	}
	for k := range defaultValuesMap {
		delete(svcValidDefaultValueMap, k)
	}

	for k, v := range svcValidDefaultValueMap {
		validKvMap[k] = v
	}

	validKvMap, err = converter.Expand(validKvMap)
	if err != nil {
		return "", err
	}

	bs, err := yaml.Marshal(validKvMap)
	return string(bs), err
}

func RenderServiceYaml(originYaml, productName, serviceName string, rs *commonmodels.RenderSet, serviceVars []string, serviceDefaultValues string) (string, error) {
	if rs == nil {
		return originYaml, nil
	}
	tmpl, err := gotemplate.New(fmt.Sprintf("%s:%s", productName, serviceName)).Parse(originYaml)
	if err != nil {
		return originYaml, fmt.Errorf("failed to build template, err: %s", err)
	}

	serviceVariable, err := extractValidSvcVariable(serviceName, rs, serviceVars, serviceDefaultValues)
	if err != nil {
		return "", fmt.Errorf("failed to extract variable for service: %s, err: %s", serviceName, err)
	}
	variableYaml, replacedKv, err := commomtemplate.SafeMergeVariableYaml(rs.DefaultValues, serviceVariable)
	if err != nil {
		return originYaml, err
	}

	variableYaml = strings.ReplaceAll(variableYaml, setting.TemplateVariableProduct, productName)
	variableYaml = strings.ReplaceAll(variableYaml, setting.TemplateVariableService, serviceName)

	variableMap := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(variableYaml), &variableMap)
	if err != nil {
		return originYaml, fmt.Errorf("failed to unmarshal variable yaml, err: %s", err)
	}

	buf := bytes.NewBufferString("")
	err = tmpl.Execute(buf, variableMap)
	if err != nil {
		return originYaml, fmt.Errorf("template validate err: %s", err)
	}

	originYaml = buf.String()

	// replace system variables
	originYaml = strings.ReplaceAll(originYaml, setting.TemplateVariableProduct, productName)
	originYaml = strings.ReplaceAll(originYaml, setting.TemplateVariableService, serviceName)

	for rk, rv := range replacedKv {
		originYaml = strings.ReplaceAll(originYaml, rk, rv)
	}

	return originYaml, nil
}

//func RenderValueForString(origin string, rs *commonmodels.RenderSet) string {
//	if rs == nil {
//		return origin
//	}
//	rs.SetKVAlias()
//	for _, v := range rs.KVs {
//		if v.State == "unused" {
//			continue
//		}
//		origin = replaceAliasValue(origin, v)
//	}
//	return origin
//}

//func replaceAliasValue(origin string, v *templatemodels.RenderKV) string {
//	for {
//		idx := strings.Index(origin, v.Alias)
//		if idx < 0 {
//			break
//		}
//		spaces := ""
//		start := 0
//		for i := idx - 1; i >= 0; i-- {
//			if string(origin[i]) != " " {
//				break
//			}
//			start++
//		}
//		spaces = origin[idx-start : idx]
//		valueStr := strings.Replace(v.Value, "\n", fmt.Sprintf("%s%s", "\n", spaces), -1)
//		origin = strings.Replace(origin, v.Alias, valueStr, 1)
//	}
//	return origin
//}

// RenderEnvService renders service with particular revision and service vars in environment
func RenderEnvService(prod *commonmodels.Product, render *commonmodels.RenderSet, service *commonmodels.ProductService) (yaml string, err error) {
	opt := &commonrepo.ServiceFindOption{
		ServiceName: service.ServiceName,
		ProductName: service.ProductName,
		Type:        service.Type,
		Revision:    service.Revision,
	}
	svcTmpl, err := commonrepo.NewServiceColl().Find(opt)
	if err != nil {
		return "", err
	}

	// Note only the keys in TemplateService.ServiceVar can work

	parsedYaml, err := RenderServiceYaml(svcTmpl.Yaml, prod.ProductName, svcTmpl.ServiceName, render, svcTmpl.ServiceVars, svcTmpl.VariableYaml)
	if err != nil {
		log.Error("failed to render service yaml, err: %s", err)
		return "", err
	}
	//parsedYaml := RenderValueForString(svcTmpl.Yaml, prod.ProductName, svcTmpl.ServiceName, render)
	parsedYaml = ParseSysKeys(prod.Namespace, prod.EnvName, prod.ProductName, service.ServiceName, parsedYaml)
	parsedYaml = replaceContainerImages(parsedYaml, svcTmpl.Containers, service.Containers)

	return parsedYaml, nil
}

func replaceContainerImages(tmpl string, ori []*commonmodels.Container, replace []*commonmodels.Container) string {

	replaceMap := make(map[string]string)
	for _, container := range replace {
		replaceMap[container.Name] = container.Image
	}

	for _, container := range ori {
		imageRex := regexp.MustCompile("image:\\s*" + container.Image)
		if _, ok := replaceMap[container.Name]; !ok {
			continue
		}
		tmpl = imageRex.ReplaceAllLiteralString(tmpl, fmt.Sprintf("image: %s", replaceMap[container.Name]))
	}

	return tmpl
}
