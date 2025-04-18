/*
Copyright 2024 The KodeRover Authors.

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
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/mongo"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/koderover/zadig/v2/pkg/util/converter"
)

func IsServiceVarsWildcard(serviceVars []string) bool {
	return len(serviceVars) == 1 && serviceVars[0] == "*"
}

func ClipVariableYamlNoErr(variableYaml string, validKeys []string) string {
	if len(variableYaml) == 0 {
		return variableYaml
	}
	if len(validKeys) == 0 {
		return ""
	}
	clippedYaml, err := ClipVariableYaml(variableYaml, validKeys)
	if err != nil {
		log.Errorf("failed to clip variable yaml, err: %s", err)
		return variableYaml
	}
	return clippedYaml
}

func ClipVariableYaml(variableYaml string, validKeys []string) (string, error) {
	if len(variableYaml) == 0 {
		return "", nil
	}
	valuesMap, err := converter.YamlToFlatMap([]byte(variableYaml))
	if err != nil {
		return "", fmt.Errorf("failed to get flat map for service variable, err: %s", err)
	}

	wildcard := IsServiceVarsWildcard(validKeys)
	if wildcard {
		return variableYaml, nil
	}
	keysSet := sets.NewString(validKeys...)
	validKvMap := make(map[string]interface{})
	for k, v := range valuesMap {
		if keysSet.Has(k) {
			validKvMap[k] = v
		}
	}

	if len(validKvMap) == 0 {
		return "", nil
	}

	validKvMap, err = converter.Expand(validKvMap)
	if err != nil {
		return "", err
	}

	bs, err := yaml.Marshal(validKvMap)
	return string(bs), err
}

func GetProductUsedTemplateSvcs(prod *models.Product) ([]*models.Service, error) {
	// filter releases, only list releases deployed by zadig
	productName, envName, serviceMap := prod.ProductName, prod.EnvName, prod.GetServiceMap()
	if len(serviceMap) == 0 {
		return nil, nil
	}
	listOpt := &commonrepo.SvcRevisionListOption{
		ProductName:      prod.ProductName,
		ServiceRevisions: make([]*commonrepo.ServiceRevision, 0),
	}
	resp := make([]*models.Service, 0)
	for _, productSvc := range serviceMap {
		listOpt.ServiceRevisions = append(listOpt.ServiceRevisions, &commonrepo.ServiceRevision{
			ServiceName: productSvc.ServiceName,
			Revision:    productSvc.Revision,
		})
	}
	templateServices, err := repository.ListServicesWithSRevision(listOpt, prod.Production)
	if err != nil {
		return nil, fmt.Errorf("failed to list template services for pruduct: %s:%s, err: %s", productName, envName, err)
	}
	return append(resp, templateServices...), nil
}

// GetReleaseNameToServiceNameMap generates mapping relationship: releaseName=>serviceName
func GetReleaseNameToServiceNameMap(prod *models.Product) (map[string]string, error) {
	productName, envName := prod.ProductName, prod.EnvName
	templateServices, err := GetProductUsedTemplateSvcs(prod)
	if err != nil {
		return nil, err
	}
	// map[ReleaseName] => serviceName
	releaseNameMap := make(map[string]string)
	for _, svcInfo := range templateServices {
		releaseNameMap[util.GeneReleaseName(svcInfo.GetReleaseNaming(), productName, prod.Namespace, envName, svcInfo.ServiceName)] = svcInfo.ServiceName
	}
	for _, svc := range prod.GetChartServiceMap() {
		releaseNameMap[svc.ReleaseName] = svc.ServiceName
	}
	return releaseNameMap, nil
}

func GetReleaseNameToChartNameMap(prod *models.Product) (map[string]string, error) {
	productName, envName := prod.ProductName, prod.EnvName
	templateServices, err := GetProductUsedTemplateSvcs(prod)
	if err != nil {
		return nil, err
	}
	// map[ReleaseName] => chartName
	releaseNameMap := make(map[string]string)
	for _, svcInfo := range templateServices {
		releaseNameMap[util.GeneReleaseName(svcInfo.GetReleaseNaming(), productName, prod.Namespace, envName, svcInfo.ServiceName)] = svcInfo.ServiceName
	}

	renderMap := prod.GetChartDeployRenderMap()
	for _, svc := range prod.GetChartServiceMap() {
		if renderInfo, ok := renderMap[svc.ReleaseName]; ok {
			releaseNameMap[svc.ReleaseName] = renderInfo.ChartName
		}
	}
	return releaseNameMap, nil
}

// GetServiceNameToReleaseNameMap generates mapping relationship: serviceName=>releaseName
func GetServiceNameToReleaseNameMap(prod *models.Product) (map[string]string, error) {
	productName, envName := prod.ProductName, prod.EnvName
	templateServices, err := GetProductUsedTemplateSvcs(prod)
	if err != nil {
		return nil, err
	}
	// map[serviceName] => ReleaseName
	releaseNameMap := make(map[string]string)
	for _, svcInfo := range templateServices {
		releaseNameMap[svcInfo.ServiceName] = util.GeneReleaseName(svcInfo.GetReleaseNaming(), productName, prod.Namespace, envName, svcInfo.ServiceName)
	}
	return releaseNameMap, nil
}

// update product image info
func UpdateProductImage(envName, productName, serviceName string, targets map[string]string, userName string, logger *zap.SugaredLogger) error {
	redisMutex := cache.NewRedisLock(fmt.Sprintf("UpdateProductImage:%s:%s", productName, envName))
	redisMutex.Lock()
	defer redisMutex.Unlock()

	prod, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{EnvName: envName, Name: productName})

	if err != nil {
		logger.Errorf("find product namespace error: %v", err)
		return err
	}

	for i, group := range prod.Services {
		for j, service := range group {
			if service.ServiceName == serviceName {
				for l, container := range service.Containers {
					if image, ok := targets[container.Name]; ok {
						prod.Services[i][j].Containers[l].Image = image
						prod.Services[i][j].Containers[l].ImageName = util.ExtractImageName(image)
					}
				}
			}
		}
	}

	session := mongotool.Session()
	defer session.EndSession(context.TODO())

	err = mongo.StartTransaction(session)
	if err != nil {
		return err
	}

	service := prod.GetServiceMap()[serviceName]
	if service != nil {
		err = CreateEnvServiceVersion(prod, service, userName, config.EnvOperationDefault, "", session, log.SugaredLogger())
		if err != nil {
			log.Errorf("CreateK8SEnvServiceVersion error: %v", err)
		}
	} else {
		log.Errorf("service %s not found in prod %s/%s", serviceName, prod.ProductName, prod.EnvName)
	}

	if err := commonrepo.NewProductCollWithSession(session).Update(prod); err != nil {
		errMsg := fmt.Sprintf("[%s][%s] update product image error: %v", prod.EnvName, prod.ProductName, err)
		logger.Errorf(errMsg)
		mongotool.AbortTransaction(session)
		return errors.New(errMsg)
	}

	return mongotool.CommitTransaction(session)
}

func GenIstioGatewayName(serviceName string) string {
	return fmt.Sprintf("%s-gateway-%s", "zadig", serviceName)
}
