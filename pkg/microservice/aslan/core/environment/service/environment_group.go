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
	"sort"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/render"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
)

type EnvGroupRequest struct {
	ProjectName string `form:"projectName"`
	Page        int    `form:"page"`
	PerPage     int    `form:"perPage"`
	ServiceName string `form:"serviceName"`
}

func ListGroups(serviceName, envName, productName string, perPage, page int, production bool, log *zap.SugaredLogger) ([]*commonservice.ServiceResp, int, error) {
	var (
		count           = 0
		allServices     = make([]*commonmodels.ProductService, 0)
		currentServices = make([]*commonmodels.ProductService, 0)
		resp            = make([]*commonservice.ServiceResp, 0)
	)

	projectType := getProjectType(productName)
	if projectType == setting.HelmDeployType {
		log.Infof("listing group for helm project is not supported: %s/%s", productName, envName)
		return resp, count, nil
	}

	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: util.GetBoolPointer(production)}
	productInfo, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return resp, count, e.ErrListGroups.AddDesc(err.Error())
	}

	//获取所有的服务
	for _, groupServices := range productInfo.Services {
		for _, service := range groupServices {
			if serviceName != "" && strings.Contains(service.ServiceName, serviceName) {
				allServices = append(allServices, service)
			} else if serviceName == "" {
				allServices = append(allServices, service)
			}
		}
	}
	count = len(allServices)
	//sort services by name
	sort.SliceStable(allServices, func(i, j int) bool { return allServices[i].ServiceName < allServices[j].ServiceName })

	// add updatable field
	latestSvcs, err := repository.GetMaxRevisionsServicesMap(productName, production)
	if err != nil {
		return resp, count, e.ErrListGroups.AddDesc(fmt.Sprintf("failed to find latest services for %s/%s", productName, envName))
	}
	for _, svc := range allServices {
		if latestSvc, ok := latestSvcs[svc.ServiceName]; ok {
			svc.Updatable = svc.Revision < latestSvc.Revision
		}
		svc.DeployStrategy = productInfo.ServiceDeployStrategy[svc.ServiceName]
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		log.Errorf("[%s][%s] failed to get kubeclient error: %v", envName, productName, err)
		return resp, count, e.ErrListGroups.AddDesc(err.Error())
	}
	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return resp, count, e.ErrListGroups.AddDesc(err.Error())
	}
	inf, err := informer.NewInformer(productInfo.ClusterID, productInfo.Namespace, cls)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return resp, count, e.ErrListGroups.AddDesc(err.Error())
	}

	//called from inner api, return all services
	if page == 0 && perPage == 0 {
		resp = envHandleFunc(getProjectType(productName), log).listGroupServices(allServices, envName, productName, inf, productInfo)
		return resp, count, nil
	}

	//针对获取环境状态的接口请求，这里不需要一次性获取所有的服务，先获取十条的数据，有异常的可以直接返回，不需要继续往下获取
	if page == -1 && perPage == -1 {
		resp = listGroupServiceStatus(allServices, envName, productName, inf, productInfo, log)
		return resp, count, nil
	}

	currentPage := page - 1
	if page*perPage < count {
		currentServices = allServices[currentPage*perPage : currentPage*perPage+perPage]
	} else {
		if currentPage*perPage > count {
			return resp, count, nil
		}
		currentServices = allServices[currentPage*perPage:]
	}
	resp = envHandleFunc(getProjectType(productName), log).listGroupServices(currentServices, envName, productName, inf, productInfo)

	respMap := make(map[string]*commonservice.ServiceResp)
	for _, serviceResp := range resp {
		respMap[serviceResp.ServiceName] = serviceResp
	}
	mseService, err := listZadigXMseReleaseServices(productInfo.Namespace, productName, envName, kubeClient)
	if err != nil {
		return resp, count, e.ErrListGroups.AddErr(errors.Wrap(err, "list mse release services"))
	}
	for _, serviceResp := range mseService {
		if _, ok := respMap[serviceResp.ServiceName]; !ok {
			resp = append(resp, serviceResp)
		}
	}
	return resp, count, nil
}

func listZadigXMseReleaseServices(namespace, productName, envName string, client client.Client) ([]*commonservice.ServiceResp, error) {
	selector := labels.Set{types.ZadigReleaseTypeLabelKey: types.ZadigReleaseTypeMseGray}.AsSelector()
	deployments, err := getter.ListDeployments(namespace, selector, client)
	if err != nil {
		return nil, err
	}
	services := make([]*commonservice.ServiceResp, 0)
	for _, deployment := range deployments {
		serviceName := deployment.GetLabels()[types.ZadigReleaseServiceNameLabelKey]
		if serviceName == "" {
			log.Warnf("listZadigXMseReleaseServices: deployment %s/%s has no service name label", deployment.Namespace, deployment.Name)
			continue
		}
		services = append(services, &commonservice.ServiceResp{
			ServiceName: serviceName,
			Type:        setting.K8SDeployType,
			Status: func() string {
				if deployment.Status.Replicas == deployment.Status.ReadyReplicas {
					return setting.PodRunning
				} else {
					return setting.PodUnstable
				}
			}(),
			Images:          wrapper.Deployment(deployment).ImageInfos(),
			ProductName:     productName,
			EnvName:         envName,
			DeployStrategy:  "deploy",
			IsZadigXRelease: true,
		})
	}
	return services, nil
}

func listGroupServiceStatus(allServices []*commonmodels.ProductService, envName, productName string, informer informers.SharedInformerFactory, productInfo *commonmodels.Product, log *zap.SugaredLogger) []*commonservice.ServiceResp {
	var (
		count           = len(allServices)
		currentServices = make([]*commonmodels.ProductService, 0)
		perPage         = setting.PerPage
		resp            = make([]*commonservice.ServiceResp, 0)
	)
	for page := 0; page*perPage < count; page++ {
		if (page+1)*perPage < count {
			currentServices = allServices[page*perPage : page*perPage+perPage]
		} else {
			currentServices = allServices[page*perPage:]
		}
		resp = envHandleFunc(getProjectType(productName), log).listGroupServices(currentServices, envName, productName, informer, productInfo)
		allRunning := true
		for _, serviceResp := range resp {
			// Service是物理机部署时，无需判断状态
			if serviceResp.Type == setting.K8SDeployType && serviceResp.Status != setting.PodRunning && serviceResp.Status != setting.PodSucceeded {
				allRunning = false
				break
			}
		}

		if !allRunning {
			return resp
		}
	}
	return resp
}

func GetIngressInfo(product *commonmodels.Product, service *commonmodels.Service, log *zap.SugaredLogger) *commonservice.IngressInfo {
	var (
		err         error
		ingressInfo = new(commonservice.IngressInfo)
	)
	ingressInfo.HostInfo = make([]resource.HostInfo, 0)

	renderSet := &commonmodels.RenderSet{}
	if product.Render != nil {
		renderSet, err = render.GetRenderSet(product.Render.Name, 0, false, product.EnvName, log)
		if err != nil {
			log.Errorf("GetRenderSet %s error: %v", product.ProductName, err)
			return ingressInfo
		}
	}
	parsedYaml, err := kube.RenderServiceYaml(service.Yaml, product.ProductName, service.ServiceName, renderSet)
	if err != nil {
		log.Errorf("RenderServiceYaml err: %s", err)
		return nil
	}
	// 渲染系统变量键值
	parsedYaml = kube.ParseSysKeys(product.Namespace, product.EnvName, product.ProductName, service.ServiceName, parsedYaml)

	parsedYaml = util.ReplaceWrapLine(parsedYaml)
	yamlContentArray := releaseutil.SplitManifests(parsedYaml)
	hostInfos := make([]resource.HostInfo, 0)

	for _, item := range yamlContentArray {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			log.Warnf("Failed to decode yaml to Unstructured, err: %s", err)
			continue
		}
		switch u.GetKind() {
		case setting.Ingress:
			clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), product.ClusterID)
			if err != nil {
				log.Errorf("failed to init clientset, clusterID: %s", product.ClusterID)
				return nil
			}

			inf, err := informer.NewInformer(product.ClusterID, product.Namespace, clientset)
			if err != nil {
				log.Errorf("failed to create informer from clientset for clusterID: %s, the error is: %s", product.ClusterID, err)
				return nil
			}

			version, err := clientset.Discovery().ServerVersion()
			if err != nil {
				log.Warnf("Failed to determine server version, error is: %s", err)
				continue
			}

			if kubeclient.VersionLessThan122(version) {
				// get the ingress info from kubernetes. For cluster version 1.22- we only search for
				// extensions/v1beta1.
				// FIXME: add networking.k8s.io/v1beta1 & networking.k8s.io/v1 support for cluster 1.22-
				ing, found, err := getter.GetExtensionsV1Beta1Ingress(product.Namespace, u.GetName(), inf)
				if err != nil || !found {
					log.Warnf("no ingress %s found in %s:%s %v", u.GetName(), service.ServiceName, product.Namespace, err)
					continue
				}
				hostInfos = append(hostInfos, wrapper.Ingress(ing).HostInfo()...)
			} else {
				ing, err := getter.GetNetworkingV1Ingress(product.Namespace, u.GetName(), inf)
				if err != nil {
					log.Warnf("no ingress %s found in %s:%s %v", u.GetName(), service.ServiceName, product.Namespace, err)
					continue
				}
				hostInfos = append(hostInfos, wrapper.GetIngressHostInfo(ing)...)
			}
		}
	}
	ingressInfo.HostInfo = hostInfos
	return ingressInfo
}
