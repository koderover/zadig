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
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/shared/kube/resource"
	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	zadigtypes "github.com/koderover/zadig/v2/pkg/types"
)

type K8sService struct {
	log *zap.SugaredLogger
}

// queryServiceStatus query service status
// If service has pods, service status = pod status (if pod is not running or succeed or failed, service status = unstable)
// If service doesn't have pods, service status = success (all objects created) or failed (fail to create some objects).
// 正常：StatusRunning or StatusSucceed
// 错误：StatusError or StatusFailed
func (k *K8sService) queryServiceStatus(serviceTmpl *commonmodels.Service, productInfo *commonmodels.Product, clientset *kubernetes.Clientset, informer informers.SharedInformerFactory) *commonservice.ZadigServiceStatusResp {
	return commonservice.QueryPodsStatus(productInfo, serviceTmpl, serviceTmpl.ServiceName, clientset, informer, k.log)
}

// queryWorkloadStatus query workload status
// only supports Deployment and StatefulSet
func (k *K8sService) queryWorkloadStatus(serviceTmpl *commonmodels.Service, productInfo *commonmodels.Product, informer informers.SharedInformerFactory) string {
	if len(serviceTmpl.Containers) > 0 {
		workloads, err := GetServiceWorkloads(serviceTmpl, productInfo, informer, k.log)
		if err != nil {
			k.log.Errorf("failed to get service workloads, err: %s", err)
			return setting.PodUnstable
		}
		for _, workload := range workloads {
			log.Infof("workload name: %s, ready: %v", workload.Name, workload.Ready)
			if !workload.Ready {
				return setting.PodUnstable
			}
		}
		return setting.PodRunning
	}
	return setting.PodSucceeded
}

func (k *K8sService) updateService(args *SvcOptArgs) error {
	newProductSvc := &commonmodels.ProductService{
		ServiceName: args.ServiceName,
		Type:        args.ServiceType,
		Revision:    0,
		Containers:  args.ServiceRev.Containers,
		ProductName: args.ProductName,
	}

	opt := &commonrepo.ProductFindOptions{Name: args.ProductName, EnvName: args.EnvName}
	prodinfo, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		k.log.Error(err)
		return errors.New(e.UpsertServiceErrMsg)
	}
	if prodinfo.IsSleeping() {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("environment is sleeping"))
	}

	currentProductSvc := prodinfo.GetServiceMap()[newProductSvc.ServiceName]
	if currentProductSvc == nil {
		return e.ErrUpdateService.AddErr(fmt.Errorf("failed to find service: %s in env: %s", newProductSvc.ServiceName, prodinfo.EnvName))
	}

	newProductSvc.Containers = currentProductSvc.Containers
	newProductSvc.Resources = currentProductSvc.Resources

	if !args.UpdateServiceTmpl {
		newProductSvc.Revision = currentProductSvc.Revision
	} else {
		latestSvcRevision, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
			ServiceName: newProductSvc.ServiceName,
			ProductName: newProductSvc.ProductName,
		}, prodinfo.Production)
		if err != nil {
			return e.ErrUpdateService.AddErr(fmt.Errorf("failed to find service, err: %s", err))
		}
		newProductSvc.Revision = latestSvcRevision.Revision

		curUsedSvc, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
			ServiceName: currentProductSvc.ServiceName,
			Revision:    currentProductSvc.Revision,
			ProductName: currentProductSvc.ProductName,
		}, prodinfo.Production)
		if err != nil {
			curUsedSvc = nil
		}
		newProductSvc.Containers = kube.CalculateContainer(currentProductSvc, curUsedSvc, latestSvcRevision.Containers, prodinfo)
	}

	switch prodinfo.Status {
	case setting.ProductStatusCreating, setting.ProductStatusUpdating, setting.ProductStatusDeleting:
		k.log.Errorf("[%s][P:%s] Product is not in valid status", args.EnvName, args.ProductName)
		return e.ErrUpdateEnv.AddDesc(e.EnvCantUpdatedMsg)
	}

	curSvcRender := prodinfo.GetSvcRender(args.ServiceName)
	globalVars := prodinfo.GlobalVariables

	globalVars, args.ServiceRev.VariableKVs, err = commontypes.UpdateGlobalVariableKVs(newProductSvc.ServiceName, globalVars, args.ServiceRev.VariableKVs, curSvcRender.OverrideYaml.RenderVariableKVs)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("failed to update global variable, err: %s", err))
	}
	args.ServiceRev.VariableYaml, err = commontypes.RenderVariableKVToYaml(args.ServiceRev.VariableKVs, true)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("failed to convert render variable to yaml, err: %s", err))
	}
	newProductSvc.GetServiceRender().OverrideYaml.RenderVariableKVs = args.ServiceRev.VariableKVs
	newProductSvc.GetServiceRender().OverrideYaml.YamlContent = args.ServiceRev.VariableYaml

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(prodinfo.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	istioClient, err := clientmanager.NewKubeClientManager().GetIstioClientSet(prodinfo.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	inf, err := clientmanager.NewKubeClientManager().GetInformer(prodinfo.ClusterID, prodinfo.Namespace)
	if err != nil {
		return e.ErrUpdateEnv.AddDesc(err.Error())
	}

	// resource will not be applied if service yaml is not changed
	previewArg := &PreviewServiceArgs{
		ProductName:           prodinfo.ProductName,
		EnvName:               prodinfo.EnvName,
		ServiceName:           args.ServiceName,
		UpdateServiceRevision: args.UpdateServiceTmpl,
		ServiceModules:        args.ServiceRev.Containers,
		VariableKVs:           args.ServiceRev.VariableKVs,
	}
	previewResult, err := PreviewService(previewArg, k.log)
	if err != nil {
		return e.ErrUpdateEnv.AddDesc(fmt.Errorf("failed to compare service yaml, err: %s", err).Error())
	}

	// nothing to apply if rendered service yaml is not changed
	if previewResult.Current.Yaml == previewResult.Latest.Yaml {
		k.log.Infof("[%s][P:%s] Service yaml is not changed", args.EnvName, args.ProductName)
	} else {
		err = kube.CheckResourceAppliedByOtherEnv(previewResult.Latest.Yaml, prodinfo, args.ServiceName)
		if err != nil {
			return e.ErrUpdateEnv.AddErr(err)
		}
		items, err := upsertService(
			prodinfo,
			newProductSvc,
			currentProductSvc,
			!prodinfo.Production, inf, kubeClient, istioClient, k.log)
		if err != nil {
			k.log.Error(err)
			newProductSvc.Error = err.Error()
			return e.ErrUpdateProduct.AddDesc(err.Error())
		}
		newProductSvc.Resources = kube.UnstructuredToResources(items)
	}

	newProductSvc.Error = ""
	for _, group := range prodinfo.Services {
		for i, service := range group {
			if service.ServiceName == args.ServiceName && service.Type == args.ServiceType {
				newProductSvc.UpdateTime = time.Now().Unix()
				group[i] = newProductSvc
			}
		}
	}

	prodinfo.ServiceDeployStrategy = commonutil.SetServiceDeployStrategyDepoly(prodinfo.ServiceDeployStrategy, args.ServiceName)

	session := mongotool.Session()
	defer session.EndSession(context.Background())

	err = mongotool.StartTransaction(session)
	if err != nil {
		return e.ErrUpdateProduct.AddErr(err)
	}

	productColl := commonrepo.NewProductCollWithSession(session)

	// Note update logic need to be optimized since we only need to update one service
	if err := productColl.Update(prodinfo); err != nil {
		k.log.Errorf("[%s][%s] Product.Update error: %v", args.EnvName, args.ProductName, err)
		mongotool.AbortTransaction(session)
		return e.ErrUpdateProduct.AddErr(err)
	}

	if err := productColl.UpdateGlobalVariable(prodinfo); err != nil {
		k.log.Errorf("[%s][%s] Product.UpdateGlobalVariable error: %v", args.EnvName, args.ProductName, err)
		mongotool.AbortTransaction(session)
		return e.ErrUpdateProduct.AddErr(err)
	}

	if err := commonutil.CreateEnvServiceVersion(prodinfo, newProductSvc, args.UpdateBy, config.EnvOperationDefault, "", session, k.log); err != nil {
		k.log.Errorf("[%s][%s] Product.CreateEnvServiceVersion for service %s error: %v", args.EnvName, args.ProductName, args.ServiceName, err)
	}

	return mongotool.CommitTransaction(session)
}

func (k *K8sService) calculateProductStatus(productInfo *commonmodels.Product, informer informers.SharedInformerFactory) (string, error) {
	if informer == nil {
		return setting.ClusterUnknown, nil
	}
	retStatus := setting.PodRunning

	allSvcs := make([]string, 0)
	for _, svc := range productInfo.GetServiceMap() {
		allSvcs = append(allSvcs, svc.ServiceName)
	}

	batchCount := 10
	for i := 0; i < len(allSvcs); {
		maxIndex := i + batchCount
		if maxIndex >= len(allSvcs) {
			maxIndex = len(allSvcs)
		}
		var wg sync.WaitGroup
		for ii := i; ii < maxIndex; ii++ {
			service := productInfo.GetServiceMap()[allSvcs[ii]]
			wg.Add(1)
			go func(service *commonmodels.ProductService) {
				defer wg.Done()
				if service == nil {
					return
				}
				serviceTmpl, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
					ServiceName: service.ServiceName,
					Revision:    service.Revision,
					ProductName: service.ProductName,
				}, productInfo.Production)
				if err != nil {
					log.Errorf("failed to get service template %s revision %s, err: %s", service.ServiceName, service.Revision, err)
					retStatus = setting.PodUnstable
					return
				}
				statusResp := k.queryWorkloadStatus(serviceTmpl, productInfo, informer)
				if statusResp != setting.PodRunning {
					retStatus = statusResp
				}
			}(service)
			wg.Wait()
		}
		if retStatus != setting.PodRunning || maxIndex >= len(allSvcs) {
			break
		} else {
			i = maxIndex
		}
	}

	return retStatus, nil
}

func (k *K8sService) listGroupServices(allServices []*commonmodels.ProductService, envName string, informer informers.SharedInformerFactory, productInfo *commonmodels.Product) []*commonservice.ServiceResp {
	var wg sync.WaitGroup
	var resp []*commonservice.ServiceResp
	var mutex sync.RWMutex

	cls, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(productInfo.ClusterID)
	if err != nil {
		log.Errorf("failed to init client set, err: %s", err)
		return nil
	}

	hostInfos := make([]resource.HostInfo, 0)
	version, err := cls.Discovery().ServerVersion()
	if err != nil {
		log.Errorf("Failed to get server version info for cluster: %s, the error is: %s", productInfo.ClusterID, err)
		return nil
	}
	if kubeclient.VersionLessThan122(version) {
		ingresses, err := getter.ListExtensionsV1Beta1Ingresses(nil, informer)
		if err == nil {
			for _, ingress := range ingresses {
				hostInfos = append(hostInfos, wrapper.Ingress(ingress).HostInfo()...)
			}
		} else {
			log.Warnf("Failed to list ingresses, the error is: %s", err)
		}
	} else {
		ingresses, err := getter.ListNetworkingV1Ingress(nil, informer)
		if err == nil {
			for _, ingress := range ingresses {
				hostInfos = append(hostInfos, wrapper.GetIngressHostInfo(ingress)...)
			}
		} else {
			log.Warnf("Failed to list ingresses, the error is: %s", err)
		}
	}

	istioClient, err := clientmanager.NewKubeClientManager().GetIstioClientSet(productInfo.ClusterID)
	if err != nil {
		log.Errorf("failed to new istio client: %s", err)
		return nil
	}

	var gwObjs *v1alpha3.GatewayList
	istioInstalled, err := kube.CheckIstiodInstalled(context.TODO(), cls)
	if err != nil {
		log.Warnf("failed to check istiod whether installed: %s", err)
	} else {
		if istioInstalled {
			zadigLabels := map[string]string{
				zadigtypes.ZadigLabelKeyGlobalOwner: zadigtypes.Zadig,
			}
			gwObjs, err = istioClient.NetworkingV1alpha3().Gateways(productInfo.Namespace).List(context.TODO(), metav1.ListOptions{
				LabelSelector: labels.FormatLabels(zadigLabels),
			})
			if err != nil {
				log.Warnf("Failed to list istio gateways, the error is: %s", err)
			}
		}
	}

	// get all services
	k8sServices, err := getter.ListServicesWithCache(nil, informer)
	if err != nil {
		log.Errorf("[%s][%s] list service error: %s", envName, productInfo.Namespace, err)
		return nil
	}

	for _, service := range allServices {
		wg.Add(1)
		go func(service *commonmodels.ProductService) {
			defer wg.Done()
			gp := &commonservice.ServiceResp{
				ServiceName:    service.ServiceName,
				Type:           service.Type,
				EnvName:        envName,
				DeployStrategy: service.DeployStrategy,
				Updatable:      service.Updatable,
				Error:          service.Error,
			}
			serviceTmpl, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
				ServiceName: service.ServiceName,
				Revision:    service.Revision,
				ProductName: service.ProductName,
			}, productInfo.Production)

			if err != nil {
				gp.Status = setting.PodFailed
				mutex.Lock()
				resp = append(resp, gp)
				mutex.Unlock()
				return
			}

			gp.ProductName = service.ProductName
			// 查询group下所有pods信息
			if informer != nil {
				statusResp := k.queryServiceStatus(serviceTmpl, productInfo, cls, informer)
				gp.Status, gp.Ready, gp.Images = statusResp.PodStatus, statusResp.Ready, statusResp.Images
				// 如果产品正在创建中，且service status为ERROR（POD还没创建出来），则判断为Pending，尚未开始创建
				if productInfo.Status == setting.ProductStatusCreating && gp.Status == setting.PodError {
					gp.Status = setting.PodPending
				}

				hostInfo := make([]resource.HostInfo, 0)
				for _, workload := range statusResp.Workloads {
					hostInfo = append(hostInfo, commonservice.FindServiceFromIngress(hostInfos, workload, k8sServices)...)
				}
				gp.Ingress = &commonservice.IngressInfo{
					HostInfo: hostInfo,
				}

			} else {
				gp.Status = setting.ClusterUnknown
			}

			gp.IstioGateway = &commonservice.IstioGatewayInfo{
				Servers: commonservice.FindServiceFromIstioGateway(gwObjs, service.ServiceName),
			}

			mutex.Lock()
			resp = append(resp, gp)
			mutex.Unlock()
		}(service)
	}

	wg.Wait()

	//把数据按照名称排序
	sort.SliceStable(resp, func(i, j int) bool { return resp[i].ServiceName < resp[j].ServiceName })

	return resp
}

func (k *K8sService) GetGroupService(service *commonmodels.ProductService, string, informer informers.SharedInformerFactory, productInfo *commonmodels.Product) *commonservice.ServiceResp {
	envName := productInfo.EnvName

	cls, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(productInfo.ClusterID)
	if err != nil {
		log.Errorf("failed to init client set, err: %s", err)
		return nil
	}

	hostInfos := make([]resource.HostInfo, 0)
	version, err := cls.Discovery().ServerVersion()
	if err != nil {
		log.Errorf("Failed to get server version info for cluster: %s, the error is: %s", productInfo.ClusterID, err)
		return nil
	}
	if kubeclient.VersionLessThan122(version) {
		ingresses, err := getter.ListExtensionsV1Beta1Ingresses(nil, informer)
		if err == nil {
			for _, ingress := range ingresses {
				hostInfos = append(hostInfos, wrapper.Ingress(ingress).HostInfo()...)
			}
		} else {
			log.Warnf("Failed to list ingresses, the error is: %s", err)
		}
	} else {
		ingresses, err := getter.ListNetworkingV1Ingress(nil, informer)
		if err == nil {
			for _, ingress := range ingresses {
				hostInfos = append(hostInfos, wrapper.GetIngressHostInfo(ingress)...)
			}
		} else {
			log.Warnf("Failed to list ingresses, the error is: %s", err)
		}
	}

	// get all services
	k8sServices, err := getter.ListServicesWithCache(nil, informer)
	if err != nil {
		log.Errorf("[%s][%s] list service error: %s", envName, productInfo.Namespace, err)
		return nil
	}

	gp := &commonservice.ServiceResp{
		ServiceName:    service.ServiceName,
		Type:           service.Type,
		EnvName:        envName,
		DeployStrategy: service.DeployStrategy,
		Updatable:      service.Updatable,
	}
	serviceTmpl, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ServiceName: service.ServiceName,
		Revision:    service.Revision,
		ProductName: service.ProductName,
	}, productInfo.Production)

	if err != nil {
		gp.Status = setting.PodFailed
		return gp
	}

	gp.ProductName = service.ProductName
	// 查询group下所有pods信息
	if informer != nil {
		statusResp := k.queryServiceStatus(serviceTmpl, productInfo, cls, informer)
		gp.Status, gp.Ready, gp.Images = statusResp.PodStatus, statusResp.Ready, statusResp.Images
		// 如果产品正在创建中，且service status为ERROR（POD还没创建出来），则判断为Pending，尚未开始创建
		if productInfo.Status == setting.ProductStatusCreating && gp.Status == setting.PodError {
			gp.Status = setting.PodPending
		}

		hostInfo := make([]resource.HostInfo, 0)
		for _, workload := range statusResp.Workloads {
			hostInfo = append(hostInfo, commonservice.FindServiceFromIngress(hostInfos, workload, k8sServices)...)
		}
		gp.Ingress = &commonservice.IngressInfo{
			HostInfo: hostInfo,
		}

	} else {
		gp.Status = setting.ClusterUnknown
	}

	return gp
}

func fetchWorkloadImages(productService *commonmodels.ProductService, product *commonmodels.Product, kubeClient client.Client) ([]*commonmodels.Container, error) {
	rederedYaml, err := kube.RenderEnvService(product, productService.Render, productService)
	if err != nil {
		return nil, fmt.Errorf("failed to render env service yaml for service: %s, err: %s", productService.ServiceName, err)
	}
	manifests := releaseutil.SplitManifests(rederedYaml)
	namespace := product.Namespace

	containers := make([]*resource.ContainerImage, 0)
	retMap := make(map[string]*commonmodels.Container)

	for _, manifest := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(manifest))
		if err != nil {
			log.Errorf("failed to convert yaml to Unstructured when check resources, manifest is\n%s\n, error: %v", manifest, err)
			continue
		}
		if u.GetKind() == setting.Deployment {
			deployment, exist, err := getter.GetDeployment(namespace, u.GetName(), kubeClient)
			if err != nil || !exist {
				log.Errorf("failed to find deployment with name: %s", u.GetName())
				continue
			}
			containers = append(containers, wrapper.Deployment(deployment).GetContainers()...)
		} else if u.GetKind() == setting.StatefulSet {
			sts, exist, err := getter.GetStatefulSet(namespace, u.GetName(), kubeClient)
			if err != nil || !exist {
				log.Errorf("failed to find sts with name: %s", u.GetName())
				continue
			}
			containers = append(containers, wrapper.StatefulSet(sts).GetContainers()...)
		}
	}

	for _, container := range containers {
		retMap[container.Name] = &commonmodels.Container{
			Name:      container.Name,
			Image:     container.Image,
			ImageName: container.ImageName,
		}
	}
	ret := make([]*commonmodels.Container, 0)
	for _, container := range retMap {
		ret = append(ret, container)
	}
	return ret, nil
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

func (k *K8sService) createGroup(username string, product *commonmodels.Product, group []*commonmodels.ProductService, informer informers.SharedInformerFactory, kubeClient client.Client) error {
	envName, productName := product.EnvName, product.ProductName
	k.log.Infof("[Namespace:%s][Product:%s] createGroup", envName, productName)
	updatableServiceNameList := make([]string, 0)

	// 异步创建无依赖的服务
	errList := &multierror.Error{
		ErrorFormat: func(es []error) string {
			points := make([]string, len(es))
			for i, err := range es {
				points[i] = fmt.Sprintf("%v", err)
			}
			return strings.Join(points, "\n")
		},
	}

	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		errList = multierror.Append(errList, err)
	}

	istioClient, err := clientmanager.NewKubeClientManager().GetIstioClientSet(prod.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to new istio client: %s", err)
	}

	var wg sync.WaitGroup
	var lock sync.Mutex
	var resources []*unstructured.Unstructured

	for i := range group {
		if !commonutil.ServiceDeployed(group[i].ServiceName, product.ServiceDeployStrategy) {
			// services are only imported, we do not deploy them again, but we need to fetch the images
			containers, err := fetchWorkloadImages(group[i], product, kubeClient)
			if err != nil {
				return fmt.Errorf("failed to fetch related containers: %s", err)
			}
			group[i].Containers = containers
			continue
		}
		wg.Add(1)
		updatableServiceNameList = append(updatableServiceNameList, group[i].ServiceName)
		go func(svc *commonmodels.ProductService) {
			defer wg.Done()
			items, err := upsertService(prod, svc, nil, !prod.Production, informer, kubeClient, istioClient, k.log)
			if err != nil {
				lock.Lock()
				switch e := err.(type) {
				case *multierror.Error:
					errList = multierror.Append(errList, e.Errors...)
				default:
					errList = multierror.Append(errList, e)
				}
				svc.Error = err.Error()
				lock.Unlock()
			}
			svc.Resources = kube.UnstructuredToResources(items)

			err = commonutil.CreateEnvServiceVersion(product, svc, username, config.EnvOperationDefault, "", nil, k.log)
			if err != nil {
				log.Errorf("failed to create env service version for service %s/%s, error: %v", product.EnvName, svc.ServiceName, err)
			}

			//  concurrent array append
			lock.Lock()
			resources = append(resources, items...)
			lock.Unlock()
		}(group[i])
	}
	wg.Wait()

	// 如果创建依赖服务组有返回错误, 停止等待
	if err := errList.ErrorOrNil(); err != nil {
		return err
	}

	if err := waitResourceRunning(kubeClient, prod.Namespace, resources, config.ServiceStartTimeout(), k.log); err != nil {
		k.log.Errorf(
			"service group %s/%+v doesn't start in %d seconds: %v",
			prod.Namespace,
			updatableServiceNameList, config.ServiceStartTimeout(), err)

		err = e.ErrUpdateEnv.AddErr(
			fmt.Errorf(e.StartPodTimeout+"\n %s", "["+strings.Join(updatableServiceNameList, "], [")+"]"))
		return err
	}
	return nil
}
