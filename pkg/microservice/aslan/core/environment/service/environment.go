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
	"go.mongodb.org/mongo-driver/bson/primitive"
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

const (
	Timeout = 60
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
	ProjectName string   `json:"projectName"`
	Status      string   `json:"status"`
	Error       string   `json:"error"`
	Name        string   `json:"name"`
	UpdateBy    string   `json:"updateBy"`
	UpdateTime  int64    `json:"updateTime"`
	IsPublic    bool     `json:"isPublic"`
	ClusterName string   `json:"clusterName"`
	ClusterID   string   `json:"cluster_id"`
	Production  bool     `json:"production"`
	Source      string   `json:"source"`
	RegistryID  string   `json:"registry_id"`
	BaseRefs    []string `json:"base_refs"`
	BaseName    string   `json:"base_name"`
	IsExisted   bool     `json:"is_existed"`

	// New Since v1.11.0
	ShareEnvEnable  bool   `json:"share_env_enable"`
	ShareEnvIsBase  bool   `json:"share_env_is_base"`
	ShareEnvBaseEnv string `json:"share_env_base_env"`
}

type ProductResp struct {
	ID          string                     `json:"id"`
	ProductName string                     `json:"product_name"`
	Namespace   string                     `json:"namespace"`
	Status      string                     `json:"status"`
	Error       string                     `json:"error"`
	EnvName     string                     `json:"env_name"`
	UpdateBy    string                     `json:"update_by"`
	UpdateTime  int64                      `json:"update_time"`
	Services    [][]string                 `json:"services"`
	Render      *commonmodels.RenderInfo   `json:"render"`
	Vars        []*templatemodels.RenderKV `json:"vars"`
	IsPublic    bool                       `json:"isPublic"`
	ClusterID   string                     `json:"cluster_id,omitempty"`
	ClusterName string                     `json:"cluster_name,omitempty"`
	RecycleDay  int                        `json:"recycle_day"`
	IsProd      bool                       `json:"is_prod"`
	IsLocal     bool                       `json:"is_local"`
	IsExisted   bool                       `json:"is_existed"`
	Source      string                     `json:"source"`
	RegisterID  string                     `json:"registry_id"`

	// New Since v1.11.0
	ShareEnvEnable  bool   `json:"share_env_enable"`
	ShareEnvIsBase  bool   `json:"share_env_is_base"`
	ShareEnvBaseEnv string `json:"share_env_base_env"`
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
	DefaultValues     string                          `json:"defaultValues"`
	ValuesData        *commonservice.ValuesDataArgs   `json:"valuesData"`
	ChartValues       []*commonservice.RenderChartArg `json:"chartValues"`
	UpdateServiceTmpl bool                            `json:"updateServiceTmpl"`
}

type CreateHelmProductArg struct {
	ProductName   string                          `json:"productName"`
	EnvName       string                          `json:"envName"`
	Namespace     string                          `json:"namespace"`
	ClusterID     string                          `json:"clusterID"`
	DefaultValues string                          `json:"defaultValues"`
	ValuesData    *commonservice.ValuesDataArgs   `json:"valuesData"`
	RegistryID    string                          `json:"registry_id"`
	ChartValues   []*commonservice.RenderChartArg `json:"chartValues"`
	BaseEnvName   string                          `json:"baseEnvName"`
	BaseName      string                          `json:"base_name,omitempty"`
	IsExisted     bool                            `json:"is_existed"`
	// New Since v1.12.0
	ShareEnv commonmodels.ProductShareEnv `json:"share_env"`
	// New Since v1.13.0
	EnvConfigs []*commonmodels.CreateUpdateCommonEnvCfgArgs `json:"env_configs"`
}

type UpdateMultiHelmProductArg struct {
	ProductName     string                          `json:"productName"`
	EnvNames        []string                        `json:"envNames"`
	ChartValues     []*commonservice.RenderChartArg `json:"chartValues"`
	DeletedServices []string                        `json:"deletedServices"`
	ReplacePolicy   string                          `json:"replacePolicy"` // TODO logic not implemented
}

type RawYamlResp struct {
	YamlContent string `json:"yamlContent"`
}

type ReleaseInstallParam struct {
	ProductName  string
	Namespace    string
	ReleaseName  string
	MergedValues string
	RenderChart  *templatemodels.RenderChart
	serviceObj   *commonmodels.Service
	DryRun       bool
}

type intervalExecutorHandler func(data *commonmodels.Service, isRetry bool, log *zap.SugaredLogger) error
type svcUpgradeFilter func(svc *commonmodels.ProductService) bool

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
	reg, _, err := commonservice.FindDefaultRegistry(false, log)
	if err != nil {
		log.Errorf("FindDefaultRegistry error: %v", err)
		return nil, err
	}

	envCMMap, err := collaboration.GetEnvCMMap([]string{projectName}, log)
	if err != nil {
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
		var baseRefs []string
		if cmSet, ok := envCMMap[collaboration.BuildEnvCMMapKey(env.ProductName, env.EnvName)]; ok {
			baseRefs = append(baseRefs, cmSet.List()...)
		}
		res = append(res, &EnvResp{
			ProjectName:     projectName,
			Name:            env.EnvName,
			IsPublic:        env.IsPublic,
			IsExisted:       env.IsExisted,
			ClusterName:     clusterName,
			Source:          env.Source,
			Production:      production,
			Status:          env.Status,
			Error:           env.Error,
			UpdateTime:      env.UpdateTime,
			UpdateBy:        env.UpdateBy,
			RegistryID:      env.RegistryID,
			ClusterID:       env.ClusterID,
			BaseRefs:        baseRefs,
			BaseName:        env.BaseName,
			ShareEnvEnable:  env.ShareEnv.Enable,
			ShareEnvIsBase:  env.ShareEnv.IsBase,
			ShareEnvBaseEnv: env.ShareEnv.BaseEnv,
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
		if product.Render != nil {
			revision = product.Render.Revision
			if product.Render.Name != renderName {
				renderName = product.Render.Name
			} else {
				for _, productSvc := range product.GetServiceMap() {
					if productSvc.Render != nil {
						revision = productSvc.Render.Revision
						break
					}
				}
			}
		}

		renderSet, err := commonservice.GetRenderSet(renderName, revision, false, product.EnvName, log)
		if err != nil {
			log.Errorf("Failed to find render set, productName: %s, namespace: %s,  err: %s", product.ProductName, product.Namespace, err)
			return e.ErrGetRenderSet.AddDesc(err.Error())
		}

		product.Vars = renderSet.KVs[:]

		// Note. the service property of kv pair stored in DB is not accuracy
		// from v1.14.0 we should fetch related service data real-time
		if len(product.Vars) == 0 {
			return nil
		}

		templateSvcsOfProduct, err := commonservice.GetProductUsedTemplateSvcs(product)
		if err != nil {
			log.Errorf("failed to get service templates applied in product, err: %s", err)
			return nil
		}

		renderKvs, err := commonservice.ListRenderKeysByTemplateSvc(templateSvcsOfProduct, log)
		if err != nil {
			log.Errorf("failed to get render kvs in product, err: %s", err)
			return nil
		}

		relatedSvcs := make(map[string][]string)
		for _, key := range renderKvs {
			relatedSvcs[key.Key] = key.Services
		}
		for _, varInfo := range product.Vars {
			varInfo.Services = relatedSvcs[varInfo.Key]
		}
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

type UpdateEnv struct {
	EnvName      string               `json:"env_name"`
	ServiceNames []string             `json:"service_names"`
	UpdateType   string               `json:"update_type,omitempty"`
	Vars         []*template.RenderKV `json:"vars,omitempty"`
}

func AutoUpdateProduct(args []*UpdateEnv, envNames []string, productName, requestID string, force bool, log *zap.SugaredLogger) ([]*EnvStatus, error) {
	mutexAutoUpdate.Lock()
	defer func() {
		mutexAutoUpdate.Unlock()
	}()

	envStatuses := make([]*EnvStatus, 0)

	project, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		return nil, err
	}

	if !force && project.ProductFeature != nil && project.ProductFeature.BasicFacility != setting.BasicFacilityCVM {
		modifiedByENV := make(map[string][]*serviceInfo)
		for _, arg := range args {
			p, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: arg.EnvName})
			if err != nil {
				log.Errorf("Failed to get product %s in %s, error: %v", productName, arg.EnvName, err)
				continue
			}

			kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), p.ClusterID)
			if err != nil {
				log.Errorf("Failed to get kube client for %s, error: %v", productName, err)
				continue
			}

			modifiedServices := getModifiedServiceFromObjectMetaList(kube.GetDirtyResources(p.Namespace, kubeClient))
			var specifyModifiedServices []*serviceInfo
			for _, modifiedService := range modifiedServices {
				if util.InStringArray(modifiedService.Name, arg.ServiceNames) {
					specifyModifiedServices = append(specifyModifiedServices, modifiedService)
				}
			}
			if len(specifyModifiedServices) > 0 {
				modifiedByENV[arg.EnvName] = specifyModifiedServices
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

	for _, arg := range args {
		err = UpdateProductV2(arg.EnvName, productName, setting.SystemUser, requestID, arg.ServiceNames, force, arg.Vars, log)
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

func UpdateProduct(serviceNames []string, existedProd, updateProd *commonmodels.Product, renderSet *commonmodels.RenderSet, log *zap.SugaredLogger) (err error) {
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
	if !prodRevs.Updatable {
		log.Errorf("[%s][P:%s] nothing to update", envName, productName)
		return
	}

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
		if serviceRev.Updatable && serviceRev.Deleted && util.InStringArray(serviceRev.ServiceName, serviceNames) {
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

	// 转化prodRevs.ServiceRevisions为serviceName+serviceType:serviceRev的map
	// 不在遍历到每个服务时再次进行遍历
	serviceRevisionMap := getServiceRevisionMap(prodRevs.ServiceRevisions)

	// 首先更新一次数据库，将产品模板的最新编排更新到数据库
	// 只更新编排，不更新服务revision等信息
	updatedServices := getUpdatedProductServices(updateProd, serviceRevisionMap, existedProd)

	updateProd.Status = setting.ProductStatusUpdating
	updateProd.Services = updatedServices
	updateProd.ShareEnv = existedProd.ShareEnv

	if err := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUpdating); err != nil {
		log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.UpdateEnvStatusErrMsg)
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

				service := &commonmodels.ProductService{
					ServiceName: svcRev.ServiceName,
					ProductName: prodService.ProductName,
					Type:        svcRev.Type,
					Revision:    svcRev.NextRevision,
				}

				service.Containers = svcRev.Containers
				service.Render = updateProd.Render

				if svcRev.Type == setting.K8SDeployType && util.InStringArray(service.ServiceName, serviceNames) {
					log.Infof("[Namespace:%s][Product:%s][Service:%s][IsNew:%v] upsert service",
						envName, productName, svcRev.ServiceName, svcRev.New)
					wg.Add(1)
					go func() {
						defer wg.Done()

						_, err := upsertService(
							existedServices[service.ServiceName] != nil,
							updateProd,
							service,
							existedServices[service.ServiceName],
							renderSet, inf, kubeClient, istioClient, log)
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
		//merge new and old services
		var updateGroup []*commonmodels.ProductService
		newServiceMap := make(map[string]*commonmodels.ProductService)
		for _, service := range groupServices {
			newServiceMap[service.ServiceName] = service
		}
		oldServiceMap := make(map[string]*commonmodels.ProductService)
		for _, existedGroupServices := range existedProd.Services {
			for _, service := range existedGroupServices {
				if util.InStringArray(service.ServiceName, deletedServices) {
					continue
				}
				oldServiceMap[service.ServiceName] = service
				if newService, ok := newServiceMap[service.ServiceName]; ok {
					if util.InStringArray(service.ServiceName, serviceNames) {
						updateGroup = append(updateGroup, newService)
					} else {
						updateGroup = append(updateGroup, service)
					}
				}
			}
		}
		for _, newService := range groupServices {
			if _, ok := oldServiceMap[newService.ServiceName]; !ok && !util.InStringArray(newService.ServiceName, deletedServices) && util.InStringArray(newService.ServiceName, serviceNames) {
				updateGroup = append(updateGroup, newService)
			}
		}

		err = commonrepo.NewProductColl().UpdateGroup(envName, productName, groupIndex, updateGroup)
		if err != nil {
			log.Errorf("Failed to update collection - service group %d. Error: %v", groupIndex, err)
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

func UpdateProductV2(envName, productName, user, requestID string, serviceNames []string, force bool, kvs []*templatemodels.RenderKV, log *zap.SugaredLogger) (err error) {
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
	project, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		return err
	}
	if !force && project.ProductFeature != nil && project.ProductFeature.BasicFacility != setting.BasicFacilityCVM {
		modifiedServices := getModifiedServiceFromObjectMetaList(kube.GetDirtyResources(exitedProd.Namespace, kubeClient))
		var specifyModifiedServices []*serviceInfo
		for _, modifiedService := range modifiedServices {
			if util.InStringArray(modifiedService.Name, serviceNames) {
				specifyModifiedServices = append(specifyModifiedServices, modifiedService)
			}
		}
		if len(specifyModifiedServices) > 0 {
			data, err := json.Marshal(specifyModifiedServices)
			if err != nil {
				log.Errorf("Marshal failure: %v", err)
			}
			log.Errorf("the following services are modified since last update: %s", data)
			return fmt.Errorf("the following services are modified since last update: %s", data)
		}
	}

	//TODO:The host update environment cannot remove deleted services
	if !force && project.ProductFeature != nil && project.ProductFeature.BasicFacility == setting.BasicFacilityCVM {
		services, err := commonrepo.NewServiceColl().ListMaxRevisionsAllSvcByProduct(productName)
		if err != nil && !commonrepo.IsErrNoDocuments(err) {
			log.Errorf("ListMaxRevisionsAllSvcByProduct: %s", err)
			return fmt.Errorf("ListMaxRevisionsAllSvcByProduct: %s", err)
		}
		if services != nil {
			svcNames := make([]string, len(services))
			for _, svc := range services {
				svcNames = append(svcNames, svc.ServiceName)
			}
			serviceNames = svcNames
		}
	}

	if project.ProductFeature != nil && project.ProductFeature.BasicFacility != setting.BasicFacilityCVM {
		err = ensureKubeEnv(exitedProd.Namespace, exitedProd.RegistryID, map[string]string{setting.ProductLabel: project.ProductName}, exitedProd.ShareEnv.Enable, kubeClient, log)

		if err != nil {
			log.Errorf("[%s][P:%s] service.UpdateProductV2 create kubeEnv error: %v", envName, productName, err)
			return err
		}

		err = commonservice.CreateRenderSetByMerge(
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
	}

	// 检查renderinfo是否为空(适配历史product)
	if exitedProd.Render == nil {
		exitedProd.Render = &commonmodels.RenderInfo{ProductTmpl: exitedProd.ProductName}
	}

	// 检查renderset是否覆盖产品所有key
	renderSet, err := commonservice.ValidateRenderSet(exitedProd.ProductName, exitedProd.Render.Name, exitedProd.EnvName, nil, log)
	if err != nil {
		log.Errorf("[%s][P:%s] validate product renderset error: %v", envName, exitedProd.ProductName, err)
		return e.ErrUpdateEnv.AddDesc(err.Error())
	}

	log.Infof("[%s][P:%s] UpdateProduct", envName, productName)

	// 查找产品模板
	updateProd, err := GetInitProduct(productName, types.GeneralEnv, false, "", log)
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
		err := UpdateProduct(serviceNames, exitedProd, updateProd, renderSet, log)
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

// fill product services and chart infos and insert renderset data
func prepareHelmProductCreation(templateProduct *templatemodels.Product, productObj *commonmodels.Product, arg *CreateHelmProductArg, serviceTmplMap map[string]*commonmodels.Service, log *zap.SugaredLogger) error {
	err := validateArgs(arg.ValuesData)
	if err != nil {
		return fmt.Errorf("failed to validate args: %s", err)
	}

	productObj.ChartInfos = make([]*templatemodels.RenderChart, 0)
	// chart infos in template product
	templateChartInfoMap := make(map[string]*templatemodels.RenderChart)
	for _, tc := range templateProduct.ChartInfos {
		templateChartInfoMap[tc.ServiceName] = tc
	}

	// user custom chart values
	cvMap := make(map[string]*templatemodels.RenderChart)
	for _, singleCV := range arg.ChartValues {
		tc, ok := templateChartInfoMap[singleCV.ServiceName]
		if !ok {
			return fmt.Errorf("failed to find chart info in product, serviceName: %s productName: %s", singleCV.ServiceName, templateProduct.ProjectName)
		}
		chartInfo := &templatemodels.RenderChart{}
		singleCV.FillRenderChartModel(chartInfo, tc.ChartVersion)
		chartInfo.ValuesYaml = tc.ValuesYaml
		productObj.ChartInfos = append(productObj.ChartInfos, chartInfo)
		cvMap[singleCV.ServiceName] = chartInfo
	}

	// default values
	defaultValuesYaml := arg.DefaultValues

	// generate service group data
	var serviceGroup [][]*commonmodels.ProductService
	for _, names := range templateProduct.Services {
		servicesResp := make([]*commonmodels.ProductService, 0)
		for _, serviceName := range names {
			// only the services chosen by use can be applied into product
			rc, ok := cvMap[serviceName]
			if !ok {
				continue
			}

			serviceTmpl, ok := serviceTmplMap[serviceName]
			if !ok {
				return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to find service info in template_service, serviceName: %s", serviceName))
			}

			serviceResp := &commonmodels.ProductService{
				ServiceName: serviceTmpl.ServiceName,
				ProductName: serviceTmpl.ProductName,
				Type:        serviceTmpl.Type,
				Revision:    serviceTmpl.Revision,
			}
			serviceResp.Containers = make([]*commonmodels.Container, 0)
			var err error
			for _, c := range serviceTmpl.Containers {
				image := c.Image
				image, err = genImageFromYaml(c, rc.ValuesYaml, defaultValuesYaml, rc.GetOverrideYaml(), rc.OverrideValues)
				if err != nil {
					errMsg := fmt.Sprintf("genImageFromYaml product template %s,service name:%s,error:%s", productObj.ProductName, rc.ServiceName, err)
					log.Error(errMsg)
					return e.ErrCreateEnv.AddDesc(errMsg)
				}
				container := &commonmodels.Container{
					Name:      c.Name,
					ImageName: util.GetImageNameFromContainerInfo(c.ImageName, c.Name),
					Image:     image,
					ImagePath: c.ImagePath,
				}
				serviceResp.Containers = append(serviceResp.Containers, container)
			}
			servicesResp = append(servicesResp, serviceResp)
		}
		serviceGroup = append(serviceGroup, servicesResp)
	}
	productObj.Services = serviceGroup

	// insert renderset info into db
	err = commonservice.CreateHelmRenderSet(&commonmodels.RenderSet{
		Name:          commonservice.GetProductEnvNamespace(arg.EnvName, arg.ProductName, arg.Namespace),
		EnvName:       arg.EnvName,
		ProductTmpl:   arg.ProductName,
		UpdateBy:      productObj.UpdateBy,
		IsDefault:     false,
		DefaultValues: arg.DefaultValues,
		ChartInfos:    productObj.ChartInfos,
		YamlData:      geneYamlData(arg.ValuesData),
	}, log)
	if err != nil {
		log.Errorf("rennderset create fail when copy creating helm product, productName: %s,envname:%s,err:%s", arg.ProductName, arg.EnvName, err)
		return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to save chart values, productName: %s,envname:%s,err:%s", arg.ProductName, arg.EnvName, err))
	}
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

	// fill all chart infos from product renderset
	err = commonservice.FillProductTemplateValuesYamls(templateProduct, log)
	if err != nil {
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	// prepare data
	serviceTmpls, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(productName)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}
	templateServiceMap := make(map[string]*commonmodels.Service)
	for _, svc := range serviceTmpls {
		templateServiceMap[svc.ServiceName] = svc
	}

	errList := new(multierror.Error)
	for _, arg := range args {
		dataValid := true
		for _, cv := range arg.ChartValues {
			if _, ok := templateServiceMap[cv.ServiceName]; !ok {
				dataValid = false
				errList = multierror.Append(errList, fmt.Errorf("failed to find service tempalte, serviceName: %s", cv.ServiceName))
				break
			}
		}
		if !dataValid {
			continue
		}
		err = createSingleHelmProduct(templateProduct, requestID, userName, arg.RegistryID, arg, templateServiceMap, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}
	return errList.ErrorOrNil()
}

func createSingleHelmProduct(templateProduct *templatemodels.Product, requestID, userName, registryID string, arg *CreateHelmProductArg, serviceTmplMap map[string]*commonmodels.Service, log *zap.SugaredLogger) error {
	productObj := &commonmodels.Product{
		ProductName:     templateProduct.ProductName,
		Revision:        1,
		Enabled:         false,
		EnvName:         arg.EnvName,
		UpdateBy:        userName,
		IsPublic:        true,
		ClusterID:       arg.ClusterID,
		Namespace:       commonservice.GetProductEnvNamespace(arg.EnvName, arg.ProductName, arg.Namespace),
		Source:          setting.SourceFromHelm,
		IsOpenSource:    templateProduct.IsOpensource,
		IsForkedProduct: false,
		RegistryID:      registryID,
		IsExisted:       arg.IsExisted,
		EnvConfigs:      arg.EnvConfigs,
		ShareEnv:        arg.ShareEnv,
	}

	// fill services and chart infos of product
	err := prepareHelmProductCreation(templateProduct, productObj, arg, serviceTmplMap, log)
	if err != nil {
		return err
	}
	return CreateProduct(userName, requestID, productObj, log)
}

type YamlProductItem struct {
	OldName  string                     `json:"old_name"`
	NewName  string                     `json:"new_name"`
	BaseName string                     `json:"base_name"`
	Vars     []*templatemodels.RenderKV `json:"vars"`
}
type CopyYamlProductArg struct {
	Items []YamlProductItem `json:"items"`
}

type HelmProductItem struct {
	OldName       string                          `json:"old_name"`
	NewName       string                          `json:"new_name"`
	BaseName      string                          `json:"base_name"`
	DefaultValues string                          `json:"default_values"`
	ChartValues   []*commonservice.RenderChartArg `json:"chart_values"`
	ValuesData    *commonservice.ValuesDataArgs   `json:"values_data"`
}

type CopyHelmProductArg struct {
	Items []HelmProductItem
}

func BulkCopyHelmProduct(projectName, user, requestID string, arg CopyHelmProductArg, log *zap.SugaredLogger) error {
	if len(arg.Items) == 0 {
		return nil
	}
	var envs []string
	for _, item := range arg.Items {
		envs = append(envs, item.OldName)
	}
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:   projectName,
		InEnvs: envs,
	})
	if err != nil {
		return err
	}
	productMap := make(map[string]*commonmodels.Product)
	for _, product := range products {
		productMap[product.EnvName] = product
	}
	var args []*CreateHelmProductArg
	for _, item := range arg.Items {
		if item.OldName == item.NewName {
			continue
		}
		if product, ok := productMap[item.OldName]; ok {
			args = append(args, &CreateHelmProductArg{
				ProductName:   projectName,
				EnvName:       item.NewName,
				Namespace:     projectName + "-" + "env" + "-" + item.NewName,
				ClusterID:     product.ClusterID,
				DefaultValues: item.DefaultValues,
				RegistryID:    product.RegistryID,
				BaseEnvName:   product.BaseName,
				BaseName:      item.BaseName,
				ChartValues:   item.ChartValues,
				ValuesData:    item.ValuesData,
			})
		} else {
			return fmt.Errorf("product:%s not exist", item.OldName)
		}
	}
	return CopyHelmProduct(projectName, user, requestID, args, log)
}

func BulkCopyYamlProduct(projectName, user, requestID string, arg CopyYamlProductArg, log *zap.SugaredLogger) error {
	if len(arg.Items) == 0 {
		return nil
	}

	var envs []string
	for _, item := range arg.Items {
		envs = append(envs, item.OldName)
	}
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:   projectName,
		InEnvs: envs,
	})
	if err != nil {
		return err
	}
	productMap := make(map[string]*commonmodels.Product)
	for _, product := range products {
		productMap[product.EnvName] = product
	}

	for _, item := range arg.Items {
		if item.OldName == item.NewName {
			continue
		}
		if product, ok := productMap[item.OldName]; ok {
			newProduct := *product
			newProduct.EnvName = item.NewName
			newProduct.Vars = item.Vars
			newProduct.Namespace = projectName + "-env-" + newProduct.EnvName
			newProduct.Render.Name = newProduct.Namespace
			util.Clear(&newProduct.ID)
			newProduct.Render.Revision = 0
			newProduct.BaseName = item.BaseName
			err = CreateProduct(user, requestID, &newProduct, log)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("product:%s not exist", item.OldName)
		}
	}
	return nil
}

// CopyYamlProduct copy product from source product
func CopyYamlProduct(user, requestID string, args *commonmodels.Product, log *zap.SugaredLogger) (err error) {
	if len(args.BaseEnvName) == 0 {
		return e.ErrCreateEnv.AddDesc("base environment name can't be nil")
	}
	baseProject, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: args.BaseEnvName,
	})
	if err != nil {
		return e.ErrCreateEnv.AddErr(fmt.Errorf("failed to find base environment: %s, err: %s", args.EnvName, err))
	}

	// use service revision defined in base environment
	servicesInBaseNev := baseProject.GetServiceMap()

	for _, svcLists := range args.Services {
		for _, svc := range svcLists {
			if baseSvc, ok := servicesInBaseNev[svc.ServiceName]; ok {
				svc.Revision = baseSvc.Revision
				svc.ProductName = baseSvc.ProductName
			}
		}
	}
	return CreateProduct(user, requestID, args, log)
}

// CreateProduct create a new product with its dependent stacks
func CreateProduct(user, requestID string, args *commonmodels.Product, log *zap.SugaredLogger) (err error) {
	log.Infof("[%s][P:%s] CreateProduct", args.EnvName, args.ProductName)
	creator := getCreatorBySource(args.Source)
	return creator.Create(user, requestID, args, log)
}

func CopyHelmProduct(productName, userName, requestID string, args []*CreateHelmProductArg, log *zap.SugaredLogger) error {
	errList := new(multierror.Error)
	templateProduct, err := templaterepo.NewProductColl().Find(productName)
	if err != nil || templateProduct == nil {
		if err != nil {
			log.Errorf("failed to query product %s, err %s ", productName, err.Error())
		}
		return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to query product %s ", productName))
	}

	// fill all chart infos from product renderset
	err = commonservice.FillProductTemplateValuesYamls(templateProduct, log)
	if err != nil {
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	for _, arg := range args {
		baseProduct, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:    productName,
			EnvName: arg.BaseName,
		})
		if err != nil {
			errList = multierror.Append(errList, fmt.Errorf("failed to query base product info name :%s,envname:%s", productName, arg.BaseEnvName))
			continue
		}
		templateSvcs, err := commonservice.GetProductUsedTemplateSvcs(baseProduct)
		templateServiceMap := make(map[string]*commonmodels.Service)
		for _, svc := range templateSvcs {
			templateServiceMap[svc.ServiceName] = svc
		}

		//services deployed in base product may be different with services in template product
		//use services in base product when copying product instead of services in template product
		svcGroups := make([][]string, 0)
		for _, svcList := range baseProduct.Services {
			svcs := make([]string, 0)
			for _, svc := range svcList {
				svcs = append(svcs, svc.ServiceName)
			}
			svcGroups = append(svcGroups, svcs)
		}
		templateProduct.Services = svcGroups

		err = copySingleHelmProduct(templateProduct, baseProduct, requestID, userName, arg, templateServiceMap, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}
	return errList.ErrorOrNil()
}

func copySingleHelmProduct(templateProduct *templatemodels.Product, productInfo *commonmodels.Product, requestID, userName string, arg *CreateHelmProductArg, serviceTmplMap map[string]*commonmodels.Service, log *zap.SugaredLogger) error {
	sourceRendersetName := productInfo.Namespace
	productInfo.ID = primitive.NilObjectID
	productInfo.Revision = 1
	productInfo.EnvName = arg.EnvName
	productInfo.UpdateBy = userName
	productInfo.ClusterID = arg.ClusterID
	productInfo.BaseName = arg.BaseName
	productInfo.Namespace = commonservice.GetProductEnvNamespace(arg.EnvName, arg.ProductName, arg.Namespace)
	productInfo.EnvConfigs = arg.EnvConfigs

	// merge chart infos, use chart info in product to override charts in template_project
	sourceRenderSet, _, err := commonrepo.NewRenderSetColl().FindRenderSet(&commonrepo.RenderSetFindOption{
		Name:        sourceRendersetName,
		EnvName:     arg.BaseName,
		ProductTmpl: arg.ProductName,
	})
	if err != nil {
		return fmt.Errorf("failed to find source renderset: %s, err: %s", productInfo.Namespace, err)
	}
	sourceChartMap := make(map[string]*templatemodels.RenderChart)
	for _, singleChart := range sourceRenderSet.ChartInfos {
		sourceChartMap[singleChart.ServiceName] = singleChart
	}
	templateCharts := templateProduct.ChartInfos
	templateProduct.ChartInfos = make([]*templatemodels.RenderChart, 0)
	for _, chart := range templateCharts {
		if chartFromSource, ok := sourceChartMap[chart.ServiceName]; ok {
			templateProduct.ChartInfos = append(templateProduct.ChartInfos, chartFromSource)
		} else {
			templateProduct.ChartInfos = append(templateProduct.ChartInfos, chart)
		}
	}

	// fill services and chart infos of product
	err = prepareHelmProductCreation(templateProduct, productInfo, arg, serviceTmplMap, log)
	if err != nil {
		return err
	}

	// clear render info
	productInfo.Render = nil
	setServiceRender(productInfo)

	// insert renderset info into db
	if len(productInfo.ChartInfos) > 0 {
		err := commonservice.CreateHelmRenderSet(&commonmodels.RenderSet{
			Name:          commonservice.GetProductEnvNamespace(arg.EnvName, arg.ProductName, arg.Namespace),
			EnvName:       arg.EnvName,
			ProductTmpl:   arg.ProductName,
			UpdateBy:      userName,
			IsDefault:     false,
			DefaultValues: arg.DefaultValues,
			YamlData:      geneYamlData(arg.ValuesData),
			ChartInfos:    productInfo.ChartInfos,
		}, log)
		if err != nil {
			log.Errorf("rennderset create fail when copy creating helm product, productName: %s,envname:%s,err:%s", arg.ProductName, arg.EnvName, err)
			return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to save chart values, productName: %s,envname:%s,err:%s", arg.ProductName, arg.EnvName, err))
		}
	}
	return CreateProduct(userName, requestID, productInfo, log)
}

func UpdateProductRecycleDay(envName, productName string, recycleDay int) error {
	return commonrepo.NewProductColl().UpdateProductRecycleDay(envName, productName, recycleDay)
}

func buildContainerMap(cs []*models.Container) map[string]*models.Container {
	containerMap := make(map[string]*models.Container)
	for _, c := range cs {
		containerMap[c.Name] = c
	}
	return containerMap
}

func UpdateHelmProduct(productName, envName, username, requestID string, overrideCharts []*commonservice.RenderChartArg, deletedServices []string, log *zap.SugaredLogger) error {
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
		err := updateProductGroup(username, productName, envName, productResp, overrideCharts, deletedSvcRevision, log)
		if err != nil {
			log.Errorf("[%s][P:%s] failed to update product %#v", envName, productName, err)
			// 发送更新产品失败消息给用户
			title := fmt.Sprintf("更新 [%s] 的 [%s] 环境失败", productName, envName)
			commonservice.SendErrorMessage(username, title, requestID, err, log)

			log.Infof("[%s][P:%s] update error to => %s", envName, productName, err)
			productResp.Status = setting.ProductStatusFailed
			if err = commonrepo.NewProductColl().UpdateStatusAndError(envName, productName, productResp.Status, err.Error()); err != nil {
				log.Errorf("[%s][%s] Product.Update error: %v", envName, productName, err)
				return
			}
		} else {
			productResp.Status = setting.ProductStatusSuccess
			if err = commonrepo.NewProductColl().UpdateStatusAndError(envName, productName, productResp.Status, ""); err != nil {
				log.Errorf("[%s][%s] Product.Update error: %v", envName, productName, err)
				return
			}
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
// return [need-Redeploy] and [need-SaveToDB]
func checkOverrideValuesChange(source *templatemodels.RenderChart, args *commonservice.RenderChartArg) (bool, bool) {
	sourceArg := &commonservice.RenderChartArg{}
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

func UpdateHelmProductDefaultValues(productName, envName, userName, requestID string, args *EnvRendersetArg, log *zap.SugaredLogger) error {
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

	err = UpdateHelmProductDefaultValuesWithRender(productRenderset, userName, requestID, args, log)
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

func UpdateHelmProductDefaultValuesWithRender(productRenderset *models.RenderSet, userName, requestID string, args *EnvRendersetArg, log *zap.SugaredLogger) error {
	equal, err := yamlutil.Equal(productRenderset.DefaultValues, args.DefaultValues)
	if err != nil {
		return fmt.Errorf("failed to unmarshal default values in renderset, err: %s", err)
	}
	productRenderset.DefaultValues = args.DefaultValues
	productRenderset.YamlData = geneYamlData(args.ValuesData)
	updatedRcList := make([]*templatemodels.RenderChart, 0)
	if !equal {
		for _, curRenderChart := range productRenderset.ChartInfos {
			updatedRcList = append(updatedRcList, curRenderChart)
		}
	}
	return UpdateHelmProductVariable(productRenderset.ProductTmpl, productRenderset.EnvName, userName, requestID, updatedRcList, productRenderset, log)
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

	requestValueMap := make(map[string]*commonservice.RenderChartArg)
	for _, arg := range args.ChartValues {
		requestValueMap[arg.ServiceName] = arg
	}

	valuesInRenderset := make(map[string]*templatemodels.RenderChart)
	for _, rc := range productRenderset.ChartInfos {
		valuesInRenderset[rc.ServiceName] = rc
	}

	updatedRcMap := make(map[string]*templatemodels.RenderChart)
	changedCharts := make([]*commonservice.RenderChartArg, 0)

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

	rcList := make([]*templatemodels.RenderChart, 0)
	for _, rc := range updatedRcMap {
		rcList = append(rcList, rc)
	}

	return UpdateHelmProductVariable(productName, envName, userName, requestID, rcList, productRenderset, log)
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

	updatedRCMap := make(map[string]*templatemodels.RenderChart)

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
	updatedRcList := make([]*templatemodels.RenderChart, 0)
	for _, updatedRc := range updatedRCMap {
		updatedRcList = append(updatedRcList, updatedRc)
	}

	err = UpdateHelmProductVariable(productName, envName, "cron", requestID, updatedRcList, productRenderset, log)
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
	updatedRcList := make([]*templatemodels.RenderChart, 0)
	updatedRCMap := make(map[string]*templatemodels.RenderChart)

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

	err = UpdateHelmProductVariable(productName, envName, userName, requestID, updatedRcList, productRenderset, log)
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

func UpdateHelmProductVariable(productName, envName, username, requestID string, updatedRcs []*templatemodels.RenderChart, renderset *commonmodels.RenderSet, log *zap.SugaredLogger) error {
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
			YamlData:      renderset.YamlData,
			ChartInfos:    renderset.ChartInfos,
		},
		log,
	); err != nil {
		log.Errorf("[%s][P:%s] create renderset error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.FindProductTmplErrMsg)
	}

	// update render info of product
	if productResp.Render == nil {
		productResp.Render = &commonmodels.RenderInfo{ProductTmpl: productResp.ProductName}
	}
	renderSet, err := FindHelmRenderSet(productResp.ProductName, productResp.Namespace, envName, log)
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
	if len(updatedRcs) == 0 {
		return nil
	}

	return updateHelmProductVariable(productResp, renderSet, oldRenderVersion, username, requestID, log)
}

func updateHelmProductVariable(productResp *commonmodels.Product, renderset *commonmodels.RenderSet, oldRenderVersion int64, userName, requestID string, log *zap.SugaredLogger) error {
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
		err := proceedHelmRelease(productName, envName, productResp, renderset, helmClient, nil, log)
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

		err = UpdateHelmProduct(productName, envName, userName, requestID, args.ChartValues, args.DeletedServices, log)
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
	prod.ChartInfos = renderSet.ChartInfos

	return prod, nil
}

func GetHelmChartVersions(productName, envName string, log *zap.SugaredLogger) ([]*commonmodels.HelmVersions, error) {
	var (
		helmVersions = make([]*commonmodels.HelmVersions, 0)
		chartInfoMap = make(map[string]*templatemodels.RenderChart)
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

			if isDelete {
				if hc, errHelmClient := helmtool.NewClientFromRestConf(restConfig, productInfo.Namespace); errHelmClient == nil {
					for _, service := range productInfo.GetServiceMap() {
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
		if tempProduct.ProductFeature != nil && tempProduct.ProductFeature.CreateEnvType == setting.SourceFromExternal {
			workloadStat, err := commonrepo.NewWorkLoadsStatColl().Find(productInfo.ClusterID, productInfo.Namespace)
			if err != nil {
				log.Errorf("workflowStat not found error:%s", err)
			}
			if workloadStat != nil {
				workloadStat.Workloads = commonservice.FilterWorkloadsByEnv(workloadStat.Workloads, productInfo.EnvName)
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
			if isDelete {
				// Delete Cluster level resources
				err = commonservice.DeleteClusterResource(labels.Set{setting.ProductLabel: productName, setting.EnvNameLabel: envName}.AsSelector(), productInfo.ClusterID, log)
				if err != nil {
					err = e.ErrDeleteProduct.AddDesc(e.DeleteServiceContainerErrMsg + ": " + err.Error())
					return
				}

				// Handles environment sharing related operations.
				err = EnsureDeleteShareEnvConfig(ctx, productInfo, istioClient)
				if err != nil {
					log.Errorf("Failed to delete share env config: %s", err)
					err = e.ErrDeleteProduct.AddDesc(e.DeleteVirtualServiceErrMsg + ": " + err.Error())
					return
				}

				s := labels.Set{setting.EnvCreatedBy: setting.EnvCreator}.AsSelector()
				if err1 := commonservice.DeleteNamespaceIfMatch(productInfo.Namespace, s, productInfo.ClusterID, log); err1 != nil {
					err = e.ErrDeleteEnv.AddDesc(e.DeleteNamespaceErrMsg + ": " + err1.Error())
					return
				}
			}
			err = commonrepo.NewProductColl().Delete(envName, productName)
			if err != nil {
				log.Errorf("Product.Delete error: %v", err)
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
	renderset, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		Name:        productInfo.Namespace,
		EnvName:     productInfo.EnvName,
		ProductTmpl: productInfo.ProductName,
	})
	if err != nil {
		log.Errorf("get renderSet error: %v", err)
		return err
	}
	rcs := make([]*template.RenderChart, 0)
	for _, v := range renderset.ChartInfos {
		if !deleteServiceSet.Has(v.ServiceName) {
			rcs = append(rcs, v)
		}
	}
	renderset.ChartInfos = rcs

	// create new renderset
	if err := commonservice.CreateHelmRenderSet(renderset, log); err != nil {
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
	rs, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		EnvName:     productInfo.EnvName,
		Name:        productInfo.Namespace,
		ProductTmpl: productInfo.ProductName,
	})
	if err != nil {
		log.Errorf("get renderSet error: %v", err)
		return err
	}
	var updatedKVs []*templatemodels.RenderKV
	for _, v := range rs.KVs {
		var updatedServices []string
		for _, service := range v.Services {
			if !util.InStringArray(service, serviceNames) {
				updatedServices = append(updatedServices, service)
			}
		}
		v.Services = updatedServices
		updatedKVs = append(updatedKVs, v)
	}
	rs.KVs = updatedKVs
	err = commonrepo.NewRenderSetColl().Update(rs)
	if err != nil {
		log.Errorf("update renderSet error: %v", err)
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
	renderChartInEnv, _, err := commonservice.GetRenderCharts(productName, envName, serviceNameListStr, log)
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

func createGroups(envName, user, requestID string, args *commonmodels.Product, eventStart int64, renderSet *commonmodels.RenderSet, informer informers.SharedInformerFactory, kubeClient client.Client, istioClient versionedclient.Interface, log *zap.SugaredLogger) {
	var err error
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

		if err := commonrepo.NewProductColl().UpdateStatus(envName, args.ProductName, status); err != nil {
			log.Errorf("[%s][P:%s] Product.UpdateStatus error: %s", envName, args.ProductName, err)
			return
		}
		if err := commonrepo.NewProductColl().UpdateErrors(envName, args.ProductName, errorMsg); err != nil {
			log.Errorf("[%s][P:%s] Product.UpdateErrors error: %s", envName, args.ProductName, err)
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
		err = envHandleFunc(getProjectType(args.ProductName), log).createGroup(envName, args.ProductName, user, group, renderSet, informer, kubeClient)
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

// upsertService 创建或者更新服务, 更新服务之前先创建服务需要的配置
func upsertService(isUpdate bool, env *commonmodels.Product,
	service *commonmodels.ProductService, prevSvc *commonmodels.ProductService,
	renderSet *commonmodels.RenderSet, informer informers.SharedInformerFactory, kubeClient client.Client, istioClient versionedclient.Interface, log *zap.SugaredLogger,
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
	kubeClient client.Client,
	log *zap.SugaredLogger,
) error {
	opt := &commonrepo.RenderSetFindOption{
		Name:        oldService.Render.Name,
		Revision:    oldService.Render.Revision,
		EnvName:     env.EnvName,
		ProductTmpl: env.ProductName,
	}
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

func FindHelmRenderSet(productName, renderName, envName string, log *zap.SugaredLogger) (*commonmodels.RenderSet, error) {
	resp := &commonmodels.RenderSet{ProductTmpl: productName}
	var err error
	if renderName != "" {
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
	}
	return resp, nil
}

func buildInstallParam(namespace, envName, defaultValues string, renderChart *templatemodels.RenderChart, serviceObj *commonmodels.Service) (*ReleaseInstallParam, error) {
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

func installProductHelmCharts(user, envName, requestID string, args *commonmodels.Product, renderset *commonmodels.RenderSet, eventStart int64, helmClient *helmtool.HelmClient,
	kclient client.Client, istioClient versionedclient.Interface, log *zap.SugaredLogger) {
	var (
		err     error
		errList = &multierror.Error{}
	)

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

	chartInfoMap := make(map[string]*templatemodels.RenderChart)
	for _, renderChart := range args.ChartInfos {
		chartInfoMap[renderChart.ServiceName] = renderChart
	}

	err = proceedHelmRelease(args.ProductName, args.EnvName, args, renderset, helmClient, nil, log)
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

func setServiceRender(args *commonmodels.Product) {
	for _, serviceGroup := range args.Services {
		for _, service := range serviceGroup {
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

func updateProductGroup(username, productName, envName string, productResp *commonmodels.Product,
	overrideCharts []*commonservice.RenderChartArg, deletedSvcRevision map[string]int64, log *zap.SugaredLogger) error {

	helmClient, err := helmtool.NewClientFromNamespace(productResp.ClusterID, productResp.Namespace)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	// uninstall services
	for serviceName, serviceRevision := range deletedSvcRevision {
		if err = UninstallServiceByName(helmClient, serviceName, productResp, serviceRevision, true); err != nil {
			log.Errorf("UninstallRelease err:%v", err)
			return e.ErrUpdateEnv.AddErr(err)
		}
	}

	renderSet, err := diffRenderSet(username, productName, envName, productResp, overrideCharts, log)
	if err != nil {
		return e.ErrUpdateEnv.AddDesc("对比环境中的value.yaml和系统默认的value.yaml失败")
	}

	productResp.ChartInfos = renderSet.ChartInfos
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
	if err = commonrepo.NewProductColl().Update(productResp); err != nil {
		log.Errorf("Failed to update env, err: %s", err)
		return err
	}

	err = proceedHelmRelease(productName, envName, productResp, renderSet, helmClient, filter, log)
	if err != nil {
		log.Errorf("error occurred when upgrading services in env: %s/%s, err: %s ", productName, envName, err)
		return err
	}

	return nil
}

// diffRenderSet get diff between renderset in product and product template
// generate a new renderset and insert into db
func diffRenderSet(username, productName, envName string, productResp *commonmodels.Product, overrideCharts []*commonservice.RenderChartArg, log *zap.SugaredLogger) (*commonmodels.RenderSet, error) {
	// default renderset
	latestRenderSet, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{Name: productName, IsDefault: true})
	if err != nil {
		log.Errorf("[RenderSet.find] err: %v", err)
		return nil, err
	}

	// chart infos in template
	latestChartInfoMap := make(map[string]*templatemodels.RenderChart)
	for _, renderInfo := range latestRenderSet.ChartInfos {
		latestChartInfoMap[renderInfo.ServiceName] = renderInfo
	}

	// chart infos from client
	renderChartArgMap := make(map[string]*commonservice.RenderChartArg)
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
	currentChartInfoMap := make(map[string]*templatemodels.RenderChart)
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
	newChartInfos := make([]*templatemodels.RenderChart, 0)

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

	if err = commonservice.CreateHelmRenderSet(
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

	renderSet, err := FindHelmRenderSet(productName, productResp.Render.Name, envName, log)
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

// find related keys of service modules when container image changed in env since imported
func getImageChangeInvolvedKeys(prodService *commonmodels.ProductService, revisionSvc, latestSvc *commonmodels.Service) sets.String {
	imageSet := sets.NewString()
	latestContainer := make(map[string]*commonmodels.Container)
	revisionSvcMap := make(map[string]*commonmodels.Container)
	for _, c := range latestSvc.Containers {
		latestContainer[c.Name] = c
	}
	for _, c := range revisionSvc.Containers {
		revisionSvcMap[c.Name] = c
	}

	for _, prodContainer := range prodService.Containers {
		revisionSvcContainer, ok := revisionSvcMap[prodContainer.Name]
		if !ok {
			continue
		}
		if prodContainer.Image == revisionSvcContainer.Image {
			continue
		}
		lc, ok := latestContainer[prodContainer.Name]
		if !ok || lc.ImagePath == nil {
			continue
		}
		imageSet.Insert(lc.ImagePath.Image, lc.ImagePath.Tag, lc.ImagePath.Repo)
	}
	return imageSet
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
	renderChartMap := make(map[string]*templatemodels.RenderChart)
	for _, renderChart := range productResp.ChartInfos {
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

func proceedHelmRelease(productName, envName string, productResp *commonmodels.Product, renderset *commonmodels.RenderSet, helmClient *helmtool.HelmClient, filter svcUpgradeFilter, log *zap.SugaredLogger) error {
	renderChartMap := make(map[string]*templatemodels.RenderChart)
	for _, renderChart := range productResp.ChartInfos {
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
