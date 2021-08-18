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

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/poetry"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListProductsRevision(productName, envName string, userID int, superUser bool, log *zap.SugaredLogger) ([]*ProductRevision, error) {
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

		poetryCtl := poetry.New(config.PoetryAPIServer(), config.PoetryAPIRootKey())
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

		prodRev, err := GetProductRevision(prod, allServiceTmpls, allRenders, newRender, log)
		if err != nil {
			log.Error(err)
			return prodRevs, err
		}
		prodRevs = append(prodRevs, prodRev)
	}
	return prodRevs, nil
}

func ListProductsRevisionByFacility(basicFacility string, log *zap.SugaredLogger) ([]*ProductRevision, error) {
	var (
		err          error
		prodRevs     = make([]*ProductRevision, 0)
		products     = make([]*commonmodels.Product, 0)
		projectNames = make([]string, 0)
	)

	temProducts, err := templaterepo.NewProductColl().ListWithOption(&templaterepo.ProductListOpt{BasicFacility: basicFacility})
	if err != nil {
		log.Errorf("Collection.TemplateProduct.List error: %s", err)
		return prodRevs, e.ErrListProducts.AddDesc(err.Error())
	}

	for _, v := range temProducts {
		projectNames = append(projectNames, v.ProductName)
	}

	products, err = commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{ExcludeStatus: setting.ProductStatusDeleting, InProjects: projectNames})
	if err != nil {
		log.Errorf("Collection.Product.List error: %s", err)
		return prodRevs, e.ErrListProducts.AddDesc(err.Error())
	}

	// 获取所有服务模板最新模板信息
	allServiceTmpls, err := commonrepo.NewServiceColl().ListAllRevisions()
	if err != nil {
		log.Errorf("ListAllRevisions error: %s", err)
		return prodRevs, e.ErrListProducts.AddDesc(err.Error())
	}

	// 获取所有渲染配置最新模板信息
	allRenders, err := commonrepo.NewRenderSetColl().ListAllRenders()
	if err != nil {
		log.Errorf("ListAllRevisions error: %s", err)
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

		prodRev, err := GetProductRevision(prod, allServiceTmpls, allRenders, newRender, log)
		if err != nil {
			log.Error(err)
			return prodRevs, err
		}
		prodRevs = append(prodRevs, prodRev)
	}
	return prodRevs, nil
}

func GetProductRevision(product *commonmodels.Product, allServiceTmpls []*commonmodels.Service, allRender []*commonmodels.RenderSet, newRender *commonmodels.RenderSet, log *zap.SugaredLogger) (*ProductRevision, error) {

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

	if product.Source == setting.SourceFromExternal {
		return prodRev, nil
	}

	// 如果当前产品版本比产品模板小, 则需要更新
	if prodRev.NextRevision > prodRev.CurrentRevision {
		prodRev.Updatable = true
	}

	// 交叉对比已创建的服务组和服务组模板
	prodRev.ServiceRevisions, err = compareGroupServicesRev(prodTmpl.Services, product, allServiceTmpls, allRender, newRender, log)
	if err != nil {
		log.Error(err)
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
func compareGroupServicesRev(servicesTmpl [][]string, productInfo *commonmodels.Product, allServiceTmpls []*commonmodels.Service, allRender []*commonmodels.RenderSet, newRender *commonmodels.RenderSet, log *zap.SugaredLogger) ([]*SvcRevision, error) {

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

	serviceRev, err = compareServicesRev(svcTmplNameList, svcList, allServiceTmpls, allRender, newRender, log)
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
func compareServicesRev(serviceTmplNames []string, services []*commonmodels.ProductService, allServiceTmpls []*commonmodels.Service, allRenders []*commonmodels.RenderSet, newRender *commonmodels.RenderSet, log *zap.SugaredLogger) ([]*SvcRevision, error) {

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
				log.Error(err)
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
			maxServiceTmpl, err := getMaxServiceRevision(allServiceTmpls, service.ServiceName, service.ProductName)
			if err != nil {
				log.Errorf("Failed to get max service revision. Service: %s; Prodcut: %s; Error: %v",
					service.ServiceName, service.ProductName, err)
				return serviceRevs, err
			}

			currentServiceTmpl, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
				ServiceName: service.ServiceName,
				ProductName: service.ProductName,
				Revision:    service.Revision,
				Type:        service.Type,
			})
			if err != nil {
				log.Errorf("Failed to get service by revision. Service: %s; Product: %s; Type: %s; Revision: %d; Error: %v",
					service.ServiceName, service.ProductName, service.Type, service.Revision, err)
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
				// 检查模板yaml渲染后是否有变化
				if isRenderedStringUpdateble(currentServiceTmpl.Yaml, maxServiceTmpl.Yaml, oldRender, newRender) {
					serviceRev.Updatable = true
				}

				serviceRevs = append(serviceRevs, serviceRev)
			} else if service.Type == setting.HelmDeployType {
				serviceRevs = append(serviceRevs, serviceRev)
			} else if service.Type == setting.PMDeployType {
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
		pmService   = &commonmodels.Service{}
	)

	for _, service := range services {
		if service.ServiceName == serviceName && service.Type == setting.K8SDeployType && service.Revision > k8sService.Revision {
			k8sService = service
		}
		if service.ServiceName == serviceName && service.Type == setting.PMDeployType && service.Revision > pmService.Revision {
			pmService = service
		}
		if service.ServiceName == serviceName && service.Type == setting.HelmDeployType && service.Revision > helmService.Revision {
			helmService = service
		}
	}
	if k8sService.ServiceName != "" {
		resp = append(resp, k8sService)
	}
	if pmService.ServiceName != "" {
		resp = append(resp, pmService)
	}
	if helmService.ServiceName != "" {
		resp = append(resp, helmService)
	}
	if len(resp) <= 0 {
		return resp, fmt.Errorf("[%s] no service found", serviceName)
	}
	return resp, nil
}

func getMaxServiceRevision(services []*commonmodels.Service, serviceName, productName string) (*commonmodels.Service, error) {
	resp := &commonmodels.Service{}
	for _, service := range services {
		if service.ServiceName == serviceName && service.ProductName == productName && service.Revision > resp.Revision {
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

func isRenderedStringUpdateble(currentString, nextString string, currentRender, nextRender *commonmodels.RenderSet) bool {
	resp := false
	currentString = commonservice.RenderValueForString(currentString, currentRender)
	nextString = commonservice.RenderValueForString(nextString, nextRender)
	if currentString != nextString {
		resp = true
	}
	return resp

}
