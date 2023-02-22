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

package workflowcontroller

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/render"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util/yaml"
	"github.com/pkg/errors"
)

type ProductServiceDeployInfo struct {
	ProductName     string
	EnvName         string
	ServiceName     string
	Uninstall       bool
	ServiceRevision int
	VariableYaml    string
	Containers      []*models.Container
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

// UpdateProductServiceDeployInfo updates deploy info of service for some product
// Including: Deploy service / Update service / Uninstall services
func UpdateProductServiceDeployInfo(deployInfo *ProductServiceDeployInfo) error {
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

	productSvc := productInfo.GetServiceMap()[deployInfo.ServiceName]

	// nothing to do when trying to uninstall a service not found in product
	if productSvc == nil && deployInfo.Uninstall {
		return errors.Errorf("service %s not found in product %s", deployInfo.ServiceName, deployInfo.ProductName)
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

	if productInfo.Services == nil {
		productInfo.Services = make([][]*models.ProductService, 0)
	}

	if !deployInfo.Uninstall {
		if productSvc == nil {
			productSvc = &models.ProductService{
				ProductName: deployInfo.ProductName,
				Type:        setting.K8SDeployType,
				ServiceName: deployInfo.ServiceName,
			}
			productInfo.Services[0] = append(productInfo.Services[0], productSvc)
		}
		if svcRender == nil {
			svcRender = &template.ServiceRender{
				ServiceName:  deployInfo.ServiceName,
				OverrideYaml: &template.CustomYaml{},
			}
			curRenderset.ServiceVariables = append(curRenderset.ServiceVariables, svcRender)
		}

		mergedVariable := deployInfo.VariableYaml
		if svcRender.OverrideYaml != nil {
			mergedVariableBs, err := yaml.Merge([][]byte{[]byte(svcRender.OverrideYaml.YamlContent), []byte(deployInfo.VariableYaml)})
			if err != nil {
				return errors.Wrapf(err, "failed to merge variable yaml for %s/%s", deployInfo.ProductName, deployInfo.EnvName)
			}
			mergedVariable = string(mergedVariableBs)
		}

		productSvc.Containers = mergeContainers(svcTemplate.Containers, productSvc.Containers, deployInfo.Containers)
		productSvc.Revision = int64(deployInfo.ServiceRevision)
		svcRender.OverrideYaml = &template.CustomYaml{
			YamlContent: mergedVariable,
		}

		err = render.CreateRenderSet(curRenderset, log.SugaredLogger())
		if err != nil {
			return errors.Wrapf(err, "failed to update renderset for %s/%s", deployInfo.ProductName, deployInfo.EnvName)
		}
		productInfo.Render.Revision = curRenderset.Revision
	} else {
		filteredRenders := make([]*template.ServiceRender, 0)
		for _, svcRender := range curRenderset.ServiceVariables {
			if svcRender.ServiceName == deployInfo.ServiceName {
				continue
			}
			filteredRenders = append(filteredRenders, svcRender)
		}
		curRenderset.ServiceVariables = filteredRenders

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

		err = commonrepo.NewRenderSetColl().Update(curRenderset)
		if err != nil {
			return errors.Wrapf(err, "failed to update renderset for %s/%s", deployInfo.ProductName, deployInfo.EnvName)
		}
	}

	err = commonrepo.NewProductColl().Update(productInfo)
	if err != nil {
		return errors.Wrapf(err, "failed to update product %s", deployInfo.ProductName)
	}
	return nil
}
