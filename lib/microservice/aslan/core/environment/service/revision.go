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

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	templaterepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo/template"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/poetry"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func ListProductsRevision(productName, envName string, userID int, superUser bool, log *xlog.Logger) ([]*ProductRevision, error) {
	var (
		err               error
		prodRevs          = make([]*ProductRevision, 0)
		products          = make([]*commonmodels.Product, 0)
		productNameMap    map[string][]int64
		productNamespaces = sets.NewString()
	)
	if superUser {
		products, err = commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{ExcludeStatus: setting.ProductStatusDeleting, Name: productName, EnvName: envName})
		if err != nil {
			log.Errorf("Collection.Product.List error: %v", err)
			return prodRevs, e.ErrListProducts.AddDesc(err.Error())
		}
	} else {
		//项目下所有公开环境
		publicProducts, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{IsPublic: true, ExcludeStatus: setting.ProductStatusDeleting, Name: productName, EnvName: envName})
		if err != nil {
			log.Errorf("Collection.Product.List List product error: %v", err)
			return prodRevs, e.ErrListProducts.AddDesc(err.Error())
		}
		for _, publicProduct := range publicProducts {
			products = append(products, publicProduct)
			productNamespaces.Insert(publicProduct.Namespace)
		}

		poetryCtl := poetry.NewPoetryServer(config.PoetryAPIServer(), config.PoetryAPIRootKey())
		productNameMap, err = poetryCtl.GetUserProject(userID, log)
		if err != nil {
			log.Errorf("Collection.Product.List GetUserProject error: %v", err)
			return prodRevs, e.ErrListProducts.AddDesc(err.Error())
		}
		for productName, roleIDs := range productNameMap {
			//用户关联角色所关联的环境
			for _, roleID := range roleIDs {
				if roleID == setting.RoleOwnerID {
					tmpProducts, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{ExcludeStatus: setting.ProductStatusDeleting, Name: productName, EnvName: envName})
					if err != nil {
						log.Errorf("Collection.Product.List Find product error: %v", err)
						return prodRevs, e.ErrListProducts.AddDesc(err.Error())
					}
					for _, product := range tmpProducts {
						if !productNamespaces.Has(product.Namespace) {
							products = append(products, product)
						}
					}
				} else {
					roleEnvs, err := poetryCtl.ListRoleEnvs(productName, envName, roleID, log)
					if err != nil {
						log.Errorf("Collection.Product.List ListRoleEnvs error: %v", err)
						return prodRevs, e.ErrListProducts.AddDesc(err.Error())
					}
					for _, roleEnv := range roleEnvs {
						product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: roleEnv.EnvName})
						if err != nil {
							log.Errorf("Collection.Product.List Find product error: %v", err)
							return prodRevs, e.ErrListProducts.AddDesc(err.Error())
						}
						products = append(products, product)
					}
				}
			}
		}
	}

	// 获取所有服务模板最新模板信息
	allServiceTmpls, err := commonrepo.NewServiceColl().ListAllRevisions()
	if err != nil {
		log.Errorf("ListAllRevisions error: %v", err)
		return prodRevs, e.ErrListProducts.AddDesc(err.Error())
	}
	// 获取所有服务配置最新模板信息
	allConfigs, err := commonrepo.NewConfigColl().ListAllRevisions()
	if err != nil {
		log.Errorf("ListAllRevisions error: %v", err)
		return prodRevs, e.ErrListProducts.AddDesc(err.Error())
	}
	// 获取所有渲染配置最新模板信息
	allRenders, err := commonrepo.NewRenderSetColl().ListAllRenders()
	if err != nil {
		log.Errorf("ListAllRevisions error: %v", err)
		return prodRevs, e.ErrListProducts.AddDesc(err.Error())
	}

	for _, prod := range products {
		newRender := &commonmodels.RenderSet{}
		if prod.Render != nil {
			newRender, err = commonservice.GetRenderSet(prod.Render.Name, 0, log)
			if err != nil {
				return prodRevs, err
			}
		}

		prodRev, err := GetProductRevision(prod, allServiceTmpls, allConfigs, allRenders, newRender, log)
		if err != nil {
			log.Error(err)
			return prodRevs, err
		}
		prodRevs = append(prodRevs, prodRev)
	}
	return prodRevs, nil
}

func GetProductRevision(product *commonmodels.Product, allServiceTmpls []*commonmodels.Service, allConfigs []*commonmodels.Config, allRender []*commonmodels.RenderSet, newRender *commonmodels.RenderSet, log *xlog.Logger) (*ProductRevision, error) {

	prodRev := new(ProductRevision)

	// 查询当前产品的最新模板信息
	productTemplatName := product.ProductName
	prodTmpl, err := templaterepo.NewProductColl().Find(productTemplatName)
	if err != nil {
		log.Errorf("[ProductTmpl.Find] %s error: %v", product.ProductName, err)
		return prodRev, e.ErrFindProductTmpl
	}

	prodRev.EnvName = product.EnvName
	prodRev.ID = product.ID.Hex()
	prodRev.ProductName = product.ProductName
	prodRev.CurrentRevision = product.Revision
	prodRev.NextRevision = prodTmpl.Revision
	prodRev.ServiceRevisions = make([]*SvcRevision, 0)
	prodRev.IsPublic = product.IsPublic

	// 如果当前产品版本比产品模板小, 则需要更新
	if prodRev.NextRevision > prodRev.CurrentRevision {
		prodRev.Updatable = true
	}

	// 交叉对比已创建的服务组和服务组模板
	prodRev.ServiceRevisions, err = compareGroupServicesRev(prodTmpl.Services, product, allServiceTmpls, allConfigs, allRender, newRender, log)
	if err != nil {
		log.Errorf("%v", err)
		return nil, e.ErrGetProductRevision.AddDesc(err.Error())
	}

	// 如果任一服务组有更新, 则认为产品需要更新
	if !prodRev.Updatable {
		prodRev.Updatable = prodRev.GroupsUpdated()
	}

	return prodRev, nil
}

// compareGroupServicesRev 服务二维数组对比revision
// parameters:
// - servicesTmpl: service templates 2-dimensional array from product template
// - product: the product environment instance
// - maxServices: distinted service and max revision
// - maxConfigs: distincted service config and max revision
func compareGroupServicesRev(servicesTmpl [][]string, productInfo *commonmodels.Product, allServiceTmpls []*commonmodels.Service, allConfigs []*commonmodels.Config, allRender []*commonmodels.RenderSet, newRender *commonmodels.RenderSet, log *xlog.Logger) ([]*SvcRevision, error) {

	var serviceRev []*SvcRevision
	svcList := make([]*commonmodels.ProductService, 0)
	svcTmplNameList := make([]string, 0)
	// 拍平服务组
	for _, services := range productInfo.Services {
		svcList = append(svcList, services...)
	}
	for _, svcsTmpl := range servicesTmpl {
		svcTmplNameList = append(svcTmplNameList, svcsTmpl...)
	}
	var err error

	serviceRev, err = compareServicesRev(svcTmplNameList, svcList, allServiceTmpls, allConfigs, allRender, newRender, log)
	if err != nil {
		log.Errorf("Failed to compare service revision. Error: %v", err)
		return serviceRev, e.ErrListProductsRevision.AddDesc(err.Error())
	}

	return serviceRev, nil
}

// compareServicesRev 拍平后服务数组对比revision
// parameters:
// - serviceTmplNames: service names from product-service template
// - services: service list of product environment instance
// - maxServices: distinted service and max revision
// - maxConfigs: distincted service config and max revision
func compareServicesRev(serviceTmplNames []string, services []*commonmodels.ProductService, allServiceTmpls []*commonmodels.Service, allConfigs []*commonmodels.Config, allRenders []*commonmodels.RenderSet, newRender *commonmodels.RenderSet, log *xlog.Logger) ([]*SvcRevision, error) {

	serviceRevs := make([]*SvcRevision, 0)

	serviceMap := make(map[string]*commonmodels.ProductService)
	for _, service := range services {
		serviceMap[service.ServiceName] = service
	}

	serviceTmplMap := make(map[string]string)
	for _, tmplName := range serviceTmplNames {
		serviceTmplMap[tmplName] = tmplName
	}

	for _, serviceTmplName := range serviceTmplNames {
		// 如果已创建的服务不包括新增的服务模板, 则认为是新增服务
		// 新增服务如果涉及多个部署方式，目前把所有部署方式都默认加进来
		if _, ok := serviceMap[serviceTmplName]; !ok {
			newServiceTmpls, err := getMaxServices(allServiceTmpls, serviceTmplName)
			if err != nil {
				log.Errorf("%v", err)
				return serviceRevs, err
			}
			for _, serviceTmpl := range newServiceTmpls {
				serviceRev := &SvcRevision{
					ServiceName:  serviceTmpl.ServiceName,
					Type:         serviceTmpl.Type,
					NextRevision: serviceTmpl.Revision,
					Updatable:    true,
					New:          true,
				}
				if serviceTmpl.Type == setting.K8SDeployType {
					serviceRev.Containers = make([]*commonmodels.Container, 0)
					for _, container := range serviceTmpl.Containers {
						serviceRev.Containers = append(serviceRev.Containers, &commonmodels.Container{
							Image: container.Image,
							Name:  container.Name,
						})
					}

					configRevs, err := getNewConfigRevs(serviceTmplName, allConfigs)
					if err != nil {
						log.Errorf("%v", err)
						return serviceRevs, err
					}
					serviceRev.ConfigRevisions = configRevs
				}
				serviceRevs = append(serviceRevs, serviceRev)
			}

		}
	}

	for _, service := range services {
		// 如果没有找到服务模板信息, 则认为服务给删除了
		if _, ok := serviceTmplMap[service.ServiceName]; !ok {
			serviceRev := &SvcRevision{
				ServiceName: service.ServiceName,
				Updatable:   true,
				Deleted:     true,
			}
			serviceRevs = append(serviceRevs, serviceRev)
		} else {
			maxServiceTmpl, err := getMaxServiceByType(allServiceTmpls, service.ServiceName, service.Type)
			if err != nil {
				log.Errorf("Failed to get max service revision. Service: %s; Type: %s; Error: %v",
					service.ServiceName, service.Type, err)
				return serviceRevs, err
			}

			currentServiceTmpl, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
				ServiceName: service.ServiceName,
				Revision:    service.Revision,
				Type:        service.Type,
			})
			if err != nil {
				log.Errorf("Failed to get service by revision. Service: %s; Type: %s; Revision: %d; Error: %v",
					service.ServiceName, service.Type, service.Revision, err)
				return serviceRevs, err
			}

			serviceRev := &SvcRevision{
				ServiceName:     service.ServiceName,
				Type:            service.Type,
				CurrentRevision: service.Revision,
				NextRevision:    maxServiceTmpl.Revision,
			}

			if serviceRev.NextRevision > serviceRev.CurrentRevision {
				serviceRev.Updatable = true
			}

			// 容器化部署方式才需要设置镜像
			// 容器化部署方式才需要对比配置文件
			if service.Type == setting.K8SDeployType {
				serviceRev.ConfigRevisions = make([]*ConfigRevision, 0)
				serviceRev.Containers = make([]*commonmodels.Container, 0)

				for _, container := range maxServiceTmpl.Containers {
					c := &commonmodels.Container{
						Image: container.Image,
						Name:  container.Name,
					}

					// reuse existed container image
					for _, exitedContainer := range service.Containers {
						if exitedContainer.Name == container.Name {
							c.Image = exitedContainer.Image
							break
						}
					}

					serviceRev.Containers = append(serviceRev.Containers, c)
				}

				oldRender := &commonmodels.RenderSet{}
				if service.Render != nil && service.Render.Name != "" {
					oldRender, err = getRenderByRevision(allRenders, service.Render.Name, service.Render.Revision)
					if err != nil {
						log.Errorf("Failed to get current render of service by revision. Service: %s, Render: %s, RenderRevision: %d, Error: %v",
							service.ServiceName, service.Render.Name, service.Render.Revision, err)
						return serviceRevs, e.ErrListProductsRevision.AddDesc(err.Error())
					}
				}
				// 交叉对比已创建的配置和待更新配置模板
				var err error
				// 检查模板yaml渲染后是否有变化
				if isRenderedStringUpdateble(currentServiceTmpl.Yaml, maxServiceTmpl.Yaml, oldRender, newRender) {
					serviceRev.Updatable = true
				}

				// 交叉对比已创建的配置和待更新配置模板
				serviceRev.ConfigRevisions, err = compareConfigsRev(service.ServiceName, service.Configs, allConfigs, oldRender, newRender)
				if err != nil {
					log.Errorf("%v", err)
					return serviceRevs, e.ErrListProductsRevision.AddDesc(err.Error())
				}

				// 如果任一配置有更新, 则认为服务需要更新
				if !serviceRev.Updatable {
					serviceRev.Updatable = serviceRev.ConfigsUpdated()
				}

				serviceRevs = append(serviceRevs, serviceRev)
			} else if service.Type == setting.HelmDeployType {
				serviceRevs = append(serviceRevs, serviceRev)
			}
		}
	}
	return serviceRevs, nil
}

// 分别获取三种type的最大revision对应的service
func getMaxServices(services []*commonmodels.Service, serviceName string) ([]*commonmodels.Service, error) {
	var (
		resp        []*commonmodels.Service
		k8sService  = &commonmodels.Service{}
		helmService = &commonmodels.Service{}
	)

	for _, service := range services {
		if service.ServiceName == serviceName && service.Type == setting.K8SDeployType && service.Revision > k8sService.Revision {
			k8sService = service
		}
		if service.ServiceName == serviceName && service.Type == setting.HelmDeployType && service.Revision > helmService.Revision {
			helmService = service
		}
	}
	if k8sService.ServiceName != "" {
		resp = append(resp, k8sService)
	}
	if helmService.ServiceName != "" {
		resp = append(resp, helmService)
	}
	if len(resp) <= 0 {
		return resp, fmt.Errorf("[%s] no service found", serviceName)
	}
	return resp, nil
}

func getNewConfigRevs(serviceTmplName string, allVers []*commonmodels.Config) ([]*ConfigRevision, error) {

	configRevs := make([]*ConfigRevision, 0)

	configMap, _ := getMaxCfgTmplMapFromAllService(serviceTmplName, allVers)
	for _, v := range configMap {
		configRev := &ConfigRevision{
			ConfigName:   v.ConfigName,
			NextRevision: v.Revision,
			Updatable:    true,
			Deleted:      false,
			New:          true,
		}
		configRevs = append(configRevs, configRev)
	}
	return configRevs, nil
}

func getMaxCfgTmplMapFromAllService(serviceName string, allVers []*commonmodels.Config) (map[string]*commonmodels.Config, error) {

	configTmplMap := make(map[string]*commonmodels.Config)

	// 待更新的配置模板Map 用于对比
	for _, ver := range allVers {
		if ver.ServiceName == serviceName {
			if tmpl, ok := configTmplMap[ver.ServiceName+ver.ConfigName]; !ok || tmpl.Revision < ver.Revision {
				configTmplMap[ver.ServiceName+ver.ConfigName] = ver
			}
		}
	}

	return configTmplMap, nil
}

func getMaxServiceByType(services []*commonmodels.Service, serviceName, serviceType string) (*commonmodels.Service, error) {
	resp := &commonmodels.Service{}
	for _, service := range services {
		if service.ServiceName == serviceName && service.Type == serviceType && service.Revision > resp.Revision {
			resp = service
		}
	}
	if resp.ServiceName == "" {
		return resp, fmt.Errorf("[%s] no service found", serviceName)
	}
	return resp, nil
}

func getRenderByRevision(renders []*commonmodels.RenderSet, renderName string, revision int64) (*commonmodels.RenderSet, error) {
	var resp *commonmodels.RenderSet
	for _, render := range renders {
		if render.Name == renderName && render.Revision == revision {
			resp = render
		}
	}
	if resp == nil {
		return resp, fmt.Errorf("[%s][%d] no renderset found", renderName, revision)
	}
	return resp, nil
}

// compareConfigTmpl 交叉对比已创建的配置和待更新的配置模板
func compareConfigsRev(serviceName string, configs []*commonmodels.ServiceConfig, allConfigs []*commonmodels.Config, oldRender *commonmodels.RenderSet, newRender *commonmodels.RenderSet) ([]*ConfigRevision, error) {

	configRevs := make([]*ConfigRevision, 0)

	// 查询所有服务的最新版本的配置模板
	configTmplMap, err := getMaxCfgTmplMapFromAllService(serviceName, allConfigs)
	if err != nil {
		return configRevs, err
	}

	// 已创建的配置Map 用于对比
	configMaps := convertServiceConfigMap(serviceName, configs)

	// 对比已创建的配置和待更新的配置模板
	for key, val := range configTmplMap {
		// 如果已创建的配置不包括新增的配置模板, 则认为配置是新增
		if _, ok := configMaps[key]; !ok {
			configRev := &ConfigRevision{
				ConfigName: val.ConfigName,
				Updatable:  true,
				New:        true,
			}
			configRevs = append(configRevs, configRev)
		}
	}

	// 对比待更新的配置模板和已创建的配置
	for key, val := range configMaps {
		configRev := &ConfigRevision{
			ConfigName:      val.ConfigName,
			CurrentRevision: val.Revision,
		}
		// 如果配置模板中没有找到已创建的配置, 则认为配置给删除
		if nextConfig, ok := configTmplMap[key]; !ok {
			configRev.Deleted = true
			configRev.Updatable = true
		} else {
			// 如果找到配置模板信息, 则对比版本
			configRev.NextRevision = configTmplMap[key].Revision
			if configRev.NextRevision > configRev.CurrentRevision {
				configRev.Updatable = true
			}
			// 判断渲染后的config有没有变化
			currentConfig, _ := getCfgTmplMapByRevision(serviceName, configRev.ConfigName, configRev.CurrentRevision, allConfigs)
			if isRenderedConfigUpdateble(currentConfig, nextConfig, oldRender, newRender) {
				configRev.Updatable = true
			}

		}
		configRevs = append(configRevs, configRev)
	}

	return configRevs, nil
}

func convertServiceConfigMap(serviceName string, in []*commonmodels.ServiceConfig) map[string]*commonmodels.ServiceConfig {
	out := make(map[string]*commonmodels.ServiceConfig)
	for _, config := range in {
		out[serviceName+config.ConfigName] = config
	}
	return out
}

func isRenderedStringUpdateble(currentString, nextString string, currentRender, nextRender *commonmodels.RenderSet) bool {
	resp := false
	currentString = commonservice.RenderValueForString(currentString, currentRender)
	nextString = commonservice.RenderValueForString(nextString, nextRender)
	if currentString != nextString {
		resp = true
	}
	return resp

}

func isRenderedConfigUpdateble(currentConfig, nextConfig *commonmodels.Config, currentRender, nextRender *commonmodels.RenderSet) bool {
	if len(currentConfig.ConfigData) != len(nextConfig.ConfigData) {
		return true
	}
	for i := range currentConfig.ConfigData {
		if commonservice.RenderValueForString(currentConfig.ConfigData[i].Value, currentRender) != commonservice.RenderValueForString(nextConfig.ConfigData[i].Value, nextRender) {
			return true
		}
	}
	return false
}

func getCfgTmplMapByRevision(serviceName, configName string, revision int64, allVers []*commonmodels.Config) (*commonmodels.Config, error) {

	configTmplMap := &commonmodels.Config{}

	// 待更新的配置模板Map 用于对比
	for _, ver := range allVers {
		if ver.ServiceName == serviceName && ver.Revision == revision && ver.ConfigName == configName {
			configTmplMap = ver
		}
	}
	return configTmplMap, nil
}
