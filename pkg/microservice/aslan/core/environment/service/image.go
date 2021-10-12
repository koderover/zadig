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
	"bytes"
	"fmt"
	"strings"
	"time"

	helmclient "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/strvals"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util"
	"github.com/koderover/zadig/pkg/util/converter"
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

func getHelmServiceName(namespace, resType, resName string, kubeClient client.Client) (string, error) {
	var annotation map[string]string
	switch resType {
	case setting.Deployment:
		deployObj, found, err := getter.GetDeployment(namespace, resName, kubeClient)
		if err != nil || !found {
			return "", fmt.Errorf("failed to find deployment %s, err %s", resName, err.Error())
		}
		annotation = deployObj.Annotations

	case setting.StatefulSet:
		statefulSet, found, err := getter.GetStatefulSet(namespace, resName, kubeClient)
		if err != nil || !found {
			return "", fmt.Errorf("failed to find stateful set %s, err %s", resName, err.Error())
		}
		annotation = statefulSet.Annotations
	}
	if annotation != nil {
		if chartRelease, ok := annotation[setting.HelmReleaseNameAnnotation]; !ok {
			return "", fmt.Errorf("failed to get release name from resource")
		} else {
			return util.ExtraServiceName(chartRelease, namespace), nil
		}
	}
	return "", fmt.Errorf("failed to get annotation from resource %s", resName)
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

func prepareData(namespace, serviceName string, containerName string, product *models.Product) (targetContainer *models.Container,
	targetChart *templatemodels.RenderChart, renderSet *models.RenderSet, serviceObj *models.Service, err error) {

	var targetProductService *models.ProductService
	for _, productService := range product.GetServiceMap() {
		if productService.ServiceName != serviceName {
			continue
		}
		targetProductService = productService
		for _, container := range productService.Containers {
			if container.Name != containerName {
				continue
			}
			targetContainer = container
			break
		}
	}
	if targetContainer == nil {
		err = fmt.Errorf("failed to find container %s", containerName)
		return
	}

	renderSet, err = commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{Name: namespace, Revision: product.Render.Revision})
	if err != nil {
		log.Errorf("[RenderSet.find] update product %s error: %v", product.ProductName, err)
		err = fmt.Errorf("failed to find redset name %s revision %d", namespace, product.Render.Revision)
		return
	}

	for _, chartInfo := range renderSet.ChartInfos {
		if chartInfo.ServiceName != serviceName {
			continue
		}
		targetChart = chartInfo
		break
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

func updateContainerForHelmChart(serviceName, resType, image, containerName string, product *models.Product) error {
	var (
		replaceValuesMap         map[string]interface{}
		replacedValuesYaml       string
		mergedValuesYaml         string
		replacedMergedValuesYaml string
		restConfig               *rest.Config
		helmClient               helmclient.Client
		namespace                string = product.Namespace
	)

	targetContainer, targetChart, renderSet, serviceObj, err := prepareData(namespace, serviceName, containerName, product)
	if err != nil {
		return e.ErrUpdateConainterImage.AddErr(err)
	}

	// update image info in product.services.container
	targetContainer.Image = image

	// prepare image replace info
	replaceValuesMap, err = util.AssignImageData(image, getValidMatchData(targetContainer.ImagePath))
	if err != nil {
		return fmt.Errorf("failed to pase image uri %s/%s, err %s", namespace, serviceName, err.Error())
	}

	// replace image into service's values.yaml
	replacedValuesYaml, err = util.ReplaceImage(targetChart.ValuesYaml, replaceValuesMap)
	if err != nil {
		return fmt.Errorf("failed to replace image uri %s/%s, err %s", namespace, serviceName, err.Error())

	}
	if replacedValuesYaml == "" {
		return errors.Errorf("failed to set new image uri into service's values.yaml %s/%s", namespace, serviceName)
	}

	// update values.yaml content in chart
	targetChart.ValuesYaml = replacedValuesYaml

	// merge override values and kvs into service's yaml
	mergedValuesYaml, err = helmtool.MergeOverrideValues(replacedValuesYaml, targetChart.GetOverrideYaml(), targetChart.OverrideValues)
	if err != nil {
		return err
	}

	// replace image into final merged values.yaml
	replacedMergedValuesYaml, err = util.ReplaceImage(mergedValuesYaml, replaceValuesMap)
	if err != nil {
		return err
	}
	if replacedMergedValuesYaml == "" {
		return err
	}

	restConfig, err = kube.GetRESTConfig(product.ClusterID)
	if err != nil {
		return err
	}
	helmClient, err = helmtool.NewClientFromRestConf(restConfig, namespace)
	if err != nil {
		return err
	}

	// when replace image, should not wait helm
	err = installOrUpgradeHelmChartWithValues(namespace, replacedMergedValuesYaml, targetChart, serviceObj, 0, helmClient)
	if err != nil {
		return err
	}

	if err = commonrepo.NewRenderSetColl().Update(renderSet); err != nil {
		log.Errorf("[RenderSet.update] product %s error: %v", product.ProductName, err)
		return errors.Errorf("failed to update render set, productName %s", product.ProductName)
	}

	if err = commonrepo.NewProductColl().Update(product); err != nil {
		log.Errorf("[%s] update product %s error: %v", namespace, product.ProductName, err)
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
	kubeClient, err := kube.GetKubeClient(product.ClusterID)
	if err != nil {
		return e.ErrUpdateConainterImage.AddErr(err)
	}

	eventStart := time.Now().Unix()

	defer func() {
		commonservice.LogProductStats(namespace, setting.UpdateContainerImageEvent, args.ProductName, requestID, eventStart, log)
	}()

	// update service in helm way
	if product.Source == setting.HelmDeployType {
		serviceName, err := getHelmServiceName(namespace, args.Type, args.Name, kubeClient)
		if err != nil {
			return e.ErrUpdateConainterImage.AddErr(err)
		}
		err = updateContainerForHelmChart(serviceName, args.Type, args.Image, args.ContainerName, product)
		if err != nil {
			return e.ErrUpdateConainterImage.AddErr(err)
		}
	} else {
		switch args.Type {
		case setting.Deployment:
			if err := updater.UpdateDeploymentImage(namespace, args.Name, args.ContainerName, args.Image, kubeClient); err != nil {
				log.Errorf("[%s] UpdateDeploymentImageByName error: %v", namespace, err)
				return e.ErrUpdateConainterImage.AddDesc("更新 Deployment 容器镜像失败")
			}
		case setting.StatefulSet:
			if err := updater.UpdateStatefulSetImage(namespace, args.Name, args.ContainerName, args.Image, kubeClient); err != nil {
				log.Errorf("[%s] UpdateStatefulsetImageByName error: %v", namespace, err)
				return e.ErrUpdateConainterImage.AddDesc("更新 StatefulSet 容器镜像失败")
			}
		default:
			return nil
		}
		// update image info in product.services.container
		for _, service := range product.GetServiceMap() {
			if service.ServiceName != args.ServiceName {
				continue
			}
			for _, container := range service.Containers {
				if container.Name != args.ContainerName {
					continue
				}
				container.Image = args.Image
				break
			}
		}
		if err := commonrepo.NewProductColl().Update(product); err != nil {
			log.Errorf("[%s] update product %s error: %v", namespace, args.ProductName, err)
			return e.ErrUpdateConainterImage.AddDesc("更新环境信息失败")
		}
	}
	return nil
}

func updateImageTagInValues(valuesYaml []byte, repo, newTag string) ([]byte, error) {
	if !bytes.Contains(valuesYaml, []byte(repo)) {
		return nil, nil
	}

	valuesMap := map[string]interface{}{}
	if err := yaml.Unmarshal(valuesYaml, &valuesMap); err != nil {
		return nil, err
	}

	valuesFlatMap, err := converter.Flatten(valuesMap)
	if err != nil {
		return nil, err
	}

	var matchingKey string
	for k, v := range valuesFlatMap {
		if val, ok := v.(string); ok && repo == val {
			matchingKey = k
		}
	}
	if matchingKey == "" {
		return nil, fmt.Errorf("key is not found for value %s in map %v", repo, valuesFlatMap)
	}

	newKey := strings.Replace(matchingKey, ".repository", ".tag", 1)
	if newKey == matchingKey {
		return nil, fmt.Errorf("field repository is not found in key %s", matchingKey)
	}
	if err := strvals.ParseInto(fmt.Sprintf("%s=%s", newKey, newTag), valuesMap); err != nil {
		return nil, err
	}

	return yaml.Marshal(valuesMap)
}
