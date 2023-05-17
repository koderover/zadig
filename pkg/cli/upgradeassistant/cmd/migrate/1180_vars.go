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

package migrate

import (
	"fmt"
	"strings"

	template2 "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"

	util2 "github.com/koderover/zadig/pkg/util"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/render"
	"github.com/koderover/zadig/pkg/tool/log"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/repository"
	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"

	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/pkg/setting"
)

// GetAffectedServices fetch affected services
// key => services
func GenerateEnvVariableAffectServices(productInfo *models.Product, renderset *models.RenderSet) (map[string]sets.String, map[string]interface{}, error) {
	productServiceRevisions, err := commonservice.GetProductUsedTemplateSvcs(productInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find revision services, err: %s", err)
	}

	variableMap := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(renderset.DefaultValues), &variableMap)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to marshal variables for renderset: %s/%s/%d", productInfo.ProductName, renderset.Name, renderset.Revision)
	}

	envVariableKeys := sets.NewString()
	for key := range variableMap {
		envVariableKeys.Insert(key)
	}

	relation := make(map[string]sets.String)

	insertSvcRelation := func(rootKey, svcName string) {
		if _, ok := relation[rootKey]; !ok {
			relation[rootKey] = sets.NewString()
		}
		relation[rootKey].Insert(svcName)
	}

	for _, productRevisionSvc := range productServiceRevisions {
		if len(productRevisionSvc.VariableYaml) == 0 {
			continue
		}

		// total root keys
		totalRootKeys := sets.NewString()
		for _, kv := range productRevisionSvc.ServiceVariableKVs {
			totalRootKeys.Insert(kv.Key)
		}

		// remove service root keys
		for _, kv := range productRevisionSvc.ServiceVars {
			totalRootKeys.Delete(util.ExtractRootKeyFromFlat(kv))
		}

		// matched global keys in environment
		for _, gk := range totalRootKeys.List() {
			if envVariableKeys.Has(gk) {
				insertSvcRelation(gk, productRevisionSvc.ServiceName)
			}
		}
	}
	return relation, nil, nil
}

func MaxRevision(revisionList []int64) int64 {
	retValue := int64(-1)
	for _, r := range revisionList {
		if r > retValue {
			retValue = r
		}
	}
	return retValue
}

var k8sProjects []*template2.Product
var allTestServiceRevisions map[string]sets.Int64
var allProductionServiceMap map[string]sets.Int64

// all test services map[productName/serviceName/revision] => service
var allTestServices map[string]*models.Service

// all product services map[productName/serviceName/revision] => service
var allProductionServices map[string]*models.Service

// all global kvs defined in services
// map[productName][VariableKey] -> types.ServiceVariableKV
var globalSvcKVsByProject map[string]map[string]*types.ServiceVariableKV

func migrateVariables() error {
	var err error

	// 1. turn variable yaml of template to service variable kv
	if err = migrateTemplateVariables(); err != nil {
		return err
	}

	k8sProjects, err = template.NewProductColl().ListWithOption(&template.ProductListOpt{
		DeployType:    setting.K8SDeployType,
		BasicFacility: setting.BasicFacilityK8S,
	})
	if err != nil {
		return errors.Wrapf(err, "list k8s projects")
	}

	allTestServiceRevisions = make(map[string]sets.Int64)
	allTestServices = make(map[string]*models.Service)

	// 2. change ServiceVars to ServiceVariableKVs for test services
	if err = migrateTemplateServiceVariables(); err != nil {
		return err
	}

	// 3. set default global vars for test services by project
	if err = generateGlobalVariables(); err != nil {
		return err
	}

	// 4. calculate env global variable <-> services relations and transfer global vars to KV format
	// 5. change env variable(global variable and service variable) to KV format from yaml format and set `UseGlobalVariable`
	if err = migrateTestProductVariables(); err != nil {
		return err
	}

	// 6. change service vars to KVS for production services
	if err = migrateProductionService(); err != nil {
		return err
	}

	// 7. change service variable to KV format from yaml for production envs
	if err = migrateProductionProductVariables(); err != nil {
		return err
	}
	return nil
}

func migrateTemplateVariables() error {
	k8sTemplates, _, err := mongodb.NewYamlTemplateColl().List(0, 0)
	if err != nil {
		return errors.Wrap(err, "list yaml templates")
	}

	for _, yamlTemplate := range k8sTemplates {
		if len(yamlTemplate.ServiceVariableKVs) > 0 {
			continue
		}

		if len(yamlTemplate.VariableYaml) == 0 {
			continue
		}
		serviceKVs, err := types.YamlToServiceVariableKV(yamlTemplate.VariableYaml, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to convert yaml to service variable kv, template name: %s", yamlTemplate.Name)
		}
		yamlTemplate.ServiceVariableKVs = serviceKVs
		err = mongodb.NewYamlTemplateColl().UpdateVariable(yamlTemplate.ID.Hex(), yamlTemplate.VariableYaml, yamlTemplate.ServiceVariableKVs)
		if err != nil {
			return errors.Wrapf(err, "failed to update yaml template, template name: %s", yamlTemplate.Name)
		}
	}
	return nil
}

func migrateTemplateServiceVariables() error {

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
			return errors.Wrapf(err, "get envs, project name: %s", project.ProductName)
		}
		for _, product := range envs {
			for _, service := range product.GetServiceMap() {
				insertSvcToMap(service.ProductName, service.ServiceName, service.Revision)
			}
		}
	}

	globalSvcKVsByProject = make(map[string]map[string]*types.ServiceVariableKV)
	insertGlobalKVS := func(productName string, kv *types.ServiceVariableKV) {
		if _, ok := globalSvcKVsByProject[productName]; !ok {
			globalSvcKVsByProject[productName] = make(map[string]*types.ServiceVariableKV)
		}
		if _, ok := globalSvcKVsByProject[productName][kv.Key]; !ok {
			globalSvcKVsByProject[productName][kv.Key] = kv
		}
	}

	for svcKey, revisions := range allTestServiceRevisions {
		names := strings.Split(svcKey, "/")
		if len(names) != 2 {
			return fmt.Errorf("invalid svc key: %s", svcKey)
		}
		projectName, serviceName := names[0], names[1]
		for _, revision := range revisions.List() {
			maxRevision := MaxRevision(revisions.List())

			tmpSvc, err := repository.QueryTemplateService(&mongodb.ServiceFindOption{
				ProductName: projectName,
				ServiceName: serviceName,
				Revision:    revision,
			}, false)
			if err != nil {
				return errors.Wrapf(err, "failed to query service: %s/%d", svcKey, revision)
			}
			allTestServices[fmt.Sprintf("%s/%s/%d", tmpSvc.ProductName, tmpSvc.ServiceName, tmpSvc.Revision)] = tmpSvc

			if len(tmpSvc.ServiceVariableKVs) > 0 || len(tmpSvc.VariableYaml) == 0 {
				continue
			}

			serviceKVs, err := types.YamlToServiceVariableKV(tmpSvc.VariableYaml, nil)
			if err != nil {
				return errors.Wrapf(err, "failed to convert yaml to service variable kv, service name: %s/%d", svcKey, revision)
			}

			// calculate global vars from services with the latest revisions
			if maxRevision == revision {
				totalRootKeys := sets.NewString()
				// all root keys
				for _, kv := range serviceKVs {
					totalRootKeys.Insert(kv.Key)
				}

				// remove service root keys
				for _, flatSvcKey := range tmpSvc.ServiceVars {
					totalRootKeys.Delete(util.ExtractRootKeyFromFlat(flatSvcKey))
				}

				// insert global kvs
				for _, kv := range serviceKVs {
					if totalRootKeys.Has(kv.Key) {
						insertGlobalKVS(projectName, kv)
					}
				}
			}

			tmpSvc.ServiceVariableKVs = serviceKVs
			err = repository.UpdateServiceVariables(tmpSvc, false)
			if err != nil {
				return errors.Wrapf(err, "failed to update service variables, service name: %s/%d", svcKey, revision)
			}
		}
	}

	return nil
}

func generateGlobalVariables() error {
	for _, project := range k8sProjects {
		globalVars, ok := globalSvcKVsByProject[project.ProductName]
		if !ok {
			log.Infof("no global vars for project: %s", project.ProductName)
			continue
		}

		if len(project.GlobalVariables) > 0 {
			continue
		}

		for _, kv := range globalVars {
			project.GlobalVariables = append(project.GlobalVariables, kv)
		}
		err := template.NewProductColl().UpdateGlobalVars(project.ProductName, project.GlobalVariables)
		if err != nil {
			return errors.Wrapf(err, "failed to update global vars for project: %s", project.ProductName)
		}

	}
	return nil
}

func migrateTestProductVariables() error {
	for _, project := range k8sProjects {
		envs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{
			Name:       project.ProductName,
			Production: util2.GetBoolPointer(false),
		})
		if err != nil {
			return errors.Wrapf(err, "get envs, project name: %s", project.ProductName)
		}

		for _, product := range envs {
			if product.Production {
				continue
			}
			product.EnsureRenderInfo()
			renderInfo, exists, err := mongodb.NewRenderSetColl().FindRenderSet(&mongodb.RenderSetFindOption{
				ProductTmpl: product.ProductName,
				Name:        product.Render.Name,
				Revision:    product.Render.Revision,
				IsDefault:   false,
			})
			if err != nil {
				return errors.Wrapf(err, "get render info, product name: %s, render name: %s, revision: %d", product.ProductName, product.Render.Name, product.Render.Revision)
			}
			if !exists {
				return fmt.Errorf("render set not found, product name: %s, render name: %s, revision: %d", product.ProductName, product.Render.Name, product.Render.Revision)
			}

			// change global variable yaml to kvs
			relations := make(map[string]sets.String)
			if len(renderInfo.DefaultValues) > 0 {
				// calculate variable yaml and service relations by keys
				// fill renderset.GlobalVariables
				var kvs map[string]interface{}
				relations, kvs, err = GenerateEnvVariableAffectServices(product, renderInfo)
				if err != nil {
					return errors.Wrapf(err, "failed to generate env variable affected services")
				}
				for k := range relations {
					var relatedSvcs []string = nil
					if svcSets, ok := relations[k]; ok {
						relatedSvcs = svcSets.List()
					}
					gvKV := &types.GlobalVariableKV{
						ServiceVariableKV: types.ServiceVariableKV{Key: k, Value: kvs[k]},
						RelatedServices:   relatedSvcs,
					}
					renderInfo.GlobalVariables = append(renderInfo.GlobalVariables, gvKV)
				}
			}

			// change service variable yaml to kvs
			// full service vars will be stored in renderset
			// UseGlobalVariable will be set correctly
			if len(renderInfo.ServiceVariables) > 0 {
				for _, svcKV := range renderInfo.ServiceVariables {
					if svcKV.OverrideYaml == nil || len(svcKV.OverrideYaml.YamlContent) == 0 || len(svcKV.OverrideYaml.RenderVaraibleKVs) > 0 {
						continue
					}

					productSvc := product.GetServiceMap()[svcKV.ServiceName]
					if productSvc == nil {
						continue
					}

					svcKvs, err := types.YamlToServiceVariableKV(svcKV.OverrideYaml.YamlContent, nil)
					if err != nil {
						return errors.Wrapf(err, "failed to convert service variable yaml to kv, service name: %s/%s/%s", product.ProductName, product.EnvName, svcKV.ServiceName)
					}

					svcVarKeys := sets.NewString()
					for _, sKV := range svcKvs {
						svcKV.OverrideYaml.RenderVaraibleKVs = append(svcKV.OverrideYaml.RenderVaraibleKVs, &types.RenderVariableKV{
							ServiceVariableKV: *sKV,
							UseGlobalVariable: false,
						})
						svcVarKeys.Insert(sKV.Key)
					}

					svcTemplate, ok := allTestServices[fmt.Sprintf("%s/%s/%d", product.ProductName, svcKV.ServiceName, productSvc.Revision)]
					if !ok {
						log.Warnf("failed to find source template service: %s", fmt.Sprintf("%s/%s/%d", product.ProductName, svcKV.ServiceName, productSvc.Revision))
						continue
					}

					// make sure all service keys exist in renderset.ServiceVariables.RenderVariableKVs
					for _, templateVars := range svcTemplate.ServiceVariableKVs {
						if svcVarKeys.Has(templateVars.Key) {
							continue
						}
						svcKV.OverrideYaml.RenderVaraibleKVs = append(svcKV.OverrideYaml.RenderVaraibleKVs, &types.RenderVariableKV{
							ServiceVariableKV: *templateVars,
							UseGlobalVariable: false,
						})
					}

					// set to use global variable if global variable is set
					for _, svcKV := range svcKV.OverrideYaml.RenderVaraibleKVs {
						if svcSet, ok := relations[svcKV.Key]; ok && svcSet.Has(svcKV.Key) {
							svcKV.UseGlobalVariable = true
						}
					}
				}
			}

			// create new render info
			err = render.CreateRenderSet(renderInfo, log.SugaredLogger())
			if err != nil {
				return errors.Wrapf(err, "failed to create renderset")
			}
			product.Render.Name = renderInfo.Name
			product.Render.Revision = renderInfo.Revision

			err = mongodb.NewProductColl().UpdateRender(product.EnvName, product.ProductName, product.Render)
			if err != nil {
				return errors.Wrapf(err, "failed to update product info: %s/%s", product.ProductName, product.EnvName)
			}
		}
	}
	return nil
}

func migrateProductionService() error {

	allProductionServiceMap = make(map[string]sets.Int64)
	insertProductionSvcToMap := func(productName, serviceName string, serviceRevision int64) {
		svcKey := fmt.Sprintf("%s/%s", productName, serviceName)
		if _, ok := allProductionServiceMap[svcKey]; !ok {
			allProductionServiceMap[svcKey] = sets.NewInt64()
		}
		allProductionServiceMap[svcKey].Insert(serviceRevision)
	}
	// all product services map[productName/serviceName/revision] => service
	allProductionServices = make(map[string]*models.Service)

	// fetch all related production services
	for _, project := range k8sProjects {
		productionSvcs, err := repository.GetMaxRevisionsServicesMap(project.ProductName, true)
		if err != nil {
			return errors.Wrapf(err, "get test services, project name: %s", project.ProductName)
		}
		for _, svc := range productionSvcs {
			insertProductionSvcToMap(svc.ProductName, svc.ServiceName, svc.Revision)
		}

		envs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{
			Name:       project.ProductName,
			Production: util2.GetBoolPointer(true),
		})
		if err != nil {
			return errors.Wrapf(err, "get envs, project name: %s", project.ProductName)
		}
		for _, product := range envs {
			for _, service := range product.GetServiceMap() {
				insertProductionSvcToMap(service.ProductName, service.ServiceName, service.Revision)
			}
		}
	}

	for svcKey, revisions := range allProductionServiceMap {
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
			}, true)
			if err != nil {
				return errors.Wrapf(err, "failed to query service: %s/%d", svcKey, revision)
			}
			allProductionServices[fmt.Sprintf("%s/%s/%d", projectName, serviceName, revision)] = tmpSvc
			if len(tmpSvc.ServiceVariableKVs) > 0 || len(tmpSvc.VariableYaml) == 0 {
				continue
			}

			serviceKVs, err := types.YamlToServiceVariableKV(tmpSvc.VariableYaml, nil)
			if err != nil {
				return errors.Wrapf(err, "failed to convert yaml to service variable kv, service name: %s/%d", svcKey, revision)
			}

			tmpSvc.ServiceVariableKVs = serviceKVs
			err = repository.UpdateServiceVariables(tmpSvc, true)
			if err != nil {
				return errors.Wrapf(err, "failed to update service variables, service name: %s/%d", svcKey, revision)
			}
		}
	}
	return nil
}

func migrateProductionProductVariables() error {
	for _, project := range k8sProjects {
		envs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{
			Name:       project.ProductName,
			Production: util2.GetBoolPointer(true),
		})
		if err != nil {
			return errors.Wrapf(err, "get envs, project name: %s", project.ProductName)
		}

		for _, product := range envs {
			if !product.Production {
				continue
			}

			product.EnsureRenderInfo()
			renderInfo, exists, err := mongodb.NewRenderSetColl().FindRenderSet(&mongodb.RenderSetFindOption{
				ProductTmpl: product.ProductName,
				Name:        product.Render.Name,
				Revision:    product.Render.Revision,
				IsDefault:   false,
			})
			if err != nil {
				return errors.Wrapf(err, "get render info, product name: %s, render name: %s, revision: %d", product.ProductName, product.Render.Name, product.Render.Revision)
			}
			if !exists {
				return fmt.Errorf("render set not found, product name: %s, render name: %s, revision: %d", product.ProductName, product.Render.Name, product.Render.Revision)
			}

			// change service variable yaml to kvs
			// full service vars will be stored in renderset
			// UseGlobalVariable will be set correctly
			for _, svcKV := range renderInfo.ServiceVariables {
				if svcKV.OverrideYaml == nil || len(svcKV.OverrideYaml.YamlContent) == 0 || len(svcKV.OverrideYaml.RenderVaraibleKVs) > 0 {
					continue
				}

				productSvc := product.GetServiceMap()[svcKV.ServiceName]
				if productSvc == nil {
					continue
				}

				svcKvs, err := types.YamlToServiceVariableKV(svcKV.OverrideYaml.YamlContent, nil)
				if err != nil {
					return errors.Wrapf(err, "failed to convert service variable yaml to kv, service name: %s/%s/%s", product.ProductName, product.EnvName, svcKV.ServiceName)
				}

				svcVarKeys := sets.NewString()
				for _, sKV := range svcKvs {
					svcKV.OverrideYaml.RenderVaraibleKVs = append(svcKV.OverrideYaml.RenderVaraibleKVs, &types.RenderVariableKV{
						ServiceVariableKV: *sKV,
						UseGlobalVariable: false,
					})
					svcVarKeys.Insert(sKV.Key)
				}

				svcTemplate, ok := allProductionServices[fmt.Sprintf("%s/%s/%d", product.ProductName, svcKV.ServiceName, productSvc.Revision)]
				if !ok {
					log.Warnf("failed to find source template service: %s", fmt.Sprintf("%s/%s/%d", product.ProductName, svcKV.ServiceName, productSvc.Revision))
					continue
				}

				// make sure all service keys exist in renderset.ServiceVariables.RenderVariableKVs
				for _, templateVars := range svcTemplate.ServiceVariableKVs {
					if svcVarKeys.Has(templateVars.Key) {
						continue
					}
					svcKV.OverrideYaml.RenderVaraibleKVs = append(svcKV.OverrideYaml.RenderVaraibleKVs, &types.RenderVariableKV{
						ServiceVariableKV: *templateVars,
						UseGlobalVariable: false,
					})
				}
			}

			// create new render info
			err = render.CreateRenderSet(renderInfo, log.SugaredLogger())
			if err != nil {
				return errors.Wrapf(err, "failed to create renderset")
			}
			product.Render.Name = renderInfo.Name
			product.Render.Revision = renderInfo.Revision

			err = mongodb.NewProductColl().UpdateRender(product.EnvName, product.ProductName, product.Render)
			if err != nil {
				return errors.Wrapf(err, "failed to update product info: %s/%s", product.ProductName, product.EnvName)
			}
		}
	}
	return nil
}
