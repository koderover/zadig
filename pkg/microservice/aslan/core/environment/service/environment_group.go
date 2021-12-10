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
	"sort"
	"strings"

	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/util"
)

func ListGroups(serviceName, envName, productName string, perPage, page int, log *zap.SugaredLogger) ([]*commonservice.ServiceResp, int, error) {
	var (
		count           = 0
		allServices     = make([]*commonmodels.ProductService, 0)
		currentServices = make([]*commonmodels.ProductService, 0)
		resp            = make([]*commonservice.ServiceResp, 0)
	)

	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
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
	//将获取到的所有服务按照名称进行排序
	sort.SliceStable(allServices, func(i, j int) bool { return allServices[i].ServiceName < allServices[j].ServiceName })

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return resp, count, e.ErrListGroups.AddDesc(err.Error())
	}

	//这种针对的是获取所有数据的接口，内部调用
	if page == 0 && perPage == 0 {
		resp = envHandleFunc(getProjectType(productName), log).listGroupServices(allServices, envName, productName, kubeClient, productInfo)
		return resp, count, nil
	}

	//针对获取环境状态的接口请求，这里不需要一次性获取所有的服务，先获取十条的数据，有异常的可以直接返回，不需要继续往下获取
	if page == -1 && perPage == -1 {
		// 获取环境的状态
		resp = listGroupServiceStatus(allServices, envName, productName, kubeClient, productInfo, log)
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
	resp = envHandleFunc(getProjectType(productName), log).listGroupServices(currentServices, envName, productName, kubeClient, productInfo)
	return resp, count, nil
}

func listGroupServiceStatus(allServices []*commonmodels.ProductService, envName, productName string, kubeClient client.Client, productInfo *commonmodels.Product, log *zap.SugaredLogger) []*commonservice.ServiceResp {
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
		resp = envHandleFunc(getProjectType(productName), log).listGroupServices(currentServices, envName, productName, kubeClient, productInfo)
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
		renderSet, err = commonservice.GetRenderSet(product.Render.Name, 0, log)
		if err != nil {
			log.Errorf("GetRenderSet %s error: %v", product.ProductName, err)
			return ingressInfo
		}
	}
	parsedYaml := commonservice.RenderValueForString(service.Yaml, renderSet)
	// 渲染系统变量键值
	parsedYaml = kube.ParseSysKeys(product.Namespace, product.EnvName, product.ProductName, service.ServiceName, parsedYaml)

	parsedYaml = util.ReplaceWrapLine(parsedYaml)
	yamlContentArray := releaseutil.SplitManifests(parsedYaml)
	hostInfos := make([]resource.HostInfo, 0)
	for _, item := range yamlContentArray {
		ing, err := serializer.NewDecoder().YamlToIngress([]byte(item))
		if err != nil || ing == nil {
			continue
		}

		hostInfos = append(hostInfos, wrapper.Ingress(ing).HostInfo()...)
	}
	ingressInfo.HostInfo = hostInfos
	return ingressInfo
}
