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
	"fmt"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util/converter"
)

func init() {
	upgradepath.RegisterHandler("1.4.0", "1.5.0", V140ToV150)
	upgradepath.RegisterHandler("1.5.0", "1.4.0", V150ToV140)
}

// V140ToV150 fill image path data for old data in product.services.containers
// use preset rules as patterns: {"image": "repository", "tag": "tag"}, {"image": "image"}
func V140ToV150() error {
	log.Info("Migrating data from 1.4.0 to 1.5.0")

	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		ExcludeStatus: []string{setting.ProductStatusDeleting, setting.ProductStatusUnknown},
		Source:        setting.SourceFromHelm,
	})
	if err != nil {
		return err
	}

	for _, product := range products {
		renderSetName := product.Namespace
		renderSetOpt := &commonrepo.RenderSetFindOption{Name: renderSetName, Revision: product.Render.Revision}
		renderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
		if err != nil {
			log.Errorf("find helm renderset[%s] error: %v", renderSetName, err)
			continue
		}
		product.ServiceRenders = renderSet.ChartInfos

		err = fillImageParseInfo(product)
		if err != nil {
			log.Error("failed to fill image parse info for product %s, err %s", product.ProductName, err.Error())
			continue
		}
	}
	return nil
}

func V150ToV140() error {
	log.Info("Rollback data from 1.5.0 to 1.4.0")
	return nil
}

func missContainerImagePath(container *commonmodels.Container) bool {
	if container.ImagePath == nil {
		return true
	}
	return container.ImagePath.Repo == "" && container.ImagePath.Image == "" && container.ImagePath.Tag == ""
}

func fillImageParseInfo(prod *commonmodels.Product) error {
	rendersetMap := make(map[string]string)
	for _, singleData := range prod.ServiceRenders {
		rendersetMap[singleData.ServiceName] = singleData.ValuesYaml
	}

	fillHappen := false

	// fill the parse info
	serviceMap := prod.GetServiceMap()
	for _, singleService := range serviceMap {
		findEmptySpec := false
		for _, container := range singleService.Containers {
			if missContainerImagePath(container) {
				findEmptySpec = true
				break
			}
		}
		if !findEmptySpec {
			continue
		}
		flatMap, err := converter.YamlToFlatMap([]byte(rendersetMap[singleService.ServiceName]))
		if err != nil {
			log.Errorf("get flat map fail for service: %s.%s, err %s", prod.ProductName, singleService.ServiceName, err.Error())
			continue
		}
		matchedPath, err := commonservice.ParseImagesByPresetRules(flatMap)
		if err != nil {
			log.Errorf("failed to parse images from service:%s.%s in product:%s", prod.ProductName, singleService.ServiceName, singleService.ProductName)
			continue
		}
		for _, container := range singleService.Containers {
			if !missContainerImagePath(container) {
				continue
			}
			err = findImageByContainerName(flatMap, matchedPath, container)
			if err != nil {
				log.Errorf("failed to find image pathL %s.%s, err: %s", prod.ProductName, singleService.ServiceName, err.Error())
				continue
			}
			fillHappen = true
			log.Infof("fill parse data to  %s.%s.%s successfully", prod.ProductName, singleService.ServiceName, container.Name)
		}
	}

	if fillHappen {
		err := commonrepo.NewProductColl().Update(prod)
		if err != nil {
			return err
		}
	}
	return nil
}

// find match rule
func findImageByContainerName(flatMap map[string]interface{}, matchedPath []map[string]string, container *commonmodels.Container) error {
	for _, searchResult := range matchedPath {
		imageURI, err := commonservice.GeneImageURI(searchResult, flatMap)
		if err != nil {
			log.Error("GeneImageURI fail, err %s", err.Error())
			continue
		}
		if container.Name != commonservice.ExtractImageName(imageURI) {
			continue
		}
		container.ImagePath = &commonmodels.ImagePathSpec{
			Repo:  searchResult[setting.PathSearchComponentRepo],
			Image: searchResult[setting.PathSearchComponentImage],
			Tag:   searchResult[setting.PathSearchComponentTag],
		}
		return nil
	}
	return fmt.Errorf("failed to find image for contianer %s", container.Image)
}
