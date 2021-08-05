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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/strvals"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/gerrit"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/shared/poetry"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/types/permission"
	"github.com/koderover/zadig/pkg/util/converter"
)

const (
	Timeout          = 60
	UpdateTypeSystem = "systemVar"
	UpdateTypeEnv    = "envVar"
)

type EnvStatus struct {
	EnvName    string `json:"env_name,omitempty"`
	Status     string `json:"status"`
	ErrMessage string `json:"err_message"`
}

type ProductResp struct {
	ID          string                   `json:"id"`
	ProductName string                   `json:"product_name"`
	Namespace   string                   `json:"namespace"`
	Status      string                   `json:"status"`
	Error       string                   `json:"error"`
	EnvName     string                   `json:"env_name"`
	UpdateBy    string                   `json:"update_by"`
	UpdateTime  int64                    `json:"update_time"`
	Services    [][]string               `json:"services"`
	Render      *commonmodels.RenderInfo `json:"render"`
	Vars        []*template.RenderKV     `json:"vars"`
	IsPublic    bool                     `json:"isPublic"`
	ClusterID   string                   `json:"cluster_id,omitempty"`
	RecycleDay  int                      `json:"recycle_day"`
	IsProd      bool                     `json:"is_prod"`
	Source      string                   `json:"source"`
}

type ProductParams struct {
	IsPublic        bool     `json:"isPublic"`
	EnvName         string   `json:"envName"`
	RoleID          int      `json:"roleId"`
	PermissionUUIDs []string `json:"permissionUUIDs"`
}

func UpdateProductPublic(productName string, args *ProductParams, log *zap.SugaredLogger) error {
	err := commonrepo.NewProductColl().UpdateIsPublic(args.EnvName, productName, args.IsPublic)
	if err != nil {
		log.Errorf("UpdateProductPublic error: %v", err)
		return fmt.Errorf("UpdateProductPublic error: %v", err)
	}

	poetryCtl := poetry.New(config.PoetryAPIServer(), config.PoetryAPIRootKey())
	if !args.IsPublic { //把公开设置成不公开
		_, err := poetryCtl.AddEnvRolePermission(productName, args.EnvName, args.PermissionUUIDs, args.RoleID, log)
		if err != nil {
			log.Errorf("UpdateProductPublic AddEnvRole error: %v", err)
			return fmt.Errorf("UpdateProductPublic AddEnvRole error: %v", err)
		}
		return nil
	}
	//把不公开设成公开 删除原来环境绑定的角色
	_, err = poetryCtl.DeleteEnvRolePermission(productName, args.EnvName, log)
	if err != nil {
		log.Errorf("UpdateProductPublic DeleteEnvRole error: %v", err)
		return fmt.Errorf("UpdateProductPublic DeleteEnvRole error: %v", err)
	}

	return nil
}

func ListProducts(productNameParam, envType string, userName string, userID int, superUser bool, log *zap.SugaredLogger) ([]*ProductResp, error) {
	var (
		err               error
		testResp          []*ProductResp
		prodResp          []*ProductResp
		products          = make([]*commonmodels.Product, 0)
		productNameMap    map[string][]int64
		productNamespaces = sets.NewString()
	)
	resp := make([]*ProductResp, 0)

	poetryCtl := poetry.New(config.PoetryAPIServer(), config.PoetryAPIRootKey())

	// 获取所有产品
	if superUser {
		products, err = commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: productNameParam})
		if err != nil {
			log.Errorf("[%s] Collections.Product.List error: %v", userName, err)
			return resp, e.ErrListEnvs.AddDesc(err.Error())
		}
	} else {
		//项目下所有公开环境
		publicProducts, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{IsPublic: true})
		if err != nil {
			log.Errorf("Collection.Product.List List product error: %v", err)
			return resp, e.ErrListProducts.AddDesc(err.Error())
		}
		for _, publicProduct := range publicProducts {
			if productNameParam == "" {
				products = append(products, publicProduct)
				productNamespaces.Insert(publicProduct.Namespace)
			} else if publicProduct.ProductName == productNameParam {
				products = append(products, publicProduct)
				productNamespaces.Insert(publicProduct.Namespace)
			}
		}

		productNameMap, err = poetryCtl.GetUserProject(userID, log)
		if err != nil {
			log.Errorf("Collection.Product.List GetUserProject error: %v", err)
			return resp, e.ErrListProducts.AddDesc(err.Error())
		}
		for productName, roleIDs := range productNameMap {
			//用户关联角色所关联的环境
			for _, roleID := range roleIDs {
				if roleID == setting.RoleOwnerID {
					tmpProducts, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: productName})
					if err != nil {
						log.Errorf("Collection.Product.List Find product error: %v", err)
						return resp, e.ErrListProducts.AddDesc(err.Error())
					}
					for _, product := range tmpProducts {
						if productNameParam == "" {
							if !productNamespaces.Has(product.Namespace) {
								products = append(products, product)
							}
						} else if product.ProductName == productNameParam {
							if !productNamespaces.Has(product.Namespace) {
								products = append(products, product)
							}
						}
					}
				} else {
					productMap := make(map[string]int)
					// 先列出环境-用户授权
					userEnvPermissionList, err := poetryCtl.GetUserEnvPermission(userID, log)
					if err != nil {
						log.Errorf("failed to get user env permission, err: %v", err)
						return resp, err
					}
					for _, userEnvPermission := range userEnvPermissionList {
						product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: userEnvPermission.EnvName})
						if err != nil {
							log.Errorf("Collection.Product.List Find product error: %v", err)
							continue
						}
						if productNameParam == "" {
							if !productNamespaces.Has(product.Namespace) && userEnvPermission.PermissionUUID == permission.TestEnvListUUID {
								products = append(products, product)
								productNamespaces = productNamespaces.Insert(product.Namespace)
								productMap[product.EnvName] = 1
							}
						} else if product.ProductName == productNameParam {
							if !productNamespaces.Has(product.Namespace) && userEnvPermission.PermissionUUID == permission.TestEnvManageUUID {
								products = append(products, product)
								productNamespaces = productNamespaces.Insert(product.Namespace)
								productMap[product.EnvName] = 1
							}
						}
					}
					// 再获取环境-角色授权
					roleEnvPermissions, err := poetryCtl.ListEnvRolePermission(productName, "", roleID, log)
					if err != nil {
						log.Errorf("Collection.Product.List ListRoleEnvs error: %v", err)
						return resp, e.ErrListProducts.AddDesc(err.Error())
					}
					for _, roleEnvPermission := range roleEnvPermissions {
						product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: roleEnvPermission.EnvName})
						if err != nil {
							log.Errorf("Collection.Product.List Find product error: %v", err)
							continue
						}
						if productNameParam == "" {
							if !productNamespaces.Has(product.Namespace) && roleEnvPermission.PermissionUUID == permission.TestEnvListUUID {
								products = append(products, product)
								productNamespaces.Insert(product.Namespace)
							}
						} else if product.ProductName == productNameParam {
							if !productNamespaces.Has(product.Namespace) && roleEnvPermission.PermissionUUID == permission.TestEnvManageUUID {
								products = append(products, product)
								productNamespaces.Insert(product.Namespace)
							}
						}
					}
				}
			}
		}
	}

	err = FillProductVars(products, log)
	if err != nil {
		return resp, err
	}

	for _, prod := range products {
		product := &ProductResp{
			ID:          prod.ID.Hex(),
			ProductName: prod.ProductName,
			EnvName:     prod.EnvName,
			Namespace:   prod.Namespace,
			Vars:        prod.Vars[:],
			IsPublic:    prod.IsPublic,
			ClusterID:   prod.ClusterID,
			UpdateTime:  prod.UpdateTime,
			UpdateBy:    prod.UpdateBy,
			RecycleDay:  prod.RecycleDay,
			Render:      prod.Render,
			Source:      prod.Source,
		}

		if prod.ClusterID != "" {
			cluster, _ := commonrepo.NewK8SClusterColl().Get(prod.ClusterID)
			if cluster != nil && cluster.Production {
				product.IsProd = true
				operatorPerm := poetryCtl.HasOperatePermission(prod.ProductName, permission.ProdEnvManageUUID, userID, superUser, log)
				viewPerm := poetryCtl.HasOperatePermission(prod.ProductName, permission.ProdEnvListUUID, userID, superUser, log)
				if envType == "" && (operatorPerm || viewPerm) {
					prodResp = append(prodResp, product)
				} else if envType == setting.ProdENV {
					prodResp = append(prodResp, product)
				}
			} else if cluster != nil && !cluster.Production {
				product.IsProd = false
				testResp = append(testResp, product)
			}
		} else {
			product.IsProd = false
			testResp = append(testResp, product)
		}
	}
	switch envType {
	case setting.ProdENV:
		resp = append(resp, prodResp...)
	case setting.TestENV:
		resp = append(resp, testResp...)
	default:
		resp = append(resp, prodResp...)
		resp = append(resp, testResp...)
	}

	sort.SliceStable(resp, func(i, j int) bool { return resp[i].ProductName < resp[j].ProductName })

	return resp, nil
}

func FillProductVars(products []*commonmodels.Product, log *zap.SugaredLogger) error {
	for _, product := range products {
		if product.Source == setting.SourceFromExternal || product.Source == setting.SourceFromHelm {
			continue
		}
		renderName := product.Namespace
		var revision int64
		// if the environment is backtracking, render.name will be different with product.Namespace
		if product.Render != nil && product.Render.Name != renderName {
			renderName = product.Render.Name
			revision = product.Render.Revision
		}
		renderSet, err := commonservice.GetRenderSet(renderName, revision, log)
		if err != nil {
			log.Errorf("Failed to find render set, productName: %s, namespace: %s,  err: %s", product.ProductName, product.Namespace, err)
			return e.ErrGetRenderSet.AddDesc(err.Error())
		}
		product.Vars = renderSet.KVs[:]
	}

	return nil
}

var mutexAutoCreate sync.RWMutex

// 自动创建环境
func AutoCreateProduct(productName, envType, requestID string, log *zap.SugaredLogger) []*EnvStatus {
	envStatuses := make([]*EnvStatus, 0)
	productObject, err := GetInitProduct(productName, log)
	if err != nil {
		log.Errorf("AutoCreateProduct err:%v", err)
		return envStatuses
	}
	if envType == setting.HelmDeployType {
		productObject.Source = setting.HelmDeployType
	}
	mutexAutoCreate.Lock()
	defer func() {
		mutexAutoCreate.Unlock()
	}()

	productObject.IsPublic = true
	productMap := make(map[string]string)
	productResps := make([]*ProductResp, 0)
	envNames := []string{"dev", "qa"}
	for _, envName := range envNames {
		productMap[envName] = envName
		productResp, err := GetProduct(setting.SystemUser, envName, productName, log)
		if err == nil && productResp != nil {
			productResps = append(productResps, productResp)
			delete(productMap, envName)
		}
	}
	switch len(productResps) {
	case 0:
		for _, envName := range envNames {
			tempProductObj := *productObject
			tempProductObj.Namespace = commonservice.GetProductEnvNamespace(envName, productName)
			tempProductObj.UpdateBy = setting.SystemUser
			tempProductObj.EnvName = envName
			err = CreateProduct(setting.SystemUser, requestID, &tempProductObj, log)
			if err != nil {
				_, messageMap := e.ErrorMessage(err)
				if errMessage, isExist := messageMap["description"]; isExist {
					if message, ok := errMessage.(string); ok {
						envStatuses = append(envStatuses, &EnvStatus{EnvName: envName, Status: setting.ProductStatusFailed, ErrMessage: message})
						continue
					}
				}
			}
			envStatuses = append(envStatuses, &EnvStatus{EnvName: envName, Status: setting.ProductStatusCreating})
		}
	case 1:
		for _, productResp := range productResps {
			if productResp.Error != "" {
				envStatuses = append(envStatuses, &EnvStatus{EnvName: productResp.EnvName, Status: setting.ProductStatusFailed, ErrMessage: productResp.Error})
				continue
			}
			envStatuses = append(envStatuses, &EnvStatus{EnvName: productResp.EnvName, Status: productResp.Status})
		}
		for envName := range productMap {
			productObject.Namespace = commonservice.GetProductEnvNamespace(envName, productName)
			productObject.UpdateBy = setting.SystemUser
			productObject.EnvName = envName
			err = CreateProduct(setting.SystemUser, requestID, productObject, log)
			if err != nil {
				_, messageMap := e.ErrorMessage(err)
				if errMessage, isExist := messageMap["description"]; isExist {
					if message, ok := errMessage.(string); ok {
						envStatuses = append(envStatuses, &EnvStatus{EnvName: envName, Status: setting.ProductStatusFailed, ErrMessage: message})
						continue
					}
				}
			}
			envStatuses = append(envStatuses, &EnvStatus{EnvName: envName, Status: setting.ProductStatusCreating})
		}
	case 2:
		for _, productResp := range productResps {
			if productResp.Error != "" {
				envStatuses = append(envStatuses, &EnvStatus{EnvName: productResp.EnvName, Status: setting.ProductStatusFailed, ErrMessage: productResp.Error})
				continue
			}
			envStatuses = append(envStatuses, &EnvStatus{EnvName: productResp.EnvName, Status: productResp.Status})
		}
	}
	return envStatuses
}

var mutexAutoUpdate sync.RWMutex

func AutoUpdateProduct(envNames []string, productName string, userID int, superUser bool, requestID string, force bool, log *zap.SugaredLogger) ([]*EnvStatus, error) {
	mutexAutoUpdate.Lock()
	defer func() {
		mutexAutoUpdate.Unlock()
	}()

	envStatuses := make([]*EnvStatus, 0)

	if !force {
		modifiedByENV := make(map[string][]*serviceInfo)
		for _, env := range envNames {
			p, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: env})
			if err != nil {
				log.Errorf("Failed to get product %s in %s, error: %v", productName, env, err)
				continue
			}

			kubeClient, err := kube.GetKubeClient(p.ClusterID)
			if err != nil {
				log.Errorf("Failed to get kube client for %s, error: %v", productName, err)
				continue
			}

			modifiedServices := getModifiedServiceFromObjectMetaList(kube.GetDirtyResources(p.Namespace, kubeClient))
			if len(modifiedServices) > 0 {
				modifiedByENV[env] = modifiedServices
			}
		}
		if len(modifiedByENV) > 0 {
			data, err := json.Marshal(modifiedByENV)
			if err != nil {
				log.Errorf("Marshal failure: %v", err)
			}
			return envStatuses, fmt.Errorf("the following services are modified since last update: %s", data)
		}
	}

	productsRevison, err := ListProductsRevision(productName, "", userID, superUser, log)
	if err != nil {
		log.Errorf("AutoUpdateProduct ListProductsRevision err:%v", err)
		return envStatuses, err
	}
	productMap := make(map[string]*ProductRevision)
	for _, productRevison := range productsRevison {
		if productRevison.ProductName == productName && sets.NewString(envNames...).Has(productRevison.EnvName) && productRevison.Updatable {
			productMap[productRevison.EnvName] = productRevison
			if len(productMap) == len(envNames) {
				break
			}
		}
	}

	for envName := range productMap {
		productInfo, err := GetProduct(setting.SystemUser, envName, productName, log)
		if err != nil {
			log.Errorf("AutoUpdateProduct GetProduct err:%v", err)
			return envStatuses, err
		}
		err = UpdateProductV2(envName, productName, setting.SystemUser, requestID, true, productInfo.Vars, log)
		if err != nil {
			log.Errorf("AutoUpdateProduct UpdateProductV2 err:%v", err)
			return envStatuses, err
		}
	}

	productResps := make([]*ProductResp, 0)
	for _, envName := range envNames {
		productResp, err := GetProduct(setting.SystemUser, envName, productName, log)
		if err == nil && productResp != nil {
			productResps = append(productResps, productResp)
		}
	}

	for _, productResp := range productResps {
		if productResp.Error != "" {
			envStatuses = append(envStatuses, &EnvStatus{EnvName: productResp.EnvName, Status: setting.ProductStatusFailed, ErrMessage: productResp.Error})
			continue
		}
		envStatuses = append(envStatuses, &EnvStatus{EnvName: productResp.EnvName, Status: productResp.Status})
	}
	return envStatuses, nil

}

func UpdateProduct(existedProd, updateProd *commonmodels.Product, renderSet *commonmodels.RenderSet, log *zap.SugaredLogger) (err error) {
	// 设置产品新的renderinfo
	updateProd.Render = &commonmodels.RenderInfo{
		Name:        renderSet.Name,
		Revision:    renderSet.Revision,
		ProductTmpl: renderSet.ProductTmpl,
		Description: renderSet.Description,
	}
	productName := existedProd.ProductName
	envName := existedProd.EnvName
	namespace := existedProd.Namespace
	updateProd.EnvName = existedProd.EnvName
	updateProd.Namespace = existedProd.Namespace

	var allServices []*commonmodels.Service
	var allConfigs []*commonmodels.Config
	var allRenders []*commonmodels.RenderSet
	var prodRevs *ProductRevision

	allServices, err = commonrepo.NewServiceColl().ListAllRevisions()
	if err != nil {
		log.Errorf("ListAllRevisions error: %v", err)
		err = e.ErrUpdateEnv.AddDesc(err.Error())
		return
	}

	allConfigs, err = commonrepo.NewConfigColl().ListAllRevisions()
	if err != nil {
		log.Errorf("ListAllRevisions error: %v", err)
		err = e.ErrUpdateEnv.AddDesc(err.Error())
		return
	}
	// 获取所有渲染配置最新模板信息
	allRenders, err = commonrepo.NewRenderSetColl().ListAllRenders()
	if err != nil {
		log.Errorf("ListAllRevisions error: %v", err)
		err = e.ErrUpdateEnv.AddDesc(err.Error())
		return
	}

	prodRevs, err = GetProductRevision(existedProd, allServices, allConfigs, allRenders, renderSet, log)
	if err != nil {
		err = e.ErrUpdateEnv.AddDesc(e.GetEnvRevErrMsg)
		return
	}

	// 无需更新
	if !prodRevs.Updatable {
		log.Errorf("[%s][P:%s] nothing to update", envName, productName)
		return
	}

	kubeClient, err := kube.GetKubeClient(existedProd.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	// 遍历产品环境和产品模板交叉对比的结果
	// 四个状态：待删除，待添加，待更新，无需更新

	// 1. 如果服务待删除：将产品模板中已经不存在，产品环境中待删除的服务进行删除。
	for _, serviceRev := range prodRevs.ServiceRevisions {
		if serviceRev.Updatable && serviceRev.Deleted {
			log.Infof("[%s][P:%s][S:%s] start to delete service", envName, productName, serviceRev.ServiceName)
			//根据namespace: EnvName, selector: productName + serviceName来删除属于该服务的所有资源
			selector := labels.Set{setting.ProductLabel: productName, setting.ServiceLabel: serviceRev.ServiceName}.AsSelector()
			err = commonservice.DeleteResourcesAsync(namespace, selector, kubeClient, log)
			if err != nil {
				//删除失败仅记录失败日志
				log.Errorf("delete resource of service %s error:%v", serviceRev.ServiceName, err)
			}

			clusterSelector := labels.Set{setting.ProductLabel: productName, setting.ServiceLabel: serviceRev.ServiceName, setting.EnvNameLabel: envName}.AsSelector()
			err = commonservice.DeleteClusterResourceAsync(clusterSelector, kubeClient, log)
			if err != nil {
				//删除失败仅记录失败日志
				log.Errorf("delete cluster resource of service %s error:%v", serviceRev.ServiceName, err)
			}
		}
	}

	// 转化prodRevs.ServiceRevisions为serviceName+serviceType:serviceRev的map
	// 不在遍历到每个服务时再次进行遍历
	serviceRevisionMap := getServiceRevisionMap(prodRevs.ServiceRevisions)

	// 首先更新一次数据库，将产品模板的最新编排更新到数据库
	// 只更新编排，不更新服务revision等信息
	updatedServices := getUpdatedProductServices(updateProd, serviceRevisionMap, existedProd)

	updateProd.Status = setting.ProductStatusUpdating
	updateProd.Services = updatedServices

	log.Infof("[Namespace:%s][Product:%s]: update service orchestration in product. Status: %s", envName, productName, updateProd.Status)
	if err = commonrepo.NewProductColl().Update(updateProd); err != nil {
		log.Errorf("[Namespace:%s][Product:%s] Product.Update error: %v", envName, productName, err)
		err = e.ErrUpdateEnv.AddErr(err)
		return
	}

	existedServices := existedProd.GetServiceMap()

	// 按照产品模板的顺序来创建或者更新服务
	for groupIndex, prodServiceGroup := range updateProd.Services {
		//Mark if there is k8s type service in this group
		groupServices := make([]*commonmodels.ProductService, 0)
		var wg sync.WaitGroup
		var lock sync.Mutex
		errList := &multierror.Error{
			ErrorFormat: func(es []error) string {
				points := make([]string, len(es))
				for i, err := range es {
					points[i] = fmt.Sprintf("%v", err)
				}

				return strings.Join(points, "\n")
			},
		}

		for _, prodService := range prodServiceGroup {
			svcRev, ok := serviceRevisionMap[prodService.ServiceName+prodService.Type]
			if !ok {
				continue
			}
			// 服务需要更新，需要upsert
			// 所有服务全部upsert一遍，确保所有服务起来
			if svcRev.Updatable {
				log.Infof("[Namespace:%s][Product:%s][Service:%s][IsNew:%v] upsert service",
					envName, productName, svcRev.ServiceName, svcRev.New)

				service := &commonmodels.ProductService{
					ServiceName: svcRev.ServiceName,
					Type:        svcRev.Type,
					Revision:    svcRev.NextRevision,
				}

				service.Containers = svcRev.Containers
				service.Configs = make([]*commonmodels.ServiceConfig, 0)
				service.Render = updateProd.Render

				if svcRev.Type == setting.K8SDeployType {
					wg.Add(1)
					go func() {
						defer wg.Done()

						_, err := upsertService(
							existedServices[service.ServiceName] != nil,
							updateProd,
							service,
							existedServices[service.ServiceName],
							renderSet, kubeClient, log)

						if err != nil {
							lock.Lock()
							switch e := err.(type) {
							case *multierror.Error:
								errList = multierror.Append(errList, e.Errors...)
							default:
								errList = multierror.Append(errList, e)
							}
							lock.Unlock()
						}
					}()
				}
				groupServices = append(groupServices, service)
			} else {
				prodService.Containers = svcRev.Containers
				var configs []*commonmodels.ServiceConfig
				for _, configRev := range svcRev.ConfigRevisions {
					configs = append(configs, &commonmodels.ServiceConfig{
						ConfigName: configRev.ConfigName,
						Revision:   configRev.CurrentRevision,
					})
				}
				prodService.Configs = configs
				prodService.Render = updateProd.Render
				groupServices = append(groupServices, prodService)
			}
		}
		wg.Wait()
		// 如果创建依赖服务组有返回错误, 停止等待
		if err = errList.ErrorOrNil(); err != nil {
			log.Error(err)
			err = e.ErrUpdateEnv.AddDesc(err.Error())
			return
		}
		err = commonrepo.NewProductColl().UpdateGroup(envName, productName, groupIndex, groupServices)
		if err != nil {
			log.Errorf("Failed to update collection - service group %d. Error: %v", groupIndex, err)
			err = e.ErrUpdateEnv.AddDesc(err.Error())
			return
		}
	}

	if err = restartServicesByChange(envName, prodRevs, log); err != nil {
		log.Errorf("[%s] restartServicesByChange error: %v", envName, err)
		err = e.ErrUpdateEnv.AddDesc(e.RestartServiceErrMsg)
		return
	}

	return nil
}

func UpdateProductV2(envName, productName, user, requestID string, force bool, kvs []*template.RenderKV, log *zap.SugaredLogger) (err error) {
	// 根据产品名称和产品创建者到数据库中查找已有产品记录
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	exitedProd, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[%s][P:%s] Product.FindByOwner error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.EnvNotFoundErrMsg)
	}

	kubeClient, err := kube.GetKubeClient(exitedProd.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	if !force {
		modifiedServices := getModifiedServiceFromObjectMetaList(kube.GetDirtyResources(exitedProd.Namespace, kubeClient))
		if len(modifiedServices) > 0 {
			data, err := json.Marshal(modifiedServices)
			if err != nil {
				log.Errorf("Marshal failure: %v", err)
			}
			return fmt.Errorf("the following services are modified since last update: %s", data)
		}
	}

	err = ensureKubeEnv(exitedProd.Namespace, kubeClient, log)

	if err != nil {
		log.Errorf("[%s][P:%s] service.UpdateProductV2 create kubeEnv error: %v", envName, productName, err)
		return err
	}

	err = commonservice.CreateRenderSet(
		&commonmodels.RenderSet{
			Name:        exitedProd.Namespace,
			EnvName:     envName,
			ProductTmpl: productName,
			KVs:         kvs,
		},
		log,
	)

	if err != nil {
		log.Errorf("[%s][P:%s] create renderset error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.FindProductTmplErrMsg)
	}

	// 检查renderinfo是否为空(适配历史product)
	if exitedProd.Render == nil {
		exitedProd.Render = &commonmodels.RenderInfo{ProductTmpl: exitedProd.ProductName}
	}

	// 检查renderset是否覆盖产品所有key
	renderSet, err := commonservice.ValidateRenderSet(exitedProd.ProductName, exitedProd.Render.Name, "", log)
	if err != nil {
		log.Errorf("[%s][P:%s] validate product renderset error: %v", envName, exitedProd.ProductName, err)
		return e.ErrUpdateEnv.AddDesc(err.Error())
	}

	log.Infof("[%s][P:%s] UpdateProduct", envName, productName)

	// 查找产品模板
	updateProd, err := GetInitProduct(productName, log)
	if err != nil {
		log.Errorf("[%s][P:%s] GetProductTemplate error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.FindProductTmplErrMsg)
	}

	switch exitedProd.Status {
	case setting.ProductStatusCreating, setting.ProductStatusUpdating, setting.ProductStatusDeleting:
		log.Errorf("[%s][P:%s] Product is not in valid status", envName, productName)
		return e.ErrUpdateEnv.AddDesc(e.EnvCantUpdatedMsg)
	default:
		// do nothing
	}

	// 设置产品状态为更新中
	if err := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUpdating); err != nil {
		log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.UpdateEnvStatusErrMsg)
	}

	go func() {
		err := UpdateProduct(exitedProd, updateProd, renderSet, log)
		if err != nil {
			log.Errorf("[%s][P:%s] failed to update product %#v", envName, productName, err)
			// 发送更新产品失败消息给用户
			title := fmt.Sprintf("更新 [%s] 的 [%s] 环境失败", productName, envName)
			commonservice.SendErrorMessage(user, title, requestID, err, log)

			// 设置产品状态
			log.Infof("[%s][P:%s] update status to => %s", envName, productName, setting.ProductStatusFailed)
			if err2 := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusFailed); err2 != nil {
				log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err2)
				return
			}

			log.Infof("[%s][P:%s] update error to => %s", envName, productName, e.String(err))
			if err2 := commonrepo.NewProductColl().UpdateErrors(envName, productName, e.String(err)); err2 != nil {
				log.Errorf("[%s][P:%s] Product.UpdateErrors error: %v", envName, productName, err2)
				return
			}
		} else {
			updateProd.Status = setting.ProductStatusSuccess

			if err = commonrepo.NewProductColl().UpdateStatus(envName, productName, updateProd.Status); err != nil {
				log.Errorf("[%s][%s] Product.Update error: %v", envName, productName, err)
				return
			}

			if err = commonrepo.NewProductColl().UpdateErrors(envName, productName, ""); err != nil {
				log.Errorf("[%s][P:%s] Product.UpdateErrors error: %v", envName, productName, err)
				return
			}
		}
	}()
	return nil
}

// CreateProduct create a new product with its dependent stacks
func CreateProduct(user, requestID string, args *commonmodels.Product, log *zap.SugaredLogger) (err error) {
	log.Infof("[%s][P:%s] CreateProduct", args.EnvName, args.ProductName)

	switch args.Source {
	case setting.SourceFromExternal:
		args.Status = setting.ProductStatusUnstable
		args.RecycleDay = config.DefaultRecycleDay()
		err = commonrepo.NewProductColl().Create(args)
		if err != nil {
			log.Errorf("[%s][%s] create product record error: %v", args.EnvName, args.ProductName, err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}
	case setting.SourceFromHelm:
		kubeClient, err := kube.GetKubeClient(args.ClusterID)
		if err != nil {
			log.Errorf("[%s][%s] GetKubeClient error: %v", args.EnvName, args.ProductName, err)
			return e.ErrCreateEnv.AddErr(err)
		}

		//判断namespace是否存在
		namespace := args.GetNamespace()
		args.Namespace = namespace
		_, found, err := getter.GetNamespace(namespace, kubeClient)
		if err != nil {
			log.Errorf("GetNamespace error: %v", err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}
		if found {
			log.Warnf("%s[%s]%s", "namespace", namespace, "已经存在,请换个环境名称尝试!")
			return e.ErrCreateEnv.AddDesc(fmt.Sprintf("%s[%s]%s", "namespace", namespace, "已经存在,请换个环境名称尝试!"))
		}

		restConfig, err := kube.GetRESTConfig(args.ClusterID)
		if err != nil {
			log.Errorf("GetRESTConfig error: %v", err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}
		helmClient, err := helmclient.NewClientFromRestConf(restConfig, namespace)
		if err != nil {
			log.Errorf("[%s][%s] NewClientFromRestConf error: %v", args.EnvName, args.ProductName, err)
			return e.ErrCreateEnv.AddErr(err)
		}

		if err := preCreateProduct(args.EnvName, args, kubeClient, log); err != nil {
			log.Errorf("CreateProduct preCreateProduct error: %v", err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}
		eventStart := time.Now().Unix()
		if args.Render == nil {
			args.Render = &commonmodels.RenderInfo{ProductTmpl: args.ProductName}
		}

		renderSet, err := FindHelmRenderSet(args.ProductName, args.Render.Name, log)
		if err != nil {
			log.Errorf("[%s][P:%s] find product renderset error: %v", args.EnvName, args.ProductName, err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}
		// 设置产品render revsion
		args.Render.Revision = renderSet.Revision
		// 记录服务当前对应render版本
		setServiceRender(args)

		args.Status = setting.ProductStatusCreating
		args.RecycleDay = config.DefaultRecycleDay()
		if args.IsForkedProduct {
			args.RecycleDay = 7
		}
		err = commonrepo.NewProductColl().Create(args)
		if err != nil {
			log.Errorf("[%s][%s] create product record error: %v", args.EnvName, args.ProductName, err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}

		go installOrUpdateHelmChart(user, args.EnvName, requestID, args, eventStart, helmClient, log)
	default:
		kubeClient, err := kube.GetKubeClient(args.ClusterID)
		if err != nil {
			return e.ErrCreateEnv.AddErr(err)
		}

		//判断namespace是否存在
		namespace := args.GetNamespace()
		args.Namespace = namespace
		_, found, err := getter.GetNamespace(namespace, kubeClient)
		if err != nil {
			log.Errorf("GetNamespace error: %v", err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}
		if found {
			return e.ErrCreateEnv.AddDesc(fmt.Sprintf("%s[%s]%s", "namespace", namespace, "已经存在,请换个环境名称尝试!"))
		}

		//创建角色环境之间的关联关系
		//todo 创建环境暂时不指定角色
		// 检查是否重复创建（TO BE FIXED）;检查k8s集群设置: Namespace/Secret .etc
		if err := preCreateProduct(args.EnvName, args, kubeClient, log); err != nil {
			log.Errorf("CreateProduct preCreateProduct error: %v", err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}

		eventStart := time.Now().Unix()
		// 检查renderinfo是否为空，避免空指针
		if args.Render == nil {
			args.Render = &commonmodels.RenderInfo{ProductTmpl: args.ProductName}
		}

		renderSet := &commonmodels.RenderSet{
			ProductTmpl: args.Render.ProductTmpl,
			Name:        args.Render.Name,
			Revision:    args.Render.Revision,
		}
		// 如果是版本回滚，则args.Render.Revision > 0
		if args.Render.Revision == 0 {
			// 检查renderset是否覆盖产品所有key
			renderSet, err = commonservice.ValidateRenderSet(args.ProductName, args.Render.Name, "", log)
			if err != nil {
				log.Errorf("[%s][P:%s] validate product renderset error: %v", args.EnvName, args.ProductName, err)
				return e.ErrCreateEnv.AddDesc(err.Error())
			}
			// 保存产品信息,并设置产品状态
			// 设置产品render revsion
			args.Render.Revision = renderSet.Revision
			// 记录服务当前对应render版本
			setServiceRender(args)
		}

		args.Status = setting.ProductStatusCreating
		args.RecycleDay = config.DefaultRecycleDay()
		err = commonrepo.NewProductColl().Create(args)
		if err != nil {
			log.Errorf("[%s][%s] create product record error: %v", args.EnvName, args.ProductName, err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}
		// 异步创建产品
		go createGroups(args.EnvName, user, requestID, args, eventStart, renderSet, kubeClient, log)
	}

	return nil
}

func UpdateProductRecycleDay(envName, productName string, recycleDay int) error {
	return commonrepo.NewProductColl().UpdateProductRecycleDay(envName, productName, recycleDay)
}

func UpdateHelmProduct(productName, envName, updateType, username, requestID string, log *zap.SugaredLogger) error {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	productResp, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("GetProduct envName:%s, productName:%s, err:%+v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(err.Error())
	}
	currentProductService := productResp.Services
	// 查找产品模板
	updateProd, err := GetInitProduct(productName, log)
	if err != nil {
		log.Errorf("[%s][P:%s] GetProductTemplate error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.FindProductTmplErrMsg)
	}
	productResp.Services = updateProd.Services

	// 设置产品状态为更新中
	if err := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUpdating); err != nil {
		log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.UpdateEnvStatusErrMsg)
	}
	//对比当前环境中的环境变量和默认的环境变量
	go func() {
		err := updateProductGroup(productName, envName, updateType, productResp, currentProductService, log)
		if err != nil {
			log.Errorf("[%s][P:%s] failed to update product %#v", envName, productName, err)
			// 发送更新产品失败消息给用户
			title := fmt.Sprintf("更新 [%s] 的 [%s] 环境失败", productName, envName)
			commonservice.SendErrorMessage(username, title, requestID, err, log)

			// 设置产品状态
			log.Infof("[%s][P:%s] update status to => %s", envName, productName, setting.ProductStatusFailed)
			if err2 := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusFailed); err2 != nil {
				log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err2)
				return
			}

			log.Infof("[%s][P:%s] update error to => %s", envName, productName, e.String(err))
			if err2 := commonrepo.NewProductColl().UpdateErrors(envName, productName, e.String(err)); err2 != nil {
				log.Errorf("[%s][P:%s] Product.UpdateErrors error: %v", envName, productName, err2)
				return
			}
		} else {
			productResp.Status = setting.ProductStatusSuccess

			if err = commonrepo.NewProductColl().UpdateStatus(envName, productName, productResp.Status); err != nil {
				log.Errorf("[%s][%s] Product.Update error: %v", envName, productName, err)
				return
			}

			if err = commonrepo.NewProductColl().UpdateErrors(envName, productName, ""); err != nil {
				log.Errorf("[%s][P:%s] Product.UpdateErrors error: %v", envName, productName, err)
				return
			}
		}
	}()
	return nil
}

func UpdateHelmProductVariable(productName, envName, username, requestID string, rcs []*template.RenderChart, log *zap.SugaredLogger) error {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	productResp, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("GetProduct envName:%s, productName:%s, err:%+v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(err.Error())
	}
	var oldRenderVersion int64
	if productResp.Render != nil {
		oldRenderVersion = productResp.Render.Revision
	}
	productResp.ChartInfos = rcs
	if err = commonservice.CreateHelmRenderSet(
		&commonmodels.RenderSet{
			Name:        productResp.Namespace,
			EnvName:     envName,
			ProductTmpl: productName,
			UpdateBy:    username,
			ChartInfos:  rcs,
		},
		log,
	); err != nil {
		log.Errorf("[%s][P:%s] create renderset error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.FindProductTmplErrMsg)
	}

	if productResp.Render == nil {
		productResp.Render = &commonmodels.RenderInfo{ProductTmpl: productResp.ProductName}
	}

	renderSet, err := FindHelmRenderSet(productResp.ProductName, productResp.Namespace, log)
	if err != nil {
		log.Errorf("[%s][P:%s] find product renderset error: %v", productResp.EnvName, productResp.ProductName, err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}
	productResp.Render.Revision = renderSet.Revision

	// 设置产品状态为更新中
	if err := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUpdating); err != nil {
		log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.UpdateEnvStatusErrMsg)
	}

	go func() {
		err := updateProductVariable(productName, envName, productResp, log)
		if err != nil {
			log.Errorf("[%s][P:%s] failed to update product %#v", envName, productName, err)
			// 发送更新产品失败消息给用户
			title := fmt.Sprintf("更新 [%s] 的 [%s] 环境失败", productName, envName)
			commonservice.SendErrorMessage(username, title, requestID, err, log)

			// 设置产品状态
			log.Infof("[%s][P:%s] update status to => %s", envName, productName, setting.ProductStatusFailed)
			if err2 := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusFailed); err2 != nil {
				log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err2)
				return
			}

			log.Infof("[%s][P:%s] update error to => %s", envName, productName, e.String(err))
			if err2 := commonrepo.NewProductColl().UpdateErrors(envName, productName, e.String(err)); err2 != nil {
				log.Errorf("[%s][P:%s] Product.UpdateErrors error: %v", envName, productName, err2)
				return
			}

			// If the update environment failed, Roll back to the previous render version and delete new revision
			log.Infof("[%s][P:%s] roll back to previous render version: %d", envName, productName, oldRenderVersion)
			if deleteRenderSetErr := commonrepo.NewRenderSetColl().DeleteRenderSet(productName, productResp.Render.Name, productResp.Render.Revision); deleteRenderSetErr != nil {
				log.Errorf("[%s][P:%s] Product delete renderSet error: %v", envName, productName, deleteRenderSetErr)
			}
			productResp.Render.Revision = oldRenderVersion
			if updateRendErr := commonrepo.NewProductColl().UpdateRender(envName, productName, productResp.Render); updateRendErr != nil {
				log.Errorf("[%s][P:%s] Product update render error: %v", envName, productName, updateRendErr)
			}
			return
		}
		productResp.Status = setting.ProductStatusSuccess
		if err = commonrepo.NewProductColl().UpdateStatus(envName, productName, productResp.Status); err != nil {
			log.Errorf("[%s][%s] Product.Update error: %v", envName, productName, err)
			return
		}

		if err = commonrepo.NewProductColl().UpdateErrors(envName, productName, ""); err != nil {
			log.Errorf("[%s][P:%s] Product.UpdateErrors error: %v", envName, productName, err)
			return
		}
	}()

	return nil
}

var mutexUpdateMultiHelm sync.RWMutex

func UpdateMultiHelmProduct(envNames []string, updateType, productName string, userID int, superUser bool, requestID string, log *zap.SugaredLogger) []*EnvStatus {
	mutexUpdateMultiHelm.Lock()
	defer func() {
		mutexUpdateMultiHelm.Unlock()
	}()

	envStatuses := make([]*EnvStatus, 0)
	productsRevison, err := ListProductsRevision("", "", userID, superUser, log)
	if err != nil {
		log.Errorf("UpdateMultiHelmProduct ListProductsRevision err:%v", err)
		return envStatuses
	}
	productMap := make(map[string]*ProductRevision)
	for _, productRevison := range productsRevison {
		if productRevison.ProductName == productName && sets.NewString(envNames...).Has(productRevison.EnvName) && productRevison.Updatable {
			productMap[productRevison.EnvName] = productRevison
			if len(productMap) == len(envNames) {
				break
			}
		}
	}

	for envName := range productMap {
		err = UpdateHelmProduct(productName, envName, updateType, setting.SystemUser, requestID, log)
		if err != nil {
			log.Errorf("UpdateMultiHelmProduct UpdateProductV2 err:%v", err)
			return envStatuses
		}
	}

	productResps := make([]*ProductResp, 0)
	for _, envName := range envNames {
		productResp, err := GetProduct(setting.SystemUser, envName, productName, log)
		if err == nil && productResp != nil {
			productResps = append(productResps, productResp)
		}
	}

	for _, productResp := range productResps {
		if productResp.Error != "" {
			envStatuses = append(envStatuses, &EnvStatus{EnvName: productResp.EnvName, Status: setting.ProductStatusFailed, ErrMessage: productResp.Error})
			continue
		}
		envStatuses = append(envStatuses, &EnvStatus{EnvName: productResp.EnvName, Status: productResp.Status})
	}
	return envStatuses
}

func GetProductInfo(username, envName, productName string, log *zap.SugaredLogger) (*commonmodels.Product, error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[User:%s][EnvName:%s][Product:%s] Product.FindByOwner error: %v", username, envName, productName, err)
		return nil, e.ErrGetEnv
	}

	renderSetName := commonservice.GetProductEnvNamespace(envName, productName)
	renderSetOpt := &commonrepo.RenderSetFindOption{Name: renderSetName, Revision: prod.Render.Revision}
	renderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
	if err != nil {
		log.Errorf("find helm renderset[%s] error: %v", renderSetName, err)
		return prod, nil
	}
	prod.ChartInfos = renderSet.ChartInfos

	return prod, nil
}

func GetProductIngress(username, productName string, log *zap.SugaredLogger) ([]*ProductIngressInfo, error) {
	productIngressInfos := make([]*ProductIngressInfo, 0)
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: productName})
	if err != nil {
		log.Errorf("[%s] Collections.Product.List error: %v", username, err)
		return productIngressInfos, e.ErrListEnvs.AddDesc(err.Error())
	}
	for _, prod := range products {
		productIngressInfo := new(ProductIngressInfo)
		productIngressInfo.EnvName = prod.EnvName
		ingressInfos := make([]*commonservice.IngressInfo, 0)

		serviceGroups, _, err := ListGroups("", prod.EnvName, productName, 0, 0, log)
		if err != nil {
			log.Errorf("GetProductIngress GetProductAndKubeClient err:%v", err)
			continue
		}
		for _, serviceGroup := range serviceGroups {
			if serviceGroup.Ingress != nil && len(serviceGroup.Ingress.HostInfo) > 0 {
				ingressInfos = append(ingressInfos, serviceGroup.Ingress)
			}
		}
		productIngressInfo.IngressInfos = ingressInfos
		productIngressInfos = append(productIngressInfos, productIngressInfo)
	}
	return productIngressInfos, nil
}

// ListRenderCharts 获取集成环境中的values.yaml的值
func ListRenderCharts(productName, envName string, log *zap.SugaredLogger) ([]*template.RenderChart, error) {
	renderSetOpt := &commonrepo.RenderSetFindOption{Name: productName}
	if envName != "" {
		opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
		productResp, err := commonrepo.NewProductColl().Find(opt)
		if err != nil {
			log.Errorf("GetProduct envName:%s, productName:%s, err:%+v", envName, productName, err)
			return nil, e.ErrListRenderSets.AddDesc(err.Error())
		}

		renderSetName := commonservice.GetProductEnvNamespace(envName, productName)
		renderSetOpt = &commonrepo.RenderSetFindOption{Name: renderSetName, Revision: productResp.Render.Revision}
	}

	renderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
	if err != nil {
		log.Errorf("find helm renderset[%s] error: %v", productName, err)
		return nil, e.ErrListRenderSets.AddDesc(err.Error())
	}
	return renderSet.ChartInfos, nil
}

func GetHelmChartVersions(productName, envName string, log *zap.SugaredLogger) ([]*commonmodels.HelmVersions, error) {
	var (
		helmVersions = make([]*commonmodels.HelmVersions, 0)
		chartInfoMap = make(map[string]*template.RenderChart)
	)
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[EnvName:%s][Product:%s] Product.FindByOwner error: %v", envName, productName, err)
		return nil, e.ErrGetEnv
	}

	renderSetName := commonservice.GetProductEnvNamespace(envName, productName)
	renderSetOpt := &commonrepo.RenderSetFindOption{Name: renderSetName, Revision: prod.Render.Revision}
	renderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
	if err != nil {
		log.Errorf("find helm renderset[%s] error: %v", renderSetName, err)
		return helmVersions, err
	}

	for _, chartInfo := range renderSet.ChartInfos {
		chartInfoMap[chartInfo.ServiceName] = chartInfo
	}

	for serviceName, chartInfo := range chartInfoMap {
		opt := &commonrepo.ServiceFindOption{
			ServiceName: serviceName,
			Type:        setting.HelmDeployType,
			ProductName: productName,
		}

		respService, err := commonrepo.NewServiceColl().Find(opt)
		if err != nil {
			log.Warnf("系统未找到当前类型[%s]的服务[%s]!", setting.HelmDeployType, serviceName)
			continue
		}

		helmVersion := &commonmodels.HelmVersions{
			ServiceName:       serviceName,
			CurrentVersion:    chartInfo.ChartVersion,
			CurrentValuesYaml: chartInfo.ValuesYaml,
			LatestVersion:     respService.HelmChart.Version,
			LatestValuesYaml:  respService.HelmChart.ValuesYaml,
		}
		helmVersions = append(helmVersions, helmVersion)
	}

	return helmVersions, nil
}

func createGroups(envName, user, requestID string, args *commonmodels.Product, eventStart int64, renderSet *commonmodels.RenderSet, kubeClient client.Client, log *zap.SugaredLogger) {
	var err error
	defer func() {
		status := setting.ProductStatusSuccess
		errorMsg := ""
		if err != nil {
			status = setting.ProductStatusFailed
			errorMsg = err.Error()

			// 发送创建产品失败消息给用户
			title := fmt.Sprintf("创建 [%s] 的 [%s] 环境失败", args.ProductName, args.EnvName)
			commonservice.SendErrorMessage(user, title, requestID, err, log)
		}

		commonservice.LogProductStats(envName, setting.CreateProductEvent, args.ProductName, requestID, eventStart, log)

		if err := commonrepo.NewProductColl().UpdateStatus(envName, args.ProductName, status); err != nil {
			log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, args.ProductName, err)
			return
		}
		if err := commonrepo.NewProductColl().UpdateErrors(envName, args.ProductName, errorMsg); err != nil {
			log.Errorf("[%s][P:%s] Product.UpdateErrors error: %v", envName, args.ProductName, err)
			return
		}
	}()

	for _, group := range args.Services {
		err = envHandleFunc(getProjectType(args.ProductName), log).createGroup(envName, args.ProductName, user, group, renderSet, kubeClient)
		if err != nil {
			args.Status = setting.ProductStatusFailed
			log.Errorf("createGroup error :%+v", err)
			return
		}
	}
}

func getProjectType(productName string) string {
	projectInfo, _ := templaterepo.NewProductColl().Find(productName)
	projectType := setting.K8SDeployType
	if projectInfo == nil || projectInfo.ProductFeature == nil {
		return projectType
	}

	if projectInfo.ProductFeature.DeployType == setting.K8SDeployType && projectInfo.ProductFeature.BasicFacility == setting.BasicFacilityK8S {
		return projectType
	}

	if projectInfo.ProductFeature.DeployType == setting.K8SDeployType && projectInfo.ProductFeature.BasicFacility == setting.BasicFacilityCVM {
		return setting.PMDeployType
	}
	return projectType
}

// createGroup create or update services in service group
//func createGroup(envName, productName, username string, group []*commonmodels.ProductService, renderSet *commonmodels.RenderSet, kubeClient client.Client, log *zap.SugaredLogger) error {
//	log.Infof("[Namespace:%s][Product:%s] createGroup", envName, productName)
//	updatableServiceNameList := make([]string, 0)
//
//	// 异步创建无依赖的服务
//	errList := &multierror.Error{
//		ErrorFormat: func(es []error) string {
//			points := make([]string, len(es))
//			for i, err := range es {
//				points[i] = fmt.Sprintf("%v", err)
//			}
//
//			return strings.Join(points, "\n")
//		},
//	}
//
//	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
//	prod, err := commonrepo.NewProductColl().Find(opt)
//	if err != nil {
//		errList = multierror.Append(errList, err)
//	}
//
//	var wg sync.WaitGroup
//	var lock sync.Mutex
//	var resources []*unstructured.Unstructured
//
//	for i := range group {
//		if group[i].Type == setting.K8SDeployType {
//			// 只有在service有Pod的时候，才需要等待pod running或者等待pod succeed
//			// 比如在group中，如果service下仅有configmap/service/ingress这些yaml的时候，不需要waitServicesRunning
//			wg.Add(1)
//			updatableServiceNameList = append(updatableServiceNameList, group[i].ServiceName)
//			go func(svc *commonmodels.ProductService) {
//				defer wg.Done()
//				items, err := upsertService(false, prod, svc, nil, renderSet, kubeClient, log)
//				if err != nil {
//					lock.Lock()
//					switch e := err.(type) {
//					case *multierror.Error:
//						errList = multierror.Append(errList, e.Errors...)
//					default:
//						errList = multierror.Append(errList, e)
//					}
//					lock.Unlock()
//				}
//
//				//  concurrent array append
//				lock.Lock()
//				resources = append(resources, items...)
//				lock.Unlock()
//			}(group[i])
//		} else if group[i].Type == setting.PMDeployType {
//			//更新非k8s服务
//			if len(group[i].EnvConfigs) > 0 {
//				serviceTempl, err := commonservice.GetServiceTemplate(group[i].ServiceName, setting.PMDeployType, productName, setting.ProductStatusDeleting, group[i].Revision, log)
//				if err != nil {
//					errList = multierror.Append(errList, err)
//				}
//				if serviceTempl != nil {
//					oldEnvConfigs := serviceTempl.EnvConfigs
//					for _, currentEnvConfig := range group[i].EnvConfigs {
//						envConfig := &commonmodels.EnvConfig{
//							EnvName: currentEnvConfig.EnvName,
//							HostIDs: currentEnvConfig.HostIDs,
//						}
//						oldEnvConfigs = append(oldEnvConfigs, envConfig)
//					}
//
//					args := &commonservice.ServiceTmplBuildObject{
//						ServiceTmplObject: &commonservice.ServiceTmplObject{
//							ProductName:  serviceTempl.ProductName,
//							ServiceName:  serviceTempl.ServiceName,
//							Visibility:   serviceTempl.Visibility,
//							Revision:     serviceTempl.Revision,
//							Type:         serviceTempl.Type,
//							Username:     username,
//							HealthChecks: serviceTempl.HealthChecks,
//							EnvConfigs:   oldEnvConfigs,
//							EnvStatuses:  []*commonmodels.EnvStatus{},
//							From:         "createEnv",
//						},
//						Build: &commonmodels.Build{Name: serviceTempl.BuildName},
//					}
//
//					if err := commonservice.UpdatePmServiceTemplate(username, args, log); err != nil {
//						errList = multierror.Append(errList, err)
//					}
//				}
//			}
//			var latestRevision int64 = group[i].Revision
//			// 获取最新版本的服务
//			if latestServiceTempl, _ := commonservice.GetServiceTemplate(group[i].ServiceName, setting.PMDeployType, productName, setting.ProductStatusDeleting, 0, log); latestServiceTempl != nil {
//				latestRevision = latestServiceTempl.Revision
//			}
//			// 更新环境
//			if latestRevision > group[i].Revision {
//				// 更新产品服务
//				for _, serviceGroup := range prod.Services {
//					for j, service := range serviceGroup {
//						if service.ServiceName == group[i].ServiceName && service.Type == setting.PMDeployType {
//							serviceGroup[j].Revision = latestRevision
//						}
//					}
//				}
//				if err := commonrepo.NewProductColl().Update(prod); err != nil {
//					log.Errorf("[%s][%s] Product.Update error: %v", envName, productName, err)
//					errList = multierror.Append(errList, err)
//				}
//			}
//			if _, err = commonservice.CreateServiceTask(&commonmodels.ServiceTaskArgs{
//				ProductName:        productName,
//				ServiceName:        group[i].ServiceName,
//				Revision:           latestRevision,
//				EnvNames:           []string{envName},
//				ServiceTaskCreator: username,
//			}, log); err != nil {
//				errList = multierror.Append(errList, err)
//			}
//		}
//	}
//
//	wg.Wait()
//
//	// 如果创建依赖服务组有返回错误, 停止等待
//	if err := errList.ErrorOrNil(); err != nil {
//		return err
//	}
//
//	if err := waitResourceRunning(kubeClient, prod.Namespace, resources, config.ServiceStartTimeout(), log); err != nil {
//		log.Errorf(
//			"service group %s/%+v doesn't start in %d seconds: %v",
//			prod.Namespace,
//			updatableServiceNameList, config.ServiceStartTimeout(), err)
//
//		err = e.ErrUpdateEnv.AddErr(
//			errors.Errorf(e.StartPodTimeout+"\n %s", "["+strings.Join(updatableServiceNameList, "], [")+"]"))
//		return err
//	}
//
//	return nil
//}

// upsertService 创建或者更新服务, 更新服务之前先创建服务需要的配置
func upsertService(isUpdate bool, env *commonmodels.Product,
	service *commonmodels.ProductService, prevSvc *commonmodels.ProductService,
	renderSet *commonmodels.RenderSet, kubeClient client.Client, log *zap.SugaredLogger,
) ([]*unstructured.Unstructured, error) {
	errList := &multierror.Error{
		ErrorFormat: func(es []error) string {
			format := "更新服务"
			if !isUpdate {
				format = "创建服务"
			}

			if len(es) == 1 {
				return fmt.Sprintf(format+" %s 失败：%v", service.ServiceName, es[0])
			}

			points := make([]string, len(es))
			for i, err := range es {
				points[i] = fmt.Sprintf("* %v", err)
			}

			return fmt.Sprintf(format+" %s 失败：\n%s", service.ServiceName, strings.Join(points, "\n"))
		},
	}

	// 如果是非容器化部署方式的服务，则现在不需要进行创建或者更新
	if service.Type != setting.K8SDeployType {
		return nil, nil
	}

	productName := env.ProductName
	envName := env.EnvName
	namespace := env.Namespace

	// 获取服务模板
	parsedYaml, err := renderService(env, renderSet, service)

	if err != nil {
		log.Errorf("Failed to render service %s, error: %v", service.ServiceName, err)
		errList = multierror.Append(errList, fmt.Errorf("service template %s error: %v", service.ServiceName, err))
		return nil, errList
	}

	manifests := releaseutil.SplitManifests(*parsedYaml)
	resources := make([]*unstructured.Unstructured, 0, len(manifests))
	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			log.Errorf("Failed to convert yaml to Unstructured, manifest is\n%s\n, error: %v", item, err)
			errList = multierror.Append(errList, err)
			continue
		}

		resources = append(resources, u)
	}

	// compatibility: prevSvc.Render could be null when prev update failed
	if prevSvc != nil && prevSvc.Render != nil {
		err = removeOldResources(resources, env, prevSvc, kubeClient, log)
		if err != nil {
			log.Errorf("Failed to remove old resources, error: %v", err)
			errList = multierror.Append(errList, err)
			return nil, errList
		}
	}

	labels := getPredefinedLabels(productName, service.ServiceName)
	clusterLabels := getPredefinedClusterLabels(productName, service.ServiceName, envName)
	var res []*unstructured.Unstructured

	for _, u := range resources {
		switch u.GetKind() {
		case setting.Ingress:
			ls := kube.MergeLabels(labels, u.GetLabels())
			as := applySystemIngressTimeouts(u.GetAnnotations())
			as = applySystemIngressClass(as)

			u.SetNamespace(namespace)
			u.SetLabels(ls)
			u.SetAnnotations(as)

			err = updater.CreateOrPatchUnstructured(u, kubeClient)
			if err != nil {
				log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err)
				errList = multierror.Append(errList, err)
				continue
			}

		case setting.Service:
			u.SetNamespace(namespace)
			u.SetLabels(kube.MergeLabels(labels, u.GetLabels()))

			if _, ok := u.GetLabels()["endpoints"]; !ok {
				selector, _, _ := unstructured.NestedStringMap(u.Object, "spec", "selector")
				err := unstructured.SetNestedStringMap(u.Object, kube.MergeLabels(labels, selector), "spec", "selector")
				if err != nil {
					// should not have happened
					panic(err)
				}
			}

			err = updater.CreateOrPatchUnstructured(u, kubeClient)
			if err != nil {
				log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err)
				errList = multierror.Append(errList, err)
				continue
			}

		case setting.Deployment, setting.StatefulSet:
			u.SetNamespace(namespace)
			u.SetAPIVersion(setting.APIVersionAppsV1)
			u.SetLabels(kube.MergeLabels(labels, u.GetLabels()))

			podLabels, _, _ := unstructured.NestedStringMap(u.Object, "spec", "template", "metadata", "labels")
			err := unstructured.SetNestedStringMap(u.Object, kube.MergeLabels(labels, podLabels), "spec", "template", "metadata", "labels")
			if err != nil {
				// should not have happened
				panic(err)
			}

			podAnnotations, _, _ := unstructured.NestedStringMap(u.Object, "spec", "template", "metadata", "annotations")
			err = unstructured.SetNestedStringMap(u.Object, applyUpdatedAnnotations(podAnnotations), "spec", "template", "metadata", "annotations")
			if err != nil {
				// should not have happened
				panic(err)
			}

			// Inject selector: s-product and s-service
			selector, _, _ := unstructured.NestedStringMap(u.Object, "spec", "selector", "matchLabels")
			err = unstructured.SetNestedStringMap(u.Object, kube.MergeLabels(labels, selector), "spec", "selector", "matchLabels")
			if err != nil {
				// should not have happened
				panic(err)
			}

			jsonData, err := u.MarshalJSON()
			if err != nil {
				log.Errorf("Failed to marshal JSON, manifest is\n%v\n, error: %v", u, err)
				errList = multierror.Append(errList, err)
				continue
			}
			obj, err := serializer.NewDecoder().JSONToRuntimeObject(jsonData)
			if err != nil {
				log.Errorf("Failed to convert JSON to Object, manifest is\n%v\n, error: %v", u, err)
				errList = multierror.Append(errList, err)
				continue
			}

			switch res := obj.(type) {
			case *appsv1.Deployment:
				// Inject imagePullSecrets if qn-registry-secret is not set
				applySystemImagePullSecrets(&res.Spec.Template.Spec)

				err = updater.CreateOrPatchDeployment(res, kubeClient)
				if err != nil {
					log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), res, err)
					errList = multierror.Append(errList, err)
					continue
				}
			case *appsv1.StatefulSet:
				// Inject imagePullSecrets if qn-registry-secret is not set
				applySystemImagePullSecrets(&res.Spec.Template.Spec)

				err = updater.CreateOrPatchStatefulSet(res, kubeClient)
				if err != nil {
					log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), res, err)
					errList = multierror.Append(errList, err)
					continue
				}
			default:
				errList = multierror.Append(errList, fmt.Errorf("object is not a appsv1.Deployment ort appsv1.StatefulSet"))
				continue
			}

		case setting.Job:
			jsonData, err := u.MarshalJSON()
			if err != nil {
				log.Errorf("Failed to marshal JSON, manifest is\n%v\n, error: %v", u, err)
				errList = multierror.Append(errList, err)
				continue
			}
			obj, err := serializer.NewDecoder().JSONToJob(jsonData)
			if err != nil {
				log.Errorf("Failed to convert JSON to Job, manifest is\n%v\n, error: %v", u, err)
				errList = multierror.Append(errList, err)
				continue
			}

			obj.Namespace = namespace
			obj.ObjectMeta.Labels = kube.MergeLabels(labels, obj.ObjectMeta.Labels)
			obj.Spec.Template.ObjectMeta.Labels = kube.MergeLabels(labels, obj.Spec.Template.ObjectMeta.Labels)

			// Inject imagePullSecrets if qn-registry-secret is not set
			applySystemImagePullSecrets(&obj.Spec.Template.Spec)

			if err := updater.DeleteJobAndWait(namespace, obj.Name, kubeClient); err != nil {
				log.Errorf("Failed to delete Job, error: %v", err)
				errList = multierror.Append(errList, err)
				continue
			}

			if err := updater.CreateJob(obj, kubeClient); err != nil {
				log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), obj, err)
				errList = multierror.Append(errList, err)
				continue
			}

		case setting.CronJob:
			jsonData, err := u.MarshalJSON()
			if err != nil {
				log.Errorf("Failed to marshal JSON, manifest is\n%v\n, error: %v", u, err)
				errList = multierror.Append(errList, err)
				continue
			}
			obj, err := serializer.NewDecoder().JSONToCronJob(jsonData)
			if err != nil {
				log.Errorf("Failed to convert JSON to CronJob, manifest is\n%v\n, error: %v", u, err)
				errList = multierror.Append(errList, err)
				continue
			}

			obj.Namespace = namespace
			obj.ObjectMeta.Labels = kube.MergeLabels(labels, obj.ObjectMeta.Labels)
			obj.Spec.JobTemplate.ObjectMeta.Labels = kube.MergeLabels(labels, obj.Spec.JobTemplate.ObjectMeta.Labels)
			obj.Spec.JobTemplate.Spec.Template.ObjectMeta.Labels = kube.MergeLabels(labels, obj.Spec.JobTemplate.Spec.Template.ObjectMeta.Labels)

			// Inject imagePullSecrets if qn-registry-secret is not set
			applySystemImagePullSecrets(&obj.Spec.JobTemplate.Spec.Template.Spec)

			err = updater.CreateOrPatchCronJob(obj, kubeClient)
			if err != nil {
				log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), obj, err)
				errList = multierror.Append(errList, err)
				continue
			}

		case setting.ClusterRole, setting.ClusterRoleBinding:
			u.SetLabels(kube.MergeLabels(clusterLabels, u.GetLabels()))

			err = updater.CreateOrPatchUnstructured(u, kubeClient)
			if err != nil {
				log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err)
				errList = multierror.Append(errList, err)
				continue
			}
		default:
			u.SetNamespace(namespace)
			u.SetLabels(kube.MergeLabels(labels, u.GetLabels()))

			err = updater.CreateOrPatchUnstructured(u, kubeClient)
			if err != nil {
				log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err)
				errList = multierror.Append(errList, err)
				continue
			}
		}

		res = append(res, u)
	}

	return res, errList.ErrorOrNil()
}

func removeOldResources(
	items []*unstructured.Unstructured,
	env *commonmodels.Product,
	oldService *commonmodels.ProductService,
	kubeClient client.Client,
	log *zap.SugaredLogger,
) error {
	opt := &commonrepo.RenderSetFindOption{Name: oldService.Render.Name, Revision: oldService.Render.Revision}
	resp, err := commonrepo.NewRenderSetColl().Find(opt)
	if err != nil {
		log.Errorf("find renderset[%s/%d] error: %v", opt.Name, opt.Revision, err)
		return err
	}

	parsedYaml, err := renderService(env, resp, oldService)
	if err != nil {
		log.Errorf("failed to find old service revision %s/%d", oldService.ServiceName, oldService.Revision)
		return err
	}

	itemsMap := make(map[string]*unstructured.Unstructured)
	for _, u := range items {
		itemsMap[fmt.Sprintf("%s/%s", u.GetKind(), u.GetName())] = u
	}

	manifests := releaseutil.SplitManifests(*parsedYaml)
	oldItemsMap := make(map[string]*unstructured.Unstructured)
	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			log.Errorf("Failed to covert from yaml to Unstructured, yaml is %s", item)
			continue
		}

		oldItemsMap[fmt.Sprintf("%s/%s", u.GetKind(), u.GetName())] = u
	}

	for name, item := range oldItemsMap {
		_, exists := itemsMap[name]
		if !exists {
			if err = updater.DeleteUnstructured(item, kubeClient); err != nil {
				log.Errorf(
					"failed to remove old item %s/%s/%s from %s/%d: %v",
					env.Namespace,
					item.GetName(),
					item.GetKind(),
					oldService.ServiceName,
					oldService.Revision, err)
				continue
			}
			log.Infof(
				"succeed to remove old item %s/%s/%s from %s/%d",
				env.Namespace,
				item.GetName(),
				item.GetKind(),
				oldService.ServiceName,
				oldService.Revision)
		}
	}

	return nil
}

func renderService(prod *commonmodels.Product, render *commonmodels.RenderSet, service *commonmodels.ProductService) (yaml *string, err error) {
	// 获取服务模板
	opt := &commonrepo.ServiceFindOption{
		ServiceName: service.ServiceName,
		Type:        service.Type,
		Revision:    service.Revision,
		//ExcludeStatus: product.ProductStatusDeleting,
	}
	svcTmpl, err := commonrepo.NewServiceColl().Find(opt)
	if err != nil {
		return nil, err
	}

	// 渲染配置集
	parsedYaml := commonservice.RenderValueForString(svcTmpl.Yaml, render)
	// 渲染系统变量键值
	parsedYaml = kube.ParseSysKeys(prod.Namespace, prod.EnvName, prod.ProductName, service.ServiceName, parsedYaml)
	// 替换服务模板容器镜像为用户指定镜像
	parsedYaml = replaceContainerImages(parsedYaml, svcTmpl.Containers, service.Containers)

	return &parsedYaml, nil
}

func replaceContainerImages(tmpl string, ori []*commonmodels.Container, replace []*commonmodels.Container) string {

	replaceMap := make(map[string]string)
	for _, container := range replace {
		replaceMap[container.Name] = container.Image
	}

	for _, container := range ori {
		imageRex := regexp.MustCompile("image:\\s*" + container.Image)
		if _, ok := replaceMap[container.Name]; !ok {
			continue
		}
		tmpl = imageRex.ReplaceAllLiteralString(tmpl, fmt.Sprintf("image: %s", replaceMap[container.Name]))
	}

	return tmpl
}

func waitResourceRunning(
	kubeClient client.Client, namespace string,
	resources []*unstructured.Unstructured, timeoutSeconds int, log *zap.SugaredLogger,
) error {
	log.Infof("wait service group to run in %d seconds", timeoutSeconds)

	return wait.Poll(1*time.Second, time.Duration(timeoutSeconds)*time.Second, func() (bool, error) {
		for _, r := range resources {
			var ready bool
			found := true
			var err error
			switch r.GetKind() {
			case setting.Deployment:
				var d *appsv1.Deployment
				d, found, err = getter.GetDeployment(namespace, r.GetName(), kubeClient)
				if err == nil && found {
					ready = wrapper.Deployment(d).Ready()
				}
			case setting.StatefulSet:
				var s *appsv1.StatefulSet
				s, found, err = getter.GetStatefulSet(namespace, r.GetName(), kubeClient)
				if err == nil && found {
					ready = wrapper.StatefulSet(s).Ready()
				}
			case setting.Job:
				var j *batchv1.Job
				j, found, err = getter.GetJob(namespace, r.GetName(), kubeClient)
				if err == nil && found {
					ready = wrapper.Job(j).Complete()
				}
			default:
				ready = true
			}

			if err != nil {
				return false, err
			}

			if !found || !ready {
				return false, nil
			}
		}

		return true, nil
	})
}

func preCreateProduct(envName string, args *commonmodels.Product, kubeClient client.Client, log *zap.SugaredLogger) error {
	var (
		productTemplateName = args.ProductName
		renderSetName       = commonservice.GetProductEnvNamespace(envName, args.ProductName)
		err                 error
	)
	// 如果 args.Render.Revision > 0 则该次操作是版本回溯
	if args.Render != nil && args.Render.Revision > 0 {
		renderSetName = args.Render.Name
	} else {
		switch args.Source {
		case setting.HelmDeployType:
			err = commonservice.CreateHelmRenderSet(
				&commonmodels.RenderSet{
					Name:        renderSetName,
					Revision:    0,
					EnvName:     envName,
					ProductTmpl: args.ProductName,
					UpdateBy:    args.UpdateBy,
					ChartInfos:  args.ChartInfos,
				},
				log,
			)
		default:
			err = commonservice.CreateRenderSet(
				&commonmodels.RenderSet{
					Name:        renderSetName,
					Revision:    0,
					EnvName:     envName,
					ProductTmpl: args.ProductName,
					UpdateBy:    args.UpdateBy,
					KVs:         args.Vars,
				},
				log,
			)

		}
		if err != nil {
			log.Errorf("[%s][P:%s] create renderset error: %v", envName, productTemplateName, err)
			return e.ErrCreateEnv.AddDesc(e.FindProductTmplErrMsg)
		}
	}

	args.Vars = nil

	var productTmpl *template.Product
	// 查询产品模板
	productTmpl, err = templaterepo.NewProductColl().Find(productTemplateName)
	if err != nil {
		log.Errorf("[%s][P:%s] get product template error: %v", envName, productTemplateName, err)
		return e.ErrCreateEnv.AddDesc(e.FindProductTmplErrMsg)
	}

	//检查产品是否包含服务
	var serviceCount int
	for _, group := range args.Services {
		serviceCount = serviceCount + len(group)
	}
	if serviceCount == 0 {
		log.Errorf("[%s][P:%s] not service found", envName, args.ProductName)
		return e.ErrCreateEnv.AddDesc(e.FindProductServiceErrMsg)
	}
	// 检查args中是否设置revision，如果没有，设为Product Tmpl当前版本
	if args.Revision == 0 {
		args.Revision = productTmpl.Revision
	}

	// 检查产品是否存在，envName和productName唯一
	opt := &commonrepo.ProductFindOptions{Name: args.ProductName, EnvName: envName}
	if _, err := commonrepo.NewProductColl().Find(opt); err == nil {
		log.Errorf("[%s][P:%s] duplicate product", envName, args.ProductName)
		return e.ErrCreateEnv.AddDesc(e.DuplicateEnvErrMsg)
	}
	maxConfigs, err := commonrepo.NewConfigColl().ListMaxRevisions("")
	if err != nil {
		log.Errorf("ConfigTmpl.ListMaxRevisions error: %v", err)
		return err
	}
	//确保Config设置
	for _, groups := range args.Services {
		for _, service := range groups {
			if service.Type == setting.K8SDeployType {
				var serviceConfigs []*commonmodels.ServiceConfig
				for _, config := range maxConfigs {
					if config.ServiceName == service.ServiceName {
						serviceConfigs = append(serviceConfigs, &commonmodels.ServiceConfig{
							ConfigName: config.ConfigName,
							Revision:   config.Revision,
						})
					}
				}
				service.Configs = serviceConfigs
			}
		}
	}

	tmpRenderInfo := &commonmodels.RenderInfo{Name: renderSetName, ProductTmpl: args.ProductName}
	if args.Render != nil && args.Render.Revision > 0 {
		tmpRenderInfo.Revision = args.Render.Revision
	}
	args.Render = tmpRenderInfo
	return ensureKubeEnv(commonservice.GetProductEnvNamespace(envName, args.ProductName), kubeClient, log)
}

func getPredefinedLabels(product, service string) map[string]string {
	ls := make(map[string]string)
	ls["s-product"] = product
	ls["s-service"] = service
	return ls
}

func getPredefinedClusterLabels(product, service, envName string) map[string]string {
	labels := getPredefinedLabels(product, service)
	labels[setting.EnvNameLabel] = envName
	return labels
}

func applySystemIngressTimeouts(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}

	if _, ok := labels[setting.IngressProxyConnectTimeoutLabel]; !ok {
		labels[setting.IngressProxyConnectTimeoutLabel] = "300"
	}

	if _, ok := labels[setting.IngressProxySendTimeoutLabel]; !ok {
		labels[setting.IngressProxySendTimeoutLabel] = "300"
	}

	if _, ok := labels[setting.IngressProxyReadTimeoutLabel]; !ok {
		labels[setting.IngressProxyReadTimeoutLabel] = "300"
	}

	return labels
}

func applySystemIngressClass(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}

	if config.DefaultIngressClass() != "" {
		if _, ok := labels[setting.IngressClassLabel]; !ok {
			labels[setting.IngressClassLabel] = config.DefaultIngressClass()
		}
	}

	return labels
}

func applyUpdatedAnnotations(annotations map[string]string) map[string]string {
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[setting.UpdatedByLabel] = fmt.Sprintf("%d", time.Now().Unix())
	return annotations
}

func applySystemResourceRequirements(podSpec *corev1.PodSpec) {
	for i, container := range podSpec.Containers {

		if container.Resources.Limits == nil {
			podSpec.Containers[i].Resources.Limits = corev1.ResourceList{}
		}

		if container.Resources.Limits.Cpu().String() == "0" {
			podSpec.Containers[i].Resources.Limits[corev1.ResourceCPU] = resource.MustParse("500m")
		}

		if container.Resources.Limits.Memory().String() == "0" {
			podSpec.Containers[i].Resources.Limits[corev1.ResourceMemory] = resource.MustParse("200Mi")
		}

		if container.Resources.Requests == nil {
			podSpec.Containers[i].Resources.Requests = corev1.ResourceList{}
		}

		if container.Resources.Requests.Cpu().String() == "0" {
			podSpec.Containers[i].Resources.Requests[corev1.ResourceCPU] = resource.MustParse("10m")
		}

		if container.Resources.Requests.Memory().String() == "0" {
			podSpec.Containers[i].Resources.Requests[corev1.ResourceMemory] = resource.MustParse("100Mi")
		}
	}
}

func applySystemImagePullSecrets(podSpec *corev1.PodSpec) {
	for _, secret := range podSpec.ImagePullSecrets {
		if secret.Name == setting.DefaultCandidateImagePullSecret {
			return
		}
	}
	podSpec.ImagePullSecrets = append(podSpec.ImagePullSecrets,
		corev1.LocalObjectReference{
			Name: setting.DefaultCandidateImagePullSecret,
		})
}

func ensureKubeEnv(namespace string, kubeClient client.Client, log *zap.SugaredLogger) error {
	err := kube.CreateNamespace(namespace, kubeClient)
	if err != nil {
		log.Errorf("[%s] get or create namespace error: %v", namespace, err)
		return e.ErrCreateNamspace.AddDesc(e.SetNamespaceErrMsg)
	}

	// 创建默认的镜像仓库secret
	if err := commonservice.EnsureDefaultRegistrySecret(namespace, kubeClient, log); err != nil {
		log.Errorf("[%s] get or create namespace error: %v", namespace, err)
		return e.ErrCreateSecret.AddDesc(e.CreateDefaultRegistryErrMsg)
	}

	return nil
}

func FindHelmRenderSet(productName, renderName string, log *zap.SugaredLogger) (*commonmodels.RenderSet, error) {
	resp := &commonmodels.RenderSet{ProductTmpl: productName}
	var err error
	if renderName != "" {
		opt := &commonrepo.RenderSetFindOption{Name: renderName}
		resp, err = commonrepo.NewRenderSetColl().Find(opt)
		if err != nil {
			log.Errorf("find helm renderset[%s] error: %v", renderName, err)
			return resp, err
		}
	}
	if renderName != "" && resp.ProductTmpl != productName {
		log.Errorf("helm renderset[%s] not match product[%s]", renderName, productName)
		return resp, fmt.Errorf("helm renderset[%s] not match product[%s]", renderName, productName)
	}
	return resp, nil
}

func installOrUpdateHelmChart(user, envName, requestID string, args *commonmodels.Product, eventStart int64, helmClient helmclient.Client, log *zap.SugaredLogger) {
	var (
		err     error
		wg      sync.WaitGroup
		errList = &multierror.Error{}
	)

	defer func() {
		status := setting.ProductStatusSuccess
		errorMsg := ""
		if err != nil {
			status = setting.ProductStatusFailed
			errorMsg = err.Error()

			// 发送创建产品失败消息给用户
			title := fmt.Sprintf("创建 [%s] 的 [%s] 环境失败", args.ProductName, args.EnvName)
			commonservice.SendErrorMessage(user, title, requestID, err, log)
		}

		commonservice.LogProductStats(envName, setting.CreateProductEvent, args.ProductName, requestID, eventStart, log)

		if err := commonrepo.NewProductColl().UpdateStatus(envName, args.ProductName, status); err != nil {
			log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, args.ProductName, err)
			return
		}
		if err := commonrepo.NewProductColl().UpdateErrors(envName, args.ProductName, errorMsg); err != nil {
			log.Errorf("[%s][P:%s] Product.UpdateErrors error: %v", envName, args.ProductName, err)
			return
		}
	}()
	chartInfoMap := make(map[string]*template.RenderChart)
	for _, renderChart := range args.ChartInfos {
		chartInfoMap[renderChart.ServiceName] = renderChart
	}
	for _, serviceGroups := range args.Services {
		for _, service := range serviceGroups {
			if renderChart, isExist := chartInfoMap[service.ServiceName]; isExist {
				chartSpec := &helmclient.ChartSpec{
					ReleaseName: fmt.Sprintf("%s-%s", args.Namespace, service.ServiceName),
					ChartName:   fmt.Sprintf("%s/%s", args.Namespace, service.ServiceName),
					Namespace:   args.Namespace,
					Wait:        true,
					Version:     renderChart.ChartVersion,
					ValuesYaml:  renderChart.ValuesYaml,
					UpgradeCRDs: true,
					Timeout:     Timeout * time.Second * 10,
				}
				// 获取服务详情
				opt := &commonrepo.ServiceFindOption{
					ServiceName:   service.ServiceName,
					Type:          service.Type,
					Revision:      service.Revision,
					ProductName:   args.ProductName,
					ExcludeStatus: setting.ProductStatusDeleting,
				}
				serviceObj, err := commonrepo.NewServiceColl().Find(opt)
				if err != nil {
					continue
				}
				wg.Add(1)
				go func(currentChartSpec *helmclient.ChartSpec, currentService *commonmodels.Service) {
					defer wg.Done()

					base, err := gerrit.GetGerritWorkspaceBasePath(currentService.RepoName)
					_, serviceFileErr := os.Stat(path.Join(base, currentService.LoadPath))
					if err != nil || os.IsNotExist(serviceFileErr) {
						if err = commonservice.DownloadService(base, currentService.ServiceName); err != nil {
							return
						}
					}

					if err = helmClient.InstallOrUpgradeChart(context.Background(), currentChartSpec, &helmclient.ChartOption{
						ChartPath: filepath.Join(base, currentService.LoadPath),
					}, log); err != nil {
						errList = multierror.Append(errList, err)
						return
					}
				}(chartSpec, serviceObj)
			}
		}
	}
	wg.Wait()
	err = errList.ErrorOrNil()
}

func setServiceRender(args *commonmodels.Product) {
	for _, serviceGroup := range args.Services {
		for _, service := range serviceGroup {
			// 当service type是k8s时才需要渲染信息
			if service.Type == setting.K8SDeployType || service.Type == setting.HelmDeployType {
				service.Render = args.Render
			}
		}
	}
}

func getServiceRevisionMap(serviceRevisionList []*SvcRevision) map[string]*SvcRevision {
	serviceRevisionMap := make(map[string]*SvcRevision)
	for _, revision := range serviceRevisionList {
		serviceRevisionMap[revision.ServiceName+revision.Type] = revision
	}
	return serviceRevisionMap
}

func restartServicesByChange(envName string, prodRev *ProductRevision, log *zap.SugaredLogger) error {
	errList := new(multierror.Error)
	for _, serviceRev := range prodRev.ServiceRevisions {
		// 如果服务部署类型不是容器化的，则跳过重启
		if serviceRev.Type != setting.K8SDeployType {
			continue
		}
		// 只有配置模板改动才会重启pod
		err := restartService(envName, prodRev.ProductName, serviceRev, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}

	return errList.ErrorOrNil()
}

// restartService 重启容器服务
func restartService(envName, productName string, svcRev *SvcRevision, log *zap.SugaredLogger) error {

	// 只有配置模板改动才会重启pod
	configUpdateable := false
	for _, configRev := range svcRev.ConfigRevisions {
		if configRev.Updatable {
			configUpdateable = true
			continue
		}
	}
	if configUpdateable {
		log.Infof("[%s][P:%s][S:%s] restart service", envName, productName, svcRev.ServiceName)

		restartArgs := &SvcOptArgs{
			ProductName: productName,
			ServiceName: svcRev.ServiceName,
		}

		err := RestartService(envName, restartArgs, log)
		if err != nil {
			return err
		}
	}

	return nil
}

func getUpdatedProductServices(updateProduct *commonmodels.Product, serviceRevisionMap map[string]*SvcRevision, currentProduct *commonmodels.Product) [][]*commonmodels.ProductService {
	currentServices := make(map[string]*commonmodels.ProductService)
	for _, group := range currentProduct.Services {
		for _, service := range group {
			currentServices[service.ServiceName+service.Type] = service
		}
	}

	updatedAllServices := make([][]*commonmodels.ProductService, 0)
	for _, group := range updateProduct.Services {
		updatedGroups := make([]*commonmodels.ProductService, 0)
		for _, service := range group {
			serviceRevision, ok := serviceRevisionMap[service.ServiceName+service.Type]
			if !ok {
				//找不到 service revision
				continue
			}
			// 已知：进入到这里的服务，serviceRevision.Deleted=False
			if serviceRevision.New {
				// 新的服务，创建新的service with revision, 并append到updatedGroups中
				// 新的服务的revision，默认Revision为0
				newService := &commonmodels.ProductService{
					ServiceName: service.ServiceName,
					Type:        service.Type,
					Revision:    0,
					Render:      updateProduct.Render,
				}
				updatedGroups = append(updatedGroups, newService)
				continue
			}
			// 不管服务需不需要更新，都拿现在的revision
			if currentService, ok := currentServices[service.ServiceName+service.Type]; ok {
				updatedGroups = append(updatedGroups, currentService)
			}
		}
		updatedAllServices = append(updatedAllServices, updatedGroups)
	}
	return updatedAllServices
}

func updateProductGroup(productName, envName, updateType string, productResp *commonmodels.Product, currentProductServices [][]*commonmodels.ProductService, log *zap.SugaredLogger) error {
	var (
		renderChartMap         = make(map[string]*template.RenderChart)
		productServiceMap      = make(map[string]*commonmodels.ProductService)
		productTemplServiceMap = make(map[string]*commonmodels.ProductService)
	)
	restConfig, err := kube.GetRESTConfig(productResp.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	helmClient, err := helmclient.NewClientFromRestConf(restConfig, productResp.Namespace)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	for _, serviceGroup := range currentProductServices {
		for _, service := range serviceGroup {
			productServiceMap[service.ServiceName] = service
		}
	}

	for _, serviceGroup := range productResp.Services {
		for _, service := range serviceGroup {
			productTemplServiceMap[service.ServiceName] = service
		}
	}

	// 找到环境里面还存在但是服务编排里面已经删除的服务卸载掉
	for serviceName := range productServiceMap {
		if _, isExist := productTemplServiceMap[serviceName]; !isExist {
			go func(namespace, serviceName string) {
				log.Infof("ready to uninstall release:%s", fmt.Sprintf("%s-%s", namespace, serviceName))
				if err = helmClient.UninstallRelease(&helmclient.ChartSpec{
					ReleaseName: fmt.Sprintf("%s-%s", namespace, serviceName),
					Namespace:   namespace,
					Wait:        true,
					Force:       true,
					Timeout:     Timeout * time.Second * 10,
				}); err != nil {
					log.Errorf("helm uninstall release %s err:%v", fmt.Sprintf("%s-%s", namespace, serviceName), err)
				}
			}(productResp.Namespace, serviceName)
		}
	}

	//比较当前环境中的变量和系统默认的最新变量
	renderSet, err := diffRenderSet(productName, envName, updateType, productResp, log)
	if err != nil {
		return e.ErrUpdateEnv.AddDesc("对比环境中的value.yaml和系统默认的value.yaml失败")
	}

	for _, renderChart := range renderSet.ChartInfos {
		renderChartMap[renderChart.ServiceName] = renderChart
	}

	errList := new(multierror.Error)
	for groupIndex, services := range productResp.Services {
		var wg sync.WaitGroup
		groupServices := make([]*commonmodels.ProductService, 0)
		for _, service := range services {
			opt := &commonrepo.ServiceFindOption{
				ServiceName:   service.ServiceName,
				Type:          setting.HelmDeployType,
				ProductName:   productName,
				Revision:      service.Revision,
				ExcludeStatus: setting.ProductStatusDeleting,
			}
			respService, err := commonrepo.NewServiceColl().Find(opt)
			if err != nil {
				log.Warnf("系统未找到当前类型[%s]的服务[%s]!", setting.HelmDeployType, service.ServiceName)
				continue
			}
			if renderChart, isExist := renderChartMap[service.ServiceName]; isExist {
				wg.Add(1)
				go func(tmpRenderChart *template.RenderChart, currentService *commonmodels.Service) {
					defer wg.Done()
					chartSpec := helmclient.ChartSpec{
						ReleaseName: fmt.Sprintf("%s-%s", productResp.Namespace, tmpRenderChart.ServiceName),
						ChartName:   fmt.Sprintf("%s/%s", productResp.Namespace, tmpRenderChart.ServiceName),
						Namespace:   productResp.Namespace,
						Wait:        true,
						Version:     tmpRenderChart.ChartVersion,
						ValuesYaml:  tmpRenderChart.ValuesYaml,
						UpgradeCRDs: true,
						Timeout:     Timeout * time.Second * 10,
					}
					base, err := gerrit.GetGerritWorkspaceBasePath(currentService.RepoName)
					_, serviceFileErr := os.Stat(path.Join(base, currentService.LoadPath))
					if err != nil || os.IsNotExist(serviceFileErr) {
						if err = commonservice.DownloadService(base, currentService.ServiceName); err != nil {
							return
						}
					}
					err = helmClient.InstallOrUpgradeChart(context.Background(), &chartSpec,
						&helmclient.ChartOption{ChartPath: filepath.Join(base, currentService.LoadPath)}, log)

					if err != nil {
						log.Errorf("install helm chart %s error :%+v", chartSpec.ReleaseName, err)
						errList = multierror.Append(errList, err)
					}
				}(renderChart, respService)
				service.Revision = respService.Revision
				groupServices = append(groupServices, service)
			}
		}
		wg.Wait()
		err = commonrepo.NewProductColl().UpdateGroup(envName, productName, groupIndex, groupServices)
		if err != nil {
			log.Errorf("[product.update] err: v%", err)
			errList = multierror.Append(errList, err)
		}
	}

	productResp.Render.Revision = renderSet.Revision
	if err = commonrepo.NewProductColl().Update(productResp); err != nil {
		log.Errorf("[product.update]  err: %v", err)
		errList = multierror.Append(errList, err)
	}

	return errList.ErrorOrNil()
}

// diffRenderSet 对比环境中的renderSet的值和服务的最新的renderSet的值
func diffRenderSet(productName, envName, updateType string, productResp *commonmodels.Product, log *zap.SugaredLogger) (*commonmodels.RenderSet, error) {
	productTemp, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("[ProductTmpl.find] err: %v", err)
		return nil, err
	}
	// 系统默认的变量
	latestRenderSet, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{Name: productName})
	if err != nil {
		log.Errorf("[RenderSet.find] err: %v", err)
		return nil, err
	}

	latestRenderSetMap := make(map[string]*template.RenderChart)
	for _, renderInfo := range latestRenderSet.ChartInfos {
		latestRenderSetMap[renderInfo.ServiceName] = renderInfo
	}

	newChartInfos := make([]*template.RenderChart, 0)
	renderSetName := commonservice.GetProductEnvNamespace(envName, productName)
	switch updateType {
	case UpdateTypeSystem:
		for _, serviceNameGroup := range productTemp.Services {
			for _, serviceName := range serviceNameGroup {
				if latestChartInfo, isExist := latestRenderSetMap[serviceName]; isExist {
					newChartInfos = append(newChartInfos, latestChartInfo)
				}
			}
		}
	case UpdateTypeEnv:
		renderSetOpt := &commonrepo.RenderSetFindOption{Name: renderSetName, Revision: productResp.Render.Revision}
		currentEnvRenderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
		if err != nil {
			log.Errorf("[RenderSet.find] err: %v", err)
			return nil, err
		}

		// 环境里面的变量
		currentEnvRenderSetMap := make(map[string]*template.RenderChart)
		for _, renderInfo := range currentEnvRenderSet.ChartInfos {
			currentEnvRenderSetMap[renderInfo.ServiceName] = renderInfo
		}

		tmpCurrentChartInfoMap := make(map[string]*template.RenderChart)
		tmpLatestChartInfoMap := make(map[string]*template.RenderChart)
		//过滤掉服务编排没有的服务，这部分不需要做diff
		for _, serviceNameGroup := range productTemp.Services {
			for _, serviceName := range serviceNameGroup {
				if currentChartInfo, isExist := currentEnvRenderSetMap[serviceName]; isExist {
					tmpCurrentChartInfoMap[serviceName] = currentChartInfo
				}
				if latestChartInfo, isExist := latestRenderSetMap[serviceName]; isExist {
					tmpLatestChartInfoMap[serviceName] = latestChartInfo
				}
			}
		}

		for serviceName, latestChartInfo := range tmpLatestChartInfoMap {
			if currentChartInfo, ok := tmpCurrentChartInfoMap[serviceName]; ok {
				//拿当前环境values.yaml的key的value去替换服务里面的values.yaml的相同的key的value
				newValuesYaml, err := overrideValues([]byte(currentChartInfo.ValuesYaml), []byte(latestChartInfo.ValuesYaml))
				if err != nil {
					log.Errorf("Failed to override values, err: %s", err)
				} else {
					latestChartInfo.ValuesYaml = string(newValuesYaml)
				}
			}
			newChartInfos = append(newChartInfos, latestChartInfo)
		}
	}

	if err = commonservice.CreateHelmRenderSet(
		&commonmodels.RenderSet{
			Name:        renderSetName,
			EnvName:     envName,
			ProductTmpl: productName,
			ChartInfos:  newChartInfos,
		},
		log,
	); err != nil {
		log.Errorf("[RenderSet.create] err: %v", err)
		return nil, err
	}

	renderSet, err := FindHelmRenderSet(productName, renderSetName, log)
	if err != nil {
		log.Errorf("[RenderSet.find] err: %v", err)
		return nil, err
	}
	return renderSet, nil
}

func overrideValues(currentValuesYaml, latestValuesYaml []byte) ([]byte, error) {
	currentValuesMap := map[string]interface{}{}
	if err := yaml.Unmarshal(currentValuesYaml, &currentValuesMap); err != nil {
		return nil, err
	}

	currentValuesFlatMap, err := converter.Flatten(currentValuesMap)
	if err != nil {
		return nil, err
	}

	latestValuesMap := map[string]interface{}{}
	if err := yaml.Unmarshal(latestValuesYaml, &latestValuesMap); err != nil {
		return nil, err
	}

	latestValuesFlatMap, err := converter.Flatten(latestValuesMap)
	if err != nil {
		return nil, err
	}

	replaceMap := make(map[string]interface{})
	for key := range latestValuesFlatMap {
		if currentValue, ok := currentValuesFlatMap[key]; ok {
			replaceMap[key] = currentValue
		}
	}

	if len(replaceMap) == 0 {
		return nil, nil
	}

	var replaceKV []string
	for k, v := range replaceMap {
		replaceKV = append(replaceKV, fmt.Sprintf("%s=%v", k, v))
	}

	if err := strvals.ParseInto(strings.Join(replaceKV, ","), latestValuesMap); err != nil {
		return nil, err
	}

	return yaml.Marshal(latestValuesMap)
}

func updateProductVariable(productName, envName string, productResp *commonmodels.Product, log *zap.SugaredLogger) error {
	renderChartMap := make(map[string]*template.RenderChart)

	restConfig, err := kube.GetRESTConfig(productResp.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	helmClient, err := helmclient.NewClientFromRestConf(restConfig, productResp.Namespace)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	for _, renderChart := range productResp.ChartInfos {
		renderChartMap[renderChart.ServiceName] = renderChart
	}

	errList := new(multierror.Error)
	for groupIndex, services := range productResp.Services {
		var wg sync.WaitGroup
		groupServices := make([]*commonmodels.ProductService, 0)
		for _, service := range services {
			if renderChart, isExist := renderChartMap[service.ServiceName]; isExist {
				opt := &commonrepo.ServiceFindOption{
					ServiceName:   service.ServiceName,
					Type:          service.Type,
					Revision:      service.Revision,
					ProductName:   productName,
					ExcludeStatus: setting.ProductStatusDeleting,
				}
				serviceObj, err := commonrepo.NewServiceColl().Find(opt)
				if err != nil {
					continue
				}
				wg.Add(1)
				go func(tmpRenderChart *template.RenderChart, currentService *commonmodels.Service) {
					defer wg.Done()

					base, err := gerrit.GetGerritWorkspaceBasePath(currentService.RepoName)
					_, serviceFileErr := os.Stat(path.Join(base, currentService.LoadPath))
					if err != nil || os.IsNotExist(serviceFileErr) {
						if err = commonservice.DownloadService(base, currentService.ServiceName); err != nil {
							return
						}
					}
					chartSpec := helmclient.ChartSpec{
						ReleaseName: fmt.Sprintf("%s-%s", productResp.Namespace, tmpRenderChart.ServiceName),
						ChartName:   fmt.Sprintf("%s/%s", productResp.Namespace, tmpRenderChart.ServiceName),
						Namespace:   productResp.Namespace,
						Wait:        true,
						Version:     tmpRenderChart.ChartVersion,
						ValuesYaml:  tmpRenderChart.ValuesYaml,
						UpgradeCRDs: true,
						Atomic:      true,
						Timeout:     Timeout * time.Second * 10,
					}
					err = helmClient.InstallOrUpgradeChart(context.Background(), &chartSpec, &helmclient.ChartOption{
						ChartPath: filepath.Join(base, currentService.LoadPath)}, log)
					if err != nil {
						errList = multierror.Append(errList, err)
						log.Errorf("install helm chart error :%+v", err)
					}
				}(renderChart, serviceObj)
			}
			groupServices = append(groupServices, service)
		}
		wg.Wait()
		err = commonrepo.NewProductColl().UpdateGroup(envName, productName, groupIndex, groupServices)
		if err != nil {
			log.Errorf("Failed to update collection - service group %d. Error: %v", groupIndex, err)
			return e.ErrUpdateEnv.AddDesc(err.Error())
		}
	}
	if err = commonrepo.NewProductColl().Update(productResp); err != nil {
		errList = multierror.Append(errList, err)
	}
	return errList.ErrorOrNil()
}
