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

	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"

	service2 "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"

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
var defaultRenderMap = make(map[string]*models.RenderSet)

// 全局变量定义
var GlobalKeys = sets.NewString([]string{"LOGBACK_CONFIGMAP", "NODE_AFFINITY", "TOLERATIONS", "IMAGE_PULL_SECRETS"}...)

// 复杂服务变量定义
var ComplexServiceKeys = sets.NewString([]string{"GENERAL_CONFIGMAP", "HOSTALIASES"}...)

var dryRun bool = false
var outPutMessages = false
var appointedTemplates string = ""
var appointedProjects string = ""

func handlerServices() error {
	err := getYamlTemplates()
	if err != nil {
		return err
	}
	err = handleServiceByTemplate()
	if err != nil {
		return err
	}
	return nil
}

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
	log.Infof("************** found %d yaml templates", len(templateMap))
	return nil
}

var allServiceRevision map[string]sets.Int64 = make(map[string]sets.Int64)
var allProjects sets.String = sets.NewString()

func handleServiceByTemplate() error {
	for _, yamlTemplate := range templateMap {
		serviceReferences, err := service.GetYamlTemplateReference(yamlTemplate.ID.Hex(), log.SugaredLogger())
		if err != nil {
			return fmt.Errorf("failed to get service references for template %s error: %s", yamlTemplate.Name, err)
		}
		for _, serviceReference := range serviceReferences {
			serviceKey := fmt.Sprintf("%s/%s", serviceReference.ProjectName, serviceReference.ServiceName)
			allServiceRevision[serviceKey] = sets.NewInt64()
			allProjects.Insert(serviceReference.ProjectName)
		}
	}

	//找到所有当前在用的服务以及版本
	temProducts, err := templaterepo.NewProductColl().ListWithOption(&templaterepo.ProductListOpt{
		DeployType:    setting.K8SDeployType,
		BasicFacility: setting.BasicFacilityK8S,
	})
	if err != nil {
		log.Errorf("handleServiceByTemplate list projects error: %s", err)
		return fmt.Errorf("handleServiceByTemplate list projects error: %s", err)
	}
	for _, project := range temProducts {
		envs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{
			Name: project.ProductName,
		})
		if err != nil {
			return fmt.Errorf("!!!!!!!!! [handleServiceByTemplate]failed to find envs in project %s , err: %s", project.ProductName, err)
		}
		for _, product := range envs {
			log.Infof("\n+++++++++++ handling product %s/%s +++++++++++", project.ProductName, product.EnvName)
			for _, service := range product.GetServiceMap() {
				allServiceRevision[fmt.Sprintf("%s/%s", project.ProductName, service.ServiceName)].Insert(service.Revision)
			}
		}
	}

	for _, yamlTemplate := range templateMap {
		handleSingleTemplate(yamlTemplate)
	}

	handleUsedOldVersionServices()
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
	log.Infof("\n-------- finished handle template %s -----------\n\n", yamlTemplate.Name)
	return
}

func handleUsedService(projectName, serviceName string, revision int64) error {
	log.Infof("\n+++++++++++ handling used service %s/%s:%d +++++++++++", projectName, serviceName, revision)
	templateService, err := mongodb.NewServiceColl().Find(&mongodb.ServiceFindOption{
		ServiceName: serviceName,
		Type:        "k8s",
		ProductName: projectName,
		Revision:    revision,
	})
	if err != nil {
		return fmt.Errorf("!!!!!!!!!! failed to find service %s/%s:%d error: %s", projectName, serviceName, revision, err)
	}

	if len(templateService.ServiceVars) != 0 || len(templateService.VariableYaml) > 0 {
		log.Infof("skipped, service %s/%s:%d has been upgraded", templateService.ProductName, templateService.ServiceName, templateService.Revision)
		return nil
	}

	defaultRenderset, ok := defaultRenderMap[templateService.ProductName]
	if !ok {
		defaultRenderset, err = mongodb.NewRenderSetColl().Find(&mongodb.RenderSetFindOption{
			ProductTmpl: templateService.ProductName,
			Name:        templateService.ProductName,
			IsDefault:   true,
		})
		if err != nil {
			log.Errorf("failed to find default renderset: %s, err: %s", templateService.ProductName, err)
			//return nil
		}
	}
	valuesMap := make(map[string]string)
	if defaultRenderset != nil {
		defaultRenderMap[templateService.ProductName] = defaultRenderset
		for _, kv := range defaultRenderset.KVs {
			valuesMap[kv.Key] = kv.Value
		}
	}

	bs, err := yaml.Marshal(valuesMap)
	if err != nil {
		return fmt.Errorf("!!!!!!!!!!!! failed to marshal valuesMap for service: %s/%s:%d error: %s", templateService.ProductName, templateService.ServiceName, templateService.Revision, err)
	}

	templateService.VariableYaml = string(bs)

	log.Infof("unused service %s/%s:%d generatedKvMap yaml:\n%s", templateService.ProductName, templateService.ServiceName, templateService.Revision, string(bs))

	if dryRun {
		return nil
	}
	err = mongodb.NewServiceColl().UpdateServiceVariables(templateService)
	if err != nil {
		return fmt.Errorf("!!!!!!!!!!!! failed to update service variables for service: %s/%s:%d error: %s", templateService.ProductName, templateService.ServiceName, templateService.Revision, err)
	}
	return nil
}

func handleSingleService(yamlTemplate *models.YamlTemplate, serviceReference *template.ServiceReference) error {
	log.Infof("\n+++++++++++ handling service %s/%s +++++++++++", serviceReference.ProjectName, serviceReference.ServiceName)
	templateService, err := mongodb.NewServiceColl().Find(&mongodb.ServiceFindOption{
		ServiceName: serviceReference.ServiceName,
		Type:        "k8s",
		ProductName: serviceReference.ProjectName,
	})
	if err != nil {
		return fmt.Errorf("!!!!!!!!!! failed to find service %s/%s error: %s", serviceReference.ProjectName, serviceReference.ServiceName, err)
	}
	allServiceRevision[fmt.Sprintf("%s/%s", templateService.ProductName, templateService.ServiceName)].Delete(templateService.Revision)

	if templateService.TemplateID != yamlTemplate.ID.Hex() || templateService.Source != setting.ServiceSourceTemplate {
		log.Infof("skipped, service %s/%s:%d is not connected with template %s", templateService.ProductName, templateService.ServiceName, templateService.Revision, yamlTemplate.Name)
		return nil
	}

	if len(templateService.ServiceVars) != 0 {
		log.Infof("skipped, service %s/%s:%d has been upgraded", templateService.ProductName, templateService.ServiceName, templateService.Revision)
		return nil
	}

	creation := &models.CreateFromYamlTemplate{}
	bs, err := json.Marshal(templateService.CreateFrom)
	if err != nil {
		return fmt.Errorf("!!!!!!!!!! failed to marshal creation data for service: %s/%s:%d, err: %s", templateService.ProductName, templateService.ServiceName, templateService.Revision, err)
	}

	err = json.Unmarshal(bs, creation)
	if err != nil {
		return fmt.Errorf("!!!!!!!!!! failed to unmarshal creation data for service: %s/%s:%d, err: %s", templateService.ProductName, templateService.ServiceName, templateService.Revision, err)
	}

	// 检查服务的template id保存是否正确
	if creation.TemplateID != templateService.TemplateID {
		log.Errorf("!!!!!!!!!! service %s/%s:%d template id is not correct, expected: %s, actual: %s", templateService.ProductName, templateService.ServiceName, templateService.Revision, creation.TemplateID, templateService.TemplateID)
		return nil
	}

	defaultRenderset, ok := defaultRenderMap[templateService.ProductName]
	if !ok {
		defaultRenderset, err = mongodb.NewRenderSetColl().Find(&mongodb.RenderSetFindOption{
			ProductTmpl: templateService.ProductName,
			Name:        templateService.ProductName,
			IsDefault:   true,
		})
		if err != nil {
			log.Errorf("failed to find default renderset: %s, err: %s", templateService.ProductName, err)
			//return nil
		}
	}
	valuesMap := make(map[string]string)
	if defaultRenderset != nil {
		defaultRenderMap[templateService.ProductName] = defaultRenderset
		for _, kv := range defaultRenderset.KVs {
			valuesMap[kv.Key] = kv.Value
		}
	}

	if len(valuesMap) > 0 {
		newVariableYaml := creation.VariableYaml
		for k, v := range valuesMap {
			newVariableYaml = strings.ReplaceAll(newVariableYaml, fmt.Sprintf("{{.%s}}", k), v)
		}
		testMap := make(map[string]interface{})
		err = yaml.Unmarshal([]byte(newVariableYaml), &testMap)
		if err != nil {
			log.Errorf("failed to unmarshal variable yaml: %s, err: %s", newVariableYaml, err)
		} else {
			log.Infof("service %s/%s:d variable yaml:\n%s", templateService.ProductName, templateService.ServiceName, templateService.Revision, string(newVariableYaml))

			// 设置service的变量以及creation参数，以及变量可见性
			//creation.VariableYaml = newVariableYaml
			templateService.VariableYaml = newVariableYaml
			templateService.ServiceVars = yamlTemplate.ServiceVars

			if dryRun {
				return nil
			}
		}
	} else {
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
			return fmt.Errorf("!!!!!!!!! failed to get yaml variables for service: %s/%s:%d, err: %s", templateService.ProductName, templateService.ServiceName, templateService.Revision, err)
		}

		if outPutMessages {
			log.Infof("service: %s/%s:%d, creationYamlKVS: %+v", templateService.ProductName, templateService.ServiceName, templateService.Revision, creationYamlKVS)
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
			return fmt.Errorf("!!!!!!!!!! failed to unmarshal creation variable yaml for service: %s/%s:%d, err: %s", templateService.ProductName, templateService.ServiceName, templateService.Revision, err)
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
					log.Infof("setting complex variable %s for service %s/%s:%d", k, templateService.ProductName, templateService.ServiceName, templateService.Revision)
				}
				generatedKvMap[k] = v
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
			log.Infof("service %s/%s:%d total generatedKvMap: \n%+v", templateService.ProductName, templateService.ServiceName, templateService.Revision, generatedKvMap)
		}

		bs, err = yaml.Marshal(generatedKvMap)
		if err != nil {
			return fmt.Errorf("!!!!!!!!!! failed to marshal generatedKvMap for service: %s/%s:%d, err: %s", templateService.ProductName, templateService.ServiceName, templateService.Revision, err)
		}

		//if outPutMessages {
		//}
		log.Infof("service %s/%s:%d generatedKvMap yaml:\n%s", templateService.ProductName, templateService.ServiceName, templateService.Revision, string(bs))

		// 更新服务可见性
		templateService.ServiceVars = yamlTemplate.ServiceVars
		templateService.VariableYaml = string(bs)

		if dryRun {
			return nil
		}
	}

	// 更新服务版本，服务走一遍重新reload的逻辑
	err = service2.ReloadServiceFromYamlTemplateImpl(templateService.CreateBy, templateService.ProductName, yamlTemplate, templateService, templateService.VariableYaml)
	if err != nil {
		return fmt.Errorf("!!!!!!!!!! failed to reload service: %s/%s, err: %s", templateService.ProductName, templateService.ServiceName, err)
	}

	return nil
}

func handleUsedOldVersionServices() error {
	for key, revisions := range allServiceRevision {
		if revisions.Len() == 0 {
			continue
		}
		log.Infof("----------- service still need be processed: %s, revisions: %+v", key, revisions.List())
		for _, revision := range revisions.List() {
			splitNames := strings.Split(key, "/")
			projectName := splitNames[0]
			serviceName := splitNames[1]
			err := handleUsedService(projectName, serviceName, revision)
			if err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}
