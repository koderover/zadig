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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
	jsonutil "github.com/koderover/zadig/pkg/util/json"
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
}

type IngressInfo struct {
	HostInfo []resource.HostInfo `json:"host_info"`
}

// ListWorkloadsInEnv returns all workloads in the given env which meat the filter.
// A filter is in this format: a=b,c=d, and it is a fuzzy matching. Which means it will return all records with a field called
// a and the value contain character b.
func ListWorkloadsInEnv(envName, productName, filter string, perPage, page int, log *zap.SugaredLogger) (int, []*ServiceResp, error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	productInfo, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return 0, nil, e.ErrListGroups.AddDesc(err.Error())
	}

	// find project info
	projectInfo, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return 0, nil, e.ErrListGroups.AddDesc(err.Error())
	}

	return ListWorkloads(envName, productInfo.ClusterID, productInfo.Namespace, productName, perPage, page, log,
		func(workloads []*Workload) []*Workload {
			if projectInfo.ProductFeature == nil || projectInfo.ProductFeature.CreateEnvType != setting.SourceFromExternal {
				return workloads
			}

			productServices, err := commonrepo.NewServiceColl().ListExternalServicesBy(productName, envName)
			if err != nil {
				log.Errorf("ListWorkloads ListExternalServicesBy err:%s", err)
				return workloads
			}
			productServiceNames := sets.NewString()
			for _, productService := range productServices {
				productServiceNames.Insert(fmt.Sprintf("%s/%s", productService.WorkloadType, productService.ServiceName))
			}

			var res []*Workload
			for _, workload := range workloads {
				if productServiceNames.Has(fmt.Sprintf("%s/%s", workload.Type, workload.Name)) {
					res = append(res, workload)
				}
			}
			return res
		},
		func(workloads []*Workload) []*Workload {
			data, err := jsonutil.ToJSON(filter)
			if err != nil {
				log.Warnf("Invalid filter, err: %s", err)
				return workloads
			}

			f := &workloadFilter{}
			if err = json.Unmarshal(data, f); err != nil {
				log.Warnf("Invalid filter, err: %s", err)
				return workloads
			}

			// it is a fuzzy matching
			var res []*Workload
			for _, workload := range workloads {
				if strings.Contains(workload.Name, f.Name) {
					res = append(res, workload)
				}
			}
			return res
		})

}

type FilterFunc func(services []*Workload) []*Workload

type workloadFilter struct {
	Name string `json:"name"`
}

type Workload struct {
	EnvName     string `json:"env_name"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	ProductName string `json:"product_name"`
	Spec        corev1.ServiceSpec `json:"-"`
}

func ListWorkloads(envName, clusterID, namespace, productName string, perPage, page int, log *zap.SugaredLogger, filter ...FilterFunc) (int, []*ServiceResp, error) {
	var (
		wg             sync.WaitGroup
		mutex          sync.Mutex
		resp           = make([]*ServiceResp, 0)
		matchedIngress = &sync.Map{}
	)

	kubeClient, err := kube.GetKubeClient(clusterID)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, namespace, err)
		return 0, resp, e.ErrListGroups.AddDesc(err.Error())
	}

	var workLoads []*Workload
	listDeployments, err := getter.ListDeployments(namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", envName, namespace, err)
		return 0, resp, e.ErrListGroups.AddDesc(err.Error())
	}
	for _, v := range listDeployments {
		workLoads = append(workLoads, &Workload{Name: v.Name, Spec: corev1.ServiceSpec{Selector: v.Spec.Selector.MatchLabels}, Type: setting.Deployment})
	}
	statefulSets, err := getter.ListStatefulSets(namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", envName, namespace, err)
		return 0, resp, e.ErrListGroups.AddDesc(err.Error())
	}
	for _, v := range statefulSets {
		workLoads = append(workLoads, &Workload{Name: v.Name, Spec: corev1.ServiceSpec{Selector: v.Spec.Selector.MatchLabels}, Type: setting.StatefulSet})
	}
	// 对于workload过滤
	for _, f := range filter {
		workLoads = f(workLoads)
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

		go func(workload *Workload) {
			defer func() {
				wg.Done()
			}()

			tmpProductName := workload.ProductName
			if tmpProductName == "" && productName != "" {
				tmpProductName = productName
			}

			productRespInfo := &ServiceResp{
				ServiceName:  workload.Name,
				EnvName:      workload.EnvName,
				Type:         setting.K8SDeployType,
				WorkLoadType: workload.Type,
				ProductName:  tmpProductName,
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
			mutex.Lock()
			defer mutex.Unlock()
			resp = append(resp, productRespInfo)
		}(workload)

	}
	// 等所有的状态都结束
	wg.Wait()
	return count, resp, nil
}

func queryExternalPodsStatus(namespace string, selector labels.Selector, kubeClient client.Client, log *zap.SugaredLogger) (string, string, []string) {
	return kube.GetSelectedPodsInfo(namespace, selector, kubeClient, log)
}
