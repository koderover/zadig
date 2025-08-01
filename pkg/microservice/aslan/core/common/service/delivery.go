/*
Copyright 2021 The KodeRover Authors.

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

package service

import (
	"fmt"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	taskmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func DeleteDeliveryInfos(projectName string, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryVersionV2Coll().DeleteByProjectName(projectName)
	if err != nil {
		log.Errorf("delete DeleteDeliveryInfo error: %v", err)
		return e.ErrDeleteDeliveryVersion
	}
	return nil
}

func GetDeliveryVersion(args *commonrepo.DeliveryVersionArgs, log *zap.SugaredLogger) (*commonmodels.DeliveryVersion, error) {
	resp, err := commonrepo.NewDeliveryVersionColl().Get(args)
	if err != nil {
		log.Errorf("get deliveryVersion error: %v", err)
		return nil, e.ErrGetDeliveryVersion
	}
	return resp, err
}

func InsertDeliveryVersion(args *commonmodels.DeliveryVersion, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryVersionColl().Insert(args)
	if err != nil {
		log.Errorf("insert deliveryVersion error: %v", err)
		return e.ErrCreateDeliveryVersion
	}
	return nil
}

func getProductEnvInfo(pipelineTask *taskmodels.Task, log *zap.SugaredLogger) (*commonmodels.Product, [][]string, error) {
	product := new(commonmodels.Product)
	product.ProductName = pipelineTask.WorkflowArgs.ProductTmplName
	product.EnvName = pipelineTask.WorkflowArgs.Namespace

	if productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    product.ProductName,
		EnvName: product.EnvName,
	}); err != nil {
		product.Namespace = product.GetDefaultNamespace()
	} else {
		product.Namespace = productInfo.Namespace
	}

	product.Render = pipelineTask.Render
	product.Services = pipelineTask.Services

	return product, product.GetGroupServiceNames(), nil
}

func updateDeliveryVersion(args *commonmodels.DeliveryVersion, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryVersionColl().Update(args)
	if err != nil {
		log.Errorf("update deliveryVersion error: %v", err)
		return e.ErrUpdateDeliveryVersion
	}
	return nil
}

func updateServiceImage(serviceName, image, containerName string, product *commonmodels.Product) {
	for _, services := range product.Services {
		for _, serviceInfo := range services {
			if serviceInfo.ServiceName == serviceName {
				for _, container := range serviceInfo.Containers {
					if container.Name == containerName {
						container.Image = image
						break
					}
				}
				break
			}
		}
	}
}

func getServiceRenderYAML(productInfo *commonmodels.Product, containers []*commonmodels.Container, serviceName, deployType string, log *zap.SugaredLogger) (string, error) {
	if deployType == setting.K8SDeployType {
		serviceInfo := productInfo.GetServiceMap()[serviceName]
		if serviceInfo == nil {
			return "", fmt.Errorf("service %s not found", serviceName)
		}

		return kube.RenderEnvService(productInfo, serviceInfo.GetServiceRender(), serviceInfo)
	}
	return "", nil
}
