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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/koderover/zadig/pkg/util"
	jsonutil "github.com/koderover/zadig/pkg/util/json"
)

// FillProductTemplateValuesYamls 返回renderSet中的renderChart信息
func FillProductTemplateValuesYamls(tmpl *templatemodels.Product, log *zap.SugaredLogger) error {
	renderSet, err := GetRenderSet(tmpl.ProductName, 0, true, "", log)
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
			ServiceName:    renderChart.ServiceName,
			ChartVersion:   renderChart.ChartVersion,
			ValuesYaml:     renderChart.ValuesYaml,
			OverrideYaml:   renderChart.OverrideYaml,
			OverrideValues: renderChart.OverrideValues,
		})
	}

	return nil
}

// 产品列表页服务Response
type ServiceResp struct {
	ServiceName        string       `json:"service_name"`
	ServiceDisplayName string       `json:"service_display_name"`
	Type               string       `json:"type"`
	Status             string       `json:"status"`
	Images             []string     `json:"images,omitempty"`
	ProductName        string       `json:"product_name"`
	EnvName            string       `json:"env_name"`
	Ingress            *IngressInfo `json:"ingress"`
	//deprecated
	Ready        string              `json:"ready"`
	EnvStatuses  []*models.EnvStatus `json:"env_statuses,omitempty"`
	WorkLoadType string              `json:"workLoadType"`
	Revision     int64               `json:"revision"`
	EnvConfigs   []*models.EnvConfig `json:"env_configs"`
}

type IngressInfo struct {
	HostInfo []resource.HostInfo `json:"host_info"`
}

func UnMarshalSourceDetail(source interface{}) (*models.CreateFromRepo, error) {
	bs, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	ret := &models.CreateFromRepo{}
	err = json.Unmarshal(bs, ret)
	return ret, err
}

func FillGitNamespace(yamlData *templatemodels.CustomYaml) error {
	if yamlData == nil || yamlData.Source != setting.SourceFromGitRepo {
		return nil
	}
	sourceDetail, err := UnMarshalSourceDetail(yamlData.SourceDetail)
	if err != nil {
		return err
	}
	if sourceDetail.GitRepoConfig == nil {
		return nil
	}
	sourceDetail.GitRepoConfig.Namespace = sourceDetail.GitRepoConfig.GetNamespace()
	yamlData.SourceDetail = sourceDetail
	return nil
}

func GetRenderCharts(productName, envName, serviceName string, log *zap.SugaredLogger) ([]*RenderChartArg, *models.RenderSet, error) {

	renderSetName := GetProductEnvNamespace(envName, productName, "")

	opt := &commonrepo.RenderSetFindOption{
		ProductTmpl: productName,
		EnvName:     envName,
		Name:        renderSetName,
	}
	rendersetObj, existed, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil {
		return nil, nil, err
	}

	if !existed {
		return nil, nil, nil
	}

	ret := make([]*RenderChartArg, 0)

	matchedRenderChartModels := make([]*templatemodels.RenderChart, 0)
	if len(serviceName) == 0 {
		matchedRenderChartModels = rendersetObj.ChartInfos
	} else {
		serverList := strings.Split(serviceName, ",")
		stringSet := sets.NewString(serverList...)
		for _, singleChart := range rendersetObj.ChartInfos {
			if !stringSet.Has(singleChart.ServiceName) {
				continue
			}
			matchedRenderChartModels = append(matchedRenderChartModels, singleChart)
		}
	}

	for _, singleChart := range matchedRenderChartModels {
		rcaObj := new(RenderChartArg)
		rcaObj.LoadFromRenderChartModel(singleChart)
		rcaObj.EnvName = envName
		err = FillGitNamespace(rendersetObj.YamlData)
		if err != nil {
			// Note, since user can always reselect the git info, error should not block normal logic
			log.Warnf("failed to fill git namespace data, err: %s", err)
		}
		rcaObj.YamlData = singleChart.OverrideYaml
		ret = append(ret, rcaObj)
	}
	return ret, rendersetObj, nil
}

// fill service display name if necessary
func fillServiceInfo(svcList []*ServiceResp, productInfo *models.Product) {
	if productInfo.Source != setting.SourceFromHelm {
		return
	}
	for _, svc := range svcList {
		svc.ServiceDisplayName = svc.ServiceName
	}
}

// ListWorkloadsInEnv returns all workloads in the given env which meet the filter.
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
				productServiceNames.Insert(productService.ServiceName)
			}
			// add services in external env data
			servicesInExternalEnv, _ := commonrepo.NewServicesInExternalEnvColl().List(&commonrepo.ServicesInExternalEnvArgs{
				ProductName: productName,
				EnvName:     envName,
			})
			for _, serviceInExternalEnv := range servicesInExternalEnv {
				productServiceNames.Insert(serviceInExternalEnv.ServiceName)
			}

			var res []*Workload
			for _, workload := range workloads {
				if productServiceNames.Has(workload.Name) {
					res = append(res, workload)
				}
			}
			return res
		},
	}

	// for helm service, only show deploys/stss created by zadig
	if projectInfo.ProductFeature != nil && projectInfo.ProductFeature.DeployType == setting.HelmDeployType {
		filterArray = append(filterArray, func(workloads []*Workload) []*Workload {
			releaseNameMap, err := GetReleaseNameToServiceNameMap(productInfo)
			if err != nil {
				log.Errorf("failed to generate relase map for product: %s:%s", productInfo.ProductName, productInfo.EnvName)
				return workloads
			}

			var res []*Workload
			for _, workload := range workloads {
				if len(workload.Annotation) == 0 {
					continue
				}
				releaseName := workload.Annotation[setting.HelmReleaseNameAnnotation]
				if _, ok := releaseNameMap[releaseName]; ok {
					res = append(res, workload)
				}
			}
			return res
		})
	}

	if filter != "" {
		filterArray = append(filterArray, func(workloads []*Workload) []*Workload {
			data, err := jsonutil.ToJSON(filter)
			if err != nil {
				log.Errorf("Invalid filter, err: %s", err)
				return workloads
			}

			f := &workloadFilter{}
			if err = json.Unmarshal(data, f); err != nil {
				log.Errorf("Invalid filter, err: %s", err)
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

	fillServiceInfo(resp, productInfo)
	return count, resp, nil
}

type FilterFunc func(services []*Workload) []*Workload

type workloadFilter struct {
	Name            string      `json:"name"`
	ServiceName     string      `json:"serviceName"`
	ServiceNameList sets.String `json:"-"`
}

type wfAlias workloadFilter

func (f *workloadFilter) UnmarshalJSON(data []byte) error {
	aliasData := &wfAlias{}
	if err := json.Unmarshal(data, aliasData); err != nil {
		return err
	}
	f.Name = aliasData.Name
	f.ServiceName = aliasData.ServiceName
	if f.ServiceName != "*" {
		serviceNames := strings.Split(f.ServiceName, "|")
		f.ServiceNameList = sets.NewString(serviceNames...)
	}
	return nil
}

func (f *workloadFilter) Match(workload *Workload) bool {
	if len(f.Name) > 0 {
		if !strings.Contains(workload.Name, f.Name) {
			return false
		}
	}
	if len(f.ServiceNameList) > 0 {
		if !f.ServiceNameList.Has(workload.ServiceName) {
			return false
		}
	}
	return true
}

type Workload struct {
	EnvName     string                 `json:"env_name"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	ProductName string                 `json:"product_name"`
	Spec        corev1.PodTemplateSpec `json:"-"`
	Images      []string               `json:"-"`
	Ready       bool                   `json:"ready"`
	Annotation  map[string]string      `json:"-"`
	ServiceName string                 `json:"service_name"` //serviceName refers to the service defines in zadig
}

// fillServiceName set service name defined in zadig to workloads, this would be helpful for helm release view
// TODO optimize this function, use release.template to filter related resources
func fillServiceName(envName, productName string, workloads []*Workload) error {
	if len(envName) == 0 {
		return nil
	}
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return err
	}
	if productInfo.Source != setting.SourceFromHelm {
		return nil
	}
	releaseNameMap, err := GetReleaseNameToServiceNameMap(productInfo)
	if err != nil {
		return err
	}
	for _, wl := range workloads {
		if chartRelease, ok := wl.Annotation[setting.HelmReleaseNameAnnotation]; ok {
			wl.ServiceName = releaseNameMap[chartRelease]
		}
	}
	return nil
}

func ListWorkloads(envName, clusterID, namespace, productName string, perPage, page int, log *zap.SugaredLogger, filter ...FilterFunc) (int, []*ServiceResp, error) {

	var resp = make([]*ServiceResp, 0)
	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), clusterID)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, namespace, err)
		return 0, resp, e.ErrListGroups.AddDesc(err.Error())
	}
	informer, err := informer.NewInformer(clusterID, namespace, cls)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, namespace, err)
		return 0, resp, e.ErrListGroups.AddDesc(err.Error())
	}
	var workLoads []*Workload
	listDeployments, err := getter.ListDeploymentsWithCache(nil, informer)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", envName, namespace, err)
		return 0, resp, e.ErrListGroups.AddDesc(err.Error())
	}

	for _, v := range listDeployments {
		workLoads = append(workLoads, &Workload{
			Name:       v.Name,
			Spec:       v.Spec.Template,
			Type:       setting.Deployment,
			Images:     wrapper.Deployment(v).ImageInfos(),
			Ready:      wrapper.Deployment(v).Ready(),
			Annotation: v.Annotations,
		})
	}
	statefulSets, err := getter.ListStatefulSetsWithCache(nil, informer)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", envName, namespace, err)
		return 0, resp, e.ErrListGroups.AddDesc(err.Error())
	}
	for _, v := range statefulSets {
		workLoads = append(workLoads, &Workload{
			Name:       v.Name,
			Spec:       v.Spec.Template,
			Type:       setting.StatefulSet,
			Images:     wrapper.StatefulSet(v).ImageInfos(),
			Ready:      wrapper.StatefulSet(v).Ready(),
			Annotation: v.Annotations,
		})
	}

	err = fillServiceName(envName, productName, workLoads)
	// err of getting service name should not block the return of workloads
	if err != nil {
		log.Warnf("failed to set service name for workloads, error: %s", err)
	}

	// 对于workload过滤
	for _, f := range filter {
		workLoads = f(workLoads)
	}

	count := len(workLoads)

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

	hostInfos := make([]resource.HostInfo, 0)
	version, err := cls.Discovery().ServerVersion()
	if err != nil {
		log.Errorf("Failed to get server version info for cluster: %s, the error is: %s", clusterID, err)
		return 0, nil, err
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
	allServices, err := getter.ListServicesWithCache(nil, informer)
	if err != nil {
		log.Errorf("[%s][%s] list service error: %s", envName, namespace, err)
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

		selector := labels.SelectorFromSet(labels.Set(workload.Spec.Labels))
		// Note: In some scenarios, such as environment sharing, there may be more containers in Pod than workload.
		// We call GetSelectedPodsInfo to get the status and readiness to keep same logic with k8s projects
		productRespInfo.Status, productRespInfo.Ready, productRespInfo.Images = kube.GetSelectedPodsInfo(selector, informer, log)

		productRespInfo.Ingress = &IngressInfo{
			HostInfo: findServiceFromIngress(hostInfos, workload, allServices),
		}

		resp = append(resp, productRespInfo)
	}

	return count, resp, nil
}

func findServiceFromIngress(hostInfos []resource.HostInfo, currentWorkload *Workload, allServices []*corev1.Service) []resource.HostInfo {
	if len(allServices) == 0 || len(hostInfos) == 0 {
		return []resource.HostInfo{}
	}
	serviceName := ""
	podLabels := labels.Set(currentWorkload.Spec.Labels)
	for _, svc := range allServices {
		if len(svc.Spec.Selector) == 0 {
			continue
		}
		if labels.SelectorFromValidatedSet(svc.Spec.Selector).Matches(podLabels) {
			serviceName = svc.Name
			break
		}
	}
	if serviceName == "" {
		return []resource.HostInfo{}
	}

	resp := make([]resource.HostInfo, 0)
	for _, hostInfo := range hostInfos {
		for _, backend := range hostInfo.Backends {
			if backend.ServiceName == serviceName {
				resp = append(resp, hostInfo)
				break
			}
		}
	}
	return resp
}

func GetProductUsedTemplateSvcs(prod *models.Product) ([]*models.Service, error) {
	// filter releases, only list releases deployed by zadig
	productName, envName, serviceMap := prod.ProductName, prod.EnvName, prod.GetServiceMap()
	if len(serviceMap) == 0 {
		return nil, nil
	}
	listOpt := &commonrepo.SvcRevisionListOption{
		ProductName:      prod.ProductName,
		ServiceRevisions: make([]*commonrepo.ServiceRevision, 0),
	}
	resp := make([]*models.Service, 0)
	for _, productSvc := range serviceMap {
		if productSvc.ProductName != "" && productSvc.ProductName != prod.ProductName {
			tmplSvc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
				ProductName: productSvc.ProductName,
				ServiceName: productSvc.ServiceName,
				Revision:    productSvc.Revision,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to find service: %s of product: %s, err: %s", productSvc.ServiceName, productSvc.ProductName, err)
			}
			resp = append(resp, tmplSvc)
			continue
		}
		listOpt.ServiceRevisions = append(listOpt.ServiceRevisions, &commonrepo.ServiceRevision{
			ServiceName: productSvc.ServiceName,
			Revision:    productSvc.Revision,
		})
	}
	templateServices, err := commonrepo.NewServiceColl().ListServicesWithSRevision(listOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to list template services for pruduct: %s:%s, err: %s", productName, envName, err)
	}
	return append(resp, templateServices...), nil
}

// GetReleaseNameToServiceNameMap generates mapping relationship: releaseName=>serviceName
func GetReleaseNameToServiceNameMap(prod *models.Product) (map[string]string, error) {
	productName, envName := prod.ProductName, prod.EnvName
	templateServices, err := GetProductUsedTemplateSvcs(prod)
	if err != nil {
		return nil, err
	}
	// map[ReleaseName] => serviceName
	releaseNameMap := make(map[string]string)
	for _, svcInfo := range templateServices {
		releaseNameMap[util.GeneReleaseName(svcInfo.GetReleaseNaming(), productName, prod.Namespace, envName, svcInfo.ServiceName)] = svcInfo.ServiceName
	}
	return releaseNameMap, nil
}

// GetServiceNameToReleaseNameMap generates mapping relationship: serviceName=>releaseName
func GetServiceNameToReleaseNameMap(prod *models.Product) (map[string]string, error) {
	productName, envName := prod.ProductName, prod.EnvName
	templateServices, err := GetProductUsedTemplateSvcs(prod)
	if err != nil {
		return nil, err
	}
	// map[serviceName] => ReleaseName
	releaseNameMap := make(map[string]string)
	for _, svcInfo := range templateServices {
		releaseNameMap[svcInfo.ServiceName] = util.GeneReleaseName(svcInfo.GetReleaseNaming(), productName, prod.Namespace, envName, svcInfo.ServiceName)
	}
	return releaseNameMap, nil
}

// GetHelmServiceName get service name from annotations of resources deployed by helm
// resType currently only support Deployment and StatefulSet
// this function needs to be optimized
func GetHelmServiceName(prod *models.Product, resType, resName string, kubeClient client.Client) (string, error) {
	res := &unstructured.Unstructured{}
	namespace := prod.Namespace

	nameMap, err := GetReleaseNameToServiceNameMap(prod)
	if err != nil {
		return "", err
	}

	res.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    resType,
	})
	found, err := getter.GetResourceInCache(namespace, resName, res, kubeClient)
	if err != nil {
		return "", fmt.Errorf("failed to find resource %s, type %s, err %s", resName, resType, err.Error())
	}
	if !found {
		return "", fmt.Errorf("failed to find resource %s, type %s", resName, resType)
	}
	annotation := res.GetAnnotations()
	if len(annotation) > 0 {
		releaseName := annotation[setting.HelmReleaseNameAnnotation]
		if serviceName, ok := nameMap[releaseName]; ok {
			return serviceName, nil
		}
	}
	return "", fmt.Errorf("failed to get annotation from resource %s, type %s", resName, resType)
}
