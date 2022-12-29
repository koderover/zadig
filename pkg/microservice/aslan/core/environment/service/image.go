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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util"
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

func getValidMatchData(spec *models.ImagePathSpec) map[string]string {
	ret := make(map[string]string)
	if spec.Repo != "" {
		ret[setting.PathSearchComponentRepo] = spec.Repo
	}
	if spec.Image != "" {
		ret[setting.PathSearchComponentImage] = spec.Image
	}
	if spec.Tag != "" {
		ret[setting.PathSearchComponentTag] = spec.Tag
	}
	return ret
}

// prepare necessary data from db
func prepareData(namespace, serviceName, containerName, image string, product *models.Product) (targetContainer *models.Container,
	targetChart *templatemodels.ServiceRender, renderSet *models.RenderSet, serviceObj *models.Service, err error) {

	imageName := commonservice.ExtractImageName(image)

	var targetProductService *models.ProductService
	for _, productService := range product.GetServiceMap() {
		if productService.ServiceName != serviceName {
			continue
		}
		targetProductService = productService
		for _, container := range productService.Containers {
			if container.ImageName == imageName {
				targetContainer = container
				break
			}
		}
		break
	}
	if targetContainer == nil {
		err = fmt.Errorf("failed to find image config: %s for container %s", imageName, containerName)
		return
	}

	renderSet, err = commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		ProductTmpl: product.ProductName,
		Name:        namespace,
		EnvName:     product.EnvName,
		Revision:    product.Render.Revision,
	})
	if err != nil {
		log.Errorf("[RenderSet.find] update product %s error: %s", product.ProductName, err.Error())
		err = fmt.Errorf("failed to find redset name %s revision %d", namespace, product.Render.Revision)
		return
	}

	for _, chartInfo := range renderSet.ChartInfos {
		if chartInfo.ServiceName == serviceName {
			targetChart = chartInfo
			break
		}
	}
	if targetChart == nil {
		err = fmt.Errorf("failed to find chart info %s", serviceName)
		return
	}

	// 获取服务详情
	opt := &commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		Revision:    targetProductService.Revision,
		ProductName: product.ProductName,
	}

	serviceObj, err = commonrepo.NewServiceColl().Find(opt)
	if err != nil {
		log.Errorf("failed to find template service, opt %+v, err :%s", *opt, err.Error())
		err = fmt.Errorf("failed to find template service, opt %+v", *opt)
		return
	}
	return
}

func updateContainerForHelmChart(serviceName, resType, image, containerName string, product *models.Product, cl client.Client) error {
	var (
		replaceValuesMap         map[string]interface{}
		replacedValuesYaml       string
		mergedValuesYaml         string
		replacedMergedValuesYaml string
		helmClient               *helmtool.HelmClient
		namespace                string = product.Namespace
	)

	targetContainer, targetChart, renderSet, serviceObj, err := prepareData(namespace, serviceName, containerName, image, product)
	if err != nil {
		return err
	}

	// update image info in product.services.container
	targetContainer.Image = image

	// prepare image replace info
	replaceValuesMap, err = commonservice.AssignImageData(image, getValidMatchData(targetContainer.ImagePath))
	if err != nil {
		return fmt.Errorf("failed to pase image uri %s/%s, err %s", namespace, serviceName, err.Error())
	}

	// replace image into service's values.yaml
	replacedValuesYaml, err = commonservice.ReplaceImage(targetChart.ValuesYaml, replaceValuesMap)
	if err != nil {
		return fmt.Errorf("failed to replace image uri %s/%s, err %s", namespace, serviceName, err.Error())

	}
	if replacedValuesYaml == "" {
		return fmt.Errorf("failed to set new image uri into service's values.yaml %s/%s", namespace, serviceName)
	}

	// update values.yaml content in chart
	targetChart.ValuesYaml = replacedValuesYaml

	// merge override values and kvs into service's yaml
	mergedValuesYaml, err = helmtool.MergeOverrideValues(replacedValuesYaml, renderSet.DefaultValues, targetChart.GetOverrideYaml(), targetChart.OverrideValues)
	if err != nil {
		return err
	}

	// replace image into final merged values.yaml
	replacedMergedValuesYaml, err = commonservice.ReplaceImage(mergedValuesYaml, replaceValuesMap)
	if err != nil {
		return err
	}

	helmClient, err = helmtool.NewClientFromNamespace(product.ClusterID, namespace)
	if err != nil {
		return err
	}

	param := &ReleaseInstallParam{
		ProductName:  serviceObj.ProductName,
		Namespace:    namespace,
		ReleaseName:  util.GeneReleaseName(serviceObj.GetReleaseNaming(), serviceObj.ProductName, namespace, product.EnvName, serviceObj.ServiceName),
		MergedValues: replacedMergedValuesYaml,
		RenderChart:  targetChart,
		serviceObj:   serviceObj,
	}

	// when replace image, should not wait
	err = installOrUpgradeHelmChartWithValues(param, false, helmClient)
	if err != nil {
		return err
	}

	if err = commonrepo.NewRenderSetColl().Update(renderSet); err != nil {
		log.Errorf("[RenderSet.update] product %s error: %s", product.ProductName, err.Error())
		return fmt.Errorf("failed to update render set, productName %s", product.ProductName)
	}

	if err = commonrepo.NewProductColl().Update(product); err != nil {
		log.Errorf("[%s] update product %s error: %s", namespace, product.ProductName, err.Error())
		return fmt.Errorf("failed to update product info, name %s", product.ProductName)
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
		err = updateContainerForHelmChart(serviceName, args.Type, args.Image, args.ContainerName, product, kubeClient)
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
