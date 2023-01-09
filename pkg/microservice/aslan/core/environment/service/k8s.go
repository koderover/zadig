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

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/koderover/zadig/pkg/tool/log"
)

type K8sService struct {
	log *zap.SugaredLogger
}

// queryServiceStatus query service status
// If service has pods, service status = pod status (if pod is not running or succeed or failed, service status = unstable)
// If service doesnt have pods, service status = success (all objects created) or failed (fail to create some objects).
// 正常：StatusRunning or StatusSucceed
// 错误：StatusError or StatusFailed
func (k *K8sService) queryServiceStatus(serviceTmpl *commonmodels.Service, productInfo *commonmodels.Product, kubeClient client.Client, clientset *kubernetes.Clientset, informer informers.SharedInformerFactory) (string, string, []string) {
	if len(serviceTmpl.Containers) > 0 {
		// 有容器时，根据pods status判断服务状态
		return queryPodsStatus(productInfo, serviceTmpl.ServiceName, kubeClient, clientset, informer, k.log)
	}

	return setting.PodSucceeded, setting.PodReady, []string{}
}

func (k *K8sService) updateService(args *SvcOptArgs) error {
	svc := &commonmodels.ProductService{
		ServiceName: args.ServiceName,
		Type:        args.ServiceType,
		//Revision:    args.ServiceRev.NextRevision,
		Revision:   0,
		Containers: args.ServiceRev.Containers,
	}

	opt := &commonrepo.ProductFindOptions{Name: args.ProductName, EnvName: args.EnvName}
	exitedProd, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		k.log.Error(err)
		return errors.New(e.UpsertServiceErrMsg)
	}

	currentProductSvc := exitedProd.GetServiceMap()[svc.ServiceName]
	if currentProductSvc == nil {
		return e.ErrUpdateService.AddErr(fmt.Errorf("failed to find service: %s in env: %s", svc.ServiceName, exitedProd.EnvName))
	}

	project, err := templaterepo.NewProductColl().Find(args.ProductName)
	if err != nil {
		k.log.Errorf("Can not find project %s, err: %s", args.ProductName, err)
		return err
	}
	serviceInfo := project.GetServiceInfo(args.ServiceName)
	if serviceInfo != nil {
		svc.ProductName = serviceInfo.Owner
	} else {
		svc.ProductName = currentProductSvc.ProductName
	}

	svc.Containers = currentProductSvc.Containers

	if !args.UpdateServiceTmpl {
		svc.Revision = currentProductSvc.Revision
	} else {
		latestSvcRevision, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
			ServiceName: svc.ServiceName,
			ProductName: svc.ProductName,
		})
		if err != nil {
			return e.ErrUpdateService.AddErr(fmt.Errorf("failed to find service, err: %s", err))
		}
		svc.Revision = latestSvcRevision.Revision

		containerMap := make(map[string]*commonmodels.Container)
		for _, container := range latestSvcRevision.Containers {
			containerMap[container.Name] = container
		}

		for _, container := range svc.Containers {
			if _, ok := containerMap[container.Name]; ok {
				containerMap[container.Name] = container
			}
		}

		svc.Containers = make([]*commonmodels.Container, 0)
		for _, container := range containerMap {
			svc.Containers = append(svc.Containers, container)
		}
	}

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

	switch exitedProd.Status {
	case setting.ProductStatusCreating, setting.ProductStatusUpdating, setting.ProductStatusDeleting:
		k.log.Errorf("[%s][P:%s] Product is not in valid status", args.EnvName, args.ProductName)
		return e.ErrUpdateEnv.AddDesc(e.EnvCantUpdatedMsg)
	}

	exitedProd.EnsureRenderInfo()
	curRenderset, _, err := commonrepo.NewRenderSetColl().FindRenderSet(&commonrepo.RenderSetFindOption{
		Name:     exitedProd.Render.Name,
		EnvName:  exitedProd.EnvName,
		Revision: exitedProd.Render.Revision,
	})
	for _, svc := range curRenderset.ServiceVariables {
		if svc.ServiceName != args.ServiceName {
			continue
		}
		svc.OverrideYaml = &template.CustomYaml{YamlContent: args.ServiceRev.VariableYaml}
	}
	err = commonservice.CreateK8sHelmRenderSet(curRenderset, k.log)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("failed to craete renderset, err: %s", err))
	}

	//// generate new renderset
	//newRender, err := commonservice.ValidateRenderSet(args.ProductName, exitedProd.Render.Name, exitedProd.EnvName, serviceInfo, k.log)
	//if err != nil {
	//	k.log.Errorf("[%s][P:%s] validate product renderset error: %v", args.EnvName, args.ProductName, err)
	//	return e.ErrUpdateProduct.AddDesc(err.Error())
	//}
	preRevision := exitedProd.Render
	exitedProd.Render = &commonmodels.RenderInfo{Name: curRenderset.Name, Revision: curRenderset.Revision, ProductTmpl: curRenderset.ProductTmpl}

	_, err = upsertService(
		exitedProd,
		svc,
		currentProductSvc,
		curRenderset, preRevision, inf, kubeClient, istioClient, k.log)

	// 如果创建依赖服务组有返回错误, 停止等待
	if err != nil {
		k.log.Error(err)
		svc.Error = err.Error()
		return e.ErrUpdateProduct.AddDesc(err.Error())
	}

	svc.Error = ""
	// 更新产品服务
	for _, group := range exitedProd.Services {
		for i, service := range group {
			if service.ServiceName == args.ServiceName && service.Type == args.ServiceType {
				group[i] = svc
			}
		}
	}

	if exitedProd.ServiceDeployStrategy != nil {
		exitedProd.ServiceDeployStrategy[args.ServiceName] = setting.ServiceDeployStrategyDeploy
	}

	// Note update logic need to be optimized since we only need to update one service
	if err := commonrepo.NewProductColl().Update(exitedProd); err != nil {
		k.log.Errorf("[%s][%s] Product.Update error: %v", args.EnvName, args.ProductName, err)
		return e.ErrUpdateProduct
	}
	return nil
}

func (k *K8sService) listGroupServices(allServices []*commonmodels.ProductService, envName, productName string, informer informers.SharedInformerFactory, productInfo *commonmodels.Product) []*commonservice.ServiceResp {
	var wg sync.WaitGroup
	var resp []*commonservice.ServiceResp
	var mutex sync.RWMutex

	svcNameSet := sets.NewString()
	for _, svc := range allServices {
		svcNameSet.Insert(svc.ServiceName)
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		log.Errorf("failed to kubeClient, err: %s", err)
		return nil
	}

	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		log.Errorf("failed to init client set, err: %s", err)
		return nil
	}

	selector := labels.Set{setting.ProductLabel: productName}.AsSelector()
	workloadMap := make(map[string][]*commonservice.Workload)
	listDeployments, err := getter.ListDeploymentsWithCache(selector, informer)
	if err != nil {
		log.Errorf("[%s][%s] get deployment list error: %v", productName, envName, err)
		return nil
	}

	for _, v := range listDeployments {
		serviceName := v.Labels[setting.ServiceLabel]
		if !svcNameSet.Has(serviceName) {
			continue
		}
		workload := &commonservice.Workload{
			Name:       v.Name,
			Spec:       v.Spec.Template,
			Type:       setting.Deployment,
			Images:     wrapper.Deployment(v).ImageInfos(),
			Ready:      wrapper.Deployment(v).Ready(),
			Annotation: v.Annotations,
		}
		workloadMap[serviceName] = append(workloadMap[serviceName], workload)
	}
	statefulSets, err := getter.ListStatefulSetsWithCache(selector, informer)
	if err != nil {
		log.Errorf("[%s][%s] get sts list error: %v", productName, envName, err)
		return nil
	}
	for _, v := range statefulSets {
		serviceName := v.Labels[setting.ServiceLabel]
		if !svcNameSet.Has(serviceName) {
			continue
		}
		workload := &commonservice.Workload{
			Name:       v.Name,
			Spec:       v.Spec.Template,
			Type:       setting.StatefulSet,
			Images:     wrapper.StatefulSet(v).ImageInfos(),
			Ready:      wrapper.StatefulSet(v).Ready(),
			Annotation: v.Annotations,
		}
		workloadMap[serviceName] = append(workloadMap[serviceName], workload)
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
				ServiceName: service.ServiceName,
				Type:        service.Type,
				EnvName:     envName,
			}
			serviceTmpl, err := commonservice.GetServiceTemplate(
				service.ServiceName, setting.K8SDeployType, service.ProductName, "", service.Revision, k.log,
			)
			if err != nil {
				gp.Status = setting.PodFailed
				mutex.Lock()
				resp = append(resp, gp)
				mutex.Unlock()
				return
			}

			gp.ProductName = serviceTmpl.ProductName
			// 查询group下所有pods信息
			if informer != nil {
				gp.Status, gp.Ready, gp.Images = k.queryServiceStatus(serviceTmpl, productInfo, kubeClient, cls, informer)
				// 如果产品正在创建中，且service status为ERROR（POD还没创建出来），则判断为Pending，尚未开始创建
				if productInfo.Status == setting.ProductStatusCreating && gp.Status == setting.PodError {
					gp.Status = setting.PodPending
				}
			} else {
				gp.Status = setting.ClusterUnknown
			}

			// ingress may be multiple workloads
			hostInfo := make([]resource.HostInfo, 0)
			for _, workload := range workloadMap[service.ServiceName] {
				hostInfo = append(hostInfo, commonservice.FindServiceFromIngress(hostInfos, workload, k8sServices)...)
			}
			gp.Ingress = &commonservice.IngressInfo{
				HostInfo: hostInfo,
			}

			//gp.Ingress = GetIngressInfo(productInfo, serviceTmpl, k.log)
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

func (k *K8sService) createGroup(username string, product *commonmodels.Product, group []*commonmodels.ProductService, renderSet *commonmodels.RenderSet, informer informers.SharedInformerFactory, kubeClient client.Client) error {
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
		// 只有在service有Pod的时候，才需要等待pod running或者等待pod succeed
		// 比如在group中，如果service下仅有configmap/service/ingress这些yaml的时候，不需要waitServicesRunning
		if !commonutil.ServiceDeployed(group[i].ServiceName, product.ServiceDeployStrategy) {
			continue
		}
		wg.Add(1)
		updatableServiceNameList = append(updatableServiceNameList, group[i].ServiceName)
		go func(svc *commonmodels.ProductService) {
			defer wg.Done()
			items, err := upsertService(prod, svc, nil, renderSet, nil, informer, kubeClient, istioClient, k.log)
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
