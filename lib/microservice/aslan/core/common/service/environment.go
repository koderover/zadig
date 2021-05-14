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
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/lib/internal/kube/resource"
	"github.com/koderover/zadig/lib/internal/kube/wrapper"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	templatemodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/template"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/kube/getter"
	"github.com/koderover/zadig/lib/tool/xlog"
)

// FillProductTemplateValuesYamls 返回renderSet中的renderChart信息
func FillProductTemplateValuesYamls(tmpl *templatemodels.Product, log *xlog.Logger) error {
	if renderSet, err := GetRenderSet(tmpl.ProductName, 0, log); err != nil {
		log.Errorf("Failed to find render set for product template %s", tmpl.ProductName)
		return err
	} else {
		serviceNames := sets.NewString()
		for _, serviceGroup := range tmpl.Services {
			for _, serviceName := range serviceGroup {
				serviceNames.Insert(serviceName)
			}
		}
		tmpl.ChartInfos = make([]*templatemodels.RenderChart, 0)
		for _, renderChart := range renderSet.ChartInfos {
			if !serviceNames.Has(renderChart.ServiceName) {
				continue
			}
			tmpl.ChartInfos = append(tmpl.ChartInfos, &templatemodels.RenderChart{
				ServiceName:  renderChart.ServiceName,
				ChartVersion: renderChart.ChartVersion,
				ValuesYaml:   renderChart.ValuesYaml,
			})
		}
	}

	return nil
}

// 产品列表页服务Response
type ServiceResp struct {
	ServiceName string              `json:"service_name"`
	Type        string              `json:"type"`
	Status      string              `json:"status"`
	Images      []string            `json:"images,omitempty"`
	ProductName string              `json:"product_name"`
	EnvName     string              `json:"env_name"`
	Ingress     *IngressInfo        `json:"ingress"`
	Ready       string              `json:"ready"`
	EnvStatuses []*models.EnvStatus `json:"env_statuses,omitempty"`
}

type IngressInfo struct {
	HostInfo []resource.HostInfo `json:"host_info"`
}

func ListGroupsBySource(envName, productName string, log *xlog.Logger) ([]*ServiceResp, []resource.Ingress, error) {
	var (
		wg             sync.WaitGroup
		resp           = make([]*ServiceResp, 0)
		ingressList    = make([]resource.Ingress, 0)
		matchedIngress = &sync.Map{}
	)
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	productInfo, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return resp, ingressList, e.ErrListGroups.AddDesc(err.Error())
	}

	kubeClient, err := kube.GetKubeClient(productInfo.ClusterId)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return resp, ingressList, e.ErrListGroups.AddDesc(err.Error())
	}

	listServices, err := getter.ListServices(productInfo.Namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", envName, productName, err)
		return resp, ingressList, e.ErrListGroups.AddDesc(err.Error())
	}
	for _, service := range listServices {
		wg.Add(1)

		go func(service *corev1.Service) {
			defer func() {
				wg.Done()
			}()

			productRespInfo := &ServiceResp{
				ServiceName: service.Name,
				EnvName:     envName,
				ProductName: productName,
				Type:        setting.K8SDeployType,
			}

			selector := labels.SelectorFromValidatedSet(service.Spec.Selector)
			productRespInfo.Status, productRespInfo.Ready, productRespInfo.Images = queryExternalPodsStatus(productInfo.Namespace, selector, kubeClient, log)

			if ingresses, err := getter.ListIngresses(productInfo.Namespace, selector, kubeClient); err == nil {
				ingressInfo := new(IngressInfo)
				hostInfos := make([]resource.HostInfo, 0)
				for _, ingress := range ingresses {
					hostInfos = append(hostInfos, wrapper.Ingress(ingress).HostInfo()...)
					matchedIngress.Store(ingress.Name, struct{}{})
				}
				ingressInfo.HostInfo = hostInfos
				productRespInfo.Ingress = ingressInfo
			}

			resp = append(resp, productRespInfo)
		}(service)

	}
	// 等所有的状态都结束
	wg.Wait()

	allIngresses, err := getter.ListIngresses(productInfo.Namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", envName, productName, err)
	}

	for _, ingress := range allIngresses {
		_, ok := matchedIngress.Load(ingress.Name)
		if !ok {
			ingressList = append(ingressList, *wrapper.Ingress(ingress).Resource())
		}
	}

	return resp, ingressList, nil
}

func queryExternalPodsStatus(namespace string, selector labels.Selector, kubeClient client.Client, log *xlog.Logger) (string, string, []string) {
	return kube.GetSelectedPodsInfo(namespace, selector, kubeClient, log)
}
