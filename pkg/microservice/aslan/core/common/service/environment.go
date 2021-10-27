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

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

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
	"github.com/koderover/zadig/pkg/util"
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
	ServiceName        string              `json:"service_name"`
	ServiceDisplayName string              `json:"service_display_name"`
	Type               string              `json:"type"`
	Status             string              `json:"status"`
	Images             []string            `json:"images,omitempty"`
	ProductName        string              `json:"product_name"`
	EnvName            string              `json:"env_name"`
	Ingress            *IngressInfo        `json:"ingress"`
	Ready              string              `json:"ready"`
	EnvStatuses        []*models.EnvStatus `json:"env_statuses,omitempty"`
	WorkLoadType       string              `json:"workLoadType"`
}

type IngressInfo struct {
	HostInfo []resource.HostInfo `json:"host_info"`
}

// fill service display name if necessary
func fillServiceDisplayName(svcList []*ServiceResp, productInfo *models.Product) {
	if productInfo.Source == setting.SourceFromHelm {
		for _, svc := range svcList {
			svc.ServiceDisplayName = util.ExtraServiceName(svc.ServiceName, productInfo.Namespace)
		}
	}
}

// ListWorkloadsInEnv returns all workloads in the given env which meat the filter.
// A filter is in this format: a=b,c=d, and it is a fuzzy matching. Which means it will return all records with a field called
// a and the value contain character b.
func ListWorkloadsInEnv(envName, productName, filter string, perPage, page int, log *zap.SugaredLogger) (int, []*ServiceResp, error) {
	log.Infof("Start to list workloads for env %s for project %s", envName, productName)
	defer log.Info("Finish to list workloads")

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

	filterArray := []FilterFunc{
		func(workloads []*Workload) []*Workload {
			if projectInfo.ProductFeature == nil || projectInfo.ProductFeature.CreateEnvType != setting.SourceFromExternal {
				return workloads
			}

			productServices, err := commonrepo.NewServiceColl().ListExternalWorkloadsBy(productName, envName)
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
	}
	if filter != "" {
		filterArray = append(filterArray, func(workloads []*Workload) []*Workload {
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
				if f.Match(workload) {
					res = append(res, workload)
				}
			}
			return res
		})
	}

	count, resp, err := ListWorkloads(envName, productInfo.ClusterID, productInfo.Namespace, productName, perPage, page, log, filterArray...)
	if err != nil {
		return count, resp, err
	}
	fillServiceDisplayName(resp, productInfo)
	return count, resp, nil
}

type FilterFunc func(services []*Workload) []*Workload

type workloadFilter struct {
	Name string `json:"name"`
}

func (f *workloadFilter) Match(workload *Workload) bool {
	return strings.Contains(workload.Name, f.Name)
}

type Workload struct {
	EnvName     string             `json:"env_name"`
	Name        string             `json:"name"`
	Type        string             `json:"type"`
	ProductName string             `json:"product_name"`
	Spec        corev1.ServiceSpec `json:"-"`
	Images      []string           `json:"-"`
	Ready       bool
}

func ListWorkloads(envName, clusterID, namespace, productName string, perPage, page int, log *zap.SugaredLogger, filter ...FilterFunc) (int, []*ServiceResp, error) {
	log.Infof("Start to list workloads in namespace %s", namespace)

	var resp = make([]*ServiceResp, 0)
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
		workLoads = append(workLoads, &Workload{Name: v.Name, Spec: corev1.ServiceSpec{Selector: v.Spec.Selector.MatchLabels}, Type: setting.Deployment, Images: wrapper.Deployment(v).ImageInfos(), Ready: wrapper.Deployment(v).Ready()})
	}
	statefulSets, err := getter.ListStatefulSets(namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", envName, namespace, err)
		return 0, resp, e.ErrListGroups.AddDesc(err.Error())
	}
	for _, v := range statefulSets {
		workLoads = append(workLoads, &Workload{Name: v.Name, Spec: corev1.ServiceSpec{Selector: v.Spec.Selector.MatchLabels}, Type: setting.StatefulSet, Images: wrapper.StatefulSet(v).ImageInfos(), Ready: wrapper.StatefulSet(v).Ready()})
	}

	log.Debugf("Found %d workloads in total", len(workLoads))

	// 对于workload过滤
	for _, f := range filter {
		workLoads = f(workLoads)
	}

	count := len(workLoads)

	log.Debugf("%d matching workloads left after filtering", count)

	//将获取到的所有服务按照名称进行排序
	sort.SliceStable(workLoads, func(i, j int) bool { return workLoads[i].Name < workLoads[j].Name })

	// 分页
	if page > 0 && perPage > 0 {
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
			Images:       workload.Images,
			Ready:        setting.PodReady,
			Status:       setting.PodRunning,
		}
		if !workload.Ready {
			productRespInfo.Status = setting.PodUnstable
			productRespInfo.Ready = setting.PodNotReady
		}
		resp = append(resp, productRespInfo)
	}

	log.Infof("Finish to list workloads in namespace %s", namespace)

	return count, resp, nil
}
