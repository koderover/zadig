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
	"regexp"
	"strings"
	"time"

	helmclient "github.com/mittwald/go-helm-client"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

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
	"github.com/koderover/zadig/pkg/util/converter"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
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

const (
	imageUrlParseRegexString = `(?P<repo>.+/)?(?P<image>[^:]+){1}(:)?(?P<tag>.+)?`
)

var (
	imageParseRegex = regexp.MustCompile(imageUrlParseRegexString)
)

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

// parse image url to map: repo=>xxx/xx/xx image=>xx tag=>xxx
func resolveImageUrl(imageUrl string) map[string]string {
	subMatchAll := imageParseRegex.FindStringSubmatch(imageUrl)
	result := make(map[string]string)
	exNames := imageParseRegex.SubexpNames()
	for i, matchedStr := range subMatchAll {
		if i != 0 && matchedStr != "" && matchedStr != ":" {
			result[exNames[i]] = matchedStr
		}
	}
	return result
}

// replace image defines in yaml by new version
func replaceImage(sourceYaml string, imageValuesMap map[string]interface{}) (string, error) {
	nestedMap, err := converter.Expand(imageValuesMap)
	if err != nil {
		return "", err
	}
	bs, err := yaml.Marshal(nestedMap)
	if err != nil {
		return "", err
	}
	mergedBs, err := yamlutil.Merge([][]byte{[]byte(sourceYaml), bs})
	if err != nil {
		return "", err
	}
	return string(mergedBs), nil
}

// AssignImageData assign image url data into match data
// matchData: image=>absolute-path repo=>absolute-path tag=>absolute-path
// return: absolute-image-path=>image-value  absolute-repo-path=>repo-value absolute-tag-path=>tag-value
func assignImageData(imageUrl string, matchData map[string]string) (map[string]interface{}, error) {
	ret := make(map[string]interface{})
	// total image url assigned into one single value
	if len(matchData) == 1 {
		for _, v := range matchData {
			ret[v] = imageUrl
		}
		return ret, nil
	}

	resolvedImageUrl := resolveImageUrl(imageUrl)

	// image url assigned into repo/image+tag
	if len(matchData) == 3 {
		ret[matchData[setting.PathSearchComponentRepo]] = strings.TrimSuffix(resolvedImageUrl[setting.PathSearchComponentRepo], "/")
		ret[matchData[setting.PathSearchComponentImage]] = resolvedImageUrl[setting.PathSearchComponentImage]
		ret[matchData[setting.PathSearchComponentTag]] = resolvedImageUrl[setting.PathSearchComponentTag]
		return ret, nil
	}

	if len(matchData) == 2 {
		// image url assigned into repo/image + tag
		if tagPath, ok := matchData[setting.PathSearchComponentTag]; ok {
			ret[tagPath] = resolvedImageUrl[setting.PathSearchComponentTag]
			for k, imagePath := range matchData {
				if k == setting.PathSearchComponentTag {
					continue
				}
				ret[imagePath] = fmt.Sprintf("%s%s", resolvedImageUrl[setting.PathSearchComponentRepo], resolvedImageUrl[setting.PathSearchComponentImage])
				break
			}
			return ret, nil
		}
		// image url assigned into repo + image(tag)
		ret[matchData[setting.PathSearchComponentRepo]] = strings.TrimSuffix(resolvedImageUrl[setting.PathSearchComponentRepo], "/")
		ret[matchData[setting.PathSearchComponentImage]] = fmt.Sprintf("%s:%s", resolvedImageUrl[setting.PathSearchComponentImage], resolvedImageUrl[setting.PathSearchComponentTag])
		return ret, nil
	}

	return nil, fmt.Errorf("match data illegal, expect length: 1-3, actual length: %d", len(matchData))
}

// prepare necessary data from db
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
		break
	}
	if targetContainer == nil {
		err = fmt.Errorf("failed to find container %s", containerName)
		return
	}

	renderSet, err = commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{Name: namespace, Revision: product.Render.Revision})
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
		return err
	}

	// update image info in product.services.container
	targetContainer.Image = image

	// prepare image replace info
	replaceValuesMap, err = assignImageData(image, getValidMatchData(targetContainer.ImagePath))
	if err != nil {
		return fmt.Errorf("failed to pase image uri %s/%s, err %s", namespace, serviceName, err.Error())
	}

	// replace image into service's values.yaml
	replacedValuesYaml, err = replaceImage(targetChart.ValuesYaml, replaceValuesMap)
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
	replacedMergedValuesYaml, err = replaceImage(mergedValuesYaml, replaceValuesMap)
	if err != nil {
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

	// when replace image, should not wait
	err = installOrUpgradeHelmChartWithValues(namespace, replacedMergedValuesYaml, targetChart, serviceObj, 0, helmClient)
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

	eventStart := time.Now().Unix()

	defer func() {
		commonservice.LogProductStats(namespace, setting.UpdateContainerImageEvent, args.ProductName, requestID, eventStart, log)
	}()

	// update service in helm way
	if product.Source == setting.HelmDeployType {
		serviceName, err := commonservice.GetHelmServiceName(namespace, args.Type, args.Name, kubeClient)
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
