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
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"k8s.io/apimachinery/pkg/util/sets"

	uamodel "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	uamongo "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	template2 "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/render"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	util2 "github.com/koderover/zadig/v2/pkg/util"
)

func ExtractRootKeyFromFlat(flatKey string) string {
	splitStrs := strings.Split(flatKey, ".")
	return strings.Split(splitStrs[0], "[")[0]
}

// GetAffectedServices fetch affected services
// key => services
func GenerateEnvVariableAffectServices(productInfo *models.Product, renderset *models.RenderSet) (map[string]sets.String, map[string]*types.ServiceVariableKV, error) {
	productServiceRevisions, err := commonutil.GetProductUsedTemplateSvcs(productInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find revision services, err: %s", err)
	}

	serviceKVS, err := types.YamlToServiceVariableKV(renderset.DefaultValues, nil)
	variableKVMap := make(map[string]*types.ServiceVariableKV)
	for _, kv := range serviceKVS {
		variableKVMap[kv.Key] = kv
	}

	envVariableKeys := sets.NewString()
	for key := range variableKVMap {
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
			totalRootKeys.Delete(ExtractRootKeyFromFlat(kv))
		}

		// matched global keys in environment
		for _, gk := range totalRootKeys.List() {
			if envVariableKeys.Has(gk) {
				insertSvcRelation(gk, productRevisionSvc.ServiceName)
			}
		}
	}
	return relation, variableKVMap, nil
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

	allProjects, err := template.NewProductColl().ListWithOption(&template.ProductListOpt{
		DeployType:    setting.K8SDeployType,
		BasicFacility: setting.BasicFacilityK8S,
	})
	if err != nil {
		return errors.Wrapf(err, "list k8s projects")
	}

	for _, project := range allProjects {
		if project.IsHostProduct() {
			continue
		}
		k8sProjects = append(k8sProjects, project)
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

	// 8. migrate workflow config and task data
	if err = migrateWorkflows(); err != nil {
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
		if len(yamlTemplate.ServiceVariableKVs) > 0 || len(yamlTemplate.VariableYaml) == 0 {
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
					totalRootKeys.Delete(ExtractRootKeyFromFlat(flatSvcKey))
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
			renderInfo, err := mongodb.NewRenderSetColl().Find(&mongodb.RenderSetFindOption{
				ProductTmpl: product.ProductName,
				Name:        product.Render.Name,
				Revision:    product.Render.Revision,
				IsDefault:   false,
			})
			if err != nil {
				return errors.Wrapf(err, "get render info, product name: %s, render name: %s, revision: %d", product.ProductName, product.Render.Name, product.Render.Revision)
			}

			if len(renderInfo.GlobalVariables) > 0 {
				continue
			}

			// change global variable yaml to kvs
			relations := make(map[string]sets.String)
			var globalKvs map[string]*types.ServiceVariableKV
			if len(renderInfo.DefaultValues) > 0 {
				// calculate variable yaml and service relations by keys
				// fill renderset.GlobalVariables
				relations, globalKvs, err = GenerateEnvVariableAffectServices(product, renderInfo)
				if err != nil {
					return errors.Wrapf(err, "failed to generate env variable affected services")
				}
				for k := range relations {
					var relatedSvcs []string = nil
					if svcSets, ok := relations[k]; ok {
						relatedSvcs = svcSets.List()
					}
					gvKV := &types.GlobalVariableKV{
						ServiceVariableKV: *globalKvs[k],
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
					if svcKV.OverrideYaml == nil {
						svcKV.OverrideYaml = &template2.CustomYaml{}
					}
					if len(svcKV.OverrideYaml.RenderVariableKVs) > 0 {
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

					// rootkeys declared in service variable yaml
					svcVarKeys := sets.NewString()
					for _, sKV := range svcKvs {
						svcKV.OverrideYaml.RenderVariableKVs = append(svcKV.OverrideYaml.RenderVariableKVs, &types.RenderVariableKV{
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
						svcKV.OverrideYaml.RenderVariableKVs = append(svcKV.OverrideYaml.RenderVariableKVs, &types.RenderVariableKV{
							ServiceVariableKV: *templateVars,
							UseGlobalVariable: false,
						})
					}

					// set to use global variable value if global variable is set
					for _, singleKV := range svcKV.OverrideYaml.RenderVariableKVs {
						if svcSet, ok := relations[singleKV.Key]; ok && svcSet.Has(svcKV.ServiceName) {
							singleKV.UseGlobalVariable = true
							singleKV.ServiceVariableKV = *globalKvs[singleKV.Key]
						}
					}

					svcKV.OverrideYaml.YamlContent, err = types.RenderVariableKVToYaml(svcKV.OverrideYaml.RenderVariableKVs)
					if err != nil {
						return errors.Wrapf(err, "failed to convert service variable kv to yaml, service name: %s/%s/%s", product.ProductName, product.EnvName, svcKV.ServiceName)
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
			uaRenderInfo := &uamodel.RenderInfo{
				Name:        product.Render.Name,
				Revision:    product.Render.Revision,
				ProductTmpl: product.Render.ProductTmpl,
				Description: product.Render.Description,
			}

			err = uamongo.NewProductColl().UpdateRender(product.EnvName, product.ProductName, uaRenderInfo)
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
			renderInfo, err := mongodb.NewRenderSetColl().Find(&mongodb.RenderSetFindOption{
				ProductTmpl: product.ProductName,
				Name:        product.Render.Name,
				Revision:    product.Render.Revision,
				IsDefault:   false,
			})
			if err != nil {
				return errors.Wrapf(err, "get render info, product name: %s, render name: %s, revision: %d", product.ProductName, product.Render.Name, product.Render.Revision)
			}

			// change service variable yaml to kvs
			// full service vars will be stored in renderset
			// UseGlobalVariable will be set correctly
			for _, svcKV := range renderInfo.ServiceVariables {
				if svcKV.OverrideYaml == nil || len(svcKV.OverrideYaml.YamlContent) == 0 || len(svcKV.OverrideYaml.RenderVariableKVs) > 0 {
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
					svcKV.OverrideYaml.RenderVariableKVs = append(svcKV.OverrideYaml.RenderVariableKVs, &types.RenderVariableKV{
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
					svcKV.OverrideYaml.RenderVariableKVs = append(svcKV.OverrideYaml.RenderVariableKVs, &types.RenderVariableKV{
						ServiceVariableKV: *templateVars,
						UseGlobalVariable: false,
					})
				}
				svcKV.OverrideYaml.YamlContent, err = types.RenderVariableKVToYaml(svcKV.OverrideYaml.RenderVariableKVs)
			}

			// create new render info
			err = render.CreateRenderSet(renderInfo, log.SugaredLogger())
			if err != nil {
				return errors.Wrapf(err, "failed to create renderset")
			}
			product.Render.Name = renderInfo.Name
			product.Render.Revision = renderInfo.Revision

			uaRenderInfo := &uamodel.RenderInfo{
				Name:        product.Render.Name,
				Revision:    product.Render.Revision,
				ProductTmpl: product.Render.ProductTmpl,
				Description: product.Render.Description,
			}
			err = uamongo.NewProductColl().UpdateRender(product.EnvName, product.ProductName, uaRenderInfo)
			if err != nil {
				return errors.Wrapf(err, "failed to update product info: %s/%s", product.ProductName, product.EnvName)
			}
		}
	}
	return nil
}

// turn flat kv to render kv
func turnFlatKVToRenderKV(originKVs []*commonmodels.ServiceKeyVal) ([]*types.RenderVariableKV, error) {
	kvs := make([]*commonmodels.VariableKV, 0)
	for _, kv := range originKVs {
		kvs = append(kvs, &commonmodels.VariableKV{
			Key:   kv.Key,
			Value: kv.Value,
		})
	}
	originYaml, err := kube.GenerateYamlFromKV(kvs)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate yaml from kv")
	}

	renderKVs, err := types.YamlToServiceVariableKV(originYaml, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert yaml to service variable kv")
	}

	return types.ServiceToRenderVariableKVs(renderKVs), nil
}

func workflowOriginKValsToVariableConfig(kvs []*commonmodels.ServiceKeyVal) []*commonmodels.DeplopyVariableConfig {
	ret := make([]*commonmodels.DeplopyVariableConfig, 0)
	kvSet := sets.NewString()
	for _, kv := range kvs {
		if kvSet.Has(ExtractRootKeyFromFlat(kv.Key)) {
			continue
		}
		kvSet.Insert(ExtractRootKeyFromFlat(kv.Key))
		ret = append(ret, &commonmodels.DeplopyVariableConfig{
			VariableKey:       ExtractRootKeyFromFlat(kv.Key),
			UseGlobalVariable: false,
		})
	}
	return ret
}

func updateWorkflowKVs(wf *models.WorkflowV4) (bool, error) {
	if wf == nil {
		return false, nil
	}
	needUpdate := false
	for _, stage := range wf.Stages {
		for _, job := range stage.Jobs {
			switch job.JobType {
			case config.JobZadigDeploy:
				jobSpec := &models.ZadigDeployJobSpec{}
				if err := commonmodels.IToi(job.Spec, jobSpec); err != nil {
					return needUpdate, errors.Wrapf(err, "unmashal job spec error")
				}
				for _, service := range jobSpec.Services {
					if len(service.VariableConfigs) > 0 {
						continue
					}
					needUpdate = true
					service.VariableConfigs = workflowOriginKValsToVariableConfig(service.KeyVals)
				}
				if needUpdate {
					job.Spec = jobSpec
				}
			}
		}
	}
	return needUpdate, nil
}

// handle workflow kvs
func migrateWorkflows() error {
	updater := &DataBulkUpdater{
		Coll:           mongodb.NewworkflowTaskv4Coll().Collection,
		WriteThreshold: 50,
	}

	// migrate workflow configs
	for _, project := range k8sProjects {
		workflows, _, err := mongodb.NewWorkflowV4Coll().List(&mongodb.ListWorkflowV4Option{ProjectName: project.ProjectName}, 0, 0)
		if err != nil {
			return errors.Wrapf(err, "failed to list workflows, project name: %s", project.ProjectName)
		}
		for _, wf := range workflows {
			needUpdate, err := updateWorkflowKVs(wf)
			if err != nil {
				return errors.Wrapf(err, "failed to update workflow kvs, %s/%s", wf.Project, wf.DisplayName)
			}
			// migrate kvs in workflow config
			if needUpdate {
				err = mongodb.NewWorkflowV4Coll().Update(wf.ID.Hex(), wf)
				if err != nil {
					return errors.Wrapf(err, "failed to update workflow, %s/%s", wf.Project, wf.DisplayName)
				}
			}

			taskCursor, err := mongodb.NewworkflowTaskv4Coll().ListByCursor(&mongodb.ListWorkflowTaskV4Option{
				WorkflowName: wf.Name,
				ProjectName:  wf.Project,
			})
			if err != nil {
				return errors.Wrapf(err, "failed to list workflow tasks, %s/%s", wf.Project, wf.DisplayName)
			}

			for taskCursor.Next(context.Background()) {
				var workflowTask models.WorkflowTask
				if err := taskCursor.Decode(&workflowTask); err != nil {
					return errors.Wrapf(err, "failed to decode workflow task, %s/%s", wf.Project, wf.DisplayName)
				}

				needUpdateOrigin, err := updateWorkflowKVs(workflowTask.OriginWorkflowArgs)
				if err != nil {
					return errors.Wrapf(err, "failed to update workflow task kvs, %s/%s", wf.Project, wf.DisplayName)
				}
				needUpdateArgs, err := updateWorkflowKVs(workflowTask.WorkflowArgs)
				if err != nil {
					return errors.Wrapf(err, "failed to update workflow task kvs, %s/%s", wf.Project, wf.DisplayName)
				}

				updateKVConfig := false
				for _, stage := range workflowTask.Stages {
					for _, job := range stage.Jobs {
						switch job.JobType {
						case string(config.JobZadigDeploy):
							jobSpec := &commonmodels.JobTaskDeploySpec{}
							if err := commonmodels.IToi(job.Spec, jobSpec); err != nil {
								return errors.Wrapf(err, "unmashal job spec error: %v", err)
							}
							if len(jobSpec.VariableKVs) > 0 {
								continue
							}
							jobSpec.VariableKVs, err = turnFlatKVToRenderKV(jobSpec.KeyVals)
							if err != nil {
								return errors.Wrapf(err, "failed to turn flat kv to render kv, %s/%s", wf.Project, wf.DisplayName)
							}
							updateKVConfig = true

							jobSpec.VariableConfigs = make([]*commonmodels.DeplopyVariableConfig, 0)
							for _, kv := range jobSpec.VariableKVs {
								jobSpec.VariableConfigs = append(jobSpec.VariableConfigs, &commonmodels.DeplopyVariableConfig{
									VariableKey:       kv.Key,
									UseGlobalVariable: false,
								})
							}
							job.Spec = jobSpec
						}
					}
				}

				if needUpdateOrigin || needUpdateArgs || updateKVConfig {
					err = updater.AddModel(mongo.NewUpdateOneModel().SetFilter(bson.D{{"_id", workflowTask.ID}}).SetUpdate(bson.D{{"$set",
						bson.D{
							{"origin_workflow_args", workflowTask.OriginWorkflowArgs},
							{"workflow_args", workflowTask.WorkflowArgs},
							{"stages", workflowTask.Stages},
						}},
					}))
					if err != nil {
						return errors.Wrapf(err, "failed to update workflow task kvs, %s/%s", wf.Project, wf.DisplayName)
					}
				}
			}
		}
	}
	return updater.Write()
}
