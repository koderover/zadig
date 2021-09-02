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
	"sync"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
)

// FillProductTemplateValuesYamls 返回renderSet中的renderChart信息
func FillProductTemplateValuesYamls(tmpl *templatemodels.Product, log *zap.SugaredLogger) error {
	renderSet, err := GetRenderSet(tmpl.ProductName, 0, log)
	if err != nil {
		log.Errorf("Failed to find render set for product template %s", tmpl.ProductName)
		return err
	}
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

	return nil
}

// 产品列表页服务Response
type ServiceResp struct {
	ServiceName  string              `json:"service_name"`
	Type         string              `json:"type"`
	Status       string              `json:"status"`
	Images       []string            `json:"images,omitempty"`
	ProductName  string              `json:"product_name"`
	EnvName      string              `json:"env_name"`
	Ingress      *IngressInfo        `json:"ingress"`
	Ready        string              `json:"ready"`
	EnvStatuses  []*models.EnvStatus `json:"env_statuses,omitempty"`
	WorkLoadType string              `json:"workLoadType"`
	OccupyBy     string              `json:"occupy_by"`
}

type IngressInfo struct {
	HostInfo []resource.HostInfo `json:"host_info"`
}

func ListGroupsBySource(envName, productName string, perPage, page int, log *zap.SugaredLogger) (int, []*ServiceResp, []resource.Ingress, error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	productInfo, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return 0, nil, nil, e.ErrListGroups.AddDesc(err.Error())
	}

	// find project info
	projectInfo, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return 0, nil, nil, e.ErrListGroups.AddDesc(err.Error())
	}
	if projectInfo.ProductFeature != nil && projectInfo.ProductFeature.CreateEnvType != "" {
		return ListK8sWorkLoads(envName, productInfo.ClusterID, productInfo.Namespace, perPage, page, log, func(workloads []*models.Workload) []*models.Workload {
			productServices, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(productName)
			if err != nil {
				log.Errorf("ListK8sWorkLoads ListMaxRevisionsByProduct err:%s", err)
				return workloads
			}
			currentProductServices := sets.NewString()
			for _, productService := range productServices {
				currentProductServices.Insert(productService.ServiceName)
			}

			var resp []*models.Workload
			for _, workload := range workloads {
				if currentProductServices.Has(workload.Name) {
					resp = append(resp, workload)
				}
			}
			return resp
		})
	}
	return ListK8sWorkLoads(envName, productInfo.ClusterID, productInfo.Namespace, perPage, page, log)

}

type FilterFunc func(services []*models.Workload) []*models.Workload

func ListK8sWorkLoads(envName, clusterID, namespace string, perPage, page int, log *zap.SugaredLogger, filter ...FilterFunc) (int, []*ServiceResp, []resource.Ingress, error) {
	var (
		wg             sync.WaitGroup
		resp           = make([]*ServiceResp, 0)
		ingressList    = make([]resource.Ingress, 0)
		matchedIngress = &sync.Map{}
	)

	kubeClient, err := kube.GetKubeClient(clusterID)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, namespace, err)
		return 0, resp, ingressList, e.ErrListGroups.AddDesc(err.Error())
	}

	var workLoads []*models.Workload
	listDeployments, err := getter.ListDeployments(namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", envName, namespace, err)
		return 0, resp, ingressList, e.ErrListGroups.AddDesc(err.Error())
	}
	for _, v := range listDeployments {
		workLoads = append(workLoads, &models.Workload{Name: v.Name, Spec: corev1.ServiceSpec{Selector: v.Spec.Selector.MatchLabels}, Type: setting.Deployment})
	}

	statefulSets, err := getter.ListStatefulSets(namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", envName, namespace, err)
		return 0, resp, ingressList, e.ErrListGroups.AddDesc(err.Error())
	}
	for _, v := range statefulSets {
		workLoads = append(workLoads, &models.Workload{Name: v.Name, Spec: corev1.ServiceSpec{Selector: v.Spec.Selector.MatchLabels}, Type: setting.StatefulSet})
	}
	// 对于workload过滤
	if len(filter) > 0 {
		workLoads = filter[0](workLoads)
	}
	count := len(workLoads)

	// 分页
	if page > 0 && perPage > 0 {
		//将获取到的所有服务按照名称进行排序
		sort.SliceStable(workLoads, func(i, j int) bool { return workLoads[i].Name < workLoads[j].Name })

		start := (page - 1) * perPage
		if start >= count {
			workLoads = nil
		} else if start+perPage >= count {
			workLoads = workLoads[start:]
		} else {
			workLoads = workLoads[start : start+perPage]
		}
	}

	for _, workload := range workLoads {
		wg.Add(1)

		go func(workload *models.Workload) {
			defer func() {
				wg.Done()
			}()

			productRespInfo := &ServiceResp{
				ServiceName:  workload.Name,
				EnvName:      envName,
				Type:         setting.K8SDeployType,
				WorkLoadType: workload.Type,
				OccupyBy:     workload.EnvName,
			}

			selector := labels.SelectorFromValidatedSet(workload.Spec.Selector)
			productRespInfo.Status, productRespInfo.Ready, productRespInfo.Images = queryExternalPodsStatus(namespace, selector, kubeClient, log)

			if ingresses, err := getter.ListIngresses(namespace, selector, kubeClient); err == nil {
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
		}(workload)

	}
	// 等所有的状态都结束
	wg.Wait()
	return count, resp, ingressList, nil
}

func queryExternalPodsStatus(namespace string, selector labels.Selector, kubeClient client.Client, log *zap.SugaredLogger) (string, string, []string) {
	return kube.GetSelectedPodsInfo(namespace, selector, kubeClient, log)
}
