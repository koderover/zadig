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
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/render"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/log"
)

type K8sService struct {
	log *zap.SugaredLogger
}

type ZadigServiceStatusResp struct {
	ServiceName string
	PodStatus   string
	Ready       string
	Ingress     []*resource.Ingress
	Images      []string
	Workloads   []*commonservice.Workload
}

// queryServiceStatus query service status
// If service has pods, service status = pod status (if pod is not running or succeed or failed, service status = unstable)
// If service doesn't have pods, service status = success (all objects created) or failed (fail to create some objects).
// 正常：StatusRunning or StatusSucceed
// 错误：StatusError or StatusFailed
func (k *K8sService) queryServiceStatus(serviceTmpl *commonmodels.Service, productInfo *commonmodels.Product, clientset *kubernetes.Clientset, informer informers.SharedInformerFactory) *ZadigServiceStatusResp {
	return queryPodsStatus(productInfo, serviceTmpl, serviceTmpl.ServiceName, clientset, informer, k.log)
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
	svc := &commonmodels.ProductService{
		ServiceName: args.ServiceName,
		Type:        args.ServiceType,
		Revision:    0,
		Containers:  args.ServiceRev.Containers,
		ProductName: args.ProductName,
	}

	opt := &commonrepo.ProductFindOptions{Name: args.ProductName, EnvName: args.EnvName}
	exitedProd, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		k.log.Error(err)
		return errors.New(e.UpsertServiceErrMsg)
	}
	if exitedProd.IsSleeping() {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("Environment is sleeping"))
	}

	currentProductSvc := exitedProd.GetServiceMap()[svc.ServiceName]
	if currentProductSvc == nil {
		return e.ErrUpdateService.AddErr(fmt.Errorf("failed to find service: %s in env: %s", svc.ServiceName, exitedProd.EnvName))
	}

	svc.Containers = currentProductSvc.Containers

	if !args.UpdateServiceTmpl {
		svc.Revision = currentProductSvc.Revision
	} else {
		latestSvcRevision, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
			ServiceName: svc.ServiceName,
			ProductName: svc.ProductName,
		}, exitedProd.Production)
		if err != nil {
			return e.ErrUpdateService.AddErr(fmt.Errorf("failed to find service, err: %s", err))
		}
		svc.Revision = latestSvcRevision.Revision

		curUsedSvc, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
			ServiceName: currentProductSvc.ServiceName,
			Revision:    currentProductSvc.Revision,
			ProductName: currentProductSvc.ProductName,
		}, exitedProd.Production)
		if err != nil {
			curUsedSvc = nil
		}
		svc.Containers = kube.CalculateContainer(currentProductSvc, curUsedSvc, latestSvcRevision.Containers, exitedProd)
	}

	switch exitedProd.Status {
	case setting.ProductStatusCreating, setting.ProductStatusUpdating, setting.ProductStatusDeleting:
		k.log.Errorf("[%s][P:%s] Product is not in valid status", args.EnvName, args.ProductName)
		return e.ErrUpdateEnv.AddDesc(e.EnvCantUpdatedMsg)
	}

	exitedProd.EnsureRenderInfo()
	curRenderset, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		Name:     exitedProd.Render.Name,
		EnvName:  exitedProd.EnvName,
		Revision: exitedProd.Render.Revision,
	})
	if err != nil {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("failed to find render set, err: %s", err))
	}

	curSvcRender := &templatemodels.ServiceRender{
		ServiceName:  args.ServiceName,
		OverrideYaml: &templatemodels.CustomYaml{},
	}
	for _, svcRender := range curRenderset.ServiceVariables {
		if svcRender.ServiceName == args.ServiceName {
			curSvcRender = svcRender
			break
		}
	}

	curRenderset.GlobalVariables, args.ServiceRev.VariableKVs, err = commontypes.UpdateGlobalVariableKVs(svc.ServiceName, curRenderset.GlobalVariables, args.ServiceRev.VariableKVs, curSvcRender.OverrideYaml.RenderVariableKVs)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("failed to update global variable, err: %s", err))
	}
	args.ServiceRev.VariableYaml, err = commontypes.RenderVariableKVToYaml(args.ServiceRev.VariableKVs)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("failed to convert render variable to yaml, err: %s", err))
	}

	foundServiceVariable := false
	for _, svc := range curRenderset.ServiceVariables {
		if svc.ServiceName != args.ServiceName {
			continue
		}

		foundServiceVariable = true
		svc.OverrideYaml = &template.CustomYaml{
			YamlContent:       args.ServiceRev.VariableYaml,
			RenderVariableKVs: args.ServiceRev.VariableKVs,
		}
	}
	if !foundServiceVariable {
		curRenderset.ServiceVariables = append(curRenderset.ServiceVariables, &template.ServiceRender{
			ServiceName: args.ServiceName,
			OverrideYaml: &template.CustomYaml{
				YamlContent:       args.ServiceRev.VariableYaml,
				RenderVariableKVs: args.ServiceRev.VariableKVs,
			},
		})
	}
	err = render.CreateK8sHelmRenderSet(curRenderset, k.log)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("failed to craete renderset, err: %s", err))
	}

	preRevision := exitedProd.Render
	exitedProd.Render = &commonmodels.RenderInfo{Name: curRenderset.Name, Revision: curRenderset.Revision, ProductTmpl: curRenderset.ProductTmpl}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), exitedProd.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), exitedProd.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), exitedProd.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddDesc(err.Error())
	}
	inf, err := informer.NewInformer(exitedProd.ClusterID, exitedProd.Namespace, cls)
	if err != nil {
		return e.ErrUpdateEnv.AddDesc(err.Error())
	}

	// resource will not be applied if service yaml is not changed
	previewArg := &PreviewServiceArgs{
		ProductName:           exitedProd.ProductName,
		EnvName:               exitedProd.EnvName,
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
		_, err = upsertService(
			exitedProd,
			svc,
			currentProductSvc,
			curRenderset, preRevision, !exitedProd.Production, inf, kubeClient, istioClient, k.log)

		if err != nil {
			k.log.Error(err)
			svc.Error = err.Error()
			return e.ErrUpdateProduct.AddDesc(err.Error())
		}
	}

	svc.Error = ""
	for _, group := range exitedProd.Services {
		for i, service := range group {
			if service.ServiceName == args.ServiceName && service.Type == args.ServiceType {
				group[i] = svc
			}
		}
	}

	exitedProd.ServiceDeployStrategy = commonutil.SetServiceDeployStrategyDepoly(exitedProd.ServiceDeployStrategy, args.ServiceName)

	// Note update logic need to be optimized since we only need to update one service
	if err := commonrepo.NewProductColl().Update(exitedProd); err != nil {
		k.log.Errorf("[%s][%s] Product.Update error: %v", args.EnvName, args.ProductName, err)
		return e.ErrUpdateProduct
	}
	return nil
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

	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), productInfo.ClusterID)
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

func fetchWorkloadImages(productService *commonmodels.ProductService, product *commonmodels.Product, renderSet *commonmodels.RenderSet, kubeClient client.Client) ([]*commonmodels.Container, error) {
	//rederedYaml, err := kube.RenderEnvService(product, renderSet.GetServiceRenderMap()[productService.ServiceName], productService)
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

func (k *K8sService) createGroup(username string, product *commonmodels.Product, group []*commonmodels.ProductService, _ *commonmodels.RenderSet, informer informers.SharedInformerFactory, kubeClient client.Client) error {
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

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), prod.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get rest config: %s", err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to new istio client: %s", err)
	}

	var wg sync.WaitGroup
	var lock sync.Mutex
	var resources []*unstructured.Unstructured

	for i := range group {
		if !commonutil.ServiceDeployed(group[i].ServiceName, product.ServiceDeployStrategy) {
			// services are only imported, we do not deploy them again, but we need to fetch the images
			//containers, err := fetchWorkloadImages(group[i], product, renderSet, kubeClient)
			containers, err := fetchWorkloadImages(group[i], product, nil, kubeClient)
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
			//items, err := upsertService(prod, svc, svc, renderSet, nil, !prod.Production, informer, kubeClient, istioClient, k.log)
			items, err := upsertService(prod, svc, svc, nil, nil, !prod.Production, informer, kubeClient, istioClient, k.log)
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
