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

package vars_upgrade

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

var templateMap = make(map[string]*models.YamlTemplate)

// 全局变量定义
var GlobalKeys = sets.NewString([]string{"LOGBACK_CONFIGMAP", "NODE_AFFINITY", "TOLERATIONS", "IMAGE_PULL_SECRETS"}...)

// 复杂服务变量定义
var ComplexServiceKeys = sets.NewString([]string{"GENERAL_CONFIGMAP", "HOSTALIASES"}...)

var dryRun bool = false
var outPutMessages = false
var appointedTemplates string = ""

func getYamlTemplates() error {
	yamlTemplates, _, err := mongodb.NewYamlTemplateColl().List(0, 0)
	if err != nil && !mongodb.IsErrNoDocuments(err) {
		log.Errorf("failed to list templates error: %s", err)
		return err
	}
	if len(yamlTemplates) == 0 {
		log.Infof("no yaml template used, skipping...")
		return nil
	}

	templateSets := sets.NewString()
	if len(appointedTemplates) > 0 {
		templateSets.Insert(strings.Split(appointedTemplates, ",")...)
	}
	filter := func(templateName string) bool {
		if len(templateSets) == 0 || templateSets.Has(templateName) {
			return true
		}
		return false
	}

	for _, yamlTemplate := range yamlTemplates {
		if !filter(yamlTemplate.Name) {
			continue
		}
		templateMap[yamlTemplate.ID.Hex()] = yamlTemplate
	}
	log.Infof("found %d yaml templates", len(templateMap))
	return nil
}

func handleServiceByTemplate() error {
	for _, yamlTemplate := range templateMap {
		handleSingleTemplate(yamlTemplate)
	}
	return nil
}

func handleSingleTemplate(yamlTemplate *models.YamlTemplate) {
	log.Infof("-------- start handle service created from template %s --------", yamlTemplate.Name)
	serviceReferences, err := service.GetYamlTemplateReference(yamlTemplate.ID.Hex(), log.SugaredLogger())
	if err != nil {
		log.Errorf("failed to get service references for template %s error: %s", yamlTemplate.Name, err)
		return
	}
	failedServices := make(map[string]sets.String)
	for _, serviceReference := range serviceReferences {
		if _, ok := failedServices[serviceReference.ProjectName]; ok {
			failedServices[serviceReference.ProjectName] = sets.NewString()
		}
		err := handleSingleService(yamlTemplate, serviceReference)
		if err != nil {
			log.Error(err)
			failedServices[serviceReference.ProjectName].Insert(serviceReference.ServiceName)
		}
	}
	if len(failedServices) > 0 {
		for project, services := range failedServices {
			log.Errorf("failed services in project %s: %+v", project, services.List())
		}
	}
	log.Infof("-------- finished handle template %s -----------", yamlTemplate.Name)
	return
}

func handleSingleService(yamlTemplate *models.YamlTemplate, serviceReference *template.ServiceReference) error {
	log.Infof("+++++++++++ handling service %s/%s +++++++++++", serviceReference.ProjectName, serviceReference.ServiceName)
	templateService, err := mongodb.NewServiceColl().Find(&mongodb.ServiceFindOption{
		ServiceName: serviceReference.ServiceName,
		Type:        "k8s",
		ProductName: serviceReference.ProjectName,
	})
	if err != nil {
		return fmt.Errorf("!!!!!!!!!! failed to find service %s/%s error: %s", serviceReference.ProjectName, serviceReference.ServiceName, err)
	}
	if templateService.TemplateID != yamlTemplate.ID.Hex() || templateService.Source != setting.ServiceSourceTemplate {
		log.Infof("skipped, service %s/%s is not connected with from template %s", templateService.ProductName, templateService.ServiceName, yamlTemplate.Name)
		return nil
	}
	if len(templateService.VariableYaml) != 0 || len(templateService.ServiceVars) != 0 {
		log.Infof("skipped, service %s/%s has been upgraded %s", templateService.ProductName, templateService.ServiceName)
		return nil
	}

	creation := &models.CreateFromYamlTemplate{}
	bs, err := json.Marshal(templateService.CreateFrom)
	if err != nil {
		return fmt.Errorf("!!!!!!!!!! failed to marshal creation data for service: %s/%s, err: %s", templateService.ProductName, templateService.ServiceName, err)
	}

	err = json.Unmarshal(bs, creation)
	if err != nil {
		return fmt.Errorf("!!!!!!!!!! failed to unmarshal creation data for service: %s/%s, err: %s", templateService.ProductName, templateService.ServiceName, err)
	}

	//cpuLimit: 50m
	//LOGBACK_CONFIGMAP: {{.LOGBACK_CONFIGMAP}}
	//port: 20221
	//tolerations:
	//- effect: NoSchedule
	//  key: {{.TOLERATION_KEY}}
	//  operator: Equal
	//  value: {{.TOLERATION_VALUE}}

	// 服务实例化时候的变量yaml，转为 KV
	// 这部分数据作为服务实例化时候的数据

	//抽取出来的template 格式的变量
	templateKv := make(map[string]string)

	//当前服务实例化时候的渲染变量
	oldServiceMap := make(map[string]interface{})

	// 生成后的变量
	generatedKvMap := make(map[string]interface{})
	creationYamlKVS, err := template.GetYamlVariables(creation.VariableYaml, log.SugaredLogger())
	if err != nil {
		return fmt.Errorf("!!!!!!!!! failed to get yaml variables for service: %s/%s, err: %s", templateService.ProductName, templateService.ServiceName, err)
	}

	if outPutMessages {
		log.Infof("service: %s/%s, creationYamlKVS: %+v", templateService.ProductName, templateService.ServiceName, creationYamlKVS)
	}

	// 将{{.xxx}} 替换为 $xxx$，便于将yaml unmarshall为map
	for _, kv := range creationYamlKVS {
		creation.VariableYaml = strings.ReplaceAll(creation.VariableYaml, fmt.Sprintf("{{.%s}}", kv.Key), "")
		templateKv[fmt.Sprintf("%s", kv.Key)] = fmt.Sprintf("{{.%s}}", kv.Key)
	}

	//cpuLimit: 50m
	//LOGBACK_CONFIGMAP: $LOGBACK_CONFIGMAP$
	//port: 20221
	//tolerations:
	//- effect: NoSchedule
	//  key: $TOLERATION_KEY$
	//  operator: Equal
	//  value: $TOLERATION_VALUE$
	err = yaml.Unmarshal([]byte(creation.VariableYaml), &oldServiceMap)
	if err != nil {
		return fmt.Errorf("!!!!!!!!!! failed to unmarshal creation variable yaml for service: %s/%s, err: %s", templateService.ProductName, templateService.ServiceName, err)
	}
	for k, v := range oldServiceMap {
		// 全局变量忽略
		if GlobalKeys.Has(strings.ToUpper(k)) {
			generatedKvMap[k] = nil
			continue
		}
		// 简单变量单独处理，只管服务初始化的变量
		if strValue, ok := v.(string); ok {
			generatedKvMap[k] = strValue
		} else if boolValue, ok := v.(bool); ok {
			generatedKvMap[k] = boolValue
		} else {
			if outPutMessages {
				log.Infof("setting complex variable %s for service %s/%s", k, templateService.ProductName, templateService.ServiceName)
			}
			generatedKvMap[k] = v
		}
	}

	defaultRenderset, err := mongodb.NewRenderSetColl().Find(&mongodb.RenderSetFindOption{
		ProductTmpl: templateService.ProductName,
		Name:        templateService.ProductName,
		IsDefault:   true,
	})
	if err != nil {
		log.Errorf("failed to find default renderset: %s, err: %s", templateService.ProductName, err)
		//return nil
	}
	valuesMap := make(map[string]string)
	if defaultRenderset != nil {
		for _, kv := range defaultRenderset.KVs {
			valuesMap[kv.Key] = kv.Value
		}
	}

	// 处理全局变量的值
	for k, _ := range oldServiceMap {
		switch k {
		case "LOGBACK_CONFIGMAP", "IMAGE_PULL_SECRETS":
			generatedKvMap[k] = valuesMap[k]
		case "TOLERATIONS":
			generatedKvMap[k] = []map[string]interface{}{
				{"KEY": valuesMap["TOLERATION_KEY"], "VALUE": valuesMap["TOLERATION_VALUE"]},
			}
		case "NODE_AFFINITY":
			generatedKvMap[k] = []map[string]interface{}{
				{"KEY": valuesMap["NODE_AFFINITY_KEY"], "VALUES": []interface{}{valuesMap["NODE_AFFINITY_VALUE"]}},
			}
		}
	}

	if outPutMessages {
		log.Infof("service %s/%s total generatedKvMap: \n%+v", templateService.ProductName, templateService.ServiceName, generatedKvMap)
	}

	bs, err = yaml.Marshal(generatedKvMap)
	if err != nil {
		return fmt.Errorf("!!!!!!!!!! failed to marshal generatedKvMap for service: %s/%s, err: %s", templateService.ProductName, templateService.ServiceName, err)
	}

	//if outPutMessages {
	//}
	log.Infof("service %s/%s generatedKvMap yaml:\n%s", templateService.ProductName, templateService.ServiceName, string(bs))

	// 更新服务可见性
	templateService.ServiceVars = yamlTemplate.ServiceVars
	templateService.VariableYaml = string(bs)

	if dryRun {
		return nil
	}
	return nil
}
