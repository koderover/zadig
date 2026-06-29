/*
Copyright 2026 The KodeRover Authors.

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
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/v2/pkg/util"
	"go.uber.org/zap"
)

const (
	TopologySyncStatusSynced    = "synced"
	TopologySyncStatusSyncing   = "syncing"
	TopologySyncStatusOutOfSync = "out_of_sync"
	TopologySyncStatusUnknown   = "unknown"

	TopologyHealthStatusHealthy     = "healthy"
	TopologyHealthStatusProgressing = "progressing"
	TopologyHealthStatusDegraded    = "degraded"
	TopologyHealthStatusUnknown     = "unknown"

	TopologyActionViewYAML    = "view_yaml"
	TopologyActionViewEvents  = "view_events"
	TopologyActionViewLogs    = "view_logs"
	TopologyActionSyncNode    = "sync_node"
	TopologyActionViewDiff    = "view_diff"
	TopologyActionRefreshNode = "refresh_node"
	TopologyActionRestartPod  = "restart_pod"
)

type TopologyArgs struct {
	ProjectName string
	EnvName     string
	Production  bool
	ServiceName string
	NodeID      string
	Refresh     bool
}

type TopologySyncRequest struct {
	ServiceNames []string `json:"service_names"`
	Strategy     string   `json:"strategy"`
}

type TopologySyncResponse struct {
	Triggered        bool   `json:"triggered"`
	AffectedServices int    `json:"affected_services"`
	Message          string `json:"message"`
}

type TopologyResponse struct {
	Env      *TopologyEnvSummary `json:"env"`
	Summary  *TopologySummary    `json:"summary"`
	Nodes    []*TopologyNode     `json:"nodes"`
	Warnings []*TopologyWarning  `json:"warnings"`
	Edges    []*TopologyEdge     `json:"edges"`
}

type TopologyEnvSummary struct {
	ProjectName     string `json:"project_name"`
	EnvName         string `json:"env_name"`
	Production      bool   `json:"production"`
	Namespace       string `json:"namespace"`
	SyncStatus      string `json:"sync_status"`
	HealthStatus    string `json:"health_status"`
	DisplayStatus   string `json:"display_status"`
	Reason          string `json:"reason"`
	Message         string `json:"message"`
	LastSyncTime    int64  `json:"last_sync_time"`
	LastSyncStatus  string `json:"last_sync_status"`
	LastRefreshTime int64  `json:"last_refresh_time"`
}

type TopologySummary struct {
	TotalNodes      int `json:"total_nodes"`
	Synced          int `json:"synced"`
	Syncing         int `json:"syncing"`
	OutOfSync       int `json:"out_of_sync"`
	Unknown         int `json:"unknown"`
	Healthy         int `json:"healthy"`
	Progressing     int `json:"progressing"`
	Degraded        int `json:"degraded"`
	RestartablePods int `json:"restartable_pods"`
}

type TopologyNode struct {
	ID            string   `json:"id"`
	ServiceName   string   `json:"service_name"`
	Kind          string   `json:"kind"`
	Group         string   `json:"group"`
	Version       string   `json:"version"`
	Namespace     string   `json:"namespace"`
	Name          string   `json:"name"`
	ParentIDs     []string `json:"parent_ids"`
	ChildIDs      []string `json:"child_ids"`
	SyncStatus    string   `json:"sync_status"`
	HealthStatus  string   `json:"health_status"`
	DisplayStatus string   `json:"display_status"`
	Ready         string   `json:"ready"`
	Reason        string   `json:"reason"`
	Message       string   `json:"message"`
	Actions       []string `json:"actions"`
}

type TopologyEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
}

type TopologyWarning struct {
	Type          string   `json:"type"`
	Message       string   `json:"message"`
	AffectedNodes []string `json:"affected_nodes"`
}

type TopologyNodeResponse struct {
	Node        *TopologyNode `json:"node"`
	RefreshedAt int64         `json:"refreshed_at"`
}

type TopologyDiffResponse struct {
	NodeID       string `json:"node_id"`
	ServiceName  string `json:"service_name"`
	ExpectedYAML string `json:"expected_yaml"`
	LiveYAML     string `json:"live_yaml"`
	SyncStatus   string `json:"sync_status"`
	Message      string `json:"message"`
}

type topologyBuilder struct {
	args         *TopologyArgs
	product      *commonmodels.Product
	kubeClient   client.Client
	clientset    kubernetes.Interface
	versionLess  bool
	now          int64
	nodes        map[string]*TopologyNode
	live         map[string]*unstructured.Unstructured
	expected     map[string]*expectedResource
	serviceOwned map[string]string
	warnings     []*TopologyWarning
	edges        []*TopologyEdge
}

type expectedResource struct {
	ServiceName string
	Manifest    string
	Object      *unstructured.Unstructured
}

type topologyHealth struct {
	Status  string
	Ready   string
	Reason  string
	Message string
}

func BuildEnvironmentTopology(args *TopologyArgs, log *zap.SugaredLogger) (*TopologyResponse, error) {
	builder, err := newTopologyBuilder(args)
	if err != nil {
		return nil, err
	}
	if err := builder.build(log); err != nil {
		return nil, err
	}
	return builder.response(), nil
}

func RefreshTopologyNode(args *TopologyArgs, log *zap.SugaredLogger) (*TopologyNodeResponse, error) {
	resp, err := BuildEnvironmentTopology(args, log)
	if err != nil {
		return nil, err
	}
	for _, node := range resp.Nodes {
		if node.ID == args.NodeID {
			return &TopologyNodeResponse{Node: node, RefreshedAt: time.Now().Unix()}, nil
		}
	}
	return nil, fmt.Errorf("node %s not found", args.NodeID)
}

func GetTopologyDiff(args *TopologyArgs, log *zap.SugaredLogger) (*TopologyDiffResponse, error) {
	builder, err := newTopologyBuilder(args)
	if err != nil {
		return nil, err
	}
	if err := builder.build(log); err != nil {
		return nil, err
	}
	node, ok := builder.nodes[args.NodeID]
	if !ok {
		return nil, fmt.Errorf("node %s not found", args.NodeID)
	}
	key := resourceKey(node.Group, node.Version, node.Kind, node.Namespace, node.Name)
	expected := builder.expected[key]
	live := builder.live[key]
	if expected == nil || live == nil || node.ServiceName == "" {
		return &TopologyDiffResponse{
			NodeID:      args.NodeID,
			ServiceName: node.ServiceName,
			SyncStatus:  TopologySyncStatusUnknown,
			Message:     "当前节点缺少期望 YAML 或 live YAML，无法查看拓扑 Diff",
		}, nil
	}
	liveYAML, err := objectToYAML(live)
	if err != nil {
		return nil, err
	}
	return &TopologyDiffResponse{
		NodeID:       args.NodeID,
		ServiceName:  node.ServiceName,
		ExpectedYAML: expected.Manifest,
		LiveYAML:     liveYAML,
		SyncStatus:   node.SyncStatus,
		Message:      node.DisplayStatus,
	}, nil
}

func SyncTopology(args *TopologyArgs, req *TopologySyncRequest, userName, requestID string, log *zap.SugaredLogger) (*TopologySyncResponse, error) {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       args.ProjectName,
		EnvName:    args.EnvName,
		Production: util.GetBoolPointer(args.Production),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get product info, err: %s", err)
	}
	if environmentIsBusy(product.Status) {
		return nil, fmt.Errorf("environment %s is %s, please wait until current operation finishes", args.EnvName, product.Status)
	}
	if getProjectType(args.ProjectName) != setting.K8SDeployType {
		return nil, fmt.Errorf("topology sync only supports Kubernetes YAML environments in the first phase")
	}

	serviceNames := normalizeSyncServices(product, req.ServiceNames)
	if len(serviceNames) == 0 {
		return &TopologySyncResponse{Triggered: false, AffectedServices: 0, Message: "没有可同步的服务"}, nil
	}

	renders := make([]*templatemodels.ServiceRender, 0, len(serviceNames))
	for _, serviceName := range serviceNames {
		serviceObj := product.GetServiceMap()[serviceName]
		if serviceObj == nil {
			return nil, fmt.Errorf("service %s not found in environment", serviceName)
		}
		renders = append(renders, serviceObj.GetServiceRender())
	}

	filter := func(svc *commonmodels.ProductService) bool {
		return util.InStringArray(svc.ServiceName, serviceNames)
	}
	err = updateK8sProduct(product, userName, requestID, nil, filter, renders, nil, nil, true, product.GlobalVariables, log)
	if err != nil {
		return nil, err
	}
	return &TopologySyncResponse{
		Triggered:        true,
		AffectedServices: len(serviceNames),
		Message:          "同步已触发，请刷新拓扑查看状态",
	}, nil
}

func newTopologyBuilder(args *TopologyArgs) (*topologyBuilder, error) {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       args.ProjectName,
		EnvName:    args.EnvName,
		Production: util.GetBoolPointer(args.Production),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get product info, err: %s", err)
	}
	if product.Source == setting.SourceFromExternal {
		return nil, fmt.Errorf("topology only supports Kubernetes environments")
	}
	if getProjectType(args.ProjectName) != setting.K8SDeployType {
		return nil, fmt.Errorf("topology only supports Kubernetes YAML environments in the first phase")
	}
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(product.ClusterID)
	if err != nil {
		return nil, err
	}
	clientset, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(product.ClusterID)
	if err != nil {
		return nil, err
	}
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}
	return &topologyBuilder{
		args:         args,
		product:      product,
		kubeClient:   kubeClient,
		clientset:    clientset,
		versionLess:  VersionLessThan121(serverVersion),
		now:          time.Now().Unix(),
		nodes:        map[string]*TopologyNode{},
		live:         map[string]*unstructured.Unstructured{},
		expected:     map[string]*expectedResource{},
		serviceOwned: map[string]string{},
	}, nil
}

func (b *topologyBuilder) build(log *zap.SugaredLogger) error {
	if err := b.collectExpected(log); err != nil {
		b.addWarning("partial_failure", err.Error(), nil)
	}
	if err := b.collectLive(log); err != nil {
		return err
	}
	b.buildRelations()
	for _, node := range b.nodes {
		b.decorateNode(node)
	}
	return nil
}

func (b *topologyBuilder) collectExpected(log *zap.SugaredLogger) error {
	for _, svc := range b.product.GetSvcList() {
		if b.args.ServiceName != "" && b.args.ServiceName != svc.ServiceName {
			continue
		}
		if svc.Type != setting.K8SDeployType && svc.Type != setting.HelmDeployType && svc.Type != setting.HelmChartDeployType {
			continue
		}
		rendered, err := kube.RenderEnvService(b.product, svc.GetServiceRender(), svc)
		if err != nil {
			log.Errorf("failed to render topology expected yaml for %s/%s/%s: %v", b.product.ProductName, b.product.EnvName, svc.ServiceName, err)
			b.addWarning("partial_failure", fmt.Sprintf("服务 %s 期望 YAML 渲染失败", svc.ServiceName), nil)
			continue
		}
		resources, _, err := kube.ManifestToUnstructured(rendered)
		if err != nil {
			log.Errorf("failed to split topology expected yaml for %s/%s/%s: %v", b.product.ProductName, b.product.EnvName, svc.ServiceName, err)
			b.addWarning("partial_failure", fmt.Sprintf("服务 %s 期望 YAML 解析失败", svc.ServiceName), nil)
		}
		manifestByKey := splitManifestByResource(rendered)
		for _, obj := range resources {
			if obj.GetNamespace() == "" {
				obj.SetNamespace(b.product.Namespace)
			}
			key := objectKey(obj)
			b.expected[key] = &expectedResource{ServiceName: svc.ServiceName, Manifest: manifestByKey[key], Object: obj}
			b.serviceOwned[key] = svc.ServiceName
		}
		for _, res := range svc.Resources {
			if res == nil {
				continue
			}
			key := resourceKey(res.Group, res.Version, res.Kind, b.product.Namespace, res.Name)
			b.serviceOwned[key] = svc.ServiceName
		}
	}
	return nil
}

func (b *topologyBuilder) collectLive(log *zap.SugaredLogger) error {
	ns := b.product.Namespace
	selector := labels.Everything()

	services, err := getter.ListServices(ns, selector, b.kubeClient)
	if err != nil {
		b.addWarning("partial_failure", fmt.Sprintf("Service 查询失败: %v", err), nil)
	} else {
		for _, item := range services {
			b.addLiveObject(item, setting.Service, "", "v1", item.Name, item.Namespace, item.Labels)
		}
	}

	deployments, err := getter.ListDeployments(ns, selector, b.kubeClient)
	if err != nil {
		b.addWarning("partial_failure", fmt.Sprintf("Deployment 查询失败: %v", err), nil)
	} else {
		for _, item := range deployments {
			b.addLiveObject(item, setting.Deployment, "apps", "v1", item.Name, item.Namespace, item.Labels)
		}
	}

	replicaSets, err := getter.ListReplicaSets(ns, selector, b.kubeClient)
	if err != nil {
		b.addWarning("partial_failure", fmt.Sprintf("ReplicaSet 查询失败: %v", err), nil)
	} else {
		for _, item := range replicaSets {
			b.addLiveObject(item, "ReplicaSet", "apps", "v1", item.Name, item.Namespace, item.Labels)
		}
	}

	statefulSets, err := getter.ListStatefulSets(ns, selector, b.kubeClient)
	if err != nil {
		b.addWarning("partial_failure", fmt.Sprintf("StatefulSet 查询失败: %v", err), nil)
	} else {
		for _, item := range statefulSets {
			b.addLiveObject(item, setting.StatefulSet, "apps", "v1", item.Name, item.Namespace, item.Labels)
		}
	}

	daemonSets, err := getter.ListDaemonSets(ns, selector, b.kubeClient)
	if err != nil {
		b.addWarning("partial_failure", fmt.Sprintf("DaemonSet 查询失败: %v", err), nil)
	} else {
		for _, item := range daemonSets {
			b.addLiveObject(item, setting.DaemonSet, "apps", "v1", item.Name, item.Namespace, item.Labels)
		}
	}

	jobs, err := getter.ListJobs(ns, selector, b.kubeClient)
	if err != nil {
		b.addWarning("partial_failure", fmt.Sprintf("Job 查询失败: %v", err), nil)
	} else {
		for _, item := range jobs {
			b.addLiveObject(item, setting.Job, "batch", "v1", item.Name, item.Namespace, item.Labels)
		}
	}

	cronJobs, cronJobBetas, err := getter.ListCronJobs(ns, selector, b.kubeClient, b.versionLess)
	if err != nil {
		b.addWarning("partial_failure", fmt.Sprintf("CronJob 查询失败: %v", err), nil)
	} else {
		for _, item := range cronJobs {
			b.addLiveObject(item, setting.CronJob, "batch", "v1", item.Name, item.Namespace, item.Labels)
		}
		for _, item := range cronJobBetas {
			b.addLiveObject(item, setting.CronJob, "batch", "v1beta1", item.Name, item.Namespace, item.Labels)
		}
	}

	pods, err := getter.ListPods(ns, selector, b.kubeClient)
	if err != nil {
		b.addWarning("partial_failure", fmt.Sprintf("Pod 查询失败: %v", err), nil)
	} else {
		for _, item := range pods {
			b.addLiveObject(item, setting.Pod, "", "v1", item.Name, item.Namespace, item.Labels)
		}
	}

	serverVersion, err := b.clientset.Discovery().ServerVersion()
	if err != nil {
		b.addWarning("partial_failure", err.Error(), nil)
	} else {
		ingresses, err := getter.ListIngresses(ns, b.kubeClient, kubeclient.VersionLessThan122(serverVersion))
		if err != nil {
			b.addWarning("partial_failure", fmt.Sprintf("Ingress 查询失败: %v", err), nil)
		} else {
			for i := range ingresses.Items {
				item := ingresses.Items[i]
				group, version := parseAPIVersion(item.GetAPIVersion())
				if version == "" {
					group, version = "networking.k8s.io", "v1"
				}
				b.addLiveUnstructured(&item, setting.Ingress, group, version)
			}
		}
	}

	configMaps, err := getter.ListConfigMaps(ns, selector, b.kubeClient)
	if err != nil {
		b.addWarning("partial_failure", fmt.Sprintf("ConfigMap 查询失败: %v", err), nil)
	} else {
		for _, item := range configMaps {
			b.addLiveObject(item, setting.ConfigMap, "", "v1", item.Name, item.Namespace, item.Labels)
		}
	}

	secrets, err := getter.ListSecrets(ns, selector, b.kubeClient)
	if err != nil {
		b.addWarning("partial_failure", fmt.Sprintf("Secret 查询失败: %v", err), nil)
	} else {
		for _, item := range secrets {
			b.addLiveObject(item, setting.Secret, "", "v1", item.Name, item.Namespace, item.Labels)
		}
	}

	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := b.kubeClient.List(context.TODO(), pvcs, &client.ListOptions{Namespace: ns, LabelSelector: selector}); err != nil {
		b.addWarning("partial_failure", fmt.Sprintf("PersistentVolumeClaim 查询失败: %v", err), nil)
	} else {
		for i := range pvcs.Items {
			item := &pvcs.Items[i]
			b.addLiveObject(item, "PersistentVolumeClaim", "", "v1", item.Name, item.Namespace, item.Labels)
		}
	}

	return nil
}

func (b *topologyBuilder) addLiveObject(obj interface{}, kind, group, version, name, namespace string, objectLabels map[string]string) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return
	}
	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(bytes, &u.Object); err != nil {
		return
	}
	u.SetKind(kind)
	u.SetAPIVersion(apiVersion(group, version))
	u.SetName(name)
	u.SetNamespace(namespace)
	if len(objectLabels) > 0 {
		u.SetLabels(objectLabels)
	}
	b.addLiveUnstructured(u, kind, group, version)
}

func (b *topologyBuilder) addLiveUnstructured(u *unstructured.Unstructured, kind, group, version string) {
	if u.GetNamespace() == "" {
		u.SetNamespace(b.product.Namespace)
	}
	serviceName := b.resolveServiceName(u)
	if b.args.ServiceName != "" && serviceName != b.args.ServiceName {
		return
	}
	id := topologyNodeID(kind, u.GetNamespace(), u.GetName())
	key := resourceKey(group, version, kind, u.GetNamespace(), u.GetName())
	b.live[key] = u.DeepCopy()
	if _, exists := b.nodes[id]; exists {
		return
	}
	b.nodes[id] = &TopologyNode{
		ID:          id,
		ServiceName: serviceName,
		Kind:        kind,
		Group:       group,
		Version:     version,
		Namespace:   u.GetNamespace(),
		Name:        u.GetName(),
		ParentIDs:   []string{},
		ChildIDs:    []string{},
		Actions:     []string{},
	}
}

func (b *topologyBuilder) buildRelations() {
	for _, node := range b.nodes {
		u := b.live[resourceKey(node.Group, node.Version, node.Kind, node.Namespace, node.Name)]
		if u == nil {
			continue
		}
		for _, ref := range u.GetOwnerReferences() {
			parent := topologyNodeID(ref.Kind, node.Namespace, ref.Name)
			if _, ok := b.nodes[parent]; ok {
				b.addEdge(parent, node.ID, "owns")
			}
		}
	}
	b.buildServicePodEdges()
	b.buildIngressEdges()
	b.buildMountedResourceEdges()
}

func (b *topologyBuilder) buildServicePodEdges() {
	pods := b.nodesByKind(setting.Pod)
	for _, svcNode := range b.nodesByKind(setting.Service) {
		u := b.live[resourceKey(svcNode.Group, svcNode.Version, svcNode.Kind, svcNode.Namespace, svcNode.Name)]
		selector, _, _ := unstructured.NestedStringMap(u.Object, "spec", "selector")
		if len(selector) == 0 {
			continue
		}
		for _, podNode := range pods {
			pod := b.live[resourceKey(podNode.Group, podNode.Version, podNode.Kind, podNode.Namespace, podNode.Name)]
			if labels.SelectorFromSet(selector).Matches(labels.Set(pod.GetLabels())) {
				b.addEdge(svcNode.ID, podNode.ID, "exposes")
			}
		}
	}
}

func (b *topologyBuilder) buildIngressEdges() {
	for _, ingressNode := range b.nodesByKind(setting.Ingress) {
		u := b.live[resourceKey(ingressNode.Group, ingressNode.Version, ingressNode.Kind, ingressNode.Namespace, ingressNode.Name)]
		serviceNames := extractIngressBackendServices(u)
		for _, svcName := range serviceNames.List() {
			svcID := topologyNodeID(setting.Service, ingressNode.Namespace, svcName)
			if _, ok := b.nodes[svcID]; ok {
				b.addEdge(ingressNode.ID, svcID, "routes_to")
			}
		}
	}
}

func (b *topologyBuilder) buildMountedResourceEdges() {
	for _, podNode := range b.nodesByKind(setting.Pod) {
		pod := b.live[resourceKey(podNode.Group, podNode.Version, podNode.Kind, podNode.Namespace, podNode.Name)]
		for _, ref := range extractPodResourceRefs(pod).List() {
			parts := strings.SplitN(ref, "/", 2)
			if len(parts) != 2 {
				continue
			}
			parent := topologyNodeID(parts[0], podNode.Namespace, parts[1])
			if _, ok := b.nodes[parent]; ok {
				b.addEdge(parent, podNode.ID, "mounts")
			}
		}
	}
}

func (b *topologyBuilder) decorateNode(node *TopologyNode) {
	u := b.live[resourceKey(node.Group, node.Version, node.Kind, node.Namespace, node.Name)]
	if u == nil {
		node.SyncStatus = TopologySyncStatusUnknown
		node.HealthStatus = TopologyHealthStatusUnknown
		node.DisplayStatus = displayStatus(node.SyncStatus, node.HealthStatus)
		return
	}
	if node.ServiceName == "" {
		node.ServiceName = b.resolveServiceName(u)
	}
	node.SyncStatus = b.calculateSyncStatus(node, u)
	health := calculateHealthStatus(u)
	node.HealthStatus = health.Status
	node.Ready = health.Ready
	node.Reason = health.Reason
	node.Message = health.Message
	node.DisplayStatus = displayStatus(node.SyncStatus, node.HealthStatus)
	node.Actions = b.nodeActions(node)
	sort.Strings(node.ParentIDs)
	sort.Strings(node.ChildIDs)
	sort.Strings(node.Actions)
}

func (b *topologyBuilder) calculateSyncStatus(node *TopologyNode, live *unstructured.Unstructured) string {
	if environmentIsSyncing(b.product.Status) {
		return TopologySyncStatusSyncing
	}
	expected := b.expected[resourceKey(node.Group, node.Version, node.Kind, node.Namespace, node.Name)]
	if expected == nil {
		return TopologySyncStatusUnknown
	}
	if normalizedEqual(expected.Object, live) {
		return TopologySyncStatusSynced
	}
	return TopologySyncStatusOutOfSync
}

func (b *topologyBuilder) nodeActions(node *TopologyNode) []string {
	actions := sets.NewString(TopologyActionViewYAML, TopologyActionViewEvents, TopologyActionRefreshNode)
	if node.Kind == setting.Pod {
		actions.Insert(TopologyActionViewLogs, TopologyActionRestartPod)
	}
	if node.ServiceName != "" && node.Kind != setting.Pod {
		actions.Insert(TopologyActionSyncNode)
		key := resourceKey(node.Group, node.Version, node.Kind, node.Namespace, node.Name)
		if b.expected[key] != nil && b.live[key] != nil {
			actions.Insert(TopologyActionViewDiff)
		}
	}
	return actions.List()
}

func (b *topologyBuilder) response() *TopologyResponse {
	nodes := make([]*TopologyNode, 0, len(b.nodes))
	for _, node := range b.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(b.edges, func(i, j int) bool {
		if b.edges[i].From == b.edges[j].From {
			return b.edges[i].To < b.edges[j].To
		}
		return b.edges[i].From < b.edges[j].From
	})

	envSync := aggregateSync(nodes)
	envHealth := aggregateHealth(nodes)
	reason, message := summarizeReason(nodes)

	return &TopologyResponse{
		Env: &TopologyEnvSummary{
			ProjectName:     b.product.ProductName,
			EnvName:         b.product.EnvName,
			Production:      b.product.Production,
			Namespace:       b.product.Namespace,
			SyncStatus:      envSync,
			HealthStatus:    envHealth,
			DisplayStatus:   displayStatus(envSync, envHealth),
			Reason:          reason,
			Message:         message,
			LastSyncTime:    b.product.UpdateTime,
			LastSyncStatus:  topologyLastSyncStatus(b.product.Status),
			LastRefreshTime: b.now,
		},
		Summary:  summarizeTopology(nodes),
		Nodes:    nodes,
		Warnings: b.warnings,
		Edges:    b.edges,
	}
}

func (b *topologyBuilder) addEdge(from, to, relation string) {
	if from == "" || to == "" || from == to {
		return
	}
	for _, edge := range b.edges {
		if edge.From == from && edge.To == to && edge.Relation == relation {
			return
		}
	}
	b.edges = append(b.edges, &TopologyEdge{From: from, To: to, Relation: relation})
	if node := b.nodes[from]; node != nil && !util.InStringArray(to, node.ChildIDs) {
		node.ChildIDs = append(node.ChildIDs, to)
	}
	if node := b.nodes[to]; node != nil && !util.InStringArray(from, node.ParentIDs) {
		node.ParentIDs = append(node.ParentIDs, from)
	}
}

func (b *topologyBuilder) addWarning(t, message string, affected []string) {
	b.warnings = append(b.warnings, &TopologyWarning{Type: t, Message: message, AffectedNodes: affected})
}

func (b *topologyBuilder) nodesByKind(kind string) []*TopologyNode {
	ret := make([]*TopologyNode, 0)
	for _, node := range b.nodes {
		if node.Kind == kind {
			ret = append(ret, node)
		}
	}
	return ret
}

func (b *topologyBuilder) resolveServiceName(u *unstructured.Unstructured) string {
	key := objectKey(u)
	if svc := b.serviceOwned[key]; svc != "" {
		return svc
	}
	if svc := u.GetLabels()[setting.ServiceLabel]; svc != "" {
		return svc
	}
	if svc := u.GetLabels()["app.kubernetes.io/name"]; b.product.GetServiceMap()[svc] != nil {
		return svc
	}
	if strings.Contains(u.GetName(), "-") {
		for svcName := range b.product.GetServiceMap() {
			if strings.HasPrefix(u.GetName(), svcName+"-") {
				return svcName
			}
		}
	}
	return ""
}

func environmentIsBusy(status string) bool {
	return status == setting.ProductStatusCreating ||
		status == setting.ProductStatusUpdating ||
		status == setting.ProductStatusDeleting
}

func environmentIsSyncing(status string) bool {
	return status == setting.ProductStatusCreating || status == setting.ProductStatusUpdating
}

func topologyLastSyncStatus(productStatus string) string {
	switch productStatus {
	case setting.ProductStatusUpdating:
		return "running"
	case setting.ProductStatusSuccess:
		return "success"
	case setting.ProductStatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

func normalizeSyncServices(product *commonmodels.Product, requested []string) []string {
	all := sets.NewString()
	for _, svc := range product.GetSvcList() {
		if svc.Type == setting.K8SDeployType || svc.Type == setting.HelmDeployType || svc.Type == setting.HelmChartDeployType {
			all.Insert(svc.ServiceName)
		}
	}
	if len(requested) == 0 {
		return all.List()
	}
	ret := sets.NewString()
	for _, name := range requested {
		if all.Has(name) {
			ret.Insert(name)
		}
	}
	return ret.List()
}

func summarizeTopology(nodes []*TopologyNode) *TopologySummary {
	s := &TopologySummary{TotalNodes: len(nodes)}
	for _, node := range nodes {
		switch node.SyncStatus {
		case TopologySyncStatusSynced:
			s.Synced++
		case TopologySyncStatusSyncing:
			s.Syncing++
		case TopologySyncStatusOutOfSync:
			s.OutOfSync++
		default:
			s.Unknown++
		}
		switch node.HealthStatus {
		case TopologyHealthStatusHealthy:
			s.Healthy++
		case TopologyHealthStatusProgressing:
			s.Progressing++
		case TopologyHealthStatusDegraded:
			s.Degraded++
		}
		if node.Kind == setting.Pod && util.InStringArray(TopologyActionRestartPod, node.Actions) {
			s.RestartablePods++
		}
	}
	return s
}

func aggregateSync(nodes []*TopologyNode) string {
	if len(nodes) == 0 {
		return TopologySyncStatusUnknown
	}
	allSynced := true
	hasUnknown := false
	for _, node := range nodes {
		switch node.SyncStatus {
		case TopologySyncStatusSyncing:
			return TopologySyncStatusSyncing
		case TopologySyncStatusOutOfSync:
			allSynced = false
		case TopologySyncStatusUnknown:
			allSynced = false
			hasUnknown = true
		case TopologySyncStatusSynced:
		default:
			allSynced = false
			hasUnknown = true
		}
	}
	if !allSynced {
		for _, node := range nodes {
			if node.SyncStatus == TopologySyncStatusOutOfSync {
				return TopologySyncStatusOutOfSync
			}
		}
		if hasUnknown {
			return TopologySyncStatusUnknown
		}
	}
	return TopologySyncStatusSynced
}

func aggregateHealth(nodes []*TopologyNode) string {
	if len(nodes) == 0 {
		return TopologyHealthStatusUnknown
	}
	allHealthy := true
	hasUnknown := false
	for _, node := range nodes {
		switch node.HealthStatus {
		case TopologyHealthStatusDegraded:
			return TopologyHealthStatusDegraded
		case TopologyHealthStatusProgressing:
			allHealthy = false
		case TopologyHealthStatusUnknown:
			allHealthy = false
			hasUnknown = true
		case TopologyHealthStatusHealthy:
		default:
			allHealthy = false
			hasUnknown = true
		}
	}
	if !allHealthy {
		for _, node := range nodes {
			if node.HealthStatus == TopologyHealthStatusProgressing {
				return TopologyHealthStatusProgressing
			}
		}
		if hasUnknown {
			return TopologyHealthStatusUnknown
		}
	}
	return TopologyHealthStatusHealthy
}

func summarizeReason(nodes []*TopologyNode) (string, string) {
	degraded, outOfSync := 0, 0
	reason := ""
	for _, node := range nodes {
		if node.HealthStatus == TopologyHealthStatusDegraded {
			degraded++
			if reason == "" {
				reason = node.Reason
			}
		}
		if node.SyncStatus == TopologySyncStatusOutOfSync {
			outOfSync++
		}
	}
	parts := make([]string, 0)
	if degraded > 0 {
		parts = append(parts, fmt.Sprintf("%d 个节点异常", degraded))
	}
	if outOfSync > 0 {
		parts = append(parts, fmt.Sprintf("%d 个节点配置偏离期望", outOfSync))
	}
	return reason, strings.Join(parts, "，")
}

func displayStatus(syncStatus, healthStatus string) string {
	switch {
	case syncStatus == TopologySyncStatusSynced && healthStatus == TopologyHealthStatusHealthy:
		return "配置已同步，运行正常"
	case syncStatus == TopologySyncStatusSynced && healthStatus == TopologyHealthStatusProgressing:
		return "配置已同步，资源启动中"
	case syncStatus == TopologySyncStatusSynced && healthStatus == TopologyHealthStatusDegraded:
		return "配置已同步，但运行异常"
	case syncStatus == TopologySyncStatusOutOfSync && healthStatus == TopologyHealthStatusHealthy:
		return "配置存在差异，但当前运行正常"
	case syncStatus == TopologySyncStatusOutOfSync && healthStatus == TopologyHealthStatusDegraded:
		return "配置存在差异，且运行异常"
	case syncStatus == TopologySyncStatusOutOfSync:
		return "配置存在差异"
	case syncStatus == TopologySyncStatusSyncing && healthStatus == TopologyHealthStatusHealthy:
		return "正在同步配置，但当前运行正常"
	case syncStatus == TopologySyncStatusSyncing && healthStatus == TopologyHealthStatusDegraded:
		return "正在同步配置，且部分资源运行异常"
	case syncStatus == TopologySyncStatusSyncing:
		return "正在同步配置，资源正在启动"
	default:
		return "状态暂时无法判断"
	}
}

func calculateHealthStatus(u *unstructured.Unstructured) topologyHealth {
	switch u.GetKind() {
	case setting.Pod:
		return calculatePodHealth(u)
	case setting.Deployment, "ReplicaSet", setting.StatefulSet, setting.DaemonSet:
		return calculateWorkloadHealth(u)
	case setting.Job:
		return calculateJobHealth(u)
	case setting.CronJob, setting.Service, setting.Ingress, setting.ConfigMap, setting.Secret, "PersistentVolumeClaim":
		return topologyHealth{Status: TopologyHealthStatusHealthy}
	default:
		return topologyHealth{Status: TopologyHealthStatusUnknown}
	}
}

func calculatePodHealth(u *unstructured.Unstructured) topologyHealth {
	statuses, _, _ := unstructured.NestedSlice(u.Object, "status", "containerStatuses")
	ready := 0
	reason, message := "", ""
	for _, item := range statuses {
		status, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if v, _, _ := unstructured.NestedBool(status, "ready"); v {
			ready++
		}
		if waiting, _, _ := unstructured.NestedMap(status, "state", "waiting"); waiting != nil {
			reason, _, _ = unstructured.NestedString(waiting, "reason")
			message, _, _ = unstructured.NestedString(waiting, "message")
			if isDegradedReason(reason) {
				return topologyHealth{Status: TopologyHealthStatusDegraded, Ready: fmt.Sprintf("%d/%d", ready, len(statuses)), Reason: reason, Message: message}
			}
		}
		if terminated, _, _ := unstructured.NestedMap(status, "state", "terminated"); terminated != nil {
			reason, _, _ = unstructured.NestedString(terminated, "reason")
			message, _, _ = unstructured.NestedString(terminated, "message")
			exitCode, _, _ := unstructured.NestedInt64(terminated, "exitCode")
			if exitCode != 0 {
				return topologyHealth{Status: TopologyHealthStatusDegraded, Ready: fmt.Sprintf("%d/%d", ready, len(statuses)), Reason: reason, Message: message}
			}
		}
	}
	phase, _, _ := unstructured.NestedString(u.Object, "status", "phase")
	readyText := fmt.Sprintf("%d/%d", ready, len(statuses))
	if phase == string(corev1.PodFailed) || phase == string(corev1.PodUnknown) {
		return topologyHealth{Status: TopologyHealthStatusDegraded, Ready: readyText, Reason: phase}
	}
	if len(statuses) > 0 && ready == len(statuses) && phase == string(corev1.PodRunning) {
		return topologyHealth{Status: TopologyHealthStatusHealthy, Ready: readyText}
	}
	return topologyHealth{Status: TopologyHealthStatusProgressing, Ready: readyText, Reason: "Progressing"}
}

func calculateWorkloadHealth(u *unstructured.Unstructured) topologyHealth {
	desired, _, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")
	if desired == 0 {
		desired = 1
	}
	ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
	available, _, _ := unstructured.NestedInt64(u.Object, "status", "availableReplicas")
	readyText := fmt.Sprintf("%d/%d", ready, desired)
	conditions, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	for _, item := range conditions {
		condition, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		cType, _, _ := unstructured.NestedString(condition, "type")
		status, _, _ := unstructured.NestedString(condition, "status")
		reason, _, _ := unstructured.NestedString(condition, "reason")
		message, _, _ := unstructured.NestedString(condition, "message")
		if (cType == "ReplicaFailure" || isDegradedReason(reason)) && status == string(corev1.ConditionTrue) {
			return topologyHealth{Status: TopologyHealthStatusDegraded, Ready: readyText, Reason: reason, Message: message}
		}
	}
	if ready >= desired && available >= desired {
		return topologyHealth{Status: TopologyHealthStatusHealthy, Ready: readyText}
	}
	return topologyHealth{Status: TopologyHealthStatusProgressing, Ready: readyText, Reason: "Progressing"}
}

func calculateJobHealth(u *unstructured.Unstructured) topologyHealth {
	failed, _, _ := unstructured.NestedInt64(u.Object, "status", "failed")
	succeeded, _, _ := unstructured.NestedInt64(u.Object, "status", "succeeded")
	if failed > 0 {
		return topologyHealth{Status: TopologyHealthStatusDegraded, Ready: fmt.Sprintf("%d", succeeded), Reason: "Failed"}
	}
	if succeeded > 0 {
		return topologyHealth{Status: TopologyHealthStatusHealthy, Ready: fmt.Sprintf("%d", succeeded)}
	}
	return topologyHealth{Status: TopologyHealthStatusProgressing, Ready: fmt.Sprintf("%d", succeeded), Reason: "Progressing"}
}

func isDegradedReason(reason string) bool {
	switch reason {
	case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "CreateContainerConfigError", "CreateContainerError", "RunContainerError", "Failed", "Unhealthy":
		return true
	default:
		return false
	}
}

func splitManifestByResource(manifest string) map[string]string {
	ret := map[string]string{}
	manifests := strings.Split(manifest, "\n---")
	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil || u.GetKind() == "" || u.GetName() == "" {
			continue
		}
		ret[objectKey(u)] = strings.TrimSpace(item) + "\n"
	}
	return ret
}

func normalizedEqual(expected, live *unstructured.Unstructured) bool {
	if expected == nil || live == nil {
		return false
	}
	left := normalizeObject(expected)
	right := normalizeObject(live)
	return reflect.DeepEqual(left.Object, right.Object)
}

func normalizeObject(obj *unstructured.Unstructured) *unstructured.Unstructured {
	ret := obj.DeepCopy()
	unstructured.RemoveNestedField(ret.Object, "status")
	unstructured.RemoveNestedField(ret.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(ret.Object, "metadata", "uid")
	unstructured.RemoveNestedField(ret.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(ret.Object, "metadata", "generation")
	unstructured.RemoveNestedField(ret.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(ret.Object, "metadata", "selfLink")
	unstructured.RemoveNestedField(ret.Object, "metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration")
	return ret
}

func objectToYAML(obj *unstructured.Unstructured) (string, error) {
	b, err := yaml.Marshal(obj.Object)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func extractIngressBackendServices(u *unstructured.Unstructured) sets.String {
	ret := sets.NewString()
	defaultBackend, _, _ := unstructured.NestedMap(u.Object, "spec", "defaultBackend", "service")
	if name, ok := defaultBackend["name"].(string); ok && name != "" {
		ret.Insert(name)
	}
	rules, _, _ := unstructured.NestedSlice(u.Object, "spec", "rules")
	for _, ruleItem := range rules {
		rule, ok := ruleItem.(map[string]interface{})
		if !ok {
			continue
		}
		paths, _, _ := unstructured.NestedSlice(rule, "http", "paths")
		for _, pathItem := range paths {
			path, ok := pathItem.(map[string]interface{})
			if !ok {
				continue
			}
			service, _, _ := unstructured.NestedMap(path, "backend", "service")
			if name, ok := service["name"].(string); ok && name != "" {
				ret.Insert(name)
			}
		}
	}
	return ret
}

func extractPodResourceRefs(u *unstructured.Unstructured) sets.String {
	ret := sets.NewString()
	volumes, _, _ := unstructured.NestedSlice(u.Object, "spec", "volumes")
	for _, item := range volumes {
		volume, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if cfg, _, _ := unstructured.NestedMap(volume, "configMap"); cfg != nil {
			if name, _ := cfg["name"].(string); name != "" {
				ret.Insert(setting.ConfigMap + "/" + name)
			}
		}
		if secret, _, _ := unstructured.NestedMap(volume, "secret"); secret != nil {
			if name, _ := secret["secretName"].(string); name != "" {
				ret.Insert(setting.Secret + "/" + name)
			}
		}
		if pvc, _, _ := unstructured.NestedMap(volume, "persistentVolumeClaim"); pvc != nil {
			if name, _ := pvc["claimName"].(string); name != "" {
				ret.Insert("PersistentVolumeClaim/" + name)
			}
		}
	}
	return ret
}

func topologyNodeID(kind, namespace, name string) string {
	return fmt.Sprintf("%s:%s:%s", strings.ToLower(kind), namespace, name)
}

func resourceKey(group, version, kind, namespace, name string) string {
	return strings.Join([]string{group, version, kind, namespace, name}, "|")
}

func objectKey(u *unstructured.Unstructured) string {
	gvk := u.GroupVersionKind()
	namespace := u.GetNamespace()
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	return resourceKey(gvk.Group, gvk.Version, gvk.Kind, namespace, u.GetName())
}

func apiVersion(group, version string) string {
	if group == "" {
		return version
	}
	return group + "/" + version
}

func parseAPIVersion(apiVersion string) (string, string) {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 1 {
		return "", parts[0]
	}
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
