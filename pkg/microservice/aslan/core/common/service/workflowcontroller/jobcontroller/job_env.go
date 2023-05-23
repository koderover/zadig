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

package jobcontroller

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/util/converter"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/render"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

type ProductServiceDeployInfo struct {
	ProductName           string
	EnvName               string
	ServiceName           string
	Uninstall             bool
	ServiceRevision       int
	VariableYaml          string
	VariableKVs           []*commontypes.RenderVariableKV
	Containers            []*models.Container
	UpdateServiceRevision bool
}

func mergeContainers(currentContainer []*models.Container, newContainers ...[]*models.Container) []*models.Container {
	containerMap := make(map[string]*models.Container)
	for _, container := range currentContainer {
		containerMap[container.Name] = container
	}

	for _, containers := range newContainers {
		for _, container := range containers {
			containerMap[container.Name] = container
		}
	}

	ret := make([]*models.Container, 0)
	for _, container := range containerMap {
		ret = append(ret, container)
	}
	return ret
}

func variableYamlNil(variableYaml string) bool {
	if len(variableYaml) == 0 {
		return true
	}
	kvMap, _ := converter.YamlToFlatMap([]byte(variableYaml))
	return len(kvMap) == 0
}

// TODO FIXME, we should not use lock here
var serviceDeployUpdateLock sync.Mutex

// UpdateProductServiceDeployInfo updates deploy info of service for some product
// Including: Deploy service / Update service / Uninstall services
func UpdateProductServiceDeployInfo(deployInfo *ProductServiceDeployInfo) error {
	serviceDeployUpdateLock.Lock()
	defer serviceDeployUpdateLock.Unlock()
	_, err := templaterepo.NewProductColl().Find(deployInfo.ProductName)
	if err != nil {
		return errors.Wrapf(err, "failed to find template product %s", deployInfo.ProductName)
	}

	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		EnvName: deployInfo.EnvName,
		Name:    deployInfo.ProductName,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to find product %s:%s", deployInfo.ProductName, deployInfo.EnvName)
	}

	svcTemplate, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: deployInfo.ProductName,
		ServiceName: deployInfo.ServiceName,
		Revision:    int64(deployInfo.ServiceRevision),
	}, productInfo.Production)
	if err != nil {
		return errors.Wrapf(err, "failed to find service %s in product %s", deployInfo.ServiceName, deployInfo.ProductName)
	}

	curRenderset, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		ProductTmpl: deployInfo.ProductName,
		EnvName:     deployInfo.EnvName,
		IsDefault:   false,
		Revision:    productInfo.Render.Revision,
		Name:        productInfo.Render.Name,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to find renderset for %s/%s", deployInfo.ProductName, deployInfo.EnvName)
	}

	var svcRender *template.ServiceRender
	for _, sRender := range curRenderset.ServiceVariables {
		if sRender.ServiceName == deployInfo.ServiceName {
			svcRender = sRender
			break
		}
	}

	if len(productInfo.Services) == 0 {
		// DO NOT REMOVE []*models.ProductService{}, it's for compatibility
		productInfo.Services = [][]*models.ProductService{[]*models.ProductService{}}
	}

	if !deployInfo.Uninstall {
		// install or update service
		sevOnline := false
		productSvc := productInfo.GetServiceMap()[deployInfo.ServiceName]
		if productSvc == nil {
			productSvc = &models.ProductService{
				ProductName: deployInfo.ProductName,
				Type:        setting.K8SDeployType,
				ServiceName: deployInfo.ServiceName,
			}
			productInfo.Services[0] = append(productInfo.Services[0], productSvc)
			sevOnline = true
		}
		if svcRender == nil {
			svcRender = &template.ServiceRender{
				ServiceName:  deployInfo.ServiceName,
				OverrideYaml: &template.CustomYaml{},
			}
			curRenderset.ServiceVariables = append(curRenderset.ServiceVariables, svcRender)
		}

		// merge render variables and deploy variables
		mergedVariableYaml := deployInfo.VariableYaml
		mergedVariableKVs := deployInfo.VariableKVs
		if svcRender.OverrideYaml != nil {
			templateVarKVs := commontypes.ServiceToRenderVariableKVs(svcTemplate.ServiceVariableKVs)
			_, mergedVariableKVs, err = commontypes.MergeRenderVariableKVs(templateVarKVs, svcRender.OverrideYaml.RenderVaraibleKVs, deployInfo.VariableKVs)
			if err != nil {
				return errors.Wrapf(err, "failed to merge render variable kv for %s/%s, %s", deployInfo.ProductName, deployInfo.EnvName, deployInfo.ServiceName)
			}
			mergedVariableYaml, mergedVariableKVs, err = commontypes.ClipRenderVariableKVs(svcTemplate.ServiceVariableKVs, mergedVariableKVs)
			if err != nil {
				return errors.Wrapf(err, "failed to clip render variable kv for %s/%s, %s", deployInfo.ProductName, deployInfo.EnvName, deployInfo.ServiceName)
			}
		}
		svcRender.OverrideYaml = &template.CustomYaml{
			YamlContent:       mergedVariableYaml,
			RenderVaraibleKVs: mergedVariableKVs,
		}

		// update global variables
		// curRenderset.GlobalVariables, _, err = commontypes.UpdateGlobalVariableKVs(deployInfo.ServiceName, curRenderset.GlobalVariables, svcRender.OverrideYaml.RenderVaraibleKVs)
		// if err != nil {
		// 	return errors.Wrapf(err, "failed to update global variable kv for %s/%s, %s", deployInfo.ProductName, deployInfo.EnvName, deployInfo.ServiceName)
		// }

		err = render.CreateRenderSet(curRenderset, log.SugaredLogger())
		if err != nil {
			return errors.Wrapf(err, "failed to update renderset for %s/%s", deployInfo.ProductName, deployInfo.EnvName)
		}

		// update product info
		productSvc.Containers = mergeContainers(svcTemplate.Containers, productSvc.Containers, deployInfo.Containers)
		productSvc.Revision = int64(deployInfo.ServiceRevision)
		productInfo.Render.Revision = curRenderset.Revision
		log.Infof("UpdateServiceRevision : %v, sevOnline: %v, variableYamlNil %v, serviceName: %s", deployInfo.UpdateServiceRevision, sevOnline, variableYamlNil(deployInfo.VariableYaml), deployInfo.ServiceName)
		if deployInfo.UpdateServiceRevision || sevOnline || !variableYamlNil(deployInfo.VariableYaml) {
			if productInfo.ServiceDeployStrategy == nil {
				productInfo.ServiceDeployStrategy = make(map[string]string)
			}
			productInfo.ServiceDeployStrategy[deployInfo.ServiceName] = setting.ServiceDeployStrategyDeploy
		}
	} else {
		// uninstall service

		// update render set
		filteredRenders := make([]*template.ServiceRender, 0)
		for _, svcRender := range curRenderset.ServiceVariables {
			if svcRender.ServiceName == deployInfo.ServiceName {
				curRenderset.GlobalVariables = commontypes.RemoveGlobalVariableRelatedService(curRenderset.GlobalVariables, svcRender.ServiceName)
			} else {
				filteredRenders = append(filteredRenders, svcRender)
			}
		}
		curRenderset.ServiceVariables = filteredRenders

		// update product service
		svcGroups := make([][]*models.ProductService, 0)
		for _, svcGroup := range productInfo.Services {
			newGroup := make([]*models.ProductService, 0)
			for _, svc := range svcGroup {
				if svc.ServiceName == deployInfo.ServiceName {
					continue
				}
				newGroup = append(newGroup, svc)
			}
			svcGroups = append(svcGroups, newGroup)
		}
		productInfo.Services = svcGroups

		// update service deploy strategy
		for svcName := range productInfo.ServiceDeployStrategy {
			if svcName == deployInfo.ServiceName {
				delete(productInfo.ServiceDeployStrategy, svcName)
			}
		}

		err := render.CreateRenderSet(curRenderset, log.SugaredLogger())
		if err != nil {
			return errors.Wrapf(err, "failed to update renderset for %s/%s", deployInfo.ProductName, deployInfo.EnvName)
		}
		productInfo.Render.Revision = curRenderset.Revision
	}

	err = commonrepo.NewProductColl().Update(productInfo)
	if err != nil {
		return errors.Wrapf(err, "failed to update product %s", deployInfo.ProductName)
	}
	return nil
}
