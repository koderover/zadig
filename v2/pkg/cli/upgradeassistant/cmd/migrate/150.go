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

package migrate

import (
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("1.4.0", "1.5.0", V140ToV150)
	upgradepath.RegisterHandler("1.5.0", "1.4.0", V150ToV140)
}

// V140ToV150 fill image path data for old data in product.services.containers
// use preset rules as patterns: {"image": "repository", "tag": "tag"}, {"image": "image"}
func V140ToV150() error {
	return nil
}

func V150ToV140() error {
	log.Info("Rollback data from 1.5.0 to 1.4.0")
	return nil
}

//func missContainerImagePath(container *commonmodels.Container) bool {
//	if container.ImagePath == nil {
//		return true
//	}
//	return container.ImagePath.Repo == "" && container.ImagePath.Image == "" && container.ImagePath.Tag == ""
//}

//func fillImageParseInfo(prod *commonmodels.Product) error {
//	rendersetMap := make(map[string]string)
//	for _, singleData := range prod.ServiceRenders {
//		rendersetMap[singleData.ServiceName] = singleData.ValuesYaml
//	}
//
//	fillHappen := false
//
//	// fill the parse info
//	serviceMap := prod.GetServiceMap()
//	for _, singleService := range serviceMap {
//		findEmptySpec := false
//		for _, container := range singleService.Containers {
//			if missContainerImagePath(container) {
//				findEmptySpec = true
//				break
//			}
//		}
//		if !findEmptySpec {
//			continue
//		}
//		flatMap, err := converter.YamlToFlatMap([]byte(rendersetMap[singleService.ServiceName]))
//		if err != nil {
//			log.Errorf("get flat map fail for service: %s.%s, err %s", prod.ProductName, singleService.ServiceName, err.Error())
//			continue
//		}
//		matchedPath, err := commonutil.ParseImagesByPresetRules(flatMap)
//		if err != nil {
//			log.Errorf("failed to parse images from service:%s.%s in product:%s", prod.ProductName, singleService.ServiceName, singleService.ProductName)
//			continue
//		}
//		for _, container := range singleService.Containers {
//			if !missContainerImagePath(container) {
//				continue
//			}
//			err = findImageByContainerName(flatMap, matchedPath, container)
//			if err != nil {
//				log.Errorf("failed to find image pathL %s.%s, err: %s", prod.ProductName, singleService.ServiceName, err.Error())
//				continue
//			}
//			fillHappen = true
//			log.Infof("fill parse data to  %s.%s.%s successfully", prod.ProductName, singleService.ServiceName, container.Name)
//		}
//	}
//
//	if fillHappen {
//		err := commonrepo.NewProductColl().Update(prod)
//		if err != nil {
//			return err
//		}
//	}
//	return nil
//}
//
//// find match rule
//func findImageByContainerName(flatMap map[string]interface{}, matchedPath []map[string]string, container *commonmodels.Container) error {
//	for _, searchResult := range matchedPath {
//		imageURI, err := commonutil.GeneImageURI(searchResult, flatMap)
//		if err != nil {
//			log.Error("GeneImageURI fail, err %s", err.Error())
//			continue
//		}
//		if container.Name != util.ExtractImageName(imageURI) {
//			continue
//		}
//		container.ImagePath = &commonmodels.ImagePathSpec{
//			Repo:      searchResult[setting.PathSearchComponentRepo],
//			Namespace: searchResult[setting.PathSearchComponentNamespace],
//			Image:     searchResult[setting.PathSearchComponentImage],
//			Tag:       searchResult[setting.PathSearchComponentTag],
//		}
//		return nil
//	}
//	return fmt.Errorf("failed to find image for contianer %s", container.Image)
//}
