/*
Copyright 2022 The KodeRover Authors.

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

package util

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
)

func GetServiceDeployStrategy(serviceName string, strategyMap map[string]string) string {
	if strategyMap == nil {
		return setting.ServiceDeployStrategyDeploy
	}
	if value, ok := strategyMap[serviceName]; !ok || value == "" {
		return setting.ServiceDeployStrategyDeploy
	} else {
		return value
	}
}

func ServiceDeployed(serviceName string, strategyMap map[string]string) bool {
	return GetServiceDeployStrategy(serviceName, strategyMap) == setting.ServiceDeployStrategyDeploy
}

func DeployStrategyChanged(serviceName string, strategyMapOld map[string]string, strategyMapNew map[string]string) bool {
	return ServiceDeployed(serviceName, strategyMapOld) != ServiceDeployed(serviceName, strategyMapNew)
}

func SetCurrentContainerImages(args *commonmodels.Service) error {
	var srvContainers []*commonmodels.Container
	for _, data := range args.KubeYamls {
		yamlDataArray := util.SplitYaml(data)
		for index, yamlData := range yamlDataArray {
			resKind := new(types.KubeResourceKind)
			// replace render variable {{.}} before Unmarshal
			yamlData = config.RenderTemplateAlias.ReplaceAllLiteralString(yamlData, "ssssssss")
			// replace $Service$ with service name
			yamlData = config.ServiceNameAlias.ReplaceAllLiteralString(yamlData, args.ServiceName)
			// replace $Product$ with product name
			yamlData = config.ProductNameAlias.ReplaceAllLiteralString(yamlData, args.ProductName)

			if err := yaml.Unmarshal([]byte(yamlData), &resKind); err != nil {
				return fmt.Errorf("unmarshal ResourceKind error: %v", err)
			}

			if resKind == nil {
				if index == 0 {
					continue
				}
				return errors.New("nil Resource Kind")
			}

			if resKind.Kind == setting.Deployment || resKind.Kind == setting.StatefulSet || resKind.Kind == setting.Job {
				containers, err := getContainers(yamlData)
				if err != nil {
					return fmt.Errorf("GetContainers error: %v", err)
				}

				srvContainers = append(srvContainers, containers...)
			} else if resKind.Kind == setting.CronJob {
				containers, err := getCronJobContainers(yamlData)
				if err != nil {
					return fmt.Errorf("GetCronjobContainers error: %v", err)
				}
				srvContainers = append(srvContainers, containers...)
			}
		}
	}

	args.Containers = uniqueSlice(srvContainers)
	return nil
}

func getContainers(data string) ([]*commonmodels.Container, error) {
	containers := make([]*commonmodels.Container, 0)

	res := new(types.KubeResource)
	if err := yaml.Unmarshal([]byte(data), &res); err != nil {
		return containers, fmt.Errorf("unmarshal yaml data error: %v", err)
	}

	for _, val := range res.Spec.Template.Spec.Containers {

		if _, ok := val["name"]; !ok {
			return containers, errors.New("yaml file missing name key")
		}

		nameStr, ok := val["name"].(string)
		if !ok {
			return containers, errors.New("error name value")
		}

		if _, ok := val["image"]; !ok {
			return containers, errors.New("yaml file missing image key")
		}

		imageStr, ok := val["image"].(string)
		if !ok {
			return containers, errors.New("error image value")
		}

		container := &commonmodels.Container{
			Name:      nameStr,
			Image:     imageStr,
			ImageName: ExtractImageName(imageStr),
		}

		containers = append(containers, container)
	}

	return containers, nil
}

func getCronJobContainers(data string) ([]*commonmodels.Container, error) {
	containers := make([]*commonmodels.Container, 0)

	res := new(types.CronjobResource)
	if err := yaml.Unmarshal([]byte(data), &res); err != nil {
		return containers, fmt.Errorf("unmarshal yaml data error: %v", err)
	}

	for _, val := range res.Spec.Template.Spec.Template.Spec.Containers {

		if _, ok := val["name"]; !ok {
			return containers, errors.New("yaml file missing name key")
		}

		nameStr, ok := val["name"].(string)
		if !ok {
			return containers, errors.New("error name value")
		}

		if _, ok := val["image"]; !ok {
			return containers, errors.New("yaml file missing image key")
		}

		imageStr, ok := val["image"].(string)
		if !ok {
			return containers, errors.New("error image value")
		}

		container := &commonmodels.Container{
			Name:      nameStr,
			Image:     imageStr,
			ImageName: ExtractImageName(imageStr),
		}

		containers = append(containers, container)
	}

	return containers, nil
}

func uniqueSlice(elements []*commonmodels.Container) []*commonmodels.Container {
	existing := make(map[string]bool)
	ret := make([]*commonmodels.Container, 0)
	for _, elem := range elements {
		key := elem.Name
		if _, ok := existing[key]; !ok {
			ret = append(ret, elem)
			existing[key] = true
		}
	}

	return ret
}

func KVs2Set(kvs []*commonmodels.ServiceKeyVal) sets.String {
	keySet := sets.NewString()
	for _, kv := range kvs {
		splitStrs := strings.Split(kv.Key, ".")
		key := strings.Split(splitStrs[0], "[")[0]
		keySet.Insert(key)
	}

	return keySet
}

func FilterKV(varKV *commonmodels.VariableKV, keySet sets.String) bool {
	if varKV == nil {
		return false
	}

	prefixKey := strings.Split(varKV.Key, ".")[0]
	prefixKey = strings.Split(prefixKey, "[")[0]
	if keySet.Has(prefixKey) {
		return true
	}
	return false
}

func ExtractRootKeyFromFlat(flatKey string) string {
	splitStrs := strings.Split(flatKey, ".")
	return strings.Split(splitStrs[0], "[")[0]
}
