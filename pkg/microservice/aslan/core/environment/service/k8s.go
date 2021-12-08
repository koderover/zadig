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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type K8sService struct {
	log *zap.SugaredLogger
}

// queryServiceStatus query service status
// If service has pods, service status = pod status (if pod is not running or succeed or failed, service status = unstable)
// If service doesnt have pods, service status = success (all objects created) or failed (fail to create some objects).
// 正常：StatusRunning or StatusSucceed
// 错误：StatusError or StatusFailed
func (k *K8sService) queryServiceStatus(namespace, envName, productName string, serviceTmpl *commonmodels.Service, kubeClient client.Client) (string, string, []string) {
	k.log.Infof("queryServiceStatus of service: %s of product: %s in namespace %s", serviceTmpl.ServiceName, productName, namespace)
	if len(serviceTmpl.Containers) > 0 {
		// 有容器时，根据pods status判断服务状态
		return queryPodsStatus(namespace, productName, serviceTmpl.ServiceName, kubeClient, k.log)
	}

	return setting.PodSucceeded, setting.PodReady, []string{}
}

func (k *K8sService) updateService(args *SvcOptArgs) error {
	svc := &commonmodels.ProductService{
		ServiceName: args.ServiceName,
		Type:        args.ServiceType,
		Revision:    args.ServiceRev.NextRevision,
		Containers:  args.ServiceRev.Containers,
	}

	project, err := templaterepo.NewProductColl().Find(args.ProductName)
	if err != nil {
		k.log.Errorf("Can not find project %s, err: %s", args.ProductName, err)
		return err
	}
	serviceInfo := project.GetServiceInfo(args.ServiceName)
	if serviceInfo == nil {
		return fmt.Errorf("service %s not found", args.ServiceName)
	}
	svc.ProductName = serviceInfo.Owner

	opt := &commonrepo.ProductFindOptions{Name: args.ProductName, EnvName: args.EnvName}
	exitedProd, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		k.log.Error(err)
		return errors.New(e.UpsertServiceErrMsg)
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), exitedProd.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	switch exitedProd.Status {
	case setting.ProductStatusCreating, setting.ProductStatusUpdating, setting.ProductStatusDeleting:
		k.log.Errorf("[%s][P:%s] Product is not in valid status", args.EnvName, args.ProductName)
		return e.ErrUpdateEnv.AddDesc(e.EnvCantUpdatedMsg)
	}

	// 适配老的产品没有renderset为空的情况
	if exitedProd.Render == nil {
		exitedProd.Render = &commonmodels.RenderInfo{ProductTmpl: exitedProd.ProductName}
	}
	// 检查renderset是否覆盖服务所有key
	newRender, err := commonservice.ValidateRenderSet(args.ProductName, exitedProd.Render.Name, serviceInfo, k.log)
	if err != nil {
		k.log.Errorf("[%s][P:%s] validate product renderset error: %v", args.EnvName, args.ProductName, err)
		return e.ErrUpdateProduct.AddDesc(err.Error())
	}
	svc.Render = &commonmodels.RenderInfo{Name: newRender.Name, Revision: newRender.Revision, ProductTmpl: newRender.ProductTmpl}

	_, err = upsertService(
		true,
		exitedProd,
		svc,
		exitedProd.GetServiceMap()[svc.ServiceName],
		newRender, kubeClient, k.log)

	// 如果创建依赖服务组有返回错误, 停止等待
	if err != nil {
		k.log.Error(err)
		return e.ErrUpdateProduct.AddDesc(err.Error())
	}

	// 更新产品服务
	for _, group := range exitedProd.Services {
		for i, service := range group {
			if service.ServiceName == args.ServiceName && service.Type == args.ServiceType {
				group[i] = svc
			}
		}
	}

	if err := commonrepo.NewProductColl().Update(exitedProd); err != nil {
		k.log.Errorf("[%s][%s] Product.Update error: %v", args.EnvName, args.ProductName, err)
		return e.ErrUpdateProduct
	}
	return nil
}

// TODO: LOU: improve the status scope and definition, like pod status, service status, environment, cluster status, ...
// TODO: LOU: rewrite it
func (k *K8sService) listGroupServices(allServices []*commonmodels.ProductService, envName, productName string, kubeClient client.Client, productInfo *commonmodels.Product) []*commonservice.ServiceResp {
	var wg sync.WaitGroup
	var resp []*commonservice.ServiceResp
	var mutex sync.RWMutex

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
			if kubeClient != nil {
				gp.Status, gp.Ready, gp.Images = k.queryServiceStatus(productInfo.Namespace, envName, productName, serviceTmpl, kubeClient)
				// 如果产品正在创建中，且service status为ERROR（POD还没创建出来），则判断为Pending，尚未开始创建
				if productInfo.Status == setting.ProductStatusCreating && gp.Status == setting.PodError {
					gp.Status = setting.PodPending
				}
			} else {
				gp.Status = setting.ClusterUnknown
			}

			//处理ingress信息
			gp.Ingress = GetIngressInfo(productInfo, serviceTmpl, k.log)
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

func (k *K8sService) createGroup(envName, productName, username string, group []*commonmodels.ProductService, renderSet *commonmodels.RenderSet, kubeClient client.Client) error {
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

	var wg sync.WaitGroup
	var lock sync.Mutex
	var resources []*unstructured.Unstructured

	for i := range group {
		// 只有在service有Pod的时候，才需要等待pod running或者等待pod succeed
		// 比如在group中，如果service下仅有configmap/service/ingress这些yaml的时候，不需要waitServicesRunning
		wg.Add(1)
		updatableServiceNameList = append(updatableServiceNameList, group[i].ServiceName)
		go func(svc *commonmodels.ProductService) {
			defer wg.Done()
			items, err := upsertService(false, prod, svc, nil, renderSet, kubeClient, k.log)
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
