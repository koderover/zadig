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

package jobcontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/mongo"
	"github.com/koderover/zadig/v2/pkg/util/converter"
)

type ProductServiceDeployInfo struct {
	ProductName           string
	EnvName               string
	ServiceName           string
	Uninstall             bool
	ServiceRevision       int
	VariableYaml          string
	VariableKVs           []*commontypes.RenderVariableKV
	Containers            []*models.Container
	UpdateServiceRevision bool
	UserName              string
	Resources             []*unstructured.Unstructured
}

func mergeContainers(currentContainer []*models.Container, newContainers ...[]*models.Container) []*models.Container {
	containerMap := make(map[string]*models.Container)
	for _, container := range currentContainer {
		containerMap[container.Name] = container
	}

	for _, containers := range newContainers {
		for _, container := range containers {
			if curContainer, ok := containerMap[container.Name]; ok {
				curContainer.Image = container.Image
				curContainer.ImageName = container.ImageName
			}
		}
	}

	ret := make([]*models.Container, 0)
	for _, container := range containerMap {
		ret = append(ret, container)
	}
	return ret
}

func variableYamlNil(variableYaml string) bool {
	if len(variableYaml) == 0 {
		return true
	}
	kvMap, _ := converter.YamlToFlatMap([]byte(variableYaml))
	return len(kvMap) == 0
}

// UpdateProductServiceDeployInfo updates deploy info of service for some product
// Including: Deploy service / Update service / Uninstall services
func UpdateProductServiceDeployInfo(deployInfo *ProductServiceDeployInfo) error {

	serviceDeployUpdateLock := cache.NewRedisLock(fmt.Sprintf("product_svc_deploystatus:%s:%s", deployInfo.ProductName, deployInfo.EnvName))
	serviceDeployUpdateLock.Lock()
	defer serviceDeployUpdateLock.Unlock()

	_, err := templaterepo.NewProductColl().Find(deployInfo.ProductName)
	if err != nil {
		return errors.Wrapf(err, "failed to find template product %s", deployInfo.ProductName)
	}

	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		EnvName: deployInfo.EnvName,
		Name:    deployInfo.ProductName,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to find product %s:%s", deployInfo.ProductName, deployInfo.EnvName)
	}

	svcTemplate, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: deployInfo.ProductName,
		ServiceName: deployInfo.ServiceName,
		Revision:    int64(deployInfo.ServiceRevision),
	}, productInfo.Production)
	if err != nil {
		return errors.Wrapf(err, "failed to find service %s in product %s", deployInfo.ServiceName, deployInfo.ProductName)
	}

	svcRender := productInfo.GetSvcRender(deployInfo.ServiceName)

	if len(productInfo.Services) == 0 {
		// DO NOT REMOVE []*models.ProductService{}, it's for compatibility
		productInfo.Services = [][]*models.ProductService{[]*models.ProductService{}}
	}

	session := mongo.Session()
	defer session.EndSession(context.TODO())

	err = mongo.StartTransaction(session)
	if err != nil {
		return err
	}

	if !deployInfo.Uninstall {
		// install or update service
		sevOnline := false
		productSvc := productInfo.GetServiceMap()[deployInfo.ServiceName]
		if productSvc == nil {
			productSvc = &models.ProductService{
				ProductName: deployInfo.ProductName,
				Type:        setting.K8SDeployType,
				ServiceName: deployInfo.ServiceName,
			}
			productInfo.Services[0] = append(productInfo.Services[0], productSvc)
			sevOnline = true
			productSvc.Render = svcRender
		}
		productSvc.UpdateTime = time.Now().Unix()

		// merge render variables and deploy variables
		mergedVariableYaml := deployInfo.VariableYaml
		mergedVariableKVs := deployInfo.VariableKVs
		if svcRender.OverrideYaml != nil && svcTemplate.Type == setting.K8SDeployType {
			templateVarKVs := commontypes.ServiceToRenderVariableKVs(svcTemplate.ServiceVariableKVs)
			_, mergedVariableKVs, err = commontypes.MergeRenderVariableKVs(templateVarKVs, svcRender.OverrideYaml.RenderVariableKVs, deployInfo.VariableKVs)
			if err != nil {
				mongo.AbortTransaction(session)
				return errors.Wrapf(err, "failed to merge render variable kv for %s/%s, %s", deployInfo.ProductName, deployInfo.EnvName, deployInfo.ServiceName)
			}
			mergedVariableYaml, mergedVariableKVs, err = commontypes.ClipRenderVariableKVs(svcTemplate.ServiceVariableKVs, mergedVariableKVs)
			if err != nil {
				mongo.AbortTransaction(session)
				return errors.Wrapf(err, "failed to clip render variable kv for %s/%s, %s", deployInfo.ProductName, deployInfo.EnvName, deployInfo.ServiceName)
			}
		}
		svcRender.OverrideYaml = &template.CustomYaml{
			YamlContent:       mergedVariableYaml,
			RenderVariableKVs: mergedVariableKVs,
		}

		// update product info
		productSvc.Containers = mergeContainers(svcTemplate.Containers, productSvc.Containers, deployInfo.Containers)
		productSvc.Revision = int64(deployInfo.ServiceRevision)
		productSvc.Resources = kube.UnstructuredToResources(deployInfo.Resources)
		log.Infof("UpdateServiceRevision : %v, sevOnline: %v, variableYamlNil %v, serviceName: %s", deployInfo.UpdateServiceRevision, sevOnline, variableYamlNil(deployInfo.VariableYaml), deployInfo.ServiceName)
		if deployInfo.UpdateServiceRevision || sevOnline || !variableYamlNil(deployInfo.VariableYaml) {
			productInfo.ServiceDeployStrategy = commonutil.SetServiceDeployStrategyDepoly(productInfo.ServiceDeployStrategy, deployInfo.ServiceName)
		}

		err = commonutil.CreateEnvServiceVersion(productInfo, productInfo.GetServiceMap()[deployInfo.ServiceName], deployInfo.UserName, config.EnvOperationDefault, "", session, log.SugaredLogger())
		if err != nil {
			log.Errorf("CreateK8SEnvServiceVersion error: %v", err)
		}
	} else {
		// uninstall service
		// update render set
		productInfo.GlobalVariables = commontypes.RemoveGlobalVariableRelatedService(productInfo.GlobalVariables, svcRender.ServiceName)

		// update product service
		svcGroups := make([][]*models.ProductService, 0)
		for _, svcGroup := range productInfo.Services {
			newGroup := make([]*models.ProductService, 0)
			for _, svc := range svcGroup {
				if svc.ServiceName == deployInfo.ServiceName {
					continue
				}
				newGroup = append(newGroup, svc)
			}
			svcGroups = append(svcGroups, newGroup)
		}
		productInfo.Services = svcGroups

		// update service deploy strategy
		for svcName := range productInfo.ServiceDeployStrategy {
			if svcName == deployInfo.ServiceName {
				delete(productInfo.ServiceDeployStrategy, svcName)
			}
		}
	}

	err = commonrepo.NewProductCollWithSession(session).Update(productInfo)
	if err != nil {
		mongo.AbortTransaction(session)
		return errors.Wrapf(err, "failed to update product %s", deployInfo.ProductName)
	}
	return mongo.CommitTransaction(session)
}
