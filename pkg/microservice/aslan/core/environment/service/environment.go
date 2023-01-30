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
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/hashicorp/go-multierror"
	helmclient "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/strvals"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	mongotemplate "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/collaboration"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
	"github.com/koderover/zadig/pkg/util/converter"
	"github.com/koderover/zadig/pkg/util/fs"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

func GetProductDeployType(projectName string) (string, error) {
	projectInfo, err := templaterepo.NewProductColl().Find(projectName)
	if err != nil {
		return "", err
	}
	if projectInfo.IsCVMProduct() {
		return setting.PMDeployType, nil
	}
	if projectInfo.IsHelmProduct() {
		return setting.HelmDeployType, nil
	}
	return setting.K8SDeployType, nil
}

func ListProducts(projectName string, envNames []string, log *zap.SugaredLogger) ([]*EnvResp, error) {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: projectName, InEnvs: envNames, IsSortByProductName: true})
	if err != nil {
		log.Errorf("Failed to list envs, err: %s", err)
		return nil, e.ErrListEnvs.AddDesc(err.Error())
	}

	var res []*EnvResp
	reg, _, err := commonservice.FindDefaultRegistry(false, log)
	if err != nil {
		log.Errorf("FindDefaultRegistry error: %v", err)
		return nil, e.ErrListEnvs.AddErr(err)
	}

	clusters, err := commonrepo.NewK8SClusterColl().List(&commonrepo.ClusterListOpts{})
	if err != nil {
		log.Errorf("failed to list clusters, err: %s", err)
		return nil, e.ErrListEnvs.AddErr(err)
	}
	clusterMap := make(map[string]*models.K8SCluster)
	for _, cluster := range clusters {
		clusterMap[cluster.ID.Hex()] = cluster
	}
	getClusterName := func(clusterID string) string {
		cluster, ok := clusterMap[clusterID]
		if ok {
			return cluster.Name
		}
		return ""
	}

	envCMMap, err := collaboration.GetEnvCMMap([]string{projectName}, log)
	if err != nil {
		return nil, err
	}
	for _, env := range envs {
		if len(env.RegistryID) == 0 {
			env.RegistryID = reg.ID.Hex()
		}

		var baseRefs []string
		if cmSet, ok := envCMMap[collaboration.BuildEnvCMMapKey(env.ProductName, env.EnvName)]; ok {
			baseRefs = append(baseRefs, cmSet.List()...)
		}
		res = append(res, &EnvResp{
			ProjectName:     projectName,
			Name:            env.EnvName,
			IsPublic:        env.IsPublic,
			IsExisted:       env.IsExisted,
			ClusterName:     getClusterName(env.ClusterID),
			Source:          env.Source,
			Production:      env.Production,
			Status:          env.Status,
			Error:           env.Error,
			UpdateTime:      env.UpdateTime,
			UpdateBy:        env.UpdateBy,
			RegistryID:      env.RegistryID,
			ClusterID:       env.ClusterID,
			Namespace:       env.Namespace,
			Alias:           env.Alias,
			BaseRefs:        baseRefs,
			BaseName:        env.BaseName,
			ShareEnvEnable:  env.ShareEnv.Enable,
			ShareEnvIsBase:  env.ShareEnv.IsBase,
			ShareEnvBaseEnv: env.ShareEnv.BaseEnv,
		})
	}

	return res, nil
}

//func FillProductVars(products []*commonmodels.Product, log *zap.SugaredLogger) error {
//	for _, product := range products {
//		if product.Source == setting.SourceFromExternal || product.Source == setting.SourceFromHelm {
//			continue
//		}
//		renderName := product.Namespace
//		var revision int64
//		// if the environment is backtracking, render.name will be different with product.Namespace
//		if product.Render != nil {
//			revision = product.Render.Revision
//			renderName = product.Render.Name
//		}
//
//		//renderSet, err := commonservice.GetRenderSet(renderName, revision, false, product.EnvName, log)
//		//if err != nil {
//		//	log.Errorf("Failed to find render set, productName: %s, namespace: %s,  err: %s", product.ProductName, product.Namespace, err)
//		//	return e.ErrGetRenderSet.AddDesc(err.Error())
//		//}
//
//		//product.Vars = renderSet.KVs[:]
//		//
//		//// Note. the service property of kv pair stored in DB is not accuracy
//		//// from v1.14.0 we should fetch related service data real-time
//		//if len(product.Vars) == 0 {
//		//	return nil
//		//}
//
//		templateSvcsOfProduct, err := commonservice.GetProductUsedTemplateSvcs(product)
//		if err != nil {
//			log.Errorf("failed to get service templates applied in product, err: %s", err)
//			return nil
//		}
//
//		renderKvs, err := commonservice.ListRenderKeysByTemplateSvc(templateSvcsOfProduct, log)
//		if err != nil {
//			log.Errorf("failed to get render kvs in product, err: %s", err)
//			return nil
//		}
//
//		relatedSvcs := make(map[string][]string)
//		for _, key := range renderKvs {
//			relatedSvcs[key.Key] = key.Services
//		}
//		for _, varInfo := range product.Vars {
//			varInfo.Services = relatedSvcs[varInfo.Key]
//		}
//	}
//	return nil
//}

var mutexAutoCreate sync.RWMutex

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

type UpdateServiceArg struct {
	ServiceName    string `json:"service_name"`
	DeployStrategy string `json:"deploy_strategy"`
	VariableYaml   string `json:"variable_yaml"`
}

type UpdateEnv struct {
	EnvName  string              `json:"env_name"`
	Services []*UpdateServiceArg `json:"services"`
	//Vars     []*template.RenderKV `json:"vars,omitempty"`
}

func UpdateMultipleK8sEnv(args []*UpdateEnv, envNames []string, productName, requestID string, force bool, log *zap.SugaredLogger) ([]*EnvStatus, error) {
	mutexAutoUpdate.Lock()
	defer func() {
		mutexAutoUpdate.Unlock()
	}()

	envStatuses := make([]*EnvStatus, 0)

	productsRevision, err := ListProductsRevision(productName, "", log)
	if err != nil {
		log.Errorf("UpdateMultipleK8sEnv ListProductsRevision err:%v", err)
		return envStatuses, err
	}
	productMap := make(map[string]*ProductRevision)
	for _, productRevision := range productsRevision {
		if productRevision.ProductName == productName && sets.NewString(envNames...).Has(productRevision.EnvName) && productRevision.Updatable {
			productMap[productRevision.EnvName] = productRevision
			if len(productMap) == len(envNames) {
				break
			}
		}
	}

	errList := &multierror.Error{}
	for _, arg := range args {
		strategyMap := make(map[string]string)
		updateSvcs := make([]*templatemodels.ServiceRender, 0)
		updateRevisionSvcs := make([]string, 0)
		for _, svc := range arg.Services {
			strategyMap[svc.ServiceName] = svc.DeployStrategy
			updateSvcs = append(updateSvcs, &templatemodels.ServiceRender{
				ServiceName:  svc.ServiceName,
				OverrideYaml: &templatemodels.CustomYaml{YamlContent: svc.VariableYaml},
			})
			updateRevisionSvcs = append(updateRevisionSvcs, svc.ServiceName)
		}

		opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: arg.EnvName}
		exitedProd, err := commonrepo.NewProductColl().Find(opt)
		if err != nil {
			log.Errorf("[%s][P:%s] Product.FindByOwner error: %v", arg.EnvName, productName, err)
			errList = multierror.Append(errList, e.ErrUpdateEnv.AddDesc(e.EnvNotFoundErrMsg))
			continue
		}

		exitedProd.EnsureRenderInfo()
		rendersetInfo, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{Name: exitedProd.Render.Name, Revision: exitedProd.Render.Revision, EnvName: arg.EnvName})
		if err != nil {
			errList = multierror.Append(errList, e.ErrUpdateEnv.AddErr(fmt.Errorf("failed to find renderset, err: %s", err)))
			continue
		}

		filter := func(svc *commonmodels.ProductService) bool {
			return util.InStringArray(svc.ServiceName, updateRevisionSvcs)
		}

		// update env default variable, particular svcs from client are involved
		// svc revision will not be updated
		err = updateK8sProduct(exitedProd, setting.SystemUser, requestID, updateRevisionSvcs, filter, updateSvcs, strategyMap, force, rendersetInfo.DefaultValues, log)
		if err != nil {
			log.Errorf("UpdateMultipleK8sEnv UpdateProductV2 err:%v", err)
			errList = multierror.Append(errList, err)
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
	return envStatuses, errList.ErrorOrNil()
}

// getServicesWithMaxRevision get all services template with max revision including involved shared services
func getServicesWithMaxRevision(projectName string) ([]*commonmodels.Service, error) {
	// list services with max revision defined in project
	allServices, err := commonrepo.NewServiceColl().ListMaxRevisions(&commonrepo.ServiceListOption{ProductName: projectName})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find services in project %s", projectName)
	}

	prodTmpl, err := templaterepo.NewProductColl().Find(projectName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find project: %s", projectName)
	}

	// list services with max revision of shared services
	if prodTmpl.SharedServices != nil {
		servicesByProject := make(map[string][]string)
		for _, serviceInfo := range prodTmpl.SharedServices {
			servicesByProject[serviceInfo.Owner] = append(servicesByProject[serviceInfo.Owner], serviceInfo.Name)
		}

		for sourceProject, services := range servicesByProject {
			inService := make([]*templatemodels.ServiceInfo, 0)
			for _, serviceName := range services {
				inService = append(inService, &templatemodels.ServiceInfo{
					Name:  serviceName,
					Owner: sourceProject,
				})
			}

			sharedServices, err := commonrepo.NewServiceColl().ListMaxRevisions(&commonrepo.ServiceListOption{
				ProductName: sourceProject,
				InServices:  inService,
			})
			if err != nil {
				return nil, errors.Wrapf(err, "failed to find shared service templates, projectName: %s", sourceProject)
			}
			allServices = append(allServices, sharedServices...)
		}
	}

	return allServices, nil
}

// TODO need optimize
// cvm and k8s yaml projects should not be handled together
func updateProductImpl(updateRevisionSvcs []string, deployStrategy map[string]string, existedProd, updateProd *commonmodels.Product, renderSet *commonmodels.RenderSet, filter svcUpgradeFilter, log *zap.SugaredLogger) (err error) {
	oldProductRender := existedProd.Render
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
	var prodRevs *ProductRevision

	// list services with max revision of project
	allServices, err = getServicesWithMaxRevision(productName)
	if err != nil {
		log.Errorf("ListAllRevisions error: %s", err)
		err = e.ErrUpdateEnv.AddDesc(err.Error())
		return
	}

	prodRevs, err = GetProductRevision(existedProd, allServices, log)
	if err != nil {
		err = e.ErrUpdateEnv.AddDesc(e.GetEnvRevErrMsg)
		return
	}

	// 无需更新
	//if !prodRevs.Updatable {
	//	log.Errorf("[%s][P:%s] nothing to update", envName, productName)
	//	return
	//}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), existedProd.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), existedProd.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), existedProd.ClusterID)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, namespace, err)
		return e.ErrUpdateEnv.AddDesc(err.Error())

	}
	inf, err := informer.NewInformer(existedProd.ClusterID, namespace, cls)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, namespace, err)
		return e.ErrUpdateEnv.AddDesc(err.Error())
	}

	// 遍历产品环境和产品模板交叉对比的结果
	// 四个状态：待删除，待添加，待更新，无需更新
	var deletedServices []string
	// 1. 如果服务待删除：将产品模板中已经不存在，产品环境中待删除的服务进行删除。
	for _, serviceRev := range prodRevs.ServiceRevisions {
		if serviceRev.Updatable && serviceRev.Deleted && util.InStringArray(serviceRev.ServiceName, updateRevisionSvcs) {
			log.Infof("[%s][P:%s][S:%s] start to delete service", envName, productName, serviceRev.ServiceName)
			//根据namespace: EnvName, selector: productName + serviceName来删除属于该服务的所有资源
			selector := labels.Set{setting.ProductLabel: productName, setting.ServiceLabel: serviceRev.ServiceName}.AsSelector()
			err = commonservice.DeleteNamespacedResource(namespace, selector, existedProd.ClusterID, log)
			if err != nil {
				//删除失败仅记录失败日志
				log.Errorf("delete resource of service %s error:%v", serviceRev.ServiceName, err)
			}
			deletedServices = append(deletedServices, serviceRev.ServiceName)
			clusterSelector := labels.Set{setting.ProductLabel: productName, setting.ServiceLabel: serviceRev.ServiceName, setting.EnvNameLabel: envName}.AsSelector()
			err = commonservice.DeleteClusterResource(clusterSelector, existedProd.ClusterID, log)
			if err != nil {
				//删除失败仅记录失败日志
				log.Errorf("delete cluster resource of service %s error:%v", serviceRev.ServiceName, err)
			}
		}
	}

	//// 转化prodRevs.ServiceRevisions为serviceName+serviceType:serviceRev的map
	//// 不在遍历到每个服务时再次进行遍历
	serviceRevisionMap := getServiceRevisionMap(prodRevs.ServiceRevisions)
	//
	//// 首先更新一次数据库，将产品模板的最新编排更新到数据库
	//// 只更新编排，不更新服务revision等信息
	//updatedServices := getUpdatedProductServices(updateProd, serviceRevisionMap, existedProd)

	updateProd.Status = setting.ProductStatusUpdating
	//updateProd.Services = updatedServices
	updateProd.ShareEnv = existedProd.ShareEnv

	if err := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUpdating); err != nil {
		log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.UpdateEnvStatusErrMsg)
	}

	existedServices := existedProd.GetServiceMap()

	// 按照产品模板的顺序来创建或者更新服务
	for groupIndex, prodServiceGroup := range updateProd.Services {
		//Mark if there is k8s type service in this group
		//groupServices := make([]*commonmodels.ProductService, 0)
		var wg sync.WaitGroup

		groupSvcs := make([]*commonmodels.ProductService, 0)
		for _, prodService := range prodServiceGroup {
			// no need to update service
			if filter != nil && !filter(prodService) {
				groupSvcs = append(groupSvcs, prodService)
				continue
			}

			service := &commonmodels.ProductService{
				ServiceName: prodService.ServiceName,
				ProductName: prodService.ProductName,
				Type:        prodService.Type,
				Revision:    prodService.Revision,
			}
			service.Containers = prodService.Containers

			// need update service revision
			if util.InStringArray(prodService.ServiceName, updateRevisionSvcs) {
				svcRev, ok := serviceRevisionMap[prodService.ServiceName+prodService.Type]
				if !ok {
					groupSvcs = append(groupSvcs, prodService)
					continue
				}
				service.Revision = svcRev.NextRevision
				service.Containers = svcRev.Containers
			}
			groupSvcs = append(groupSvcs, service)

			if prodService.Type == setting.K8SDeployType {
				log.Infof("[Namespace:%s][Product:%s][Service:%s] upsert service", envName, productName, prodService.ServiceName)
				wg.Add(1)
				go func() {
					defer wg.Done()
					if !commonutil.ServiceDeployed(prodService.ServiceName, deployStrategy) {
						return
					}
					_, errUpsertService := upsertService(
						updateProd,
						service,
						existedServices[service.ServiceName],
						renderSet, oldProductRender, inf, kubeClient, istioClient, log)
					if errUpsertService != nil {
						service.Error = errUpsertService.Error()
					} else {
						service.Error = ""
					}
				}()
			}

			//if svcRev.Updatable || commonutil.DeployStrategyChanged(svcRev.ServiceName, existedProd.ServiceDeployStrategy, deployStrategy) {
			//	service := &commonmodels.ProductService{
			//		ServiceName: svcRev.ServiceName,
			//		ProductName: prodService.ProductName,
			//		Type:        svcRev.Type,
			//		Revision:    svcRev.NextRevision,
			//	}
			//	service.Containers = svcRev.Containers
			//
			//	if svcRev.Type == setting.K8SDeployType && util.InStringArray(service.ServiceName, updateRevisionSvcs) {
			//		log.Infof("[Namespace:%s][Product:%s][Service:%s][IsNew:%v] upsert service",
			//			envName, productName, svcRev.ServiceName, svcRev.New)
			//		wg.Add(1)
			//		go func() {
			//			defer wg.Done()
			//			if !commonutil.ServiceDeployed(svcRev.ServiceName, deployStrategy) {
			//				return
			//			}
			//			_, errUpsertService := upsertService(
			//				updateProd,
			//				service,
			//				existedServices[service.ServiceName],
			//				renderSet, oldProductRender, inf, kubeClient, istioClient, log)
			//			if errUpsertService != nil {
			//				service.Error = errUpsertService.Error()
			//			} else {
			//				service.Error = ""
			//			}
			//		}()
			//	}
			//	groupServices = append(groupServices, service)
			//} else {
			//	prodService.Containers = svcRev.Containers
			//	groupServices = append(groupServices, prodService)
			//}
		}
		wg.Wait()

		////merge new and old services
		//var updateGroup []*commonmodels.ProductService
		//newServiceMap := make(map[string]*commonmodels.ProductService)
		//for _, service := range groupServices {
		//	newServiceMap[service.ServiceName] = service
		//}
		//oldServiceMap := make(map[string]*commonmodels.ProductService)
		//for _, existedGroupServices := range existedProd.Services {
		//	for _, service := range existedGroupServices {
		//		if util.InStringArray(service.ServiceName, deletedServices) {
		//			continue
		//		}
		//		oldServiceMap[service.ServiceName] = service
		//		if newService, ok := newServiceMap[service.ServiceName]; ok {
		//			if util.InStringArray(service.ServiceName, updateRevisionSvcs) {
		//				updateGroup = append(updateGroup, newService)
		//			} else {
		//				updateGroup = append(updateGroup, service)
		//			}
		//		}
		//	}
		//}
		//for _, newService := range groupServices {
		//	if _, ok := oldServiceMap[newService.ServiceName]; !ok && !util.InStringArray(newService.ServiceName, deletedServices) && util.InStringArray(newService.ServiceName, updateRevisionSvcs) {
		//		updateGroup = append(updateGroup, newService)
		//	}
		//}

		err = commonrepo.NewProductColl().UpdateGroup(envName, productName, groupIndex, groupSvcs)
		if err != nil {
			log.Errorf("Failed to update collection - service group %d. Error: %v", groupIndex, err)
			err = e.ErrUpdateEnv.AddDesc(err.Error())
			return
		}
	}

	err = commonrepo.NewProductColl().UpdateRender(envName, productName, updateProd.Render)
	if err != nil {
		log.Errorf("failed to update product render error: %v", err)
		err = e.ErrUpdateEnv.AddDesc(err.Error())
		return
	}

	// store deploy strategy
	if deployStrategy != nil {
		if existedProd.ServiceDeployStrategy == nil {
			existedProd.ServiceDeployStrategy = deployStrategy
		} else {
			for k, v := range deployStrategy {
				existedProd.ServiceDeployStrategy[k] = v
			}
		}
		err = commonrepo.NewProductColl().UpdateDeployStrategy(envName, productName, existedProd.ServiceDeployStrategy)
		if err != nil {
			log.Errorf("Failed to update deploy strategy data, error: %v", err)
			err = e.ErrUpdateEnv.AddDesc(err.Error())
			return
		}
	}

	return nil
}

func UpdateProductRegistry(envName, productName, registryID string, log *zap.SugaredLogger) (err error) {
	opt := &commonrepo.ProductFindOptions{EnvName: envName, Name: productName}
	exitedProd, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("UpdateProductRegistry find product by envName:%s,error: %v", envName, err)
		return e.ErrUpdateEnv.AddDesc(e.EnvNotFoundErrMsg)
	}
	err = commonrepo.NewProductColl().UpdateRegistry(envName, productName, registryID)
	if err != nil {
		log.Errorf("UpdateProductRegistry UpdateRegistry by envName:%s registryID:%s error: %v", envName, registryID, err)
		return e.ErrUpdateEnv.AddErr(err)
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), exitedProd.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}
	err = ensureKubeEnv(exitedProd.Namespace, registryID, map[string]string{setting.ProductLabel: productName}, false, kubeClient, log)

	if err != nil {
		log.Errorf("UpdateProductRegistry ensureKubeEnv by envName:%s,error: %v", envName, err)
		return err
	}
	return nil
}

func UpdateMultiCVMProducts(envNames []string, productName, user, requestID string, log *zap.SugaredLogger) ([]*EnvStatus, error) {
	errList := &multierror.Error{}
	for _, env := range envNames {
		err := UpdateCVMProduct(env, productName, user, requestID, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}

	productResps := make([]*ProductResp, 0)
	for _, envName := range envNames {
		productResp, err := GetProduct(setting.SystemUser, envName, productName, log)
		if err == nil && productResp != nil {
			productResps = append(productResps, productResp)
		}
	}

	envStatuses := make([]*EnvStatus, 0)
	for _, productResp := range productResps {
		if productResp.Error != "" {
			envStatuses = append(envStatuses, &EnvStatus{EnvName: productResp.EnvName, Status: setting.ProductStatusFailed, ErrMessage: productResp.Error})
			continue
		}
		envStatuses = append(envStatuses, &EnvStatus{EnvName: productResp.EnvName, Status: productResp.Status})
	}
	return envStatuses, errList.ErrorOrNil()
}

func UpdateCVMProduct(envName, productName, user, requestID string, log *zap.SugaredLogger) error {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	exitedProd, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[%s][P:%s] Product.FindByOwner error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.EnvNotFoundErrMsg)
	}
	return updateCVMProduct(exitedProd, user, requestID, log)
}

// CreateProduct create a new product with its dependent stacks
func CreateProduct(user, requestID string, args *commonmodels.Product, log *zap.SugaredLogger) (err error) {
	log.Infof("[%s][P:%s] CreateProduct", args.EnvName, args.ProductName)
	creator := getCreatorBySource(args.Source)
	args.UpdateBy = user
	return creator.Create(user, requestID, args, log)
}

func UpdateProductRecycleDay(envName, productName string, recycleDay int) error {
	return commonrepo.NewProductColl().UpdateProductRecycleDay(envName, productName, recycleDay)
}

func UpdateProductAlias(envName, productName, alias string) error {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("failed to query product info, name %s", envName))
	}
	if !productInfo.Production {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("cannot set alias for non-production environment %s", envName))
	}
	err = commonrepo.NewProductColl().UpdateProductAlias(envName, productName, alias)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}
	return nil
}

func buildContainerMap(cs []*models.Container) map[string]*models.Container {
	containerMap := make(map[string]*models.Container)
	for _, c := range cs {
		containerMap[c.Name] = c
	}
	return containerMap
}

func updateHelmProduct(productName, envName, username, requestID string, overrideCharts []*commonservice.HelmSvcRenderArg, deletedServices []string, log *zap.SugaredLogger) error {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	productResp, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("GetProduct envName:%s, productName:%s, err:%+v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(err.Error())
	}

	// create product data from product template
	templateProd, err := GetInitProduct(productName, types.GeneralEnv, false, "", log)
	if err != nil {
		log.Errorf("[%s][P:%s] GetProductTemplate error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.FindProductTmplErrMsg)
	}

	// set image and render to the value used on current environment
	deletedSvcSet := sets.NewString(deletedServices...)
	deletedSvcRevision := make(map[string]int64)
	// services need to be created or updated
	serviceNeedUpdateOrCreate := sets.NewString()
	for _, chart := range overrideCharts {
		serviceNeedUpdateOrCreate.Insert(chart.ServiceName)
	}

	productServiceMap := productResp.GetServiceMap()

	// get deleted services map[serviceName]=>serviceRevision
	for _, svc := range productServiceMap {
		if deletedSvcSet.Has(svc.ServiceName) {
			deletedSvcRevision[svc.ServiceName] = svc.Revision
		}
	}

	// use service definition from service template, but keep the image info
	allServices := make([][]*commonmodels.ProductService, 0)
	for _, svrs := range templateProd.Services {
		svcGroup := make([]*commonmodels.ProductService, 0)
		for _, svr := range svrs {
			if deletedSvcSet.Has(svr.ServiceName) {
				continue
			}
			ps, ok := productServiceMap[svr.ServiceName]
			// only update or insert services
			if !ok && !serviceNeedUpdateOrCreate.Has(svr.ServiceName) {
				continue
			}

			// existed service has nothing to update
			if ok && !serviceNeedUpdateOrCreate.Has(svr.ServiceName) {
				svcGroup = append(svcGroup, ps)
				continue
			}

			svcGroup = append(svcGroup, svr)
			if ps == nil {
				continue
			}

			templateContainMap := buildContainerMap(svr.Containers)
			prodContainMap := buildContainerMap(ps.Containers)
			for name, container := range templateContainMap {
				if pc, ok := prodContainMap[name]; ok {
					container.Image = pc.Image
				}
			}
		}
		allServices = append(allServices, svcGroup)
	}
	productResp.Services = allServices

	// set status to updating
	if err := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUpdating); err != nil {
		log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.UpdateEnvStatusErrMsg)
	}

	//对比当前环境中的环境变量和默认的环境变量
	go func() {
		errMsg := ""
		err := updateHelmProductGroup(username, productName, envName, productResp, overrideCharts, deletedSvcRevision, log)
		if err != nil {
			errMsg = err.Error()
			log.Errorf("[%s][P:%s] failed to update product %#v", envName, productName, err)
			// 发送更新产品失败消息给用户
			title := fmt.Sprintf("更新 [%s] 的 [%s] 环境失败", productName, envName)
			commonservice.SendErrorMessage(username, title, requestID, err, log)

			log.Infof("[%s][P:%s] update error to => %s", envName, productName, err)
			productResp.Status = setting.ProductStatusFailed
		} else {
			productResp.Status = setting.ProductStatusSuccess
		}
		if err = commonrepo.NewProductColl().UpdateStatusAndError(envName, productName, productResp.Status, errMsg); err != nil {
			log.Errorf("[%s][%s] Product.Update error: %v", envName, productName, err)
			return
		}
	}()
	return nil
}

func genImageFromYaml(c *commonmodels.Container, valuesYaml, defaultValues, overrideYaml, overrideValues string) (string, error) {
	mergeYaml, err := helmtool.MergeOverrideValues(valuesYaml, defaultValues, overrideYaml, overrideValues)
	if err != nil {
		return "", err
	}
	mergedValuesYamlFlattenMap, err := converter.YamlToFlatMap([]byte(mergeYaml))
	if err != nil {
		return "", err
	}
	imageRule := templatemodels.ImageSearchingRule{
		Repo:  c.ImagePath.Repo,
		Image: c.ImagePath.Image,
		Tag:   c.ImagePath.Tag,
	}
	image, err := commonservice.GeneImageURI(imageRule.GetSearchingPattern(), mergedValuesYamlFlattenMap)
	if err != nil {
		return "", err
	}
	return image, nil
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
	opt := &commonrepo.RenderSetFindOption{
		Name:        productInfo.Render.Name,
		Revision:    productInfo.Render.Revision,
		EnvName:     productInfo.EnvName,
		ProductTmpl: productInfo.ProductName,
	}
	renderSet, err := commonrepo.NewRenderSetColl().Find(opt)
	if err != nil {
		log.Errorf("renderset Find error, productName:%s, envName:%s, err:%s", productInfo.ProductName, productInfo.EnvName, err)
		return "", "", fmt.Errorf("failed to query renderset info, name %s", productInfo.Render.Name)
	}

	// find target render chart from render set
	var targetChart *templatemodels.ServiceRender
	for _, chart := range renderSet.ChartInfos {
		if chart.ServiceName == serviceName {
			targetChart = chart
			break
		}
	}

	switch usageScenario {
	case usageScenarioUpdateEnv:
		imageRelatedKey := sets.NewString()
		proSvcMap := productInfo.GetServiceMap()
		proSvc := proSvcMap[serviceName]
		if proSvc != nil {
			curEnvService, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
				ServiceName: serviceName,
				ProductName: productName,
				Type:        setting.HelmDeployType,
				Revision:    proSvc.Revision,
			})
			if err != nil {
				log.Errorf("failed to query service, name %s, Revision %d,err %s", serviceName, proSvc.Revision, err)
				return "", "", fmt.Errorf("failed to query service, name %s,Revision %d,err %s", serviceName, proSvc.Revision, err)
			}
		L:
			for _, curSvcContainer := range curEnvService.Containers {
				if checkServiceImageUpdated(curSvcContainer, proSvc) {
					for _, container := range templateService.Containers {
						if curSvcContainer.Name == container.Name && container.ImagePath != nil {
							imageRelatedKey.Insert(container.ImagePath.Image, container.ImagePath.Repo, container.ImagePath.Tag)
							continue L
						}
					}
				}
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

func GetAffectedServices(productName, envName string, arg *K8sRendersetArg, log *zap.SugaredLogger) (map[string][]string, error) {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: envName})
	if err != nil {
		return nil, fmt.Errorf("failed to find product info, err: %s", err)
	}
	productServiceRevisions, err := commonservice.GetProductUsedTemplateSvcs(productInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to find revision services, err: %s", err)
	}

	renderset, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		ProductTmpl: productName,
		EnvName:     envName,
		IsDefault:   false,
		Name:        productInfo.Render.Name,
		Revision:    productInfo.Render.Revision,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find render, err: %s", err)
	}

	ret := make(map[string][]string)
	ret["services"] = make([]string, 0)
	diffKeys, err := yamlutil.DiffFlatKeys(renderset.DefaultValues, arg.VariableYaml)
	if err != nil {
		return ret, err
	}

	for _, singleSvc := range productServiceRevisions {
		containsKey, err := yamlutil.ContainsFlatKey(singleSvc.VariableYaml, diffKeys)
		if err != nil {
			return ret, err
		}
		if containsKey {
			ret["services"] = append(ret["services"], singleSvc.ServiceName)
		}
	}
	return ret, nil
}

func GeneEstimatedValues(productName, envName, serviceName, scene, format string, arg *EstimateValuesArg, log *zap.SugaredLogger) (interface{}, error) {
	chartValues, defaultValues, err := prepareEstimatedData(productName, envName, serviceName, scene, arg.DefaultValues, log)
	if err != nil {
		return nil, e.ErrUpdateRenderSet.AddDesc(fmt.Sprintf("failed to prepare data, err %s", err))
	}

	tempArg := &commonservice.HelmSvcRenderArg{OverrideValues: arg.OverrideValues}
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
// return [need-Redeploy] and [need-SaveToDB]
func checkOverrideValuesChange(source *templatemodels.ServiceRender, args *commonservice.HelmSvcRenderArg) (bool, bool) {
	sourceArg := &commonservice.HelmSvcRenderArg{}
	sourceArg.LoadFromRenderChartModel(source)

	same := sourceArg.DiffValues(args)
	switch same {
	case commonservice.Different:
		return true, true
	case commonservice.LogicSame:
		return false, true
	case commonservice.Same:
		return false, false
	}
	return false, false
}

func validateArgs(args *commonservice.ValuesDataArgs) error {
	if args == nil || args.YamlSource != setting.SourceFromVariableSet {
		return nil
	}
	_, err := commonrepo.NewVariableSetColl().Find(&commonrepo.VariableSetFindOption{ID: args.SourceID})
	if err != nil {
		return err
	}
	return nil
}

func UpdateProductDefaultValues(productName, envName, userName, requestID string, args *EnvRendersetArg, log *zap.SugaredLogger) error {
	// validate if yaml content is legal
	err := yaml.Unmarshal([]byte(args.DefaultValues), map[string]interface{}{})
	if err != nil {
		return err
	}

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		log.Errorf("UpdateHelmProductRenderset GetProductEnv envName:%s productName: %s error, error msg:%s", envName, productName, err)
		return err
	}

	opt := &commonrepo.RenderSetFindOption{
		Name:        product.Namespace,
		EnvName:     envName,
		ProductTmpl: productName,
	}
	productRenderset, _, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil || productRenderset == nil {
		if err != nil {
			log.Errorf("query renderset fail when updating helm product:%s render charts, err %s", productName, err.Error())
		}
		return e.ErrUpdateEnv.AddDesc(fmt.Sprintf("failed to query renderset for envirionment: %s", envName))
	}

	err = validateArgs(args.ValuesData)
	if err != nil {
		return e.ErrUpdateEnv.AddDesc(fmt.Sprintf("failed to validate args: %s", err))
	}

	err = UpdateProductDefaultValuesWithRender(productRenderset, userName, requestID, args, log)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		log.Errorf("UpdateHelmProductRenderset GetKubeClient error, error msg:%s", err)
		return err
	}
	return ensureKubeEnv(product.Namespace, product.RegistryID, map[string]string{setting.ProductLabel: product.ProductName}, false, kubeClient, log)
}

func UpdateProductDefaultValuesWithRender(productRenderset *models.RenderSet, userName, requestID string, args *EnvRendersetArg, log *zap.SugaredLogger) error {
	equal, err := yamlutil.Equal(productRenderset.DefaultValues, args.DefaultValues)
	if err != nil {
		return fmt.Errorf("failed to unmarshal default values in renderset, err: %s", err)
	}
	productRenderset.DefaultValues = args.DefaultValues
	productRenderset.YamlData = geneYamlData(args.ValuesData)
	updatedSvcList := make([]*templatemodels.ServiceRender, 0)
	if !equal {
		if args.DeployType == setting.K8SDeployType {
			//updatedSvcList = productRenderset.ServiceVariables
			relatedSvcs, _ := GetAffectedServices(productRenderset.ProductTmpl, productRenderset.EnvName, &K8sRendersetArg{VariableYaml: args.DefaultValues}, log)
			if relatedSvcs != nil {
				svcSet := sets.NewString(relatedSvcs["services"]...)
				svcVariableMap := make(map[string]*templatemodels.ServiceRender)
				for _, svc := range productRenderset.ServiceVariables {
					svcVariableMap[svc.ServiceName] = svc
				}
				for _, svc := range svcSet.List() {
					if curVariable, ok := svcVariableMap[svc]; ok {
						updatedSvcList = append(updatedSvcList, curVariable)
					} else {
						updatedSvcList = append(updatedSvcList, &templatemodels.ServiceRender{
							ServiceName: svc,
						})
					}
				}
			}
		} else {
			updatedSvcList = productRenderset.ChartInfos
		}
	}
	return UpdateProductVariable(productRenderset.ProductTmpl, productRenderset.EnvName, userName, requestID, updatedSvcList, productRenderset, args.DeployType, log)
}

func UpdateHelmProductCharts(productName, envName, userName, requestID string, args *EnvRendersetArg, log *zap.SugaredLogger) error {
	if len(args.ChartValues) == 0 {
		return nil
	}

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		log.Errorf("UpdateHelmProductRenderset GetProductEnv envName:%s productName: %s error, error msg:%s", envName, productName, err)
		return err
	}
	opt := &commonrepo.RenderSetFindOption{
		Name:        product.Namespace,
		EnvName:     envName,
		ProductTmpl: productName,
	}
	productRenderset, _, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil || productRenderset == nil {
		if err != nil {
			log.Errorf("query renderset fail when updating helm product:%s render charts, err %s", productName, err)
		}
		return e.ErrUpdateEnv.AddDesc(fmt.Sprintf("failed to query renderset for envirionment: %s", envName))
	}

	requestValueMap := make(map[string]*commonservice.HelmSvcRenderArg)
	for _, arg := range args.ChartValues {
		requestValueMap[arg.ServiceName] = arg
	}

	valuesInRenderset := make(map[string]*templatemodels.ServiceRender)
	for _, rc := range productRenderset.ChartInfos {
		valuesInRenderset[rc.ServiceName] = rc
	}

	updatedRcMap := make(map[string]*templatemodels.ServiceRender)
	changedCharts := make([]*commonservice.HelmSvcRenderArg, 0)

	// update override values
	for serviceName, arg := range requestValueMap {
		arg.EnvName = envName
		rcValues, ok := valuesInRenderset[serviceName]
		if !ok {
			log.Errorf("failed to find current chart values for service: %s", serviceName)
			return e.ErrUpdateEnv.AddDesc(fmt.Sprintf("failed to find current chart values for service: %s", serviceName))
		}

		arg.FillRenderChartModel(rcValues, rcValues.ChartVersion)
		changedCharts = append(changedCharts, arg)
		updatedRcMap[serviceName] = rcValues
	}

	// update service to latest revision acts like update service templates
	if args.UpdateServiceTmpl {
		updateEnvArg := &UpdateMultiHelmProductArg{
			ProductName: productName,
			EnvNames:    []string{envName},
			ChartValues: changedCharts,
		}
		_, err = UpdateMultipleHelmEnv(requestID, userName, updateEnvArg, log)
		return err
	}

	rcList := make([]*templatemodels.ServiceRender, 0)
	for _, rc := range updatedRcMap {
		rcList = append(rcList, rc)
	}

	return UpdateProductVariable(productName, envName, userName, requestID, rcList, productRenderset, setting.HelmDeployType, log)
}

func geneYamlData(args *commonservice.ValuesDataArgs) *templatemodels.CustomYaml {
	if args == nil {
		return nil
	}
	ret := &templatemodels.CustomYaml{
		Source:   args.YamlSource,
		AutoSync: args.AutoSync,
	}
	if args.YamlSource == setting.SourceFromVariableSet {
		ret.Source = setting.SourceFromVariableSet
		ret.SourceID = args.SourceID
	} else if args.GitRepoConfig != nil && args.GitRepoConfig.CodehostID > 0 {
		repoData := &models.CreateFromRepo{
			GitRepoConfig: &templatemodels.GitRepoConfig{
				CodehostID: args.GitRepoConfig.CodehostID,
				Owner:      args.GitRepoConfig.Owner,
				Namespace:  args.GitRepoConfig.Namespace,
				Repo:       args.GitRepoConfig.Repo,
				Branch:     args.GitRepoConfig.Branch,
			},
		}
		if len(args.GitRepoConfig.ValuesPaths) > 0 {
			repoData.LoadPath = args.GitRepoConfig.ValuesPaths[0]
		}
		args.YamlSource = setting.SourceFromGitRepo
		ret.SourceDetail = repoData
	}
	return ret
}

func SyncHelmProductEnvironment(productName, envName, requestID string, log *zap.SugaredLogger) error {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		log.Errorf("UpdateHelmProductRenderset GetProductEnv envName:%s productName: %s error, error msg:%s", envName, productName, err)
		return err
	}
	opt := &commonrepo.RenderSetFindOption{
		Name:        product.Namespace,
		EnvName:     envName,
		ProductTmpl: productName,
	}
	productRenderset, _, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil || productRenderset == nil {
		if err != nil {
			log.Errorf("query renderset fail when updating helm product:%s render charts, err %s", productName, err.Error())
		}
		return e.ErrUpdateEnv.AddDesc(fmt.Sprintf("failed to query renderset for envirionment: %s", envName))
	}

	updatedRCMap := make(map[string]*templatemodels.ServiceRender)

	changed, defaultValues, err := SyncYamlFromSource(productRenderset.YamlData, productRenderset.DefaultValues)
	if err != nil {
		log.Errorf("failed to update default values of env %s:%s", productRenderset.ProductTmpl, productRenderset.EnvName)
		return err
	}
	if changed {
		productRenderset.DefaultValues = defaultValues
		for _, curRenderChart := range productRenderset.ChartInfos {
			updatedRCMap[curRenderChart.ServiceName] = curRenderChart
		}
	}
	for _, chartInfo := range productRenderset.ChartInfos {
		if chartInfo.OverrideYaml == nil {
			continue
		}
		changed, values, err := SyncYamlFromSource(chartInfo.OverrideYaml, chartInfo.OverrideYaml.YamlContent)
		if err != nil {
			return err
		}
		if changed {
			chartInfo.OverrideYaml.YamlContent = values
			updatedRCMap[chartInfo.ServiceName] = chartInfo
		}
	}
	if len(updatedRCMap) == 0 {
		return nil
	}

	// content of values.yaml changed, environment will be updated
	updatedRcList := make([]*templatemodels.ServiceRender, 0)
	for _, updatedRc := range updatedRCMap {
		updatedRcList = append(updatedRcList, updatedRc)
	}

	err = UpdateProductVariable(productName, envName, "cron", requestID, updatedRcList, productRenderset, setting.HelmDeployType, log)
	if err != nil {
		return err
	}
	return err
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
	opt := &commonrepo.RenderSetFindOption{
		Name:        product.Namespace,
		EnvName:     envName,
		ProductTmpl: productName,
	}
	productRenderset, _, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil || productRenderset == nil {
		if err != nil {
			log.Errorf("query renderset fail when updating helm product:%s render charts, err %s", productName, err.Error())
		}
		return e.ErrUpdateEnv.AddDesc(fmt.Sprintf("failed to query renderset for envirionment: %s", envName))
	}

	// render charts need to be updated
	updatedRcList := make([]*templatemodels.ServiceRender, 0)
	updatedRCMap := make(map[string]*templatemodels.ServiceRender)

	// default values change
	if args.DefaultValues != productRenderset.DefaultValues {
		for _, curRenderChart := range productRenderset.ChartInfos {
			updatedRCMap[curRenderChart.ServiceName] = curRenderChart
		}
		productRenderset.DefaultValues = args.DefaultValues
	}
	productRenderset.YamlData = geneYamlData(args.ValuesData)

	for _, requestRenderChart := range args.ChartValues {
		// update renderset info
		for _, curRenderChart := range productRenderset.ChartInfos {
			if curRenderChart.ServiceName != requestRenderChart.ServiceName {
				continue
			}
			if _, needSaveData := checkOverrideValuesChange(curRenderChart, requestRenderChart); !needSaveData {
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

	err = UpdateProductVariable(productName, envName, userName, requestID, updatedRcList, productRenderset, setting.HelmDeployType, log)
	if err != nil {
		return err
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		log.Errorf("UpdateHelmProductRenderset GetKubeClient error, error msg:%s", err)
		return err
	}
	return ensureKubeEnv(product.Namespace, product.RegistryID, map[string]string{setting.ProductLabel: product.ProductName}, false, kubeClient, log)
}

func UpdateProductVariable(productName, envName, username, requestID string, updatedSvcs []*templatemodels.ServiceRender, renderset *commonmodels.RenderSet, deployType string, log *zap.SugaredLogger) error {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	productResp, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("GetProduct envName:%s, productName:%s, err:%+v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(err.Error())
	}
	productResp.ServiceRenders = updatedSvcs

	if productResp.ServiceDeployStrategy == nil {
		productResp.ServiceDeployStrategy = make(map[string]string)
	}
	needUpdateStrategy := false
	for _, rc := range updatedSvcs {
		if !commonutil.ServiceDeployed(rc.ServiceName, productResp.ServiceDeployStrategy) {
			needUpdateStrategy = true
			productResp.ServiceDeployStrategy[rc.ServiceName] = setting.ServiceDeployStrategyDeploy
		}
	}
	if needUpdateStrategy {
		err = commonrepo.NewProductColl().UpdateDeployStrategy(envName, productResp.ProductName, productResp.ServiceDeployStrategy)
		if err != nil {
			log.Errorf("[%s][P:%s] failed to update product deploy strategy: %s", productResp.EnvName, productResp.ProductName, err)
			return e.ErrUpdateEnv.AddErr(err)
		}
	}

	if err = commonservice.CreateK8sHelmRenderSet(
		&commonmodels.RenderSet{
			Name:             productResp.Namespace,
			EnvName:          envName,
			ProductTmpl:      productName,
			UpdateBy:         username,
			DefaultValues:    renderset.DefaultValues,
			YamlData:         renderset.YamlData,
			ChartInfos:       renderset.ChartInfos,
			ServiceVariables: renderset.ServiceVariables,
		},
		log,
	); err != nil {
		log.Errorf("[%s][P:%s] create renderset error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.FindProductTmplErrMsg)
	}

	productResp.EnsureRenderInfo()
	renderSet, err := FindProductRenderSet(productResp.ProductName, productResp.Render.Name, envName, log)
	if err != nil {
		log.Errorf("[%s][P:%s] find product renderset error: %s", productResp.EnvName, productResp.ProductName, err)
		return e.ErrUpdateEnv.AddDesc(err.Error())
	}
	productResp.Render.Revision = renderSet.Revision
	productResp.Render.Name = renderSet.Name

	// update render used in product
	err = commonrepo.NewProductColl().UpdateRender(envName, productResp.ProductName, productResp.Render)
	if err != nil {
		log.Errorf("[%s][P:%s] failed to update product renderset: %s", productResp.EnvName, productResp.ProductName, err)
		return e.ErrUpdateEnv.AddErr(err)
	}

	// only update renderset value to db, no need to upgrade chart release
	if len(updatedSvcs) == 0 {
		return nil
	}

	if deployType == setting.K8SDeployType {
		return updateK8sProductVariable(productResp, renderSet, username, requestID, log)
	} else {
		return updateHelmProductVariable(productResp, renderSet, username, requestID, log)
	}
}

func updateK8sProductVariable(productResp *commonmodels.Product, renderset *commonmodels.RenderSet, userName, requestID string, log *zap.SugaredLogger) error {
	filter := func(service *commonmodels.ProductService) bool {
		for _, sr := range productResp.ServiceRenders {
			if sr.ServiceName == service.ServiceName {
				return true
			}
		}
		return false
	}
	return updateK8sProduct(productResp, userName, requestID, nil, filter, renderset.ServiceVariables, nil, false, renderset.DefaultValues, log)
}

func updateHelmProductVariable(productResp *commonmodels.Product, renderset *commonmodels.RenderSet, userName, requestID string, log *zap.SugaredLogger) error {
	envName, productName := productResp.EnvName, productResp.ProductName
	restConfig, err := kube.GetRESTConfig(productResp.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	helmClient, err := helmtool.NewClientFromRestConf(restConfig, productResp.Namespace)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	// set product status to updating
	if err := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUpdating); err != nil {
		log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.UpdateEnvStatusErrMsg)
	}

	go func() {
		err := proceedHelmRelease(productResp, renderset, helmClient, nil, log)
		if err != nil {
			log.Errorf("error occurred when upgrading services in env: %s/%s, err: %s ", productName, envName, err)
			// 发送更新产品失败消息给用户
			title := fmt.Sprintf("更新 [%s] 的 [%s] 环境失败", productName, envName)
			commonservice.SendErrorMessage(userName, title, requestID, err, log)
		}
		productResp.Status = setting.ProductStatusSuccess
		if err = commonrepo.NewProductColl().UpdateStatusAndError(envName, productName, productResp.Status, ""); err != nil {
			log.Errorf("[%s][%s] Product.Update error: %v", envName, productName, err)
			return
		}
	}()
	return nil
}

var mutexUpdateMultiHelm sync.RWMutex

func UpdateMultipleHelmEnv(requestID, userName string, args *UpdateMultiHelmProductArg, log *zap.SugaredLogger) ([]*EnvStatus, error) {
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
		// NOTE. there is no need to check if product is updatable anymore
		productMap[productRevision.EnvName] = productRevision
		if len(productMap) == len(envNames) {
			break
		}
	}

	// ensure related services exist in template services
	templateProduct, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("failed to find template pruduct: %s, err: %s", productName, err)
		return envStatuses, err
	}
	serviceNameSet := sets.NewString()
	for _, svcGroup := range templateProduct.Services {
		serviceNameSet.Insert(svcGroup...)
	}
	for _, chartValue := range args.ChartValues {
		if !serviceNameSet.Has(chartValue.ServiceName) {
			return envStatuses, fmt.Errorf("failed to find service: %s in product template", chartValue.ServiceName)
		}
	}

	// extract values.yaml and update renderset
	for envName := range productMap {
		renderSet, _, err := commonrepo.NewRenderSetColl().FindRenderSet(&commonrepo.RenderSetFindOption{
			Name:        commonservice.GetProductEnvNamespace(envName, productName, ""),
			EnvName:     envName,
			ProductTmpl: productName,
		})
		if err != nil || renderSet == nil {
			if err != nil {
				log.Warnf("query renderset fail for product %s env: %s", productName, envName)
			}
			return envStatuses, e.ErrUpdateEnv.AddDesc(fmt.Sprintf("failed to query renderset for env: %s", envName))
		}

		err = updateHelmProduct(productName, envName, userName, requestID, args.ChartValues, args.DeletedServices, log)
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
	renderSetOpt := &commonrepo.RenderSetFindOption{Name: renderSetName, Revision: prod.Render.Revision, ProductTmpl: productName}
	renderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
	if err != nil {
		log.Errorf("find helm renderset[%s] error: %v", renderSetName, err)
		return prod, nil
	}
	prod.ServiceRenders = renderSet.ChartInfos
	//prod.Vars = renderSet.KVs

	return prod, nil
}

func GetHelmChartVersions(productName, envName string, log *zap.SugaredLogger) ([]*commonmodels.HelmVersions, error) {
	var (
		helmVersions = make([]*commonmodels.HelmVersions, 0)
		chartInfoMap = make(map[string]*templatemodels.ServiceRender)
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
	renderSetOpt := &commonrepo.RenderSetFindOption{Name: renderSetName, Revision: prod.Render.Revision, ProductTmpl: prod.ProductName}
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
			serviceListOpt.InServices = append(serviceListOpt.InServices, &templatemodels.ServiceInfo{
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

func DeleteProduct(username, envName, productName, requestID string, isDelete bool, log *zap.SugaredLogger) (err error) {
	eventStart := time.Now().Unix()
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: envName})
	if err != nil {
		log.Errorf("find product error: %v", err)
		return err
	}

	// delete informer's cache
	informer.DeleteInformer(productInfo.ClusterID, productInfo.Namespace)

	envCMMap, err := collaboration.GetEnvCMMap([]string{productName}, log)
	if err != nil {
		return err
	}
	if cmSets, ok := envCMMap[collaboration.BuildEnvCMMapKey(productName, envName)]; ok {
		return fmt.Errorf("this is a base environment, collaborations:%v is related", cmSets.List())
	}

	restConfig, err := kube.GetRESTConfig(productInfo.ClusterID)
	if err != nil {
		return e.ErrDeleteEnv.AddErr(err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return e.ErrDeleteEnv.AddErr(err)
	}

	err = commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusDeleting)
	if err != nil {
		log.Errorf("[%s][%s] update product status error: %v", username, productInfo.Namespace, err)
		return e.ErrDeleteEnv.AddDesc("更新环境状态失败: " + err.Error())
	}

	log.Infof("[%s] delete product %s", username, productInfo.Namespace)
	commonservice.LogProductStats(username, setting.DeleteProductEvent, productName, requestID, eventStart, log)

	ctx := context.TODO()
	switch productInfo.Source {
	case setting.SourceFromHelm:
		// Handles environment sharing related operations.
		err = EnsureDeleteShareEnvConfig(ctx, productInfo, istioClient)
		if err != nil {
			log.Errorf("Failed to delete share env config for env %s of product %s: %s", productInfo.EnvName, productInfo.ProductName, err)
		}

		err = commonrepo.NewProductColl().Delete(envName, productName)
		if err != nil {
			log.Errorf("Product.Delete error: %v", err)
		}

		go func() {
			errList := &multierror.Error{}
			defer func() {
				if errList.ErrorOrNil() != nil {
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 失败!", productName, envName)
					commonservice.SendErrorMessage(username, title, requestID, errList.ErrorOrNil(), log)
					_ = commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUnknown)
				} else {
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 成功!", productName, envName)
					content := fmt.Sprintf("namespace:%s", productInfo.Namespace)
					commonservice.SendMessage(username, title, content, requestID, log)
				}
			}()

			if isDelete && !productInfo.Production {
				if hc, errHelmClient := helmtool.NewClientFromRestConf(restConfig, productInfo.Namespace); errHelmClient == nil {
					for _, service := range productInfo.GetServiceMap() {
						if !commonutil.ServiceDeployed(service.ServiceName, productInfo.ServiceDeployStrategy) {
							continue
						}
						if err = UninstallServiceByName(hc, service.ServiceName, productInfo, service.Revision, true); err != nil {
							log.Warnf("UninstallRelease for service %s err:%s", service.ServiceName, err)
							errList = multierror.Append(errList, err)
						}
					}
				} else {
					log.Errorf("failed to get helmClient, err: %s", errHelmClient)
					errList = multierror.Append(errList, e.ErrDeleteEnv.AddErr(errHelmClient))
					return
				}

				s := labels.Set{setting.EnvCreatedBy: setting.EnvCreator}.AsSelector()
				if err := commonservice.DeleteNamespaceIfMatch(productInfo.Namespace, s, productInfo.ClusterID, log); err != nil {
					errList = multierror.Append(errList, e.ErrDeleteEnv.AddDesc(e.DeleteNamespaceErrMsg+": "+err.Error()))
					return
				}
			}
		}()
	case setting.SourceFromExternal:
		err = commonrepo.NewProductColl().Delete(envName, productName)
		if err != nil {
			log.Errorf("Product.Delete error: %v", err)
		}

		tempProduct, err := mongotemplate.NewProductColl().Find(productName)
		if err != nil {
			log.Errorf("project not found error:%s", err)
		}

		if tempProduct.IsHostProduct() {
			workloadStat, err := commonrepo.NewWorkLoadsStatColl().Find(productInfo.ClusterID, productInfo.Namespace)
			if err != nil {
				log.Errorf("workflowStat not found error:%s", err)
			}
			if workloadStat != nil {
				workloadStat.Workloads = commonservice.FilterWorkloadsByEnv(workloadStat.Workloads, productName, productInfo.EnvName)
				if err := commonrepo.NewWorkLoadsStatColl().UpdateWorkloads(workloadStat); err != nil {
					log.Errorf("update workloads fail error:%s", err)
				}
			}

			currentEnvServices, err := commonrepo.NewServiceColl().ListExternalWorkloadsBy(productName, envName)
			if err != nil {
				log.Errorf("failed to list external workload, error:%s", err)
			}

			externalEnvServices, err := commonrepo.NewServicesInExternalEnvColl().List(&commonrepo.ServicesInExternalEnvArgs{
				ProductName:    productName,
				ExcludeEnvName: envName,
			})
			if err != nil {
				log.Errorf("failed to list external service, error:%s", err)
			}

			externalEnvServiceM := make(map[string]bool)
			for _, externalEnvService := range externalEnvServices {
				externalEnvServiceM[externalEnvService.ServiceName] = true
			}

			deleteServices := sets.NewString()
			for _, currentEnvService := range currentEnvServices {
				if _, isExist := externalEnvServiceM[currentEnvService.ServiceName]; !isExist {
					deleteServices.Insert(currentEnvService.ServiceName)
				}
			}
			err = commonrepo.NewServiceColl().BatchUpdateExternalServicesStatus(productName, "", setting.ProductStatusDeleting, deleteServices.List())
			if err != nil {
				log.Errorf("UpdateStatus external services error:%s", err)
			}
			// delete services_in_external_env data
			if err = commonrepo.NewServicesInExternalEnvColl().Delete(&commonrepo.ServicesInExternalEnvArgs{
				ProductName: productName,
				EnvName:     envName,
			}); err != nil {
				log.Errorf("remove services in external env error:%s", err)
			}
		}
	case setting.SourceFromPM:
		err = commonrepo.NewProductColl().Delete(envName, productName)
		if err != nil {
			log.Errorf("Product.Delete error: %v", err)
		}
	default:
		go func() {
			var err error
			err = commonrepo.NewProductColl().Delete(envName, productName)
			if err != nil {
				log.Errorf("Product.Delete error: %v", err)
			}
			defer func() {
				if err != nil {
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 失败!", productName, envName)
					commonservice.SendErrorMessage(username, title, requestID, err, log)
					_ = commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUnknown)
				} else {
					title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 成功!", productName, envName)
					content := fmt.Sprintf("namespace:%s", productInfo.Namespace)
					commonservice.SendMessage(username, title, content, requestID, log)
				}
			}()
			if isDelete && !productInfo.Production {
				// Delete Cluster level resources
				err = commonservice.DeleteClusterResource(labels.Set{setting.ProductLabel: productName, setting.EnvNameLabel: envName}.AsSelector(), productInfo.ClusterID, log)
				if err != nil {
					err = e.ErrDeleteProduct.AddDesc(e.DeleteServiceContainerErrMsg + ": " + err.Error())
					return
				}

				// Delete the namespace-scope resources
				err = commonservice.DeleteNamespacedResource(productInfo.Namespace, labels.Set{setting.ProductLabel: productName}.AsSelector(), productInfo.ClusterID, log)
				if err != nil {
					err = e.ErrDeleteProduct.AddDesc(e.DeleteServiceContainerErrMsg + ": " + err.Error())
					return
				}

				// Handles environment sharing related operations.
				err = EnsureDeleteShareEnvConfig(ctx, productInfo, istioClient)
				if err != nil {
					log.Errorf("Failed to delete share env config: %s, env: %s/%s", err, productInfo.ProductName, productInfo.EnvName)
					err = e.ErrDeleteProduct.AddDesc(e.DeleteVirtualServiceErrMsg + ": " + err.Error())
					return
				}

				s := labels.Set{setting.EnvCreatedBy: setting.EnvCreator}.AsSelector()
				if err = commonservice.DeleteNamespaceIfMatch(productInfo.Namespace, s, productInfo.ClusterID, log); err != nil {
					err = e.ErrDeleteEnv.AddDesc(e.DeleteNamespaceErrMsg + ": " + err.Error())
					return
				}
			}
		}()
	}

	return nil
}

func DeleteProductServices(userName, requestID, envName, productName string, serviceNames []string, log *zap.SugaredLogger) (err error) {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: envName})
	if err != nil {
		log.Errorf("find product error: %v", err)
		return err
	}
	if getProjectType(productName) == setting.HelmDeployType {
		return deleteHelmProductServices(userName, requestID, productInfo, serviceNames, log)
	}
	return deleteK8sProductServices(productInfo, serviceNames, log)
}

func deleteHelmProductServices(userName, requestID string, productInfo *commonmodels.Product, serviceNames []string, log *zap.SugaredLogger) error {
	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}
	helmClient, err := helmtool.NewClientFromRestConf(restConfig, productInfo.Namespace)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	ctx := context.TODO()
	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	deleteServiceSet := sets.NewString(serviceNames...)
	deletedSvcRevision := make(map[string]int64)

	for serviceGroupIndex, serviceGroup := range productInfo.Services {
		var group []*commonmodels.ProductService
		for _, service := range serviceGroup {
			if !deleteServiceSet.Has(service.ServiceName) {
				group = append(group, service)
			} else {
				deletedSvcRevision[service.ServiceName] = service.Revision
			}
		}
		err := commonrepo.NewProductColl().UpdateGroup(productInfo.EnvName, productInfo.ProductName, serviceGroupIndex, group)
		if err != nil {
			log.Errorf("update product error: %v", err)
			return err
		}
	}

	for _, singleName := range serviceNames {
		delete(productInfo.ServiceDeployStrategy, singleName)
	}
	err = commonrepo.NewProductColl().UpdateDeployStrategy(productInfo.EnvName, productInfo.ProductName, productInfo.ServiceDeployStrategy)
	if err != nil {
		log.Errorf("failed to update product deploy strategy, err: %s", err)
	}

	renderset, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		Name:        productInfo.Namespace,
		EnvName:     productInfo.EnvName,
		ProductTmpl: productInfo.ProductName,
	})
	if err != nil {
		log.Errorf("get renderSet error: %v", err)
		return err
	}
	rcs := make([]*template.ServiceRender, 0)
	for _, v := range renderset.ChartInfos {
		if !deleteServiceSet.Has(v.ServiceName) {
			rcs = append(rcs, v)
		}
	}
	renderset.ChartInfos = rcs

	// create new renderset
	if err := commonservice.CreateK8sHelmRenderSet(renderset, log); err != nil {
		log.Errorf("failed to create renderset, name %s, err: %s", renderset.Name, err)
		return e.ErrUpdateEnv.AddErr(err)
	}

	productInfo.Render.Revision = renderset.Revision
	err = commonrepo.NewProductColl().UpdateRender(renderset.EnvName, productInfo.ProductName, productInfo.Render)
	if err != nil {
		log.Errorf("failed to update product render info, renderName: %s, err: %s", productInfo.Render.Name, err)
		return e.ErrUpdateEnv.AddErr(err)
	}

	go func() {
		failedServices := sync.Map{}
		wg := sync.WaitGroup{}
		for service, revision := range deletedSvcRevision {
			wg.Add(1)
			go func(product *models.Product, serviceName string, revision int64) {
				defer wg.Done()
				templateSvc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{ServiceName: serviceName, Revision: revision, ProductName: product.ProductName})
				if err != nil {
					failedServices.Store(serviceName, err.Error())
					return
				}
				log.Infof("uninstall release for service: %s", serviceName)
				if !commonutil.ServiceDeployed(serviceName, productInfo.ServiceDeployStrategy) {
					return
				}
				if errUninstall := UninstallService(helmClient, productInfo, templateSvc, false); errUninstall != nil {
					errStr := fmt.Sprintf("helm uninstall service %s err: %s", serviceName, errUninstall)
					failedServices.Store(serviceName, errStr)
					log.Error(errStr)
				}
			}(productInfo, service, revision)
		}
		wg.Wait()
		errList := make([]string, 0)
		failedServices.Range(func(key, value interface{}) bool {
			errList = append(errList, value.(string))
			return true
		})
		// send err message to user
		if len(errList) > 0 {
			title := fmt.Sprintf("[%s] 的 [%s] 环境服务删除失败", productInfo.ProductName, productInfo.EnvName)
			commonservice.SendErrorMessage(userName, title, requestID, errors.New(strings.Join(errList, "\n")), log)
		}

		if productInfo.ShareEnv.Enable && !productInfo.ShareEnv.IsBase {
			err = EnsureGrayEnvConfig(ctx, productInfo, kclient, istioClient)
			if err != nil {
				log.Errorf("Failed to ensure gray env config: %s", err)
			}
		}
	}()

	return nil
}

func deleteK8sProductServices(productInfo *commonmodels.Product, serviceNames []string, log *zap.SugaredLogger) error {
	for serviceGroupIndex, serviceGroup := range productInfo.Services {
		var group []*commonmodels.ProductService
		for _, service := range serviceGroup {
			if !util.InStringArray(service.ServiceName, serviceNames) {
				group = append(group, service)
			}
		}
		err := commonrepo.NewProductColl().UpdateGroup(productInfo.EnvName, productInfo.ProductName, serviceGroupIndex, group)
		if err != nil {
			log.Errorf("update product error: %v", err)
			return err
		}
	}

	for _, singleName := range serviceNames {
		delete(productInfo.ServiceDeployStrategy, singleName)
	}
	err := commonrepo.NewProductColl().UpdateDeployStrategy(productInfo.EnvName, productInfo.ProductName, productInfo.ServiceDeployStrategy)
	if err != nil {
		log.Errorf("failed to update product deploy strategy, err: %s", err)
	}

	rs, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		EnvName:     productInfo.EnvName,
		Name:        productInfo.Namespace,
		ProductTmpl: productInfo.ProductName,
	})
	if err != nil {
		log.Errorf("get renderSet error: %v", err)
		return err
	}
	//var updatedKVs []*templatemodels.RenderKV
	//for _, v := range rs.KVs {
	//	var updatedServices []string
	//	for _, service := range v.Services {
	//		if !util.InStringArray(service, serviceNames) {
	//			updatedServices = append(updatedServices, service)
	//		}
	//	}
	//	v.Services = updatedServices
	//	updatedKVs = append(updatedKVs, v)
	//}
	//rs.KVs = updatedKVs
	validServiceVars := make([]*templatemodels.ServiceRender, 0)
	for _, sr := range rs.ServiceVariables {
		if !util.InStringArray(sr.ServiceName, serviceNames) {
			validServiceVars = append(validServiceVars, sr)
		}
	}
	rs.ServiceVariables = validServiceVars
	err = commonrepo.NewRenderSetColl().Update(rs)
	if err != nil {
		log.Errorf("failed to update renderSet, error: %v", err)
		return err
	}

	ctx := context.TODO()
	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %s", err)
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get rest config: %s", err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to new istio client: %s", err)
	}

	for _, name := range serviceNames {
		selector := labels.Set{setting.ProductLabel: productInfo.ProductName, setting.ServiceLabel: name}.AsSelector()

		err = EnsureDeleteZadigService(ctx, productInfo, selector, kclient, istioClient)
		if err != nil {
			// Only record and do not block subsequent traversals.
			log.Errorf("Failed to delete Zadig service: %s", err)
		}

		err = commonservice.DeleteNamespacedResource(productInfo.Namespace, selector, productInfo.ClusterID, log)
		if err != nil {
			// Only record and do not block subsequent traversals.
			log.Errorf("delete resource of service %s error:%v", name, err)
		}
	}

	if productInfo.ShareEnv.Enable && !productInfo.ShareEnv.IsBase {
		err = EnsureGrayEnvConfig(ctx, productInfo, kclient, istioClient)
		if err != nil {
			log.Errorf("Failed to ensure gray env config: %s", err)
			return fmt.Errorf("failed to ensure gray env config: %s", err)
		}
	}

	return nil
}

func GetEstimatedRenderCharts(productName, envName, serviceNameListStr string, log *zap.SugaredLogger) ([]*commonservice.HelmSvcRenderArg, error) {

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
	renderChartInEnv, _, err := commonservice.GetSvcRenderArgs(productName, envName, serviceNameListStr, log)
	if err != nil {
		log.Errorf("find render charts in env fail, env %s err %s", envName, err.Error())
		return nil, e.ErrGetRenderSet.AddDesc("failed to get render charts in env")
	}

	rcMap := make(map[string]*commonservice.HelmSvcRenderArg)
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
		serviceOption.InServices = append(serviceOption.InServices, &templatemodels.ServiceInfo{
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
			rcMap[singleService.ServiceName] = &commonservice.HelmSvcRenderArg{
				EnvName:      envName,
				ServiceName:  singleService.ServiceName,
				ChartVersion: singleService.HelmChart.Version,
			}
		}
	}

	ret := make([]*commonservice.HelmSvcRenderArg, 0, len(rcMap))
	for _, rc := range rcMap {
		ret = append(ret, rc)
	}
	return ret, nil
}

func createGroups(user, requestID string, args *commonmodels.Product, eventStart int64, renderSet *commonmodels.RenderSet, informer informers.SharedInformerFactory, kubeClient client.Client, istioClient versionedclient.Interface, log *zap.SugaredLogger) {
	var err error
	envName := args.EnvName
	defer func() {
		status := setting.ProductStatusSuccess
		errorMsg := ""
		if err != nil {
			status = setting.ProductStatusFailed
			errorMsg = err.Error()

			// 发送创建产品失败消息给用户
			title := fmt.Sprintf("创建 [%s] 的 [%s] 环境失败:%s", args.ProductName, args.EnvName, errorMsg)
			commonservice.SendErrorMessage(user, title, requestID, err, log)
		}

		commonservice.LogProductStats(envName, setting.CreateProductEvent, args.ProductName, requestID, eventStart, log)

		if err = commonrepo.NewProductColl().UpdateStatusAndError(envName, args.ProductName, status, errorMsg); err != nil {
			log.Errorf("[%s][%s] Product.Update set product status error: %v", envName, args.ProductName, err)
			return
		}
	}()

	err = initEnvConfigSetAction(args.EnvName, args.Namespace, args.ProductName, user, args.EnvConfigs, false, kubeClient)
	if err != nil {
		args.Status = setting.ProductStatusFailed
		log.Errorf("initEnvConfigSet error :%s", err)
		return
	}

	for _, group := range args.Services {
		err = envHandleFunc(getProjectType(args.ProductName), log).createGroup(user, args, group, renderSet, informer, kubeClient)
		if err != nil {
			args.Status = setting.ProductStatusFailed
			log.Errorf("createGroup error :%+v", err)
			return
		}
	}

	// If the user does not enable environment sharing, end. Otherwise, continue to perform environment sharing operations.
	if !args.ShareEnv.Enable {
		return
	}

	// Note: Currently, only sub-environments can be created, but baseline environments cannot be created.
	err = EnsureGrayEnvConfig(context.TODO(), args, kubeClient, istioClient)
	if err != nil {
		args.Status = setting.ProductStatusFailed
		log.Errorf("Failed to ensure environment sharing in env %s of product %s: %s", args.EnvName, args.ProductName, err)
		return
	}
}

func getProjectType(productName string) string {
	projectInfo, _ := templaterepo.NewProductColl().Find(productName)
	projectType := setting.K8SDeployType
	if projectInfo == nil || projectInfo.ProductFeature == nil {
		return projectType
	}

	if projectInfo.ProductFeature.DeployType == setting.HelmDeployType {
		return setting.HelmDeployType
	}

	if projectInfo.ProductFeature.DeployType == setting.K8SDeployType && projectInfo.ProductFeature.BasicFacility == setting.BasicFacilityK8S {
		return projectType
	}

	if projectInfo.ProductFeature.DeployType == setting.K8SDeployType && projectInfo.ProductFeature.BasicFacility == setting.BasicFacilityCVM {
		return setting.PMDeployType
	}
	return projectType
}

// upsertService
func upsertService(env *commonmodels.Product, service *commonmodels.ProductService, prevSvc *commonmodels.ProductService,
	renderSet *commonmodels.RenderSet, preRenderInfo *commonmodels.RenderInfo, informer informers.SharedInformerFactory, kubeClient client.Client, istioClient versionedclient.Interface, log *zap.SugaredLogger,
) ([]*unstructured.Unstructured, error) {
	isUpdate := prevSvc == nil
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

	if service.Type != setting.K8SDeployType {
		return nil, nil
	}

	productName := env.ProductName
	envName := env.EnvName
	namespace := env.Namespace

	// 获取服务模板
	parsedYaml, err := kube.RenderEnvService(env, renderSet, service)

	if err != nil {
		log.Errorf("Failed to render service %s, error: %v", service.ServiceName, err)
		errList = multierror.Append(errList, fmt.Errorf("service template %s error: %v", service.ServiceName, err))
		return nil, errList
	}

	manifests := releaseutil.SplitManifests(parsedYaml)
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
	if prevSvc != nil && preRenderInfo != nil {
		err = removeOldResources(resources, env, prevSvc, preRenderInfo, kubeClient, log)
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

			u.SetNamespace(namespace)
			u.SetLabels(ls)

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

			if istioClient != nil {
				err = EnsureUpdateZadigService(context.TODO(), env, u.GetName(), kubeClient, istioClient)
				if err != nil {
					log.Errorf("Failed to update Zadig service %s for env %s of product %s: %s", u.GetName(), env.EnvName, env.ProductName, err)
					errList = multierror.Append(errList, err)
					continue
				}
			}
		case setting.Deployment, setting.StatefulSet:
			// compatibility flag, We add a match label in spec.selector field pre 1.10.
			needSelectorLabel := false

			u.SetNamespace(namespace)
			u.SetLabels(kube.MergeLabels(labels, u.GetLabels()))

			switch u.GetKind() {
			case setting.Deployment:
				needSelectorLabel = deploymentSelectorLabelExists(u.GetName(), namespace, informer, log)
			case setting.StatefulSet:
				needSelectorLabel = statefulsetSelectorLabelExists(u.GetName(), namespace, informer, log)
			}

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

			if needSelectorLabel {
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
				errList = multierror.Append(errList, fmt.Errorf("object is not a appsv1.Deployment or appsv1.StatefulSet"))
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
	oldRenderInfo *commonmodels.RenderInfo,
	kubeClient client.Client,
	log *zap.SugaredLogger,
) error {
	opt := &commonrepo.RenderSetFindOption{
		Name:        oldRenderInfo.Name,
		Revision:    oldRenderInfo.Revision,
		EnvName:     env.EnvName,
		ProductTmpl: env.ProductName,
	}
	oldRenderset, err := commonrepo.NewRenderSetColl().Find(opt)
	if err != nil {
		log.Errorf("find renderset[%s/%d] error: %v", opt.Name, opt.Revision, err)
		return err
	}

	parsedYaml, err := kube.RenderEnvService(env, oldRenderset, oldService)
	if err != nil {
		log.Errorf("failed to find old service revision %s/%d", oldService.ServiceName, oldService.Revision)
		return err
	}

	itemsMap := make(map[string]*unstructured.Unstructured)
	for _, u := range items {
		itemsMap[fmt.Sprintf("%s/%s", u.GetKind(), u.GetName())] = u
	}

	manifests := releaseutil.SplitManifests(parsedYaml)
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
		item.SetNamespace(env.Namespace)
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
	if args.Render != nil && args.Render.Revision > 0 {
		renderSetName = args.Render.Name
	} else {
		switch args.Source {
		case setting.HelmDeployType:
			err = commonservice.CreateK8sHelmRenderSet(
				&commonmodels.RenderSet{
					Name:        renderSetName,
					Revision:    0,
					EnvName:     envName,
					ProductTmpl: args.ProductName,
					UpdateBy:    args.UpdateBy,
					ChartInfos:  args.ServiceRenders,
				},
				log,
			)
		default:
			err = commonservice.CreateRenderSet(
				&commonmodels.RenderSet{
					Name:             renderSetName,
					Revision:         0,
					EnvName:          envName,
					ProductTmpl:      args.ProductName,
					UpdateBy:         args.UpdateBy,
					ServiceVariables: args.ServiceRenders,
					//KVs:         args.Vars,
				},
				log,
			)

		}
		if err != nil {
			log.Errorf("[%s][P:%s] create renderset error: %v", envName, productTemplateName, err)
			return e.ErrCreateEnv.AddDesc(e.FindProductTmplErrMsg)
		}
	}

	//args.Vars = nil

	var productTmpl *templatemodels.Product
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
	if serviceCount == 0 && !args.Production {
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
		return ensureKubeEnv(args.Namespace, args.RegistryID, map[string]string{setting.ProductLabel: args.ProductName}, args.ShareEnv.Enable, kubeClient, log)
	}
	return nil
}

func preCreateNSAndSecret(productFeature *templatemodels.ProductFeature) bool {
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

func applyUpdatedAnnotations(annotations map[string]string) map[string]string {
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[setting.UpdatedByLabel] = fmt.Sprintf("%d", time.Now().Unix())
	return annotations
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

func ensureKubeEnv(namespace, registryId string, customLabels map[string]string, enableShare bool, kubeClient client.Client, log *zap.SugaredLogger) error {
	err := kube.CreateNamespace(namespace, customLabels, enableShare, kubeClient)
	if err != nil {
		log.Errorf("[%s] get or create namespace error: %v", namespace, err)
		return e.ErrCreateNamspace.AddDesc(err.Error())
	}

	// 创建默认的镜像仓库secret
	if err := commonservice.EnsureDefaultRegistrySecret(namespace, registryId, kubeClient, log); err != nil {
		log.Errorf("[%s] get or create namespace error: %v", namespace, err)
		return e.ErrCreateSecret.AddDesc(e.CreateDefaultRegistryErrMsg)
	}

	return nil
}

func FindProductRenderSet(productName, renderName, envName string, log *zap.SugaredLogger) (*commonmodels.RenderSet, error) {
	resp := &commonmodels.RenderSet{ProductTmpl: productName}
	var err error
	opt := &commonrepo.RenderSetFindOption{
		Name:        renderName,
		ProductTmpl: productName,
		EnvName:     envName,
	}
	resp, err = commonrepo.NewRenderSetColl().Find(opt)
	if err != nil {
		log.Errorf("find helm renderset[%s] error: %v", renderName, err)
		return resp, err
	}
	return resp, nil
}

func buildInstallParam(namespace, envName, defaultValues string, renderChart *templatemodels.ServiceRender, serviceObj *commonmodels.Service) (*ReleaseInstallParam, error) {
	mergedValues, err := helmtool.MergeOverrideValues(renderChart.ValuesYaml, defaultValues, renderChart.GetOverrideYaml(), renderChart.OverrideValues)
	if err != nil {
		return nil, fmt.Errorf("failed to merge override yaml %s and values %s, err: %s", renderChart.GetOverrideYaml(), renderChart.OverrideValues, err)
	}
	ret := &ReleaseInstallParam{
		ProductName:  serviceObj.ProductName,
		Namespace:    namespace,
		ReleaseName:  util.GeneReleaseName(serviceObj.GetReleaseNaming(), serviceObj.ProductName, namespace, envName, serviceObj.ServiceName),
		MergedValues: mergedValues,
		RenderChart:  renderChart,
		serviceObj:   serviceObj,
	}
	return ret, nil
}

func installOrUpgradeHelmChartWithValues(param *ReleaseInstallParam, isRetry bool, helmClient *helmtool.HelmClient) error {
	namespace, valuesYaml, renderChart, serviceObj := param.Namespace, param.MergedValues, param.RenderChart, param.serviceObj
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
		ReleaseName:   param.ReleaseName,
		ChartName:     chartPath,
		Namespace:     namespace,
		Version:       renderChart.ChartVersion,
		ValuesYaml:    valuesYaml,
		UpgradeCRDs:   true,
		CleanupOnFail: true,
		MaxHistory:    10,
		DryRun:        param.DryRun,
	}
	if isRetry {
		chartSpec.Replace = true
	}

	// If the target environment is a shared environment and a sub env, we need to clear the deployed K8s Service.
	ctx := context.TODO()
	if !chartSpec.DryRun {
		err = EnsureDeletePreCreatedServices(ctx, param.ProductName, param.Namespace, chartSpec, helmClient)
		if err != nil {
			return fmt.Errorf("failed to ensure deleting pre-created K8s Services for product %q in namespace %q: %s", param.ProductName, param.Namespace, err)
		}
	}

	helmClient, err = helmClient.Clone()
	if err != nil {
		return fmt.Errorf("failed to clone helm client: %s", err)
	}

	var release *release.Release
	release, err = helmClient.InstallOrUpgradeChart(ctx, chartSpec, nil)
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to install or upgrade helm chart %s/%s",
			namespace, serviceObj.ServiceName)
	} else {
		if !chartSpec.DryRun {
			err = EnsureZadigServiceByManifest(ctx, param.ProductName, param.Namespace, release.Manifest)
			if err != nil {
				err = errors.WithMessagef(err, "failed to ensure Zadig Service %s", err)
			}
		}
	}

	return err
}

func installProductHelmCharts(user, requestID string, args *commonmodels.Product, renderset *commonmodels.RenderSet, eventStart int64, helmClient *helmtool.HelmClient,
	kclient client.Client, istioClient versionedclient.Interface, log *zap.SugaredLogger) {
	var (
		err     error
		errList = &multierror.Error{}
	)
	envName := args.EnvName

	defer func() {
		if err != nil {
			title := fmt.Sprintf("创建 [%s] 的 [%s] 环境失败", args.ProductName, args.EnvName)
			commonservice.SendErrorMessage(user, title, requestID, err, log)
		}

		commonservice.LogProductStats(envName, setting.CreateProductEvent, args.ProductName, requestID, eventStart, log)

		status := setting.ProductStatusSuccess
		if err = commonrepo.NewProductColl().UpdateStatusAndError(envName, args.ProductName, status, ""); err != nil {
			log.Errorf("[%s][P:%s] Product.UpdateStatusAndError error: %v", envName, args.ProductName, err)
			return
		}
	}()

	chartInfoMap := make(map[string]*templatemodels.ServiceRender)
	for _, renderChart := range args.ServiceRenders {
		chartInfoMap[renderChart.ServiceName] = renderChart
	}

	err = proceedHelmRelease(args, renderset, helmClient, nil, log)
	if err != nil {
		log.Errorf("error occurred when installing services in env: %s/%s, err: %s ", args.ProductName, envName, err)
		errList = multierror.Append(errList, err)
	}

	// Note: For the sub env, try to supplement information relevant to the base env.
	if args.ShareEnv.Enable && !args.ShareEnv.IsBase {
		shareEnvErr := EnsureGrayEnvConfig(context.TODO(), args, kclient, istioClient)
		if shareEnvErr != nil {
			errList = multierror.Append(errList, shareEnvErr)
		}
	}

	err = errList.ErrorOrNil()
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
			if serviceRevision.New {
				// 新的服务，创建新的service with revision, 并append到updatedGroups中
				// 新的服务的revision，默认Revision为0
				newService := &commonmodels.ProductService{
					ServiceName: service.ServiceName,
					ProductName: service.ProductName,
					Type:        service.Type,
					Revision:    0,
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

func batchExecutorWithRetry(retryCount uint64, interval time.Duration, serviceList []*commonmodels.Service, handler intervalExecutorHandler, log *zap.SugaredLogger) []error {
	bo := backoff.NewConstantBackOff(time.Second * 3)
	retryBo := backoff.WithMaxRetries(bo, retryCount)
	errList := make([]error, 0)
	isRetry := false
	_ = backoff.Retry(func() error {
		failedServices := make([]*commonmodels.Service, 0)
		errList = batchExecutor(interval, serviceList, &failedServices, isRetry, handler, log)
		if len(errList) == 0 {
			return nil
		}
		log.Infof("%d services waiting to retry", len(failedServices))
		serviceList = failedServices
		isRetry = true
		return fmt.Errorf("%d services apply failed", len(errList))
	}, retryBo)
	return errList
}

func batchExecutor(interval time.Duration, serviceList []*commonmodels.Service, failedServices *[]*commonmodels.Service, isRetry bool, handler intervalExecutorHandler, log *zap.SugaredLogger) []error {
	if len(serviceList) == 0 {
		return nil
	}
	errList := make([]error, 0)
	for _, data := range serviceList {
		err := handler(data, isRetry, log)
		if err != nil {
			errList = append(errList, err)
			*failedServices = append(*failedServices, data)
			log.Errorf("service:%s apply failed, err %s", data.ServiceName, err)
		}
		time.Sleep(interval)
	}
	return errList
}

func updateHelmProductGroup(username, productName, envName string, productResp *commonmodels.Product,
	overrideCharts []*commonservice.HelmSvcRenderArg, deletedSvcRevision map[string]int64, log *zap.SugaredLogger) error {

	helmClient, err := helmtool.NewClientFromNamespace(productResp.ClusterID, productResp.Namespace)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	// uninstall services
	for serviceName, serviceRevision := range deletedSvcRevision {
		if !commonutil.ServiceDeployed(serviceName, productResp.ServiceDeployStrategy) {
			continue
		}
		if productResp.ServiceDeployStrategy != nil {
			delete(productResp.ServiceDeployStrategy, serviceName)
		}
		if err = UninstallServiceByName(helmClient, serviceName, productResp, serviceRevision, true); err != nil {
			log.Errorf("UninstallRelease err:%v", err)
			return e.ErrUpdateEnv.AddErr(err)
		}
	}

	renderSet, err := diffRenderSet(username, productName, envName, productResp, overrideCharts, log)
	if err != nil {
		return e.ErrUpdateEnv.AddDesc("对比环境中的value.yaml和系统默认的value.yaml失败")
	}

	productResp.ServiceRenders = renderSet.ChartInfos
	svcNameSet := sets.NewString()
	for _, singleChart := range overrideCharts {
		if singleChart.EnvName != envName {
			continue
		}
		svcNameSet.Insert(singleChart.ServiceName)
	}

	filter := func(svc *commonmodels.ProductService) bool {
		return svcNameSet.Has(svc.ServiceName)
	}

	productResp.Render.Revision = renderSet.Revision

	if productResp.ServiceDeployStrategy != nil {
		for _, chart := range overrideCharts {
			productResp.ServiceDeployStrategy[chart.ServiceName] = chart.DeployStrategy
		}
	}

	if err = commonrepo.NewProductColl().Update(productResp); err != nil {
		log.Errorf("Failed to update env, err: %s", err)
		return err
	}

	err = proceedHelmRelease(productResp, renderSet, helmClient, filter, log)
	if err != nil {
		log.Errorf("error occurred when upgrading services in env: %s/%s, err: %s ", productName, envName, err)
		return err
	}

	return nil
}

// diffRenderSet get diff between renderset in product and product template
// generate a new renderset and insert into db
func diffRenderSet(username, productName, envName string, productResp *commonmodels.Product, overrideCharts []*commonservice.HelmSvcRenderArg, log *zap.SugaredLogger) (*commonmodels.RenderSet, error) {
	// default renderset
	latestRenderSet, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{Name: productName, IsDefault: true})
	if err != nil {
		log.Errorf("[RenderSet.find] err: %v", err)
		return nil, err
	}

	// chart infos in template
	latestChartInfoMap := make(map[string]*templatemodels.ServiceRender)
	for _, renderInfo := range latestRenderSet.ChartInfos {
		latestChartInfoMap[renderInfo.ServiceName] = renderInfo
	}

	// chart infos from client
	renderChartArgMap := make(map[string]*commonservice.HelmSvcRenderArg)
	for _, singleArg := range overrideCharts {
		if singleArg.EnvName == envName {
			renderChartArgMap[singleArg.ServiceName] = singleArg
		}
	}

	renderSetOpt := &commonrepo.RenderSetFindOption{
		Name:        productResp.Render.Name,
		Revision:    productResp.Render.Revision,
		ProductTmpl: productName,
	}
	currentEnvRenderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
	if err != nil {
		log.Errorf("[RenderSet.find] err: %v", err)
		return nil, err
	}
	defaultValues, yamlData := currentEnvRenderSet.DefaultValues, currentEnvRenderSet.YamlData

	// chart infos in product
	currentChartInfoMap := make(map[string]*templatemodels.ServiceRender)
	for _, renderInfo := range currentEnvRenderSet.ChartInfos {
		currentChartInfoMap[renderInfo.ServiceName] = renderInfo
	}

	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	productCur, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("GetProduct envName:%s, productName:%s, err:%s", envName, productName, err)
		return nil, fmt.Errorf("GetProduct envName:%s, productName:%s, err:%s", envName, productName, err)
	}
	serviceMap := productCur.GetServiceMap()
	serviceRespMap := productResp.GetServiceMap()
	newChartInfos := make([]*templatemodels.ServiceRender, 0)

	for serviceName, latestChartInfo := range latestChartInfoMap {
		currentChartInfo, okC := currentChartInfoMap[serviceName]
		renderArg, okR := renderChartArgMap[serviceName]
		if !okR && !okC {
			continue
		}

		// no need to update service revision in renderset.services
		if !okR {
			newChartInfos = append(newChartInfos, currentChartInfo)
			continue
		}

		serviceInfoResp := serviceRespMap[serviceName]
		serviceInfoCur := serviceMap[serviceName]
		imageRelatedKey := sets.NewString()
		if serviceInfoResp != nil && serviceInfoCur != nil {
			curEnvService, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
				ServiceName: serviceName,
				ProductName: productName,
				Type:        setting.HelmDeployType,
				Revision:    serviceInfoCur.Revision,
			})
			if err != nil {
				log.Errorf("failed to query service, name %s, Revision %d,err %s", serviceName, serviceInfoCur.Revision, err)
				return nil, fmt.Errorf("failed to query service, name %s,Revision %d,err %s", serviceName, serviceInfoCur.Revision, err)
			}
		L:
			for _, curSvcContainers := range curEnvService.Containers {
				if checkServiceImageUpdated(curSvcContainers, serviceInfoCur) {
					for _, container := range serviceInfoResp.Containers {
						if curSvcContainers.Name == container.Name && container.ImagePath != nil {
							imageRelatedKey.Insert(container.ImagePath.Image, container.ImagePath.Repo, container.ImagePath.Tag)
							continue L
						}
					}
				}
			}
		}

		// use the variables in current product when updating services
		if okC {
			// use the value of the key of the current values.yaml to replace the value of the same key of the values.yaml in the service
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
		if okR {
			renderArg.FillRenderChartModel(latestChartInfo, latestChartInfo.ChartVersion)
		}
		newChartInfos = append(newChartInfos, latestChartInfo)
	}

	if err = commonservice.CreateK8sHelmRenderSet(
		&commonmodels.RenderSet{
			Name:          productResp.Render.Name,
			EnvName:       envName,
			ProductTmpl:   productName,
			ChartInfos:    newChartInfos,
			DefaultValues: defaultValues,
			YamlData:      yamlData,
			UpdateBy:      username,
		},
		log,
	); err != nil {
		log.Errorf("[RenderSet.create] err: %v", err)
		return nil, err
	}

	renderSet, err := FindProductRenderSet(productName, productResp.Render.Name, envName, log)
	if err != nil {
		log.Errorf("[RenderSet.find] err: %v", err)
		return nil, err
	}
	return renderSet, nil
}

// checkServiceImageUpdated If the service does not do any mirroring iterations on the platform, the latest YAML is used when updating the environment
func checkServiceImageUpdated(curContainer *commonmodels.Container, serviceInfo *commonmodels.ProductService) bool {
	for _, proContainer := range serviceInfo.Containers {
		if curContainer.Name == proContainer.Name && curContainer.Image == proContainer.Image {
			return false
		}
	}
	return true
}

// for keys exist in both yaml, current values will override the latest values
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

func dryRunInstallRelease(productResp *commonmodels.Product, renderset *commonmodels.RenderSet, helmClient *helmtool.HelmClient, log *zap.SugaredLogger) error {
	productName, _ := productResp.ProductName, productResp.EnvName
	renderChartMap := make(map[string]*templatemodels.ServiceRender)
	for _, renderChart := range productResp.ServiceRenders {
		renderChartMap[renderChart.ServiceName] = renderChart
	}

	handler := func(serviceObj *commonmodels.Service, log *zap.SugaredLogger) (err error) {
		param, errBuildParam := buildInstallParam(productResp.Namespace, renderset.EnvName, renderset.DefaultValues, renderChartMap[serviceObj.ServiceName], serviceObj)
		if errBuildParam != nil {
			return errBuildParam
		}
		param.DryRun = true
		err = installOrUpgradeHelmChartWithValues(param, false, helmClient)
		return
	}

	errList := new(multierror.Error)
	var errLock sync.Mutex
	appendErr := func(err error) {
		errLock.Lock()
		defer errLock.Unlock()
		errList = multierror.Append(errList, err)
	}

	var wg sync.WaitGroup
	for _, groupServices := range productResp.Services {
		serviceList := make([]*commonmodels.Service, 0)
		for _, service := range groupServices {
			if _, ok := renderChartMap[service.ServiceName]; !ok {
				continue
			}
			if !commonutil.ServiceDeployed(service.ServiceName, productResp.ServiceDeployStrategy) {
				continue
			}
			opt := &commonrepo.ServiceFindOption{
				ServiceName: service.ServiceName,
				Type:        service.Type,
				Revision:    service.Revision,
				ProductName: productName,
			}
			serviceObj, err := commonrepo.NewServiceColl().Find(opt)
			if err != nil {
				appendErr(fmt.Errorf("failed to find service %s, err %s", service.ServiceName, err.Error()))
				continue
			}
			serviceList = append(serviceList, serviceObj)
		}

		for _, svc := range serviceList {
			wg.Add(1)
			go func(service *models.Service) {
				defer wg.Done()
				err := handler(service, log)
				if err != nil {
					appendErr(fmt.Errorf("failed to dryRun install chart for service: %s, err: %s", service.ServiceName, err))
				}
			}(svc)
		}
	}
	wg.Wait()
	return errList.ErrorOrNil()
}

func proceedHelmRelease(productResp *commonmodels.Product, renderset *commonmodels.RenderSet, helmClient *helmtool.HelmClient, filter svcUpgradeFilter, log *zap.SugaredLogger) error {
	productName, envName := productResp.ProductName, productResp.EnvName
	renderChartMap := make(map[string]*templatemodels.ServiceRender)
	for _, renderChart := range productResp.ServiceRenders {
		renderChartMap[renderChart.ServiceName] = renderChart
	}

	prodServiceMap := productResp.GetServiceMap()
	handler := func(serviceObj *commonmodels.Service, isRetry bool, log *zap.SugaredLogger) (err error) {
		defer func() {
			if prodSvc, ok := prodServiceMap[serviceObj.ServiceName]; ok {
				if err != nil {
					prodSvc.Error = err.Error()
				} else {
					prodSvc.Error = ""
				}
			}
		}()
		param, errBuildParam := buildInstallParam(productResp.Namespace, renderset.EnvName, renderset.DefaultValues, renderChartMap[serviceObj.ServiceName], serviceObj)
		if errBuildParam != nil {
			err = fmt.Errorf("failed to generate install param, service: %s, namespace: %s, err: %s", serviceObj.ServiceName, productResp.Namespace, errBuildParam)
			return
		}
		errInstall := installOrUpgradeHelmChartWithValues(param, isRetry, helmClient)
		if errInstall != nil {
			log.Errorf("failed to upgrade service: %s, namespace: %s, isRetry: %v, err: %s", serviceObj.ServiceName, productResp.Namespace, isRetry, errInstall)
			err = fmt.Errorf("failed to upgrade service %s, err: %s", serviceObj.ServiceName, errInstall)
		}
		return
	}

	errList := new(multierror.Error)
	for groupIndex, groupServices := range productResp.Services {
		serviceList := make([]*commonmodels.Service, 0)
		for _, service := range groupServices {
			if _, ok := renderChartMap[service.ServiceName]; !ok {
				continue
			}
			if filter != nil && !filter(service) {
				continue
			}
			if !commonutil.ServiceDeployed(service.ServiceName, productResp.ServiceDeployStrategy) {
				continue
			}
			opt := &commonrepo.ServiceFindOption{
				ServiceName: service.ServiceName,
				Type:        service.Type,
				Revision:    service.Revision,
				ProductName: productName,
			}
			serviceObj, err := commonrepo.NewServiceColl().Find(opt)
			if err != nil {
				log.Errorf("failed to find service %s, err %s", service.ServiceName, err.Error())
				return err
			}
			serviceList = append(serviceList, serviceObj)
		}
		groupServiceErr := batchExecutorWithRetry(3, time.Millisecond*500, serviceList, handler, log)
		if groupServiceErr != nil {
			errList = multierror.Append(errList, groupServiceErr...)
		}
		err := commonrepo.NewProductColl().UpdateGroup(envName, productName, groupIndex, groupServices)
		if err != nil {
			log.Errorf("Failed to update service group %d. Error: %v", groupIndex, err)
			return err
		}
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

func deploymentSelectorLabelExists(resourceName, namespace string, informer informers.SharedInformerFactory, log *zap.SugaredLogger) bool {
	deployment, err := informer.Apps().V1().Deployments().Lister().Deployments(namespace).Get(resourceName)
	// default we assume the deployment is new so we don't need to add selector labels
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Errorf("Failed to find deployment in the namespace: %s, the error is: %s", namespace, err)
		}
		return false
	}
	// since the 2 predefined labels are always together, we just check for only one
	// if the match label exists, we return true. otherwise we return false
	if _, ok := deployment.Spec.Selector.MatchLabels["s-product"]; ok {
		return true
	}
	return false
}

func statefulsetSelectorLabelExists(resourceName, namespace string, informer informers.SharedInformerFactory, log *zap.SugaredLogger) bool {
	sts, err := informer.Apps().V1().StatefulSets().Lister().StatefulSets(namespace).Get(resourceName)
	// default we assume the deployment is new so we don't need to add selector labels
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Errorf("Failed to find deployment in the namespace: %s, the error is: %s", namespace, err)
		}
		return false
	}
	// since the 2 predefined labels are always together, we just check for only one
	// if the match label exists, we return true. otherwise we return false
	if _, ok := sts.Spec.Selector.MatchLabels["s-product"]; ok {
		return true
	}
	return false
}
