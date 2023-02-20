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
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/pkg/setting"
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
		return errors.Wrapf(err, "failed to find product %s", deployInfo.ProductName)
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

	if !deployInfo.Uninstall {
		if productInfo.Services == nil {
			productInfo.Services = make([][]*models.ProductService, 0)
		}
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
				ServiceName: deployInfo.ServiceName,
			}
			curRenderset.ServiceVariables = append(curRenderset.ServiceVariables, svcRender)
		}

		productSvc.Containers = mergeContainers(svcTemplate.Containers, productSvc.Containers, deployInfo.Containers)
		productSvc.Revision = int64(deployInfo.ServiceRevision)
		svcRender.OverrideYaml = &template.CustomYaml{
			YamlContent: deployInfo.VariableYaml,
		}
	} else {

	}

	return nil
}
