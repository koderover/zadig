/*
Copyright 2023 The KodeRover Authors.

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
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/version"
	k8sversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
)

type WorkloadImage struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type ResourceCommon struct {
	Name         string           `json:"name"`
	Type         string           `json:"type"`
	Images       []*WorkloadImage `json:"images,omitempty"`
	CreationTime string           `json:"creation_time"`
}

func (rc *ResourceCommon) GetCreationTime() string {
	return rc.CreationTime
}

type WorkloadDeploySts struct {
	*ResourceCommon
	Ready string `json:"ready"`
}

type SortableByCreationTime interface {
	GetCreationTime() string
}

type K8sResourceResp struct {
	Count     int                      `json:"count"`
	Workloads []interface{}            `json:"workloads,omitempty"`
	Resources []SortableByCreationTime `json:"resources,omitempty"`
}

type WorkloadDaemonSet struct {
	*ResourceCommon
	Desired   int `json:"desired"`
	Current   int `json:"current"`
	Ready     int `json:"ready"`
	UpToDate  int `json:"up_to_date"`
	Available int `json:"available"`
}

type WorkloadJob struct {
	*ResourceCommon
	Completions string `json:"completions"`
	Duration    string `json:"duration"`
	Age         string `json:"age"`
}

type WorkloadCronJob struct {
	*ResourceCommon
	Schedule     string `json:"schedule"`
	Suspend      bool   `json:"suspend"`
	Active       int    `json:"active"`
	LastSchedule string `json:"last_schedule"`
	Age          string `json:"age"`
}

type WorkloadItem interface {
	GetName() string
	ImageInfos() []string
}

type ResourceService struct {
	*ResourceCommon
	ServiceType string `json:"service_type"`
	ClusterIP   string `json:"cluster_ip"`
	Ports       string `json:"ports"`
}

type ResourceIngress struct {
	*ResourceCommon
	Class   string `json:"class"`
	Hosts   string `json:"hosts"`
	Address string `json:"address"`
	Ports   string `json:"ports"`
}

type ResourcePVC struct {
	*ResourceCommon
	Status       string `json:"status"`
	Volume       string `json:"volume"`
	Capacity     string `json:"capacity"`
	AccessModes  string `json:"access_modes"`
	StorageClass string `json:"storage_class"`
}

type ResourceConfigMap struct {
	*ResourceCommon
}

type ResourceSecret struct {
	*ResourceCommon
	SecretType string `json:"secret_type"`
}

func VersionLessThan121(ver *k8sversion.Info) bool {
	v121, _ := version.ParseGeneric("v1.21.0")
	currVersion, _ := version.ParseGeneric(ver.String())
	return currVersion.LessThan(v121)
}

func (resp *K8sResourceResp) handlePageFilter(page, pageSize int) *K8sResourceResp {
	if page > 0 && pageSize > 0 {
		start := (page - 1) * pageSize
		if start >= resp.Count {
			resp.Workloads = nil
		} else if start+pageSize >= resp.Count {
			resp.Workloads = resp.Workloads[start:]
		} else {
			resp.Workloads = resp.Workloads[start : start+pageSize]
		}
	}
	return resp
}

func getK8sPodYaml(ns, name string, kc client.Client) (string, error) {
	gvk := schema.GroupVersionKind{
		Group:   "",
		Kind:    "Pod",
		Version: "v1",
	}
	yamlStr, exist, err := getter.GetResourceYamlInCache(ns, name, gvk, kc)
	if err != nil {
		return "", err
	}
	if !exist {
		return "", fmt.Errorf("pod: %s does not exist", name)
	}
	return string(yamlStr), nil
}

func getK8sConfigMapYaml(ns, name string, kc client.Client) (string, error) {
	yamlStr, exist, err := getter.GetConfigMapYaml(ns, name, kc)
	if err != nil {
		return "", err
	}
	if !exist {
		return "", fmt.Errorf("configmap: %s does not exist", name)
	}
	return string(yamlStr), nil
}

func getK8sSecretYaml(ns, name string, kc client.Client) (string, error) {
	yamlStr, exist, err := getter.GetSecretYaml(ns, name, kc)
	if err != nil {
		return "", err
	}
	if !exist {
		return "", fmt.Errorf("secret: %s does not exist", name)
	}
	return string(yamlStr), nil
}

func getK8sPVCYaml(ns, name string, kc client.Client) (string, error) {
	yamlStr, exist, err := getter.GetPVCYaml(ns, name, kc)
	if err != nil {
		return "", err
	}
	if !exist {
		return "", fmt.Errorf("pvc: %s does not exist", name)
	}
	return string(yamlStr), nil
}

func getK8sServiceYaml(ns, name string, kc client.Client) (string, error) {
	yamlStr, exist, err := getter.GetServiceYaml(ns, name, kc)
	if err != nil {
		return "", err
	}
	if !exist {
		return "", fmt.Errorf("service: %s does not exist", name)
	}
	return string(yamlStr), nil
}

func getK8sIngressYaml(ns, name string, kc client.Client, cs *kubernetes.Clientset) (string, error) {
	unstructuredRes, exist, err := getter.GetUnstructuredIngress(ns, name, kc, cs)
	if err != nil {
		return "", err
	}
	if !exist {
		return "", fmt.Errorf("service: %s does not exist", name)
	}

	yamlStr, err := yaml.Marshal(unstructuredRes)
	if err != nil {
		return "", err
	}

	return string(yamlStr), nil
}

func getWorkloadCommonInfo(item WorkloadItem, workloadType string, creationTime time.Time) *ResourceCommon {
	resp := &ResourceCommon{
		Name:   item.GetName(),
		Type:   workloadType,
		Images: nil,
	}
	for _, image := range item.ImageInfos() {
		resp.Images = append(resp.Images, &WorkloadImage{
			Name:  "",
			Image: image,
		})
	}
	resp.CreationTime = creationTime.Format("2006-01-02 15:04:05")
	return resp
}

func ListDeployments(page, pageSize int, namespace string, kc client.Client, informer informers.SharedInformerFactory) (*K8sResourceResp, error) {
	deployments, err := getter.ListDeploymentsWithCache(nil, informer)
	if err != nil {
		return nil, err
	}

	resp := &K8sResourceResp{
		Count:     len(deployments),
		Workloads: make([]interface{}, 0),
	}

	for _, deploy := range deployments {
		wrappedRes := wrapper.Deployment(deploy)
		resp.Workloads = append(resp.Workloads, &WorkloadDeploySts{
			ResourceCommon: getWorkloadCommonInfo(wrappedRes, setting.Deployment, deploy.CreationTimestamp.Time),
			Ready:          fmt.Sprintf("%d/%d", deploy.Status.AvailableReplicas, deploy.Status.Replicas),
		})
	}

	return resp.handlePageFilter(page, pageSize), nil
}

func ListStatefulSets(page, pageSize int, namespace string, kc client.Client, informer informers.SharedInformerFactory) (*K8sResourceResp, error) {
	stss, err := getter.ListStatefulSetsWithCache(nil, informer)
	if err != nil {
		return nil, err
	}

	resp := &K8sResourceResp{
		Count:     len(stss),
		Workloads: make([]interface{}, 0),
	}

	for _, sts := range stss {
		wrappedRes := wrapper.StatefulSet(sts)
		resp.Workloads = append(resp.Workloads, &WorkloadDeploySts{
			ResourceCommon: getWorkloadCommonInfo(wrappedRes, setting.StatefulSet, sts.CreationTimestamp.Time),
			Ready:          fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, sts.Status.Replicas),
		})
	}
	return resp.handlePageFilter(page, pageSize), nil
}

func ListDaemonSets(page, pageSize int, namespace string, kc client.Client) (*K8sResourceResp, error) {
	dss, err := getter.ListDaemonsets(namespace, nil, kc)
	if err != nil {
		return nil, err
	}

	resp := &K8sResourceResp{
		Count:     len(dss),
		Workloads: make([]interface{}, 0),
	}

	for _, ds := range dss {
		wrappedRes := wrapper.Daemenset(ds)
		resp.Workloads = append(resp.Workloads, &WorkloadDaemonSet{
			ResourceCommon: getWorkloadCommonInfo(wrappedRes, "DaemonSet", ds.CreationTimestamp.Time),
			Desired:        int(ds.Status.DesiredNumberScheduled),
			Current:        int(ds.Status.CurrentNumberScheduled),
			Ready:          int(ds.Status.NumberReady),
			UpToDate:       int(ds.Status.UpdatedNumberScheduled),
			Available:      int(ds.Status.NumberAvailable),
		})
	}
	return resp.handlePageFilter(page, pageSize), nil
}

func ListJobs(page, pageSize int, namespace string, kc client.Client) (*K8sResourceResp, error) {
	jobs, err := getter.ListJobs(namespace, nil, kc)
	if err != nil {
		return nil, err
	}

	resp := &K8sResourceResp{
		Count:     len(jobs),
		Workloads: make([]interface{}, 0),
	}

	for _, job := range jobs {
		wrappedRes := wrapper.Job(job)
		resp.Workloads = append(resp.Workloads, &WorkloadJob{
			ResourceCommon: getWorkloadCommonInfo(wrappedRes, setting.Job, job.CreationTimestamp.Time),
			Completions:    fmt.Sprintf("%d/%d", job.Status.Succeeded, *job.Spec.Completions),
			Duration:       wrappedRes.GetDuration(),
			Age:            wrappedRes.GetAge(),
		})
	}
	return resp.handlePageFilter(page, pageSize), nil
}

func ListCronJobs(page, pageSize int, clusterID, namespace string, kc client.Client) (*K8sResourceResp, error) {
	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), clusterID)
	if err != nil {
		return nil, err
	}

	k8sServerVersion, err := cls.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	resp := &K8sResourceResp{
		Workloads: make([]interface{}, 0),
	}

	if !VersionLessThan121(k8sServerVersion) {
		cronJobs, err := getter.ListCronJobs(namespace, nil, kc)
		if err != nil {
			return nil, err
		}
		for _, job := range cronJobs {
			wrappedRes := wrapper.CronJob(job)
			suspend := false
			if job.Spec.Suspend != nil {
				suspend = *job.Spec.Suspend
			}
			lastSchedule := ""
			if job.Status.LastScheduleTime != nil {
				lastSchedule = duration.HumanDuration(time.Now().Sub(job.Status.LastScheduleTime.Time))
			}
			resp.Workloads = append(resp.Workloads, &WorkloadCronJob{
				ResourceCommon: getWorkloadCommonInfo(wrappedRes, setting.CronJob, job.CreationTimestamp.Time),
				Schedule:       job.Spec.Schedule,
				Suspend:        suspend,
				Active:         len(job.Status.Active),
				LastSchedule:   lastSchedule,
				Age:            wrappedRes.GetAge(),
			})
		}
		resp.Count += len(cronJobs)
	} else {
		cronJobV1Betas, err := getter.ListCronJobsV1Beta(namespace, nil, kc)
		if err != nil {
			return nil, err
		}
		for _, job := range cronJobV1Betas {
			wrappedRes := wrapper.CronJobV1Beta(job)
			suspend := false
			if job.Spec.Suspend != nil {
				suspend = *job.Spec.Suspend
			}
			lastSchedule := ""
			if job.Status.LastScheduleTime != nil {
				lastSchedule = duration.HumanDuration(time.Now().Sub(job.Status.LastScheduleTime.Time))
			}
			resp.Workloads = append(resp.Workloads, &WorkloadCronJob{
				ResourceCommon: getWorkloadCommonInfo(wrappedRes, setting.CronJob, job.CreationTimestamp.Time),
				Schedule:       job.Spec.Schedule,
				Suspend:        suspend,
				Active:         len(job.Status.Active),
				LastSchedule:   lastSchedule,
				Age:            wrappedRes.GetAge(),
			})
		}
		resp.Count += len(cronJobV1Betas)
	}

	return resp.handlePageFilter(page, pageSize), nil
}

func ListServices(page, pageSize int, namespace string, kc client.Client, informer informers.SharedInformerFactory) (*K8sResourceResp, error) {
	services, err := getter.ListServicesWithCache(nil, informer)
	if err != nil {
		return nil, err
	}

	resp := &K8sResourceResp{
		Count:     len(services),
		Resources: make([]SortableByCreationTime, 0),
	}

	for _, svc := range services {
		portStr := make([]string, 0)
		for _, port := range svc.Spec.Ports {
			if port.NodePort > 0 {
				portStr = append(portStr, fmt.Sprintf("%d:%d/%s", port.Port, port.NodePort, port.Protocol))
			} else {
				portStr = append(portStr, fmt.Sprintf("%d/%s", port.Port, port.Protocol))
			}
		}
		resp.Resources = append(resp.Resources, &ResourceService{
			ResourceCommon: &ResourceCommon{
				Name:         svc.Name,
				Type:         setting.Service,
				CreationTime: svc.CreationTimestamp.Time.Format("2006-01-02 15:04:05"),
			},
			ServiceType: string(svc.Spec.Type),
			ClusterIP:   svc.Spec.ClusterIP,
			Ports:       strings.Join(portStr, ","),
		})
	}

	return resp.handlePageFilter(page, pageSize), nil
}

func ListIngressOverview(page, pageSize int, clusterID, namespace string, kc client.Client, log *zap.SugaredLogger) (*K8sResourceResp, error) {
	cliSet, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), clusterID)
	if err != nil {
		return nil, err
	}
	version, err := cliSet.Discovery().ServerVersion()
	if err != nil {
		log.Errorf("Failed to get server version info for cluster: %s, the error is: %s", clusterID, err)
		return nil, err
	}

	ingresses, err := getter.ListIngresses(namespace, kc, kubeclient.VersionLessThan122(version))
	if err != nil {
		return nil, err
	}

	resp := &K8sResourceResp{
		Count:     len(ingresses.Items),
		Resources: make([]SortableByCreationTime, 0),
	}

	for _, ingress := range ingresses.Items {
		ingress.SetManagedFields(nil)
		ingress.SetResourceVersion("")

		if ingress.GetAPIVersion() == v1.SchemeGroupVersion.String() {
			itemJson, err := ingress.MarshalJSON()
			if err != nil {
				return nil, err
			}
			itemIngress := &v1.Ingress{}
			err = json.Unmarshal(itemJson, itemIngress)
			if err != nil {
				return nil, err
			}

			hosts := make([]string, 0)
			for _, rule := range itemIngress.Spec.Rules {
				hosts = append(hosts, rule.Host)
			}
			hostInfo := strings.Join(hosts, ",")

			address, ports := "", ""
			for _, addr := range itemIngress.Status.LoadBalancer.Ingress {
				address += addr.IP + ","
				for _, portStatus := range addr.Ports {
					if portStatus.Error == nil {
						ports += string(portStatus.Port) + ","
					}
				}
			}
			address = strings.TrimSuffix(address, ",")
			ports = strings.TrimSuffix(ports, ",")
			if len(address) > 0 && len(ports) == 0 {
				ports = "80"
			}

			ingressClassName := ""
			if itemIngress.Spec.IngressClassName != nil {
				ingressClassName = *itemIngress.Spec.IngressClassName
			}

			resp.Resources = append(resp.Resources, &ResourceIngress{
				ResourceCommon: &ResourceCommon{
					Name:         ingress.GetName(),
					Type:         setting.Ingress,
					CreationTime: ingress.GetCreationTimestamp().Time.Format("2006-01-02 15:04:05"),
				},
				Hosts:   hostInfo,
				Class:   ingressClassName,
				Address: address,
				Ports:   ports,
			})
		}

		if ingress.GetAPIVersion() == extensionsv1beta1.SchemeGroupVersion.String() {
			itemJson, err := ingress.MarshalJSON()
			if err != nil {
				return nil, err
			}
			itemIngress := &extensionsv1beta1.Ingress{}
			err = json.Unmarshal(itemJson, itemIngress)
			if err != nil {
				return nil, err
			}
			hostInfo, ports := "", ""
			for _, rule := range itemIngress.Spec.Rules {
				hostInfo += rule.Host + ","
				if rule.HTTP != nil {
					for _, path := range rule.HTTP.Paths {
						ports += path.Backend.ServicePort.String() + ","
					}
				}
			}
			hostInfo = strings.TrimSuffix(hostInfo, ",")
			address := ""
			for _, addr := range itemIngress.Status.LoadBalancer.Ingress {
				address += addr.IP + ","
			}
			address = strings.TrimSuffix(address, ",")
			ports = strings.TrimSuffix(ports, ",")

			resp.Resources = append(resp.Resources, &ResourceIngress{
				ResourceCommon: &ResourceCommon{
					Name:         ingress.GetName(),
					Type:         setting.Ingress,
					CreationTime: ingress.GetCreationTimestamp().Time.Format("2006-01-02 15:04:05"),
				},
				Hosts:   hostInfo,
				Address: address,
				Ports:   ports,
			})
		}
	}

	sort.SliceStable(resp.Resources, func(i, j int) bool {
		return resp.Resources[i].GetCreationTime() < resp.Resources[j].GetCreationTime()
	})

	return resp.handlePageFilter(page, pageSize), nil
}

func ListPVCs(page, pageSize int, namespace string, kc client.Client) (*K8sResourceResp, error) {
	pvcs, err := getter.ListPvcs(namespace, nil, kc)
	if err != nil {
		return nil, err
	}

	resp := &K8sResourceResp{
		Count:     len(pvcs),
		Resources: make([]SortableByCreationTime, 0),
	}
	for _, pvc := range pvcs {
		accessModes := make([]string, 0)
		for _, am := range pvc.Spec.AccessModes {
			accessModes = append(accessModes, string(am))
		}
		resPvc := &ResourcePVC{
			ResourceCommon: &ResourceCommon{
				Name:         pvc.Name,
				Type:         setting.PersistentVolumeClaim,
				CreationTime: pvc.CreationTimestamp.Time.Format("2006-01-02 15:04:05"),
			},
			Status:      string(pvc.Status.Phase),
			Volume:      pvc.Spec.VolumeName,
			Capacity:    pvc.Spec.Resources.Requests.Storage().String(),
			AccessModes: strings.Join(accessModes, ","),
		}
		if pvc.Spec.StorageClassName != nil {
			resPvc.StorageClass = *pvc.Spec.StorageClassName
		}
		resp.Resources = append(resp.Resources, resPvc)
	}
	return resp.handlePageFilter(page, pageSize), nil
}

func ListConfigMapOverview(page, pageSize int, namespace string, kc client.Client) (*K8sResourceResp, error) {
	configMaps, err := getter.ListConfigMaps(namespace, nil, kc)
	if err != nil {
		return nil, err
	}

	resp := &K8sResourceResp{
		Count:     len(configMaps),
		Resources: make([]SortableByCreationTime, 0),
	}
	for _, configMap := range configMaps {
		resp.Resources = append(resp.Resources, &ResourceConfigMap{
			ResourceCommon: &ResourceCommon{
				Name:         configMap.Name,
				Type:         setting.ConfigMap,
				CreationTime: configMap.CreationTimestamp.Time.Format("2006-01-02 15:04:05"),
			},
		})
	}
	return resp.handlePageFilter(page, pageSize), nil
}

func ListK8sSecretOverview(page, pageSize int, namespace string, kc client.Client) (*K8sResourceResp, error) {
	secrets, err := getter.ListSecrets(namespace, kc)
	if err != nil {
		return nil, err
	}

	resp := &K8sResourceResp{
		Count:     len(secrets),
		Resources: make([]SortableByCreationTime, 0),
	}
	for _, secret := range secrets {
		resp.Resources = append(resp.Resources, &ResourceSecret{
			ResourceCommon: &ResourceCommon{
				Name:         secret.Name,
				Type:         setting.Secret,
				CreationTime: secret.CreationTimestamp.Time.Format("2006-01-02 15:04:05"),
			},
			SecretType: string(secret.Type),
		})
	}
	return resp.handlePageFilter(page, pageSize), nil
}

func getRelatedIngress(namespace string, services []*resource.Service, kubeClient client.Client, cs *kubernetes.Clientset, log *zap.SugaredLogger) []*resource.Ingress {
	if len(services) == 0 {
		return nil
	}

	version, err := cs.Discovery().ServerVersion()
	if err != nil {
		log.Errorf("failed to get k8s server version, err: %s", err)
		return nil
	}

	k8sIngresses, err := getter.ListIngresses(namespace, kubeClient, kubeclient.VersionLessThan122(version))
	if err != nil {
		log.Errorf("failed to list ingresses, err: %s", err)
		return nil
	}

	serviceMap := make(map[string]*resource.Service)
	for _, svc := range services {
		serviceMap[svc.Name] = svc
	}

	ingresses := make([]*resource.Ingress, 0)
	for _, k8sIngresses := range k8sIngresses.Items {

		k8sIngresses.SetManagedFields(nil)
		k8sIngresses.SetResourceVersion("")

		if k8sIngresses.GetAPIVersion() == v1.SchemeGroupVersion.String() {
			itemJson, err := k8sIngresses.MarshalJSON()
			if err != nil {
				log.Errorf("failed to marshal ingress to json, err: %s", err)
				continue
			}
			itemIngress := &v1.Ingress{}
			err = json.Unmarshal(itemJson, itemIngress)
			if err != nil {
				log.Errorf("failed to unmarshal to ingress, err: %s", err)
				continue
			}

			var hostInfos []resource.HostInfo
			for _, rule := range itemIngress.Spec.Rules {
				if rule.HTTP == nil {
					continue
				}
				info := resource.HostInfo{
					Host:     rule.Host,
					Backends: []resource.Backend{},
				}
				for _, path := range rule.HTTP.Paths {
					if svc, ok := serviceMap[path.Backend.Service.Name]; !ok {
						continue
					} else {
						for _, port := range svc.Ports {
							if port.ServicePort == path.Backend.Service.Port.Number {
								backend := resource.Backend{
									ServiceName: path.Backend.Service.Name,
									ServicePort: path.Backend.Service.Port.String(),
								}
								info.Backends = append(info.Backends, backend)
								break
							}
						}
					}
				}
				if len(info.Backends) > 0 {
					hostInfos = append(hostInfos, info)
				}
			}
			if len(hostInfos) > 0 {
				resp := &resource.Ingress{
					Name:     itemIngress.Name,
					Labels:   itemIngress.Labels,
					HostInfo: hostInfos,
				}
				for _, lb := range itemIngress.Status.LoadBalancer.Ingress {
					resp.IPs = append(resp.IPs, lb.IP)
				}
				ingresses = append(ingresses, resp)
			}
		}

		if k8sIngresses.GetAPIVersion() == extensionsv1beta1.SchemeGroupVersion.String() {
			itemJson, err := k8sIngresses.MarshalJSON()
			if err != nil {
				log.Errorf("failed to marshal ingress to json, err: %s", err)
				continue
			}
			itemIngress := &extensionsv1beta1.Ingress{}
			err = json.Unmarshal(itemJson, itemIngress)
			if err != nil {
				log.Errorf("failed to unmarshal to ingress, err: %s", err)
				continue
			}

			var hostInfos []resource.HostInfo
			for _, rule := range itemIngress.Spec.Rules {
				if rule.HTTP == nil {
					continue
				}
				info := resource.HostInfo{
					Host:     rule.Host,
					Backends: []resource.Backend{},
				}
				for _, path := range rule.HTTP.Paths {
					if svc, ok := serviceMap[path.Backend.ServiceName]; !ok {
						continue
					} else {
						for _, port := range svc.Ports {
							if fmt.Sprintf("%d", port.ServicePort) == path.Backend.ServicePort.String() {
								backend := resource.Backend{
									ServiceName: path.Backend.ServiceName,
									ServicePort: path.Backend.ServicePort.String(),
								}
								info.Backends = append(info.Backends, backend)
								break
							}
						}
					}
				}
				if len(info.Backends) > 0 {
					hostInfos = append(hostInfos, info)
				}
			}
			if len(hostInfos) > 0 {
				resp := &resource.Ingress{
					Name:     itemIngress.Name,
					Labels:   itemIngress.Labels,
					HostInfo: hostInfos,
				}
				for _, lb := range itemIngress.Status.LoadBalancer.Ingress {
					resp.IPs = append(resp.IPs, lb.IP)
				}
				ingresses = append(ingresses, resp)
			}
		}
	}
	return ingresses
}

func getRelatedServices(namespace string, kubeClient client.Client, podTemplateLabels map[string]string, log *zap.SugaredLogger) []*resource.Service {
	k8sServices, err := getter.ListServices(namespace, nil, kubeClient)
	if err != nil {
		log.Warnf("failed to get service in ns: %s, err: %s", namespace, err)
		return nil
	}
	services := make([]*resource.Service, 0)
	podLabels := labels.Set(podTemplateLabels)
	for _, svc := range k8sServices {
		if labels.SelectorFromValidatedSet(svc.Spec.Selector).Matches(podLabels) {
			services = append(services, wrapper.Service(svc).Resource())
		}
	}
	return services
}

func getDeployWorkloadResource(d *appsv1.Deployment, matchLabels map[string]string, kubeClient client.Client, cs *kubernetes.Clientset, log *zap.SugaredLogger) (*resource.Workload, []*resource.Service, []*resource.Ingress) {
	pods, err := getter.ListPods(d.Namespace, labels.SelectorFromValidatedSet(matchLabels), kubeClient)
	if err != nil {
		log.Warnf("Failed to get pods, err: %s", err)
	}
	services := getRelatedServices(d.Namespace, kubeClient, d.Spec.Template.GetLabels(), log)
	ingresses := getRelatedIngress(d.Namespace, services, kubeClient, cs, log)
	return wrapper.Deployment(d).WorkloadResource(pods), getRelatedServices(d.Namespace, kubeClient, d.Spec.Template.GetLabels(), log), ingresses
}

func getStsWorkloadResource(s *appsv1.StatefulSet, matchLabels map[string]string, kubeClient client.Client, cs *kubernetes.Clientset, log *zap.SugaredLogger) (*resource.Workload, []*resource.Service, []*resource.Ingress) {
	pods, err := getter.ListPods(s.Namespace, labels.SelectorFromValidatedSet(matchLabels), kubeClient)
	if err != nil {
		log.Warnf("Failed to get pods, err: %s", err)
	}
	services := getRelatedServices(s.Namespace, kubeClient, s.Spec.Template.GetLabels(), log)
	ingresses := getRelatedIngress(s.Namespace, services, kubeClient, cs, log)
	return wrapper.StatefulSet(s).WorkloadResource(pods), services, ingresses
}

func getDaemonSetWorkloadResource(d *appsv1.DaemonSet, matchLabels map[string]string, kubeClient client.Client, cs *kubernetes.Clientset, log *zap.SugaredLogger) (*resource.Workload, []*resource.Service, []*resource.Ingress) {
	pods, err := getter.ListPods(d.Namespace, labels.SelectorFromValidatedSet(matchLabels), kubeClient)
	if err != nil {
		log.Warnf("Failed to get pods, err: %s", err)
	}
	services := getRelatedServices(d.Namespace, kubeClient, d.Spec.Template.GetLabels(), log)
	ingresses := getRelatedIngress(d.Namespace, services, kubeClient, cs, log)
	return wrapper.Daemenset(d).WorkloadResource(pods), services, ingresses
}

func getJobWorkloadResource(d *batchv1.Job, matchLabels map[string]string, kubeClient client.Client, cs *kubernetes.Clientset, log *zap.SugaredLogger) (*resource.Workload, []*resource.Service, []*resource.Ingress) {
	pods, err := getter.ListPods(d.Namespace, labels.SelectorFromValidatedSet(matchLabels), kubeClient)
	if err != nil {
		log.Warnf("Failed to get pods, err: %s", err)
	}
	services := getRelatedServices(d.Namespace, kubeClient, d.Spec.Template.GetLabels(), log)
	ingresses := getRelatedIngress(d.Namespace, services, kubeClient, cs, log)
	return wrapper.Job(d).WorkloadResource(pods), services, ingresses
}

func getK8sDeploymentYaml(ns, name string, kc client.Client) (string, error) {
	bs, exist, err := getter.GetDeploymentYaml(ns, name, kc)
	if err != nil {
		return "", err
	}
	if !exist {
		return "", fmt.Errorf("deploy: %s does not exist", name)
	}
	return string(bs), err
}

func GetDeployWorkloadData(ns, name string, kc client.Client, cs *kubernetes.Clientset, log *zap.SugaredLogger) (*WorkloadCommonData, error) {
	object, exist, err := getter.GetDeployment(ns, name, kc)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, fmt.Errorf("deploy: %s does not exist", name)
	}
	resp := &WorkloadCommonData{
		Name: name,
	}

	resp.WorkloadDetail, resp.Services, resp.Ingresses = getDeployWorkloadResource(object, object.Spec.Selector.MatchLabels, kc, cs, log)
	return resp, nil
}

func getK8sStsYaml(ns, name string, kc client.Client) (string, error) {
	bs, exist, err := getter.GetStatefulSetYaml(ns, name, kc)
	if err != nil {
		return "", err
	}
	if !exist {
		return "", fmt.Errorf("statefulSet: %s does not exist", name)
	}
	return string(bs), err
}

func GetStsWorkloadData(ns, name string, kc client.Client, cs *kubernetes.Clientset, log *zap.SugaredLogger) (*WorkloadCommonData, error) {
	object, exist, err := getter.GetStatefulSet(ns, name, kc)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, fmt.Errorf("statefulset: %s does not exist", name)
	}
	resp := &WorkloadCommonData{
		Name: name,
	}
	resp.WorkloadDetail, resp.Services, resp.Ingresses = getStsWorkloadResource(object, object.Spec.Selector.MatchLabels, kc, cs, log)
	return resp, nil
}

func getK8sDaemonSetYaml(ns, name string, kc client.Client) (string, error) {
	bs, exist, err := getter.GetDaemonSetYaml(ns, name, kc)
	if err != nil {
		return "", err
	}
	if !exist {
		return "", fmt.Errorf("daemonSet: %s does not exist", name)
	}
	return string(bs), err
}

func GetDsWorkloadData(ns, name string, kc client.Client, cs *kubernetes.Clientset, log *zap.SugaredLogger) (*WorkloadCommonData, error) {
	object, exist, err := getter.GetDaemonSet(ns, name, kc)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, fmt.Errorf("daemonset: %s does not exist", name)
	}
	resp := &WorkloadCommonData{
		Name: name,
	}
	resp.WorkloadDetail, resp.Services, resp.Ingresses = getDaemonSetWorkloadResource(object, object.Spec.Selector.MatchLabels, kc, cs, log)
	return resp, nil
}

func getK8sJobYaml(ns, name string, kc client.Client) (string, error) {
	bs, exist, err := getter.GetJobYaml(ns, name, kc)
	if err != nil {
		return "", err
	}
	if !exist {
		return "", fmt.Errorf("job: %s does not exist", name)
	}
	return string(bs), err
}

func GetJobData(ns, name string, kc client.Client, cs *kubernetes.Clientset, log *zap.SugaredLogger) (*WorkloadCommonData, error) {
	object, exist, err := getter.GetJob(ns, name, kc)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, fmt.Errorf("job: %s does not exist", name)
	}
	resp := &WorkloadCommonData{
		Name: name,
	}
	resp.WorkloadDetail, resp.Services, resp.Ingresses = getJobWorkloadResource(object, object.Spec.Selector.MatchLabels, kc, cs, log)
	return resp, nil
}

func getK8sCronJobYaml(ns, name string, kc client.Client, cs *kubernetes.Clientset) (string, error) {
	k8sServerVersion, err := cs.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	bs, exist, err := getter.GetCronJobYaml(ns, name, kc, VersionLessThan121(k8sServerVersion))
	if err != nil {
		return "", err
	}
	if !exist {
		return "", fmt.Errorf("cronJob: %s does not exist", name)
	}
	return string(bs), err
}
