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
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
)

type UpdateContainerImageArgs struct {
	Type          string `json:"type"`
	ProductName   string `json:"product_name"`
	EnvName       string `json:"env_name"`
	ServiceName   string `json:"service_name"`
	Name          string `json:"name"`
	ContainerName string `json:"container_name"`
	Image         string `json:"image"`
}

func updateContainerForHelmChart(serviceName, image, containerName string, product *models.Product) error {
	namespace := product.Namespace
	targetProductService := product.GetServiceMap()[serviceName]
	if targetProductService == nil {
		return fmt.Errorf("failed to find service in product: %s", serviceName)
	}

	opt := &commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		Revision:    targetProductService.Revision,
		ProductName: product.ProductName,
	}

	serviceObj, err := commonrepo.NewServiceColl().Find(opt)
	if err != nil {
		log.Errorf("failed to find template service, opt %+v, err :%s", *opt, err.Error())
		err = fmt.Errorf("failed to find template service, opt %+v", *opt)
		return err
	}

	renderSet, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		ProductTmpl: product.ProductName,
		Name:        product.Render.Name,
		EnvName:     product.EnvName,
		Revision:    product.Render.Revision,
	})
	if err != nil {
		err = fmt.Errorf("failed to find redset name %s revision %d", namespace, product.Render.Revision)
		return err
	}

	err = kube.UpgradeHelmRelease(product, renderSet, targetProductService, serviceObj, []string{image}, "", 0)
	if err != nil {
		return fmt.Errorf("failed to upgrade helm release, err: %s", err.Error())
	}
	return nil
}

func UpdateContainerImage(requestID string, args *UpdateContainerImageArgs, log *zap.SugaredLogger) error {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{EnvName: args.EnvName, Name: args.ProductName})
	if err != nil {
		return e.ErrUpdateConainterImage.AddErr(err)
	}

	namespace := product.Namespace
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return e.ErrUpdateConainterImage.AddErr(err)
	}
	// aws secrets needs to be refreshed
	regs, err := commonservice.ListRegistryNamespaces("", true, log)
	if err != nil {
		log.Errorf("Failed to get registries to update container images, the error is: %s", err)
		return err
	}
	for _, reg := range regs {
		if reg.RegProvider == config.RegistryTypeAWS {
			if err := kube.CreateOrUpdateRegistrySecret(namespace, reg, false, kubeClient); err != nil {
				retErr := fmt.Errorf("failed to update pull secret for registry: %s, the error is: %s", reg.ID.Hex(), err)
				log.Errorf("%s\n", retErr.Error())
				return retErr
			}
		}
	}

	eventStart := time.Now().Unix()

	defer func() {
		commonservice.LogProductStats(namespace, setting.UpdateContainerImageEvent, args.ProductName, requestID, eventStart, log)
	}()

	// update service in helm way
	if product.Source == setting.HelmDeployType {
		serviceName, err := commonservice.GetHelmServiceName(product, args.Type, args.Name, kubeClient)
		if err != nil {
			return e.ErrUpdateConainterImage.AddErr(err)
		}
		err = updateContainerForHelmChart(serviceName, args.Image, args.ContainerName, product)
		if err != nil {
			return e.ErrUpdateConainterImage.AddErr(err)
		}
	} else {
		switch args.Type {
		case setting.Deployment:
			if err := updater.UpdateDeploymentImage(namespace, args.Name, args.ContainerName, args.Image, kubeClient); err != nil {
				log.Errorf("[%s] UpdateDeploymentImageByName error: %s", namespace, err.Error())
				return e.ErrUpdateConainterImage.AddDesc("更新 Deployment 容器镜像失败")
			}
		case setting.StatefulSet:
			if err := updater.UpdateStatefulSetImage(namespace, args.Name, args.ContainerName, args.Image, kubeClient); err != nil {
				log.Errorf("[%s] UpdateStatefulsetImageByName error: %s", namespace, err.Error())
				return e.ErrUpdateConainterImage.AddDesc("更新 StatefulSet 容器镜像失败")
			}
		default:
			return e.ErrUpdateConainterImage.AddDesc(fmt.Sprintf("不支持的资源类型: %s", args.Type))
		}
		// update image info in product.services.container
		for _, service := range product.GetServiceMap() {
			if service.ServiceName != args.ServiceName {
				continue
			}
			for _, container := range service.Containers {
				if container.Name == args.ContainerName {
					container.Image = args.Image
					break
				}
			}
			break
		}
		if err := commonrepo.NewProductColl().Update(product); err != nil {
			log.Errorf("[%s] update product %s error: %s", namespace, args.ProductName, err.Error())
			return e.ErrUpdateConainterImage.AddDesc("更新环境信息失败")
		}
	}
	return nil
}
