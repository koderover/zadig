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

	"gorm.io/gorm/utils"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"

	template3 "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"

	"gopkg.in/yaml.v3"

	"k8s.io/utils/strings/slices"

	template2 "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/render"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/types"
	util2 "github.com/koderover/zadig/pkg/util"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

var k8sProjects []*template2.Product
var k8sTemplates []*models.YamlTemplate
var defaultRenderMap = make(map[string]*models.RenderSet)

var templateMap = make(map[string]*models.YamlTemplate)

var allTestServiceRevisions map[string]sets.Int64

// all test services map[productName/serviceName/revision] => service
var allTestServices map[string]*models.Service

// 准备数据，查出所有的项目
// 查出所有被使用的服务信息
// 查出所有项目的默认 renderset信息
func prepareData() error {

	var err error
	k8sProjects, err = template.NewProductColl().ListWithOption(&template.ProductListOpt{
		DeployType:    setting.K8SDeployType,
		BasicFacility: setting.BasicFacilityK8S,
	})
	if err != nil {
		return errors.Wrapf(err, "list k8s projects")
	}

	log.Infof("------------ %d k8s projects found", len(k8sProjects))

	allTestServiceRevisions = make(map[string]sets.Int64)
	allTestServices = make(map[string]*models.Service)

	insertSvcToMap := func(productName, serviceName string, serviceRevision int64) {
		svcKey := fmt.Sprintf("%s/%s", productName, serviceName)
		if _, ok := allTestServiceRevisions[svcKey]; !ok {
			allTestServiceRevisions[svcKey] = sets.NewInt64()
		}
		allTestServiceRevisions[svcKey].Insert(serviceRevision)
	}

	// fetch all related test services
	for _, project := range k8sProjects {
		testSvcs, err := repository.GetMaxRevisionsServicesMap(project.ProductName, false)
		if err != nil {
			return errors.Wrapf(err, "get test services, project name: %s", project.ProductName)
		}
		for _, svc := range testSvcs {
			insertSvcToMap(svc.ProductName, svc.ServiceName, svc.Revision)
		}

		envs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{
			Name:       project.ProductName,
			Production: util2.GetBoolPointer(false),
		})
		if err != nil {
			return errors.Wrapf(err, "!!!!!!!!!!!! get envs, project name: %s", project.ProductName)
		}
		for _, product := range envs {
			for _, service := range product.GetServiceMap() {
				insertSvcToMap(service.ProductName, service.ServiceName, service.Revision)
			}
		}

		defaultRenderset, ok := defaultRenderMap[project.ProductName]
		if !ok {
			defaultRenderset, err = mongodb.NewRenderSetColl().Find(&mongodb.RenderSetFindOption{
				ProductTmpl: project.ProductName,
				Name:        project.ProductName,
				IsDefault:   true,
			})
			if err != nil {
				return fmt.Errorf("!!!!!!!!!! failed to find default renderset: %s, err: %s", project.ProductName, err)
			}
		}
		defaultRenderMap[project.ProductName] = defaultRenderset
	}

	k8sTemplates, _, err = mongodb.NewYamlTemplateColl().List(0, 0)
	if err != nil {
		return errors.Wrap(err, "list yaml templates")
	}

	return nil
}

func handleAllTestServices() error {
	for svcKey, revisions := range allTestServiceRevisions {
		names := strings.Split(svcKey, "/")
		if len(names) != 2 {
			return fmt.Errorf("invalid svc key: %s", svcKey)
		}
		projectName, serviceName := names[0], names[1]
		for _, revision := range revisions.List() {
			tmpSvc, err := repository.QueryTemplateService(&mongodb.ServiceFindOption{
				ProductName: projectName,
				ServiceName: serviceName,
				Revision:    revision,
			}, false)
			if err != nil {
				return errors.Wrapf(err, "failed to query service: %s/%d", svcKey, revision)
			}
			allTestServices[fmt.Sprintf("%s/%s/%d", tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision)] = tmpSvc

			err = handleSingleTemplateService(tmpSvc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getValueFromKVListByKey(key string, kvs map[string]*types.ServiceVariableKV) interface{} {
	if kv, ok := kvs[key]; ok {
		return kv.Value
	}
	return nil
}

// handle single service; build service variables
func handleSingleTemplateService(tmpSvc *models.Service) error {
	if len(tmpSvc.ServiceVariableKVs) > 0 {
		return nil
	}

	log.Infof("----------- handling service variable, service: %s/%s:%d", tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision)

	creation := &models.CreateFromYamlTemplate{}
	bs, err := json.Marshal(tmpSvc.CreateFrom)
	if err != nil {
		return fmt.Errorf("!!!!!!!!!! failed to marshal creation data for service: %s/%s:%d, err: %s", tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision, err)
	}

	err = json.Unmarshal(bs, creation)
	if err != nil {
		return fmt.Errorf("!!!!!!!!!! failed to unmarshal creation data for service: %s/%s:%d, err: %s", tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision, err)
	}

	defaultRenderset := defaultRenderMap[tmpSvc.ProductName]
	if defaultRenderset == nil {
		return fmt.Errorf("!!!!!!!! failed to find target default render: %s", tmpSvc.ProductName)
	}

	usedGlobalVars := make([]*types.ServiceVariableKV, 0)
	// 1. 替换全局变量；保证creation中的yaml为合法yaml
	// 2. 添加全局变量的，生成完全体的变量值
	replacedYaml := creation.VariableYaml
	log.Infof("--------- replaced yaml: %s", replacedYaml)
	for _, kv := range defaultRenderset.KVs {
		replacedYaml = strings.ReplaceAll(replacedYaml, kv.Alias, kv.Value)
		if slices.Contains(kv.Services, tmpSvc.ServiceName) {
			usedGlobalVars = append(usedGlobalVars, &types.ServiceVariableKV{
				Key:   kv.Key,
				Value: kv.Value,
				Type:  types.ServiceVariableKVTypeString,
			})
		}
	}
	log.Infof("--------- replaced yaml after merge: %s", replacedYaml)

	// 保证variable yaml 中没有残留的template变量
	kvs, err := template3.GetYamlVariables(replacedYaml, log.SugaredLogger())
	if err != nil {
		return fmt.Errorf("!!!!!!!! failed to get variable from yaml for ensure: %s, err: %s", replacedYaml, err)
	}
	// parse go template variables
	for _, kv := range kvs {
		newValue := ""
		if kv.Key == "LOGBACK_CONFIGMAP" {
			newValue = "lotus-logback-cm"
		}
		replacedYaml = strings.ReplaceAll(replacedYaml, fmt.Sprintf("{{.%s}}", kv.Key), newValue)
		log.Infof("-------------- 数据兼容， service: %s/%s/%d, key: %s, value: %s", tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision, kv.Key, newValue)
	}

	log.Infof("--------- replaced yaml after back merge: %s", replacedYaml)

	// 使用当前的creation参数，生成新版本的KV数据
	serviceKVs, err := types.YamlToServiceVariableKV(replacedYaml, nil)
	if err != nil {
		return errors.Wrapf(err, "!!!!!!!!!! failed to convert yaml to service variable kv, replacedYaml: %s, service name: %s/%s/%d", replacedYaml, tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision)
	}
	svcKVMap := make(map[string]*types.ServiceVariableKV)
	for _, kv := range serviceKVs {
		svcKVMap[kv.Key] = kv
	}
	// 增加全局变量的KV 配置
	for _, kv := range usedGlobalVars {
		svcKVMap[kv.Key] = kv
	}

	for _, kv := range svcKVMap {
		tmpSvc.ServiceVariableKVs = append(tmpSvc.ServiceVariableKVs, kv)
	}
	variableYaml, err := types.ServiceVariableKVToYaml(tmpSvc.ServiceVariableKVs)
	if err != nil {
		return errors.Wrapf(err, "!!!!!!!!!!!!!! failed to generate yaml from service variables, service: %s/%s/%d", tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision)
	}
	tmpSvc.VariableYaml = variableYaml

	if outPutMessages {
		log.Infof("----------- service: %s/%s:%d, variable: \n%s", tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision, tmpSvc.VariableYaml)
	}

	if !write {
		return nil
	}
	err = repository.UpdateServiceVariables(tmpSvc, false)
	if err != nil {
		return errors.Wrapf(err, "!!!!!!!!!! failed to update service variables, service name: %s/%s/%d", tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision)
	}

	return nil
}

// 从1.15.0 的renderset中，根据原始KV值，生成全局变量的KV值
func generateGlobalVariableKVSFromOriginalKV(renderset *models.RenderSet) ([]*types.ServiceVariableKV, error) {
	variableKVs := make([]*types.ServiceVariableKV, 0)
	// 将当前显示的全局变量转为项目内全局变量
	for _, kv := range renderset.KVs {
		variableKVs = append(variableKVs, &types.ServiceVariableKV{
			Key:   kv.Key,
			Value: kv.Value,
			Type:  types.ServiceVariableKVTypeString,
		})
	}

	yamlStr, err := types.ServiceVariableKVToYaml(variableKVs)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get yaml from service kvs, rendersdet: %s/%v", renderset.Name, renderset.Revision)
	}
	yamlMap := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(yamlStr), &yamlMap)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to ")
	}

	svcKVMap := make(map[string]*types.ServiceVariableKV)
	for _, kv := range variableKVs {
		svcKVMap[kv.Key] = kv
	}

	//处理特殊的全局变量:
	// TOLERATIONS_KEY + TOLERATIONS_VALUE => TOLERATIONS
	// NODE_AFFINITY_KEY + NODE_AFFINITY_VALUE => NODE_AFFINITY
	handledSpecialKeys := sets.NewString()
	for _, kv := range svcKVMap {
		if handledSpecialKeys.Has(kv.Key) {
			continue
		}
		if kv.Key == "NODE_AFFINITY_KEY" || kv.Key == "NODE_AFFINITY_VALUE" {
			handledSpecialKeys.Insert("NODE_AFFINITY_KEY", "NODE_AFFINITY_VALUE")
			yamlMap["NODE_AFFINITY"] = []map[string]interface{}{
				{"KEY": getValueFromKVListByKey("NODE_AFFINITY_KEY", svcKVMap), "VALUES": []string{getValueFromKVListByKey("NODE_AFFINITY_VALUE", svcKVMap).(string)}},
			}
		} else if kv.Key == "TOLERATION_KEY" || kv.Key == "TOLERATION_VALUE" {
			handledSpecialKeys.Insert("TOLERATION_KEY", "TOLERATION_VALUE")
			yamlMap["TOLERATIONS"] = []map[string]interface{}{
				{"KEY": getValueFromKVListByKey("TOLERATION_KEY", svcKVMap), "VALUE": getValueFromKVListByKey("TOLERATION_VALUE", svcKVMap)},
			}
		}
	}
	yamlStrBS, err := yaml.Marshal(yamlMap)
	if err != nil {
		return nil, errors.Wrapf(err, "!!!!!!!!!! failed to marshal yaml map")
	}

	variableKVs, err = types.YamlToServiceVariableKV(string(yamlStrBS), nil)
	return variableKVs, nil
}

// 生成项目中的全局变量信息
func handleProjectGlobalVariables() error {
	for _, project := range k8sProjects {
		defaultRenderset, ok := defaultRenderMap[project.ProductName]
		if !ok {
			return fmt.Errorf("!!!!!!! failed to find ")
		}

		if len(project.GlobalVariables) > 0 {
			continue
		}

		var err error
		project.GlobalVariables, err = generateGlobalVariableKVSFromOriginalKV(defaultRenderset)
		if err != nil {
			return errors.Wrapf(err, "!!!!!!!!!!!!! ailed to generate variable kvs from original kv, project: %s", project.ProductName)
		}

		if outPutMessages {
			log.Infof("----------- project: %s, global variables count: %d", project.ProductName, len(project.GlobalVariables))
			for i, kv := range project.GlobalVariables {
				log.Infof("----------- project: %s, global variable[%d/%d], key: %s, type: %v, value: %v", project.ProductName, i+1, len(project.GlobalVariables), kv.Key, kv.Type, kv.Value)
			}
		}

		if !write {
			continue
		}

		err = template.NewProductColl().UpdateGlobalVars(project.ProductName, project.GlobalVariables)
		if err != nil {
			return errors.Wrapf(err, "!!!!!!!!!! failed to update product global variables, product name: %s", project.ProductName)
		}
	}

	return nil
}

// 转换环境全局变量，将原本KV转为新版本KV
func handleK8sEnvGlobalVariables() error {
	for _, project := range k8sProjects {
		envs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{
			Name:       project.ProductName,
			Production: util2.GetBoolPointer(false),
		})
		if err != nil {
			return errors.Wrapf(err, "!!!!!!!!!!!! get envs, project name: %s", project.ProductName)
		}
		for _, product := range envs {
			product.EnsureRenderInfo()

			targetRevision := product.Render.Revision
			for _, productSvc := range product.GetServiceMap() {
				if productSvc.Render != nil && productSvc.Render.Revision > targetRevision {
					targetRevision = productSvc.Render.Revision
				}
			}

			if targetRevision != product.Render.Revision {
				log.Infof("revision not match, product: %s/%s, target revision: %d, current revision: %d", product.ProductName, product.EnvName, targetRevision, product.Render.Revision)
			}

			targetRenderset, err := mongodb.NewRenderSetColl().Find(&mongodb.RenderSetFindOption{
				ProductTmpl: project.ProductName,
				Name:        product.Render.Name,
				Revision:    targetRevision,
				IsDefault:   false,
			})
			if err != nil {
				return fmt.Errorf("!!!!!!!!!! failed to find product renderset: %s/%s, err: %s", product.ProductName, product.EnvName, err)
			}

			if len(targetRenderset.GlobalVariables) > 0 {
				continue
			}

			log.Infof("------------- start to handle env global variables, %s/%s", project.ProductName, product.EnvName)

			// 当前的服务关联关系 map[KEY]=>[]SERVICES
			svcRelations := make(map[string]sets.String)
			for _, kv := range targetRenderset.KVs {
				svcRelations[kv.Key] = sets.NewString(kv.Services...)
			}

			globalVariableKV := make([]*types.GlobalVariableKV, 0)
			serviceVariables, err := generateGlobalVariableKVSFromOriginalKV(targetRenderset)
			if err != nil {
				return errors.Wrapf(err, "!!!!!!!!!!!!! ailed to generate variable kvs from original kv, project: %s", project.ProductName)
			}

			// 将全局变量的KV插入到服务变量的内容中，设置为使用全局变量
			insertServiceUsedVariable := func(svcName string, svcKv *types.ServiceVariableKV, renderSet *models.RenderSet) error {
				var targetSvcRender *template2.ServiceRender
				for _, kv := range renderSet.ServiceVariables {
					if kv.ServiceName == svcName {
						targetSvcRender = kv
						break
					}
				}
				if targetSvcRender == nil {
					targetSvcRender = &template2.ServiceRender{
						ServiceName: svcName,
						OverrideYaml: &template2.CustomYaml{
							YamlContent:       "",
							RenderVaraibleKVs: nil,
						},
					}
					renderSet.ServiceVariables = append(renderSet.ServiceVariables, targetSvcRender)
				}

				targetSvcRender.OverrideYaml.RenderVaraibleKVs = append(targetSvcRender.OverrideYaml.RenderVaraibleKVs, &types.RenderVariableKV{
					ServiceVariableKV: *svcKv,
					UseGlobalVariable: true,
				})
				targetSvcRender.OverrideYaml.YamlContent, err = types.RenderVariableKVToYaml(targetSvcRender.OverrideYaml.RenderVaraibleKVs)
				return errors.Wrapf(err, "!!!!!!!!!! failed to render variable kvs to yaml, service name: %s", svcName)
			}

			// 继承原先的变量与服务关联关系
			for _, svcKV := range serviceVariables {
				var relatedSvcs []string
				if svcSets, ok := svcRelations[svcKV.Key]; ok {
					relatedSvcs = svcSets.List()
				}
				globalVariableKV = append(globalVariableKV, &types.GlobalVariableKV{
					ServiceVariableKV: *svcKV,
					RelatedServices:   relatedSvcs,
				})

				// 1.15.0中没有服务变量，仅需将全局变量写入服务render
				for _, svc := range relatedSvcs {
					err = insertServiceUsedVariable(svc, svcKV, targetRenderset)
					if err != nil {
						return err
					}
				}
			}
			targetRenderset.GlobalVariables = globalVariableKV

			if outPutMessages {
				log.Infof("--------------- env: %s/%s, global variables count: %v", project.ProductName, product.EnvName, len(globalVariableKV))
				for i, variable := range globalVariableKV {
					log.Infof("++++++++++++ env: %s/%s, global variable [%d/%d] key: %s, value: %v, related services: %v", project.ProductName, product.EnvName, i+1, len(globalVariableKV), variable.Key, variable.Value, variable.RelatedServices)
				}
			}

			if !write {
				continue
			}

			// create new render info
			err = render.CreateRenderSet(targetRenderset, log.SugaredLogger())
			if err != nil {
				return errors.Wrapf(err, "failed to create renderset")
			}
			product.Render.Name = targetRenderset.Name
			product.Render.Revision = targetRenderset.Revision

			err = mongodb.NewProductColl().UpdateRender(product.EnvName, product.ProductName, product.Render)
			if err != nil {
				return errors.Wrapf(err, "failed to update product info: %s/%s", product.ProductName, product.EnvName)
			}
			if product.ServiceDeployStrategy == nil {
				product.ServiceDeployStrategy = make(map[string]string)
				err = mongodb.NewProductColl().UpdateDeployStrategy(product.EnvName, product.ProductName, product.ServiceDeployStrategy)
				if err != nil {
					return errors.Wrapf(err, "failed to update product deploy info: %s/%s", product.ProductName, product.EnvName)
				}
			}
		}
	}

	return nil
}

// 处理k8s模板，将变量的变量定义移除，将变量的KV转为新版本KV
func handleTemplates() error {
	for _, yamlTemplate := range k8sTemplates {
		if len(yamlTemplate.ServiceVariableKVs) > 0 {
			continue
		}

		log.Infof("----------------- handling k8s template: %s", yamlTemplate.Name)

		// 提取出变量的变量 {{.IMAGE_PULL_SECRETS}} 等值
		kvs, err := template3.GetYamlVariables(yamlTemplate.VariableYaml, log.SugaredLogger())
		if err != nil {
			return errors.Wrapf(err, "!!!!!!!!!!!! failed to get variable from yaml: %s", yamlTemplate.VariableYaml)
		}
		// 将变量的变量替换为空字符串
		for _, kv := range kvs {
			yamlTemplate.VariableYaml = strings.ReplaceAll(yamlTemplate.VariableYaml, fmt.Sprintf("{{.%s}}", kv.Key), "")
		}

		// 生成模板默认变量的KV值
		serviceKVs, err := types.YamlToServiceVariableKV(yamlTemplate.VariableYaml, nil)
		if err != nil {
			return errors.Wrapf(err, "!!!!!!!!!!!! failed to convert yaml to service variable kv, template name: %s", yamlTemplate.Name)
		}
		yamlTemplate.ServiceVariableKVs = serviceKVs

		if outPutMessages {
			log.Infof("--------------- template: %s, service variable count: %v, variable yaml: %s", yamlTemplate.Name, len(serviceKVs), yamlTemplate.VariableYaml)
		}

		if !write {
			continue
		}

		err = mongodb.NewYamlTemplateColl().UpdateVariable(yamlTemplate.ID.Hex(), yamlTemplate.VariableYaml, yamlTemplate.ServiceVariableKVs)
		if err != nil {
			return errors.Wrapf(err, "!!!!!!!!!!! failed to update yaml template, template name: %s", yamlTemplate.Name)
		}
	}
	return nil
}

var SpecialKeys []string = []string{"NODE_AFFINITY_KEY", "NODE_AFFINITY_VALUE", "TOLERATION_KEY", "TOLERATION_VALUE"}

// 对于从模板创建并强管理的服务，自动同步，服务版本 + 1，但是不应用到环境
func syncTemplateSvcs() error {
	templateMap = make(map[string]*models.YamlTemplate)
	for _, template := range k8sTemplates {
		templateMap[template.ID.Hex()] = template
	}

	for _, project := range k8sProjects {
		maxRevisionServices, err := repository.ListMaxRevisionsServices(project.ProductName, false)
		if err != nil {
			return errors.Wrapf(err, "!!!!!!!!!!!! failed to list max revision services, project: %s", project.ProductName)
		}
		for _, tmpSvc := range maxRevisionServices {
			if tmpSvc.Source != "template" {
				continue
			}

			// 确保templateID正确，service.templateID 和 creation.TemplateID需要一致
			creation := &models.CreateFromYamlTemplate{}
			bs, err := json.Marshal(tmpSvc.CreateFrom)
			if err != nil {
				return fmt.Errorf("!!!!!!!!!! failed to marshal creation data for service: %s/%s:%d, err: %s", tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision, err)
			}

			err = json.Unmarshal(bs, creation)
			if err != nil {
				return fmt.Errorf("!!!!!!!!!! failed to unmarshal creation data for service: %s/%s:%d, err: %s", tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision, err)
			}

			if creation.TemplateID != tmpSvc.TemplateID {
				log.Errorf("!!!!!!!!!!!!!! template id not equal, service: %s/%s:%d, template id: %s, creation template id: %s", tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision, tmpSvc.TemplateID, creation.TemplateID)
				continue
			}
			if !tmpSvc.AutoSync {
				log.Infof("+++++++++++++++ template service: %s/%s not configured to auto sync, need sync manually!", project.ProductName, tmpSvc.ServiceName)
				continue
			}

			log.Infof("-------------- syncing template service: %s/%s", project.ProductName, tmpSvc.ServiceName)

			yamlTemplate := templateMap[tmpSvc.TemplateID]
			if yamlTemplate == nil {
				return fmt.Errorf("!!!!!!!!!! failed to find template: %s", tmpSvc.TemplateID)
			}

			if !write {
				continue
			}

			cleanVariables := make([]*types.ServiceVariableKV, 0)
			for _, kv := range tmpSvc.ServiceVariableKVs {
				if utils.Contains(SpecialKeys, kv.Key) {
					continue
				}
				cleanVariables = append(cleanVariables, kv)
			}
			tmpSvc.ServiceVariableKVs = cleanVariables

			// 重新从模板reload服务
			err = service.ReloadServiceFromYamlTemplateWithMerge(tmpSvc.CreateBy, tmpSvc.ProductName, yamlTemplate, tmpSvc)
			if err != nil {
				return errors.Wrapf(err, "!!!!!!!!!! failed to reload service: %s/%s", tmpSvc.ProductName, tmpSvc.ServiceName)
			}
		}
	}
	return nil
}
