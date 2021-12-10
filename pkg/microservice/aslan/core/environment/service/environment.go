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
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/hashicorp/go-multierror"
	helmclient "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
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
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util"
	"github.com/koderover/zadig/pkg/util/converter"
	"github.com/koderover/zadig/pkg/util/fs"
)

const (
	Timeout          = 60
	UpdateTypeSystem = "systemVar"
	UpdateTypeEnv    = "envVar"
)

const (
	usageScenarioCreateEnv       = "createEnv"
	usageScenarioUpdateEnv       = "updateEnv"
	usageScenarioUpdateRenderSet = "updateRenderSet"
)

type EnvStatus struct {
	EnvName    string `json:"env_name,omitempty"`
	Status     string `json:"status"`
	ErrMessage string `json:"err_message"`
}

type EnvResp struct {
	ProjectName string `json:"projectName"`
	Status      string `json:"status"`
	Error       string `json:"error"`
	Name        string `json:"name"`
	UpdateBy    string `json:"updateBy"`
	UpdateTime  int64  `json:"updateTime"`
	IsPublic    bool   `json:"isPublic"`
	ClusterName string `json:"clusterName"`
	ClusterID   string `json:"cluster_id"`
	Production  bool   `json:"production"`
	Source      string `json:"source"`
	RegistryID  string `json:"registry_id"`
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
	ClusterName string                   `json:"cluster_name,omitempty"`
	RecycleDay  int                      `json:"recycle_day"`
	IsProd      bool                     `json:"is_prod"`
	IsLocal     bool                     `json:"is_local"`
	Source      string                   `json:"source"`
	RegisterID  string                   `json:"registry_id"`
}

type ProductParams struct {
	IsPublic        bool     `json:"isPublic"`
	EnvName         string   `json:"envName"`
	RoleID          int      `json:"roleId"`
	PermissionUUIDs []string `json:"permissionUUIDs"`
}

type EstimateValuesArg struct {
	DefaultValues  string                  `json:"defaultValues"`
	OverrideYaml   string                  `json:"overrideYaml"`
	OverrideValues []*commonservice.KVPair `json:"overrideValues,omitempty"`
}

type EnvRenderChartArg struct {
	ChartValues []*commonservice.RenderChartArg `json:"chartValues"`
}

type EnvRendersetArg struct {
	DefaultValues string                          `json:"defaultValues"`
	ChartValues   []*commonservice.RenderChartArg `json:"chartValues"`
}

type CreateHelmProductArg struct {
	ProductName   string                          `json:"productName"`
	EnvName       string                          `json:"envName"`
	Namespace     string                          `json:"namespace"`
	ClusterID     string                          `json:"clusterID"`
	DefaultValues string                          `json:"defaultValues"`
	RegistryID    string                          `json:"registry_id"`
	ChartValues   []*commonservice.RenderChartArg `json:"chartValues"`
}

type UpdateMultiHelmProductArg struct {
	ProductName   string                          `json:"productName"`
	EnvNames      []string                        `json:"envNames"`
	ChartValues   []*commonservice.RenderChartArg `json:"chartValues"`
	ReplacePolicy string                          `json:"replacePolicy"` // TODO logic not implemented
}

type RawYamlResp struct {
	YamlContent string `json:"yamlContent"`
}

type intervalExecutorHandler func(data *commonmodels.Service, log *zap.SugaredLogger) error

func ListProducts(projectName string, envNames []string, log *zap.SugaredLogger) ([]*EnvResp, error) {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: projectName, InEnvs: envNames, IsSortByProductName: true})
	if err != nil {
		log.Errorf("Failed to list envs, err: %s", err)
		return nil, e.ErrListEnvs.AddDesc(err.Error())
	}

	clusterMap := make(map[string]*commonmodels.K8SCluster)
	clusters, err := commonrepo.NewK8SClusterColl().List(nil)
	if err != nil {
		log.Errorf("Failed to list clusters in db, err: %s", err)
		return nil, err
	}

	for _, cls := range clusters {
		clusterMap[cls.ID.Hex()] = cls
	}

	var res []*EnvResp
	reg, err := commonservice.FindDefaultRegistry(log)
	if err != nil {
		log.Errorf("FindDefaultRegistry error: %v", err)
		return nil, err
	}
	for _, env := range envs {
		clusterID := env.ClusterID
		production := false
		clusterName := ""
		cluster, ok := clusterMap[clusterID]
		if len(env.RegistryID) == 0 {
			env.RegistryID = reg.ID.Hex()
		}
		if ok {
			production = cluster.Production
			clusterName = cluster.Name
		}

		res = append(res, &EnvResp{
			ProjectName: projectName,
			Name:        env.EnvName,
			IsPublic:    env.IsPublic,
			ClusterName: clusterName,
			Source:      env.Source,
			Production:  production,
			Status:      env.Status,
			Error:       env.Error,
			UpdateTime:  env.UpdateTime,
			UpdateBy:    env.UpdateBy,
			RegistryID:  env.RegistryID,
			ClusterID:   env.ClusterID,
		})
	}

	return res, nil
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

	mutexAutoCreate.Lock()
	defer func() {
		mutexAutoCreate.Unlock()
	}()

	envStatus := make([]*EnvStatus, 0)
	envNames := []string{"dev", "qa"}
	for _, envName := range envNames {
		devStatus := &EnvStatus{
			EnvName: envName,
		}
		status, err := autoCreateProduct(envType, envName, productName, requestID, setting.SystemUser, log)
		devStatus.Status = status
		if err != nil {
			devStatus.ErrMessage = err.Error()
		}
		envStatus = append(envStatus, devStatus)
	}
	return envStatus
}

var mutexAutoUpdate sync.RWMutex

func AutoUpdateProduct(envNames []string, productName, requestID string, force bool, log *zap.SugaredLogger) ([]*EnvStatus, error) {
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

			kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), p.ClusterID)
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

	productsRevison, err := ListProductsRevision(productName, "", log)
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
	var allRenders []*commonmodels.RenderSet
	var prodRevs *ProductRevision

	allServices, err = commonrepo.NewServiceColl().ListAllRevisions()
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

	prodRevs, err = GetProductRevision(existedProd, allServices, allRenders, renderSet, log)
	if err != nil {
		err = e.ErrUpdateEnv.AddDesc(e.GetEnvRevErrMsg)
		return
	}

	// 无需更新
	if !prodRevs.Updatable {
		log.Errorf("[%s][P:%s] nothing to update", envName, productName)
		return
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), existedProd.ClusterID)
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
					ProductName: prodService.ProductName,
					Type:        svcRev.Type,
					Revision:    svcRev.NextRevision,
				}

				service.Containers = svcRev.Containers
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
								errList = multierror.Append(errList, errors.New(e.Error()))
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

	return nil
}

func UpdateProductRegistry(namespace, registryID string, log *zap.SugaredLogger) (err error) {
	opt := &commonrepo.ProductFindOptions{Namespace: namespace}
	exitedProd, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("UpdateProductRegistry find product by namespace:%s,error: %v", namespace, err)
		return e.ErrUpdateEnv.AddDesc(e.EnvNotFoundErrMsg)
	}
	err = commonrepo.NewProductColl().UpdateRegistry(namespace, registryID)
	if err != nil {
		log.Errorf("UpdateProductRegistry UpdateRegistry by namespace:%s registryID:%s error: %v", namespace, registryID, err)
		return e.ErrUpdateEnv.AddErr(err)
	}
	kubeClient, err := kube.GetKubeClient(exitedProd.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}
	err = ensureKubeEnv(exitedProd.Namespace, registryID, kubeClient, log)

	if err != nil {
		log.Errorf("UpdateProductRegistry ensureKubeEnv by namespace:%s,error: %v", namespace, err)
		return err
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

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), exitedProd.ClusterID)
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

	err = ensureKubeEnv(exitedProd.Namespace, exitedProd.RegistryID, kubeClient, log)

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
	renderSet, err := commonservice.ValidateRenderSet(exitedProd.ProductName, exitedProd.Render.Name, nil, log)
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

			log.Infof("[%s][P:%s] update error to => %s", envName, productName, err)
			if err2 := commonrepo.NewProductColl().UpdateErrors(envName, productName, err.Error()); err2 != nil {
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

func CreateHelmProduct(productName, userName, requestID string, args []*CreateHelmProductArg, log *zap.SugaredLogger) error {
	templateProduct, err := templaterepo.NewProductColl().Find(productName)
	if err != nil || templateProduct == nil {
		if err != nil {
			log.Errorf("failed to query product %s, err %s ", productName, err.Error())
		}
		return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to query product %s ", productName))
	}

	err = commonservice.FillProductTemplateValuesYamls(templateProduct, log)
	if err != nil {
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	// generate service group data
	allServiceInfoMap := templateProduct.AllServiceInfoMap()
	var serviceGroup [][]*commonmodels.ProductService
	for _, names := range templateProduct.Services {
		servicesResp := make([]*commonmodels.ProductService, 0)
		for _, serviceName := range names {
			opt := &commonrepo.ServiceFindOption{
				ServiceName:   serviceName,
				ProductName:   allServiceInfoMap[serviceName].Owner,
				ExcludeStatus: setting.ProductStatusDeleting,
			}

			serviceTmpl, err := commonrepo.NewServiceColl().Find(opt)
			if err != nil {
				errMsg := fmt.Sprintf("Can not find service with option %+v, error: %v", opt, err)
				log.Error(errMsg)
				return e.ErrCreateEnv.AddDesc(errMsg)
			}

			serviceResp := &commonmodels.ProductService{
				ServiceName: serviceTmpl.ServiceName,
				ProductName: serviceTmpl.ProductName,
				Type:        serviceTmpl.Type,
				Revision:    serviceTmpl.Revision,
			}
			if serviceTmpl.Type == setting.K8SDeployType || serviceTmpl.Type == setting.HelmDeployType {
				serviceResp.Containers = make([]*commonmodels.Container, 0)
				for _, c := range serviceTmpl.Containers {
					container := &commonmodels.Container{
						Name:      c.Name,
						Image:     c.Image,
						ImagePath: c.ImagePath,
					}
					serviceResp.Containers = append(serviceResp.Containers, container)
				}
			}
			servicesResp = append(servicesResp, serviceResp)
		}
		serviceGroup = append(serviceGroup, servicesResp)
	}

	errList := new(multierror.Error)
	for _, arg := range args {
		err = createSingleHelmProduct(templateProduct, serviceGroup, requestID, userName, arg.RegistryID, arg, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}
	return errList.ErrorOrNil()
}

func createSingleHelmProduct(templateProduct *template.Product, serviceGroup [][]*commonmodels.ProductService, requestID, userName, registryID string, arg *CreateHelmProductArg, log *zap.SugaredLogger) error {
	productObj := &commonmodels.Product{
		ProductName:     templateProduct.ProductName,
		Revision:        1,
		Enabled:         false,
		EnvName:         arg.EnvName,
		UpdateBy:        userName,
		Services:        serviceGroup,
		IsPublic:        true,
		ClusterID:       arg.ClusterID,
		Namespace:       commonservice.GetProductEnvNamespace(arg.EnvName, arg.ProductName, arg.Namespace),
		Source:          setting.SourceFromHelm,
		IsOpenSource:    templateProduct.IsOpensource,
		ChartInfos:      templateProduct.ChartInfos,
		IsForkedProduct: false,
		RegistryID:      registryID,
	}

	customChartValueMap := make(map[string]*commonservice.RenderChartArg)
	for _, singleCV := range arg.ChartValues {
		customChartValueMap[singleCV.ServiceName] = singleCV
	}

	for _, latestChart := range productObj.ChartInfos {
		if singleCV, ok := customChartValueMap[latestChart.ServiceName]; ok {
			singleCV.FillRenderChartModel(latestChart, latestChart.ChartVersion)
		}
	}

	// default values
	defaultValuesYaml := arg.DefaultValues

	// insert renderset info into db
	if len(productObj.ChartInfos) > 0 {
		err := commonservice.CreateHelmRenderSet(&commonmodels.RenderSet{
			Name:          commonservice.GetProductEnvNamespace(arg.EnvName, arg.ProductName, arg.Namespace),
			EnvName:       arg.EnvName,
			ProductTmpl:   arg.ProductName,
			UpdateBy:      userName,
			IsDefault:     false,
			DefaultValues: defaultValuesYaml,
			ChartInfos:    productObj.ChartInfos,
		}, log)
		if err != nil {
			log.Errorf("rennderset create fail when creating helm product, productName: %s", arg.ProductName)
			return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to save chart values, productName: %s", arg.ProductName))
		}
	}

	return CreateProduct(userName, requestID, productObj, log)
}

// CreateProduct create a new product with its dependent stacks
func CreateProduct(user, requestID string, args *commonmodels.Product, log *zap.SugaredLogger) (err error) {
	log.Infof("[%s][P:%s] CreateProduct", args.EnvName, args.ProductName)
	creator := getCreatorBySource(args.Source)
	return creator.Create(user, requestID, args, log)
}

func UpdateProductRecycleDay(envName, productName string, recycleDay int) error {
	return commonrepo.NewProductColl().UpdateProductRecycleDay(envName, productName, recycleDay)
}

func UpdateHelmProduct(productName, envName, updateType, username, requestID string, overrideCharts []*commonservice.RenderChartArg, log *zap.SugaredLogger) error {
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
		err := updateProductGroup(username, productName, envName, updateType, productResp, currentProductService, overrideCharts, log)
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

			log.Infof("[%s][P:%s] update error to => %s", envName, productName, err)
			if err2 := commonrepo.NewProductColl().UpdateErrors(envName, productName, err.Error()); err2 != nil {
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

func prepareEstimatedData(productName, envName, serviceName, usageScenario, defaultValues string, log *zap.SugaredLogger) (string, string, error) {
	var err error
	templateService, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		ProductName: productName,
		Type:        setting.HelmDeployType,
	})
	if err != nil {
		log.Errorf("failed to query service, name %s, err %s", serviceName, err)
		return "", "", fmt.Errorf("failed to query service, name %s", serviceName)
	}

	if usageScenario == usageScenarioCreateEnv {
		return templateService.HelmChart.ValuesYaml, defaultValues, nil
	}

	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to query product info, name %s", envName)
	}

	// find chart info from cur render set
	opt := &mongodb.RenderSetFindOption{Name: productInfo.Render.Name, Revision: productInfo.Render.Revision}
	renderSet, err := mongodb.NewRenderSetColl().Find(opt)
	if err != nil {
		log.Errorf("renderset Find error, productName:%s, envName:%s, err:%s", productInfo.ProductName, productInfo.EnvName, err)
		return "", "", fmt.Errorf("failed to query renderset info, name %s", productInfo.Render.Name)
	}

	// find target render chart from render set
	var targetChart *templatemodels.RenderChart
	for _, chart := range renderSet.ChartInfos {
		if chart.ServiceName == serviceName {
			targetChart = chart
			break
		}
	}

	switch usageScenario {
	case usageScenarioUpdateEnv:
		imageRelatedKey := sets.NewString()
		for _, container := range templateService.Containers {
			if container.ImagePath != nil {
				imageRelatedKey.Insert(container.ImagePath.Image, container.ImagePath.Repo, container.ImagePath.Tag)
			}
		}

		curValuesYaml := ""
		if targetChart != nil { // service has been applied into environment, use current values.yaml
			curValuesYaml = targetChart.ValuesYaml
		}

		// merge environment values
		mergedBs, err := overrideValues([]byte(curValuesYaml), []byte(templateService.HelmChart.ValuesYaml), imageRelatedKey)
		if err != nil {
			return "", "", errors.Wrapf(err, "failed to override values")
		}
		return string(mergedBs), renderSet.DefaultValues, nil
	case usageScenarioUpdateRenderSet:
		if targetChart == nil {
			return "", "", fmt.Errorf("failed to find chart info, name: %s", serviceName)
		}
		return targetChart.ValuesYaml, renderSet.DefaultValues, nil
	default:
		return "", "", fmt.Errorf("unrecognized usageScenario:%s", usageScenario)
	}
}

func GeneEstimatedValues(productName, envName, serviceName, scene, format string, arg *EstimateValuesArg, log *zap.SugaredLogger) (interface{}, error) {
	chartValues, defaultValues, err := prepareEstimatedData(productName, envName, serviceName, scene, arg.DefaultValues, log)
	if err != nil {
		return nil, e.ErrUpdateRenderSet.AddDesc(fmt.Sprintf("failed to prepare data, err %s", err))
	}

	tempArg := &commonservice.RenderChartArg{OverrideValues: arg.OverrideValues}
	mergeValues, err := helmtool.MergeOverrideValues(chartValues, defaultValues, arg.OverrideYaml, tempArg.ToOverrideValueString())
	if err != nil {
		return nil, e.ErrUpdateRenderSet.AddDesc(fmt.Sprintf("failed to merge values, err %s", err))
	}

	switch format {
	case "flatMap":
		mapData, err := converter.YamlToFlatMap([]byte(mergeValues))
		if err != nil {
			return nil, e.ErrUpdateRenderSet.AddDesc(fmt.Sprintf("failed to generate flat map , err %s", err))
		}
		return mapData, nil
	default:
		return &RawYamlResp{YamlContent: mergeValues}, nil
	}
}

// check if override values or yaml content changes
func checkOverrideValuesChange(source *template.RenderChart, args *commonservice.RenderChartArg) bool {
	if source.OverrideValues != args.ToOverrideValueString() || source.GetOverrideYaml() != args.OverrideYaml {
		return true
	}
	return false
}

func UpdateHelmProductRenderset(productName, envName, userName, requestID string, args *EnvRendersetArg, log *zap.SugaredLogger) error {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		log.Errorf("UpdateHelmProductRenderset GetProductEnv envName:%s productName: %s error, error msg:%s", envName, productName, err)
		return err
	}
	opt := &commonrepo.RenderSetFindOption{Name: product.Namespace}
	productRenderset, _, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil || productRenderset == nil {
		if err != nil {
			log.Infof("query renderset fail when updating helm product:%s render charts, err %s", productName, err.Error())
		}
		return e.ErrUpdateEnv.AddDesc(fmt.Sprintf("failed to query renderset for envirionment: %s", envName))
	}

	// render charts need to be updated
	updatedRcList := make([]*template.RenderChart, 0)
	updatedRCMap := make(map[string]*template.RenderChart)

	// default values change
	if args.DefaultValues != productRenderset.DefaultValues {
		for _, curRenderChart := range productRenderset.ChartInfos {
			updatedRCMap[curRenderChart.ServiceName] = curRenderChart
		}
		productRenderset.DefaultValues = args.DefaultValues
	}

	for _, requestRenderChart := range args.ChartValues {
		// update renderset info
		for _, curRenderChart := range productRenderset.ChartInfos {
			if curRenderChart.ServiceName != requestRenderChart.ServiceName {
				continue
			}
			if !checkOverrideValuesChange(curRenderChart, requestRenderChart) {
				continue
			}
			requestRenderChart.FillRenderChartModel(curRenderChart, curRenderChart.ChartVersion)
			updatedRCMap[curRenderChart.ServiceName] = curRenderChart
			break
		}
	}

	for _, updatedRc := range updatedRCMap {
		updatedRcList = append(updatedRcList, updatedRc)
	}

	err = UpdateHelmProductVariable(productName, envName, userName, requestID, updatedRcList, productRenderset, log)
	if err != nil {
		return err
	}
	kubeClient, err := kube.GetKubeClient(product.ClusterID)
	if err != nil {
		log.Errorf("UpdateHelmProductRenderset GetKubeClient error, error msg:%s", err)
		return err
	}
	return ensureKubeEnv(product.Namespace, product.RegistryID, kubeClient, log)
}

func UpdateHelmProductVariable(productName, envName, username, requestID string, updatedRcs []*template.RenderChart, renderset *commonmodels.RenderSet, log *zap.SugaredLogger) error {
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
	productResp.ChartInfos = updatedRcs

	if err = commonservice.CreateHelmRenderSet(
		&commonmodels.RenderSet{
			Name:          productResp.Namespace,
			EnvName:       envName,
			ProductTmpl:   productName,
			UpdateBy:      username,
			DefaultValues: renderset.DefaultValues,
			ChartInfos:    renderset.ChartInfos,
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

	return updateHelmProductVariable(productResp, renderSet, oldRenderVersion, username, requestID, log)
}

func updateHelmProductVariable(productResp *commonmodels.Product, renderset *commonmodels.RenderSet, oldRenderVersion int64, userName, requestID string, log *zap.SugaredLogger) error {

	envName, productName := productResp.EnvName, productResp.ProductName

	// 设置产品状态为更新中
	if err := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUpdating); err != nil {
		log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.UpdateEnvStatusErrMsg)
	}

	go func() {
		err := updateProductVariable(productName, envName, productResp, renderset, log)
		if err != nil {
			log.Errorf("[%s][P:%s] failed to update product %#v", envName, productName, err)
			// 发送更新产品失败消息给用户
			title := fmt.Sprintf("更新 [%s] 的 [%s] 环境失败", productName, envName)
			commonservice.SendErrorMessage(userName, title, requestID, err, log)

			// 设置产品状态
			log.Infof("[%s][P:%s] update status to => %s", envName, productName, setting.ProductStatusFailed)
			if err2 := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusFailed); err2 != nil {
				log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err2)
				return
			}

			log.Infof("[%s][P:%s] update error to => %s", envName, productName, err)
			if err2 := commonrepo.NewProductColl().UpdateErrors(envName, productName, err.Error()); err2 != nil {
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

func UpdateMultipleHelmEnv(requestID string, args *UpdateMultiHelmProductArg, log *zap.SugaredLogger) ([]*EnvStatus, error) {
	mutexUpdateMultiHelm.Lock()
	defer func() {
		mutexUpdateMultiHelm.Unlock()
	}()

	envNames, productName := args.EnvNames, args.ProductName

	envStatuses := make([]*EnvStatus, 0)
	productsRevision, err := ListProductsRevision(productName, "", log)
	if err != nil {
		log.Errorf("UpdateMultiHelmProduct ListProductsRevision err:%v", err)
		return envStatuses, err
	}

	envNameSet := sets.NewString(envNames...)
	productMap := make(map[string]*ProductRevision)
	for _, productRevision := range productsRevision {
		if productRevision.ProductName != productName || !envNameSet.Has(productRevision.EnvName) {
			continue
		}
		if !productRevision.Updatable {
			continue
		}
		productMap[productRevision.EnvName] = productRevision
		if len(productMap) == len(envNames) {
			break
		}
	}

	serviceList, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(args.ProductName)
	if err != nil {
		log.Infof("query services from product: %s fail, error %s", args.ProductName, err.Error())
		return envStatuses, e.ErrUpdateEnv.AddDesc("failed to query services")
	}

	serviceMap := make(map[string]*commonmodels.Service)
	for _, singleService := range serviceList {
		serviceMap[singleService.ServiceName] = singleService
	}

	// extract values.yaml and update renderset
	for envName, _ := range productMap {
		renderSet, _, err := commonrepo.NewRenderSetColl().FindRenderSet(&commonrepo.RenderSetFindOption{
			Name: commonservice.GetProductEnvNamespace(envName, productName, ""),
		})
		if err != nil || renderSet == nil {
			if err != nil {
				log.Warnf("query renderset fail for product %s env: %s", productName, envName)
			}
			return envStatuses, e.ErrUpdateEnv.AddDesc(fmt.Sprintf("failed to query renderset for env: %s", envName))
		}

		err = UpdateHelmProduct(productName, envName, UpdateTypeEnv, setting.SystemUser, requestID, args.ChartValues, log)
		if err != nil {
			log.Errorf("UpdateMultiHelmProduct UpdateProductV2 err:%v", err)
			return envStatuses, e.ErrUpdateEnv.AddDesc(err.Error())
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

func GetProductInfo(username, envName, productName string, log *zap.SugaredLogger) (*commonmodels.Product, error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[User:%s][EnvName:%s][Product:%s] Product.FindByOwner error: %v", username, envName, productName, err)
		return nil, e.ErrGetEnv
	}

	renderSetName := prod.Namespace
	renderSetOpt := &commonrepo.RenderSetFindOption{Name: renderSetName, Revision: prod.Render.Revision}
	renderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
	if err != nil {
		log.Errorf("find helm renderset[%s] error: %v", renderSetName, err)
		return prod, nil
	}
	prod.ChartInfos = renderSet.ChartInfos

	return prod, nil
}

func GetProductIngress(productName string, log *zap.SugaredLogger) ([]*ProductIngressInfo, error) {
	productIngressInfos := make([]*ProductIngressInfo, 0)
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: productName})
	if err != nil {
		log.Errorf("Failed to list envs, err: %s", err)
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

	prodTmpl, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("[EnvName:%s][Product:%s] get product template error: %v", envName, productName, err)
		return nil, e.ErrGetEnv
	}

	//当前环境的renderset
	renderSetName := prod.Namespace
	renderSetOpt := &commonrepo.RenderSetFindOption{Name: renderSetName, Revision: prod.Render.Revision}
	renderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
	if err != nil {
		log.Errorf("find helm renderset[%s] error: %v", renderSetName, err)
		return helmVersions, err
	}
	for _, chartInfo := range renderSet.ChartInfos {
		chartInfoMap[chartInfo.ServiceName] = chartInfo
	}

	//当前环境内的服务信息
	prodServiceMap := prod.GetServiceMap()

	// all services
	serviceListOpt := &commonrepo.ServiceListOption{
		ProductName: productName,
		Type:        setting.HelmDeployType,
	}
	for _, serviceGroup := range prodTmpl.Services {
		for _, serviceName := range serviceGroup {
			serviceListOpt.InServices = append(serviceListOpt.InServices, &template.ServiceInfo{
				Name:  serviceName,
				Owner: productName,
			})
		}
	}
	//当前项目内最新的服务信息
	latestServices, err := commonrepo.NewServiceColl().ListMaxRevisions(serviceListOpt)
	if err != nil {
		log.Errorf("find service revision list error: %v", err)
		return helmVersions, err
	}

	for _, latestSvc := range latestServices {
		if prodService, ok := prodServiceMap[latestSvc.ServiceName]; ok {
			delete(prodServiceMap, latestSvc.ServiceName)
			if latestSvc.Revision == prodService.Revision {
				continue
			}
			helmVersion := &commonmodels.HelmVersions{
				ServiceName:      latestSvc.ServiceName,
				LatestVersion:    latestSvc.HelmChart.Version,
				LatestValuesYaml: latestSvc.HelmChart.ValuesYaml,
			}
			if chartInfo, ok := chartInfoMap[latestSvc.ServiceName]; ok {
				helmVersion.CurrentVersion = chartInfo.ChartVersion
				helmVersion.CurrentValuesYaml = chartInfo.ValuesYaml
			}
			helmVersions = append(helmVersions, helmVersion)
		} else { // new service
			helmVersion := &commonmodels.HelmVersions{
				ServiceName:      latestSvc.ServiceName,
				LatestVersion:    latestSvc.HelmChart.Version,
				LatestValuesYaml: latestSvc.HelmChart.ValuesYaml,
			}
			helmVersions = append(helmVersions, helmVersion)
		}
	}

	// deleted service
	for _, prodService := range prodServiceMap {
		helmVersion := &commonmodels.HelmVersions{
			ServiceName: prodService.ServiceName,
		}
		if chartInfo, ok := chartInfoMap[prodService.ServiceName]; ok {
			helmVersion.CurrentVersion = chartInfo.ChartVersion
			helmVersion.CurrentValuesYaml = chartInfo.ValuesYaml
		}
		helmVersions = append(helmVersions, helmVersion)
	}

	return helmVersions, nil
}

func GetEstimatedRenderCharts(productName, envName, serviceNameListStr string, log *zap.SugaredLogger) ([]*commonservice.RenderChartArg, error) {

	var serviceNameList []string
	// no service appointed, find all service templates
	if serviceNameListStr == "" {
		prodTmpl, err := templaterepo.NewProductColl().Find(productName)
		if err != nil {
			log.Errorf("query product: %s fail, err %s", productName, err.Error())
			return nil, e.ErrGetRenderSet.AddDesc(fmt.Sprintf("query product info fail"))
		}
		for _, singleService := range prodTmpl.AllServiceInfos() {
			serviceNameList = append(serviceNameList, singleService.Name)
		}
		serviceNameListStr = strings.Join(serviceNameList, ",")
	} else {
		serviceNameList = strings.Split(serviceNameListStr, ",")
	}

	// find renderchart info in env
	renderChartInEnv, err := GetRenderCharts(productName, envName, serviceNameListStr, log)
	if err != nil {
		log.Errorf("find render charts in env fail, env %s err %s", envName, err.Error())
		return nil, e.ErrGetRenderSet.AddDesc("failed to get render charts in env")
	}

	rcMap := make(map[string]*commonservice.RenderChartArg)
	for _, rc := range renderChartInEnv {
		rcMap[rc.ServiceName] = rc
	}

	serviceOption := &commonrepo.ServiceListOption{
		ProductName: productName,
		Type:        setting.HelmDeployType,
	}

	for _, serviceName := range serviceNameList {
		if _, ok := rcMap[serviceName]; ok {
			continue
		}
		serviceOption.InServices = append(serviceOption.InServices, &template.ServiceInfo{
			Name:  serviceName,
			Owner: productName,
		})
	}

	if len(serviceOption.InServices) > 0 {
		serviceList, err := commonrepo.NewServiceColl().ListMaxRevisions(serviceOption)
		if err != nil {
			log.Errorf("list service fail, productName %s err %s", productName, err.Error())
			return nil, e.ErrGetRenderSet.AddDesc("failed to get service template info")
		}
		for _, singleService := range serviceList {
			rcMap[singleService.ServiceName] = &commonservice.RenderChartArg{
				EnvName:      envName,
				ServiceName:  singleService.ServiceName,
				ChartVersion: singleService.HelmChart.Version,
			}
		}
	}

	ret := make([]*commonservice.RenderChartArg, 0, len(rcMap))
	for _, rc := range rcMap {
		ret = append(ret, rc)
	}
	return ret, nil
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
				return fmt.Sprintf(format+" %s 失败:%v", service.ServiceName, es[0])
			}

			points := make([]string, len(es))
			for i, err := range es {
				points[i] = fmt.Sprintf("* %v", err)
			}

			return fmt.Sprintf(format+" %s 失败:\n%s", service.ServiceName, strings.Join(points, "\n"))
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

			podLabels, _, err := unstructured.NestedStringMap(u.Object, "spec", "template", "metadata", "labels")
			if err != nil {
				podLabels = nil
			}
			err = unstructured.SetNestedStringMap(u.Object, kube.MergeLabels(labels, podLabels), "spec", "template", "metadata", "labels")
			if err != nil {
				log.Errorf("merge label failed err:%s", err)
				u.Object = setFieldValueIsNotExist(u.Object, kube.MergeLabels(labels, podLabels), "spec", "template", "metadata", "labels")
			}

			podAnnotations, _, err := unstructured.NestedStringMap(u.Object, "spec", "template", "metadata", "annotations")
			if err != nil {
				podAnnotations = nil
			}
			err = unstructured.SetNestedStringMap(u.Object, applyUpdatedAnnotations(podAnnotations), "spec", "template", "metadata", "annotations")
			if err != nil {
				log.Errorf("merge annotation failed err:%s", err)
				u.Object = setFieldValueIsNotExist(u.Object, applyUpdatedAnnotations(podAnnotations), "spec", "template", "metadata", "annotations")
			}

			// Inject selector: s-product and s-service
			selector, _, err := unstructured.NestedStringMap(u.Object, "spec", "selector", "matchLabels")
			if err != nil {
				selector = nil
			}

			err = unstructured.SetNestedStringMap(u.Object, kube.MergeLabels(labels, selector), "spec", "selector", "matchLabels")
			if err != nil {
				log.Errorf("merge selector failed err:%s", err)
				u.Object = setFieldValueIsNotExist(u.Object, kube.MergeLabels(labels, selector), "spec", "selector", "matchLabels")
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
				// Inject resource request and limit
				applySystemResourceRequirements(&res.Spec.Template.Spec)
				// Inject imagePullSecrets if qn-registry-secret is not set
				applySystemImagePullSecrets(&res.Spec.Template.Spec)

				err = updater.CreateOrPatchDeployment(res, kubeClient)
				if err != nil {
					log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), res, err)
					errList = multierror.Append(errList, err)
					continue
				}
			case *appsv1.StatefulSet:
				// Inject resource request and limit
				applySystemResourceRequirements(&res.Spec.Template.Spec)
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

			applySystemResourceRequirements(&obj.Spec.Template.Spec)
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

			applySystemResourceRequirements(&obj.Spec.JobTemplate.Spec.Template.Spec)
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
		ProductName: service.ProductName,
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

func preCreateProduct(envName string, args *commonmodels.Product, kubeClient client.Client,
	log *zap.SugaredLogger) error {
	var (
		productTemplateName = args.ProductName
		renderSetName       = commonservice.GetProductEnvNamespace(envName, args.ProductName, args.Namespace)
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

	tmpRenderInfo := &commonmodels.RenderInfo{Name: renderSetName, ProductTmpl: args.ProductName}
	if args.Render != nil && args.Render.Revision > 0 {
		tmpRenderInfo.Revision = args.Render.Revision
	}

	args.Render = tmpRenderInfo
	if preCreateNSAndSecret(productTmpl.ProductFeature) {
		return ensureKubeEnv(args.Namespace, args.RegistryID, kubeClient, log)
	}
	return nil
}

func preCreateNSAndSecret(productFeature *template.ProductFeature) bool {
	if productFeature == nil {
		return true
	}
	if productFeature != nil && productFeature.BasicFacility != setting.BasicFacilityCVM {
		return true
	}
	return false
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
		if secret.Name == setting.DefaultImagePullSecret {
			return
		}
	}
	podSpec.ImagePullSecrets = append(podSpec.ImagePullSecrets,
		corev1.LocalObjectReference{
			Name: setting.DefaultImagePullSecret,
		})
}

func ensureKubeEnv(namespace string, registryId string, kubeClient client.Client, log *zap.SugaredLogger) error {
	err := kube.CreateNamespace(namespace, kubeClient)
	if err != nil {
		log.Errorf("[%s] get or create namespace error: %v", namespace, err)
		return e.ErrCreateNamspace.AddDesc(e.SetNamespaceErrMsg)
	}

	// 创建默认的镜像仓库secret
	if err := commonservice.EnsureDefaultRegistrySecret(namespace, registryId, kubeClient, log); err != nil {
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

func installOrUpgradeHelmChart(namespace string, renderChart *template.RenderChart, defaultValues string, serviceObj *commonmodels.Service, timeout time.Duration, helmClient helmclient.Client) error {
	mergedValuesYaml, err := helmtool.MergeOverrideValues(renderChart.ValuesYaml, defaultValues, renderChart.GetOverrideYaml(), renderChart.OverrideValues)
	if err != nil {
		err = errors.WithMessagef(err, "failed to merge override yaml %s and values %s", renderChart.GetOverrideYaml(), renderChart.OverrideValues)
		return err
	}
	return installOrUpgradeHelmChartWithValues(namespace, mergedValuesYaml, renderChart, serviceObj, timeout, helmClient)
}

func installOrUpgradeHelmChartWithValues(namespace, valuesYaml string, renderChart *template.RenderChart, serviceObj *commonmodels.Service, timeout time.Duration, helmClient helmclient.Client) error {
	base := config.LocalServicePathWithRevision(serviceObj.ProductName, serviceObj.ServiceName, serviceObj.Revision)
	if err := commonservice.PreloadServiceManifestsByRevision(base, serviceObj); err != nil {
		log.Warnf("failed to get chart of revision: %d for service: %s, use latest version",
			serviceObj.Revision, serviceObj.ServiceName)
		// use the latest version when it fails to download the specific version
		base = config.LocalServicePath(serviceObj.ProductName, serviceObj.ServiceName)
		if err = commonservice.PreLoadServiceManifests(base, serviceObj); err != nil {
			log.Errorf("failed to load chart info for service %v", serviceObj.ServiceName)
			return fmt.Errorf("failed to load chart info for service %s", serviceObj.ServiceName)
		}
	}

	chartFullPath := filepath.Join(base, serviceObj.ServiceName)
	chartPath, err := fs.RelativeToCurrentPath(chartFullPath)
	if err != nil {
		log.Errorf("Failed to get relative path %s, err: %s", chartFullPath, err)
		return err
	}

	chartSpec := &helmclient.ChartSpec{
		ReleaseName: util.GeneHelmReleaseName(namespace, serviceObj.ServiceName),
		ChartName:   chartPath,
		Namespace:   namespace,
		Version:     renderChart.ChartVersion,
		ValuesYaml:  valuesYaml,
		UpgradeCRDs: true,
	}
	if timeout > 0 {
		chartSpec.Wait = true
		chartSpec.Timeout = timeout
	}

	if _, err = helmClient.InstallOrUpgradeChart(context.TODO(), chartSpec); err != nil {
		return err
	}
	return nil
}

func installProductHelmCharts(user, envName, requestID string, args *commonmodels.Product, renderset *commonmodels.RenderSet, eventStart int64, helmClient helmclient.Client, log *zap.SugaredLogger) {
	var (
		err     error
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

	handler := func(serviceObj *commonmodels.Service, logger *zap.SugaredLogger) error {
		renderChart := chartInfoMap[serviceObj.ServiceName]
		err = installOrUpgradeHelmChart(args.Namespace, renderChart, renderset.DefaultValues, serviceObj, 0, helmClient)
		if err != nil {
			return err
		}
		return nil
	}

	for _, serviceGroups := range args.Services {
		serviceList := make([]*commonmodels.Service, 0)
		for _, svc := range serviceGroups {
			_, ok := chartInfoMap[svc.ServiceName]
			if !ok {
				continue
			}

			// 获取服务详情
			opt := &commonrepo.ServiceFindOption{
				ServiceName:   svc.ServiceName,
				Type:          svc.Type,
				Revision:      svc.Revision,
				ProductName:   args.ProductName,
				ExcludeStatus: setting.ProductStatusDeleting,
			}
			serviceObj, err := commonrepo.NewServiceColl().Find(opt)
			if err != nil {
				errList = multierror.Append(errList, errors.Wrapf(err, "failed to find template servce, serviceName %s", svc.ServiceName))
				continue
			}

			serviceList = append(serviceList, serviceObj)
		}
		serviceGroupErr := intervalExecutorWithRetry(5, time.Millisecond*2500, serviceList, handler, log)
		if serviceGroupErr != nil {
			errList = multierror.Append(errList, serviceGroupErr...)
		}
	}

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
					ProductName: service.ProductName,
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

func intervalExecutorWithRetry(retryCount uint64, interval time.Duration, serviceList []*commonmodels.Service, handler intervalExecutorHandler, log *zap.SugaredLogger) []error {
	bo := backoff.NewConstantBackOff(time.Second * 3)
	retryBo := backoff.WithMaxRetries(bo, retryCount)
	errList := make([]error, 0)
	_ = backoff.Retry(func() error {
		failedServices := make([]*commonmodels.Service, 0)
		errList = intervalExecutor(interval, serviceList, &failedServices, handler, log)
		if len(errList) == 0 {
			return nil
		}
		log.Infof("%d services waiting to retry", len(failedServices))
		serviceList = failedServices
		return fmt.Errorf("%d services apply failed", len(errList))
	}, retryBo)
	return errList
}

func intervalExecutor(interval time.Duration, serviceList []*commonmodels.Service, failedServices *[]*commonmodels.Service, handler intervalExecutorHandler, log *zap.SugaredLogger) []error {
	if len(serviceList) == 0 {
		return nil
	}
	wg := sync.WaitGroup{}
	wg.Add(len(serviceList))
	errList := make([]error, 0)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for _, data := range serviceList {
		go func() {
			defer wg.Done()
			err := handler(data, log)
			if err != nil {
				errList = append(errList, err)
				*failedServices = append(*failedServices, data)
				log.Errorf("service:%s apply failed, err %s", data.ServiceName, err)
			}
		}()
		<-ticker.C
	}
	wg.Wait()
	return errList
}

func updateProductGroup(username, productName, envName, updateType string, productResp *commonmodels.Product, currentProductServices [][]*commonmodels.ProductService, overrideCharts []*commonservice.RenderChartArg, log *zap.SugaredLogger) error {
	var (
		renderChartMap         = make(map[string]*template.RenderChart)
		productServiceMap      = make(map[string]*commonmodels.ProductService)
		productTemplServiceMap = make(map[string]*commonmodels.ProductService)
	)
	restConfig, err := kube.GetRESTConfig(productResp.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	helmClient, err := helmtool.NewClientFromRestConf(restConfig, productResp.Namespace)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	for _, serviceGroup := range currentProductServices {
		for _, service := range serviceGroup {
			productServiceMap[service.ServiceName] = service
		}
	}

	for _, service := range productResp.GetServiceMap() {
		productTemplServiceMap[service.ServiceName] = service
	}

	// 找到环境里面还存在但是服务编排里面已经删除的服务卸载掉
	for serviceName := range productServiceMap {
		if _, isExist := productTemplServiceMap[serviceName]; !isExist {
			go func(namespace, serviceName string) {
				log.Infof("ready to uninstall release:%s", fmt.Sprintf("%s-%s", namespace, serviceName))
				if err = helmClient.UninstallRelease(&helmclient.ChartSpec{
					ReleaseName: util.GeneHelmReleaseName(namespace, serviceName),
					Namespace:   namespace,
					Wait:        true,
					//Force:       true,
					//Timeout:     Timeout * time.Second * 10,
				}); err != nil {
					log.Errorf("helm uninstall release %s err:%v", fmt.Sprintf("%s-%s", namespace, serviceName), err)
				}
			}(productResp.Namespace, serviceName)
		}
	}

	//比较当前环境中的变量和系统默认的最新变量
	renderSet, err := diffRenderSet(username, productName, envName, updateType, productResp, overrideCharts, log)
	if err != nil {
		return e.ErrUpdateEnv.AddDesc("对比环境中的value.yaml和系统默认的value.yaml失败")
	}

	svcNameSet := sets.NewString()
	for _, singleChart := range overrideCharts {
		if singleChart.EnvName != envName {
			continue
		}
		svcNameSet.Insert(singleChart.ServiceName)
	}

	for _, renderChart := range renderSet.ChartInfos {
		renderChartMap[renderChart.ServiceName] = renderChart
	}

	handler := func(serviceObj *commonmodels.Service, log *zap.SugaredLogger) error {
		renderChart := renderChartMap[serviceObj.ServiceName]
		err = installOrUpgradeHelmChart(productResp.Namespace, renderChart, renderSet.DefaultValues, serviceObj, 0, helmClient)
		if err != nil {
			return errors.Wrapf(err, "failed to install or upgrade service %s", serviceObj.ServiceName)
		}
		return nil
	}

	errList := new(multierror.Error)
	for groupIndex, services := range productResp.Services {
		serviceList := make([]*commonmodels.Service, 0)
		for _, svc := range services {
			_, ok := renderChartMap[svc.ServiceName]
			if !ok {
				continue
			}
			// service is not in update list
			if !svcNameSet.Has(svc.ServiceName) {
				continue
			}

			opt := &commonrepo.ServiceFindOption{
				ServiceName:   svc.ServiceName,
				Type:          svc.Type,
				Revision:      svc.Revision,
				ProductName:   svc.ProductName,
				ExcludeStatus: setting.ProductStatusDeleting,
			}
			serviceObj, err := commonrepo.NewServiceColl().Find(opt)
			if err != nil {
				log.Errorf("failed to find service with opt %+v, err: %s", opt, err)
				errList = multierror.Append(errList, errors.Wrapf(err, "failed to find template servce, serviceName %s", svc.ServiceName))
				continue
			}

			serviceList = append(serviceList, serviceObj)
		}

		serviceGroupErr := intervalExecutorWithRetry(5, time.Millisecond*2500, serviceList, handler, log)
		if serviceGroupErr != nil {
			errList = multierror.Append(errList, serviceGroupErr...)
		}

		if err = commonrepo.NewProductColl().UpdateGroup(envName, productName, groupIndex, services); err != nil {
			log.Errorf("Failed to update service group %d, err: %s", groupIndex, err)
			errList = multierror.Append(errList, err)
		}
	}

	productResp.Render.Revision = renderSet.Revision
	if err = commonrepo.NewProductColl().Update(productResp); err != nil {
		log.Errorf("Failed to update env, err: %s", err)
		errList = multierror.Append(errList, err)
	}

	return errList.ErrorOrNil()
}

// diffRenderSet 对比环境中的renderSet的值和服务的最新的renderSet的值
func diffRenderSet(username, productName, envName, updateType string, productResp *commonmodels.Product, overrideCharts []*commonservice.RenderChartArg, log *zap.SugaredLogger) (*commonmodels.RenderSet, error) {
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

	renderChartArgMap := make(map[string]*commonservice.RenderChartArg)
	for _, singleArg := range overrideCharts {
		if singleArg.EnvName != envName {
			continue
		}
		renderChartArgMap[singleArg.ServiceName] = singleArg
	}

	newChartInfos := make([]*template.RenderChart, 0)
	defaultValues := ""
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
		renderSetOpt := &commonrepo.RenderSetFindOption{Name: productResp.Render.Name, Revision: productResp.Render.Revision}
		currentEnvRenderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
		if err != nil {
			log.Errorf("[RenderSet.find] err: %v", err)
			return nil, err
		}
		defaultValues = currentEnvRenderSet.DefaultValues

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

		serviceMap := productResp.GetServiceMap()
		for serviceName, latestChartInfo := range tmpLatestChartInfoMap {
			if currentChartInfo, ok := tmpCurrentChartInfoMap[serviceName]; ok {
				serviceInfo := serviceMap[serviceName]
				imageRelatedKey := sets.NewString()
				if serviceInfo != nil {
					for _, container := range serviceInfo.Containers {
						if container.ImagePath != nil {
							imageRelatedKey.Insert(container.ImagePath.Image, container.ImagePath.Repo, container.ImagePath.Tag)
						}
					}
				}

				//拿当前环境values.yaml的key的value去替换服务里面的values.yaml的相同的key的value
				newValuesYaml, err := overrideValues([]byte(currentChartInfo.ValuesYaml), []byte(latestChartInfo.ValuesYaml), imageRelatedKey)
				if err != nil {
					log.Errorf("Failed to override values for service %s, err: %s", serviceName, err)
				} else {
					latestChartInfo.ValuesYaml = string(newValuesYaml)
				}

				// user override value in cur environment
				latestChartInfo.OverrideValues = currentChartInfo.OverrideValues
				latestChartInfo.OverrideYaml = currentChartInfo.OverrideYaml
			}
			// user override value form request
			if renderArg, ok := renderChartArgMap[serviceName]; ok {
				renderArg.FillRenderChartModel(latestChartInfo, latestChartInfo.ChartVersion)
			}
			newChartInfos = append(newChartInfos, latestChartInfo)
		}
	}

	if err = commonservice.CreateHelmRenderSet(
		&commonmodels.RenderSet{
			Name:          productResp.Render.Name,
			EnvName:       envName,
			ProductTmpl:   productName,
			ChartInfos:    newChartInfos,
			DefaultValues: defaultValues,
			UpdateBy:      username,
		},
		log,
	); err != nil {
		log.Errorf("[RenderSet.create] err: %v", err)
		return nil, err
	}

	renderSet, err := FindHelmRenderSet(productName, productResp.Render.Name, log)
	if err != nil {
		log.Errorf("[RenderSet.find] err: %v", err)
		return nil, err
	}
	return renderSet, nil
}

// for keys exist in both yaml, current values will override latest values
// only for images
func overrideValues(currentValuesYaml, latestValuesYaml []byte, imageRelatedKey sets.String) ([]byte, error) {
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
		if !imageRelatedKey.Has(key) {
			continue
		}
		if currentValue, ok := currentValuesFlatMap[key]; ok {
			replaceMap[key] = currentValue
		}
	}

	if len(replaceMap) == 0 {
		return latestValuesYaml, nil
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

func updateProductVariable(productName, envName string, productResp *commonmodels.Product, renderset *commonmodels.RenderSet, log *zap.SugaredLogger) error {
	restConfig, err := kube.GetRESTConfig(productResp.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	helmClient, err := helmtool.NewClientFromRestConf(restConfig, productResp.Namespace)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	renderChartMap := make(map[string]*template.RenderChart)
	for _, renderChart := range productResp.ChartInfos {
		renderChartMap[renderChart.ServiceName] = renderChart
	}

	handler := func(service *commonmodels.Service, log *zap.SugaredLogger) error {
		renderChart := renderChartMap[service.ServiceName]
		err = installOrUpgradeHelmChart(productResp.Namespace, renderChart, renderset.DefaultValues, service, 0, helmClient)
		if err != nil {
			return errors.Wrapf(err, "failed to upgrade service %s", service.ServiceName)
		}
		return nil
	}

	errList := new(multierror.Error)
	for groupIndex, services := range productResp.Services {
		serviceList := make([]*commonmodels.Service, 0)
		groupServices := make([]*commonmodels.ProductService, 0)
		for _, service := range services {
			if _, isExist := renderChartMap[service.ServiceName]; isExist {
				opt := &commonrepo.ServiceFindOption{
					ServiceName: service.ServiceName,
					Type:        service.Type,
					Revision:    service.Revision,
					ProductName: productName,
					//ExcludeStatus: setting.ProductStatusDeleting,
				}
				serviceObj, err := commonrepo.NewServiceColl().Find(opt)
				if err != nil {
					log.Errorf("failed to find service %s, err %s", service.ServiceName, err.Error())
					continue
				}
				serviceList = append(serviceList, serviceObj)
			}
			groupServices = append(groupServices, service)
		}
		groupServiceErr := intervalExecutorWithRetry(5, time.Millisecond*2500, serviceList, handler, log)
		if groupServiceErr != nil {
			errList = multierror.Append(errList, groupServiceErr...)
		}
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

func setFieldValueIsNotExist(obj map[string]interface{}, value interface{}, fields ...string) map[string]interface{} {
	m := obj
	for _, field := range fields[:len(fields)-1] {
		if val, ok := m[field]; ok {
			if valMap, ok := val.(map[string]interface{}); ok {
				m = valMap
			} else {
				newVal := make(map[string]interface{})
				m[field] = newVal
				m = newVal
			}
		}
	}
	m[fields[len(fields)-1]] = value
	return obj
}
