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

	"helm.sh/helm/v3/pkg/strvals"
	"sigs.k8s.io/yaml"

	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/kube/updater"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/util/converter"
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

func UpdateContainerImage(args *UpdateContainerImageArgs, log *xlog.Logger) error {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{EnvName: args.EnvName, Name: args.ProductName})
	if err != nil {
		return e.ErrUpdateConainterImage.AddErr(err)
	}

	namespace := product.Namespace
	kubeClient, err := kube.GetKubeClient(product.ClusterId)
	if err != nil {
		return e.ErrUpdateConainterImage.AddErr(err)
	}

	eventStart := time.Now().Unix()

	defer func() {
		commonservice.LogProductStats(namespace, setting.UpdateContainerImageEvent, args.ProductName, eventStart, log)
	}()

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

	oldImageName := ""
	for _, group := range product.Services {
		for _, service := range group {
			for _, container := range service.Containers {
				if container.Name == args.ContainerName {
					oldImageName = container.Image
					container.Image = args.Image
					break
				}
			}
		}
	}

	//如果环境是helm环境也要更新renderSet
	if product.Source == setting.HelmDeployType && oldImageName != "" {
		renderSetName := product.GetNamespace()
		renderSetOpt := &commonrepo.RenderSetFindOption{Name: renderSetName, Revision: product.Render.Revision}
		renderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
		if err != nil {
			log.Errorf("[RenderSet.find] update product %s error: %v", args.ProductName, err)
			return e.ErrUpdateConainterImage.AddDesc("更新环境信息失败")
		}
		oldImageRepo := strings.Split(oldImageName, ":")[0]
		newImageTag := strings.Split(args.Image, ":")[1]

		for _, chartInfo := range renderSet.ChartInfos {
			newValues, err := updateImageTagInValues([]byte(chartInfo.ValuesYaml), oldImageRepo, newImageTag)
			if err != nil {
				log.Errorf("Failed to update image tag, err: %v", err)
				continue
			} else if newValues == nil {
				continue
			}
			chartInfo.ValuesYaml = string(newValues)
		}

		if err := commonrepo.NewRenderSetColl().Update(renderSet); err != nil {
			log.Errorf("[RenderSet.update] product %s error: %v", args.ProductName, err)
			return e.ErrUpdateConainterImage.AddDesc("更新环境信息失败")
		}
	}

	if err := commonrepo.NewProductColl().Update(product); err != nil {
		log.Errorf("[%s] update product %s error: %v", namespace, args.ProductName, err)
		return e.ErrUpdateConainterImage.AddDesc("更新环境信息失败")
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
		if repo == v.(string) {
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
