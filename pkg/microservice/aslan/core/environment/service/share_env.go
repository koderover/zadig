/*
Copyright 2022 The KodeRover Authors.

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
	"strings"
	"time"

	types "github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/protobuf/encoding/protojson"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/tool/kube/util"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	zadigtypes "github.com/koderover/zadig/v2/pkg/types"
	zadigutil "github.com/koderover/zadig/v2/pkg/util"
)

const istioNamespace = "istio-system"
const istioProxyName = "istio-proxy"
const zadigEnvoyFilter = "zadig-share-env"
const envoyFilterNetworkHttpConnectionManager = "envoy.filters.network.http_connection_manager"
const envoyFilterHttpRouter = "envoy.filters.http.router"
const envoyFilterLua = "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua"

const zadigNamePrefix = "zadig"
const zadigMatchXEnv = "x-env"

// Slice of `<workload name>.<workload type>` and error are returned.
func CheckWorkloadsK8sServices(ctx context.Context, envName, productName string) ([]string, error) {
	timeStart := time.Now()
	defer func() {
		log.Infof("[CheckWorkloadsK8sServices]Time consumed: %s", time.Since(timeStart))
	}()

	prod, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query env `%s` in project `%s`: %s", envName, productName, err)
	}

	ns := prod.Namespace
	clusterID := prod.ClusterID

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get kube client: %s", err)
	}

	return checkWorkloadsHaveK8sService(ctx, kclient, ns)
}

func EnableBaseEnv(ctx context.Context, envName, productName string) error {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return fmt.Errorf("failed to query env `%s` in project `%s`: %s", envName, productName, err)
	}
	if prod.IsSleeping() {
		return fmt.Errorf("Environment is sleeping")
	}

	ns := prod.Namespace
	clusterID := prod.ClusterID

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %s", err)
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return fmt.Errorf("failed to get rest config: %s", err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to new istio client: %s", err)
	}

	// 1. Ensure `istio-injection=enabled` label on the namespace.
	err = ensureIstioLabel(ctx, kclient, ns)
	if err != nil {
		return fmt.Errorf("failed to ensure istio label on namespace `%s`: %s", ns, err)
	}

	// 2. Ensure Pods that are not injected with `istio-proxy`.
	err = ensurePodsWithIsitoProxy(ctx, kclient, ns)
	if err != nil {
		return fmt.Errorf("failed to ensure pods with istio-proxy in namespace `%s`: %s", ns, err)
	}

	// 3. Ensure `VirtualService` (subsets are not required) in current namespace.
	err = ensureVirtualServices(ctx, prod, kclient, istioClient)
	if err != nil {
		return fmt.Errorf("failed to ensure VirtualServices in namespace `%s`: %s", ns, err)
	}

	// 4. Ensure `EnvoyFilter` in istio namespace.
	err = ensureEnvoyFilter(ctx, istioClient, clusterID, istioNamespace, zadigEnvoyFilter, nil)
	if err != nil {
		return fmt.Errorf("failed to ensure EnvoyFilter in namespace `%s`: %s", istioNamespace, err)
	}

	// 5. Update the environment configuration.
	return ensureBaseEnvConfig(ctx, prod)
}

func DisableBaseEnv(ctx context.Context, envName, productName string) error {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return fmt.Errorf("failed to query env `%s` in project `%s`: %s", envName, productName, err)
	}
	if prod.IsSleeping() {
		return fmt.Errorf("Environment is sleeping")
	}

	ns := prod.Namespace
	clusterID := prod.ClusterID

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %s", err)
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return fmt.Errorf("failed to get rest config: %s", err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to new istio client: %s", err)
	}

	// 1. Delete all associated subenvironments.
	err = ensureDeleteAssociatedEnvs(ctx, prod)
	if err != nil {
		return fmt.Errorf("failed to delete associated subenvironments of base ns `%s`: %s", ns, err)
	}

	// 2. Delete EnvoyFilter in the namespace of Istio installation.
	err = ensureDeleteEnvoyFilter(ctx, prod, istioClient)
	if err != nil {
		return fmt.Errorf("failed to delete EnvoyFilter: %s", err)
	}

	// 3. Delete Gateway delivered by the Zadig.
	err = deleteGateways(ctx, kclient, istioClient, ns)
	if err != nil {
		return fmt.Errorf("failed to delete EnvoyFilter: %s", err)
	}

	// 4. Delete all VirtualServices delivered by the Zadig.
	err = deleteVirtualServices(ctx, kclient, istioClient, ns)
	if err != nil {
		return fmt.Errorf("failed to delete VirtualServices that Zadig created in ns `%s`: %s", ns, err)
	}

	// 5. Remove the `istio-injection=enabled` label of the namespace.
	err = removeIstioLabel(ctx, kclient, ns)
	if err != nil {
		return fmt.Errorf("failed to remove istio label on ns `%s`: %s", ns, err)
	}

	// 6. Restart the istio-Proxy injected Pods.
	err = removePodsIstioProxy(ctx, kclient, ns)
	if err != nil {
		return fmt.Errorf("failed to remove istio-proxy from pods in ns `%s`: %s", ns, err)
	}

	// 7. Update the environment configuration.
	return ensureDisableBaseEnvConfig(ctx, prod)
}

func CheckShareEnvReady(ctx context.Context, envName, op, productName string) (*ShareEnvReady, error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return nil, fmt.Errorf("failed to query env `%s` in project `%s`: %s", envName, productName, err)
	}

	ns := prod.Namespace
	clusterID := prod.ClusterID

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get kube client: %s", err)
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get rest config: %s", err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to new istio client: %s", err)
	}

	shareEnvOp := ShareEnvOp(op)

	// 1. Check whether namespace has labeled `istio-injection=enabled`.
	isNamespaceHasIstioLabel, err := checkIstioLabel(ctx, kclient, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to check whether namespace `%s` has labeled `istio-injection=enabled`: %s", ns, err)
	}

	// 2. Check whether all workloads have K8s Service.
	workloadsHaveNoK8sService, err := checkWorkloadsHaveK8sService(ctx, kclient, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to check whether all workloads in ns `%s` have K8s Service: %s", ns, err)
	}

	isWorkloadsHaveNoK8sService := true
	if len(workloadsHaveNoK8sService) > 0 {
		isWorkloadsHaveNoK8sService = false
	}

	// 3. Check whether all VirtualServices have been deployed.
	isVirtualServicesDeployed, err := checkVirtualServicesDeployed(ctx, kclient, istioClient, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to check whether all VirtualServices in ns `%s` have been deployed: %s", ns, err)
	}

	// 4. Check whether all Pods have istio-proxy and are ready.
	allHaveIstioProxy, allPodsReady, err := checkPodsWithIstioProxyAndReady(ctx, kclient, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to check whether all pods in ns `%s` have istio-proxy and are ready: %s", ns, err)
	}

	res := &ShareEnvReady{
		Checks: ShareEnvReadyChecks{
			NamespaceHasIstioLabel:  isNamespaceHasIstioLabel,
			WorkloadsHaveK8sService: isWorkloadsHaveNoK8sService,
			VirtualServicesDeployed: isVirtualServicesDeployed,
			PodsHaveIstioProxy:      allHaveIstioProxy,
			WorkloadsReady:          allPodsReady,
		},
	}
	res.CheckAndSetReady(shareEnvOp)

	return res, nil
}

func checkWorkloadsHaveK8sService(ctx context.Context, kclient client.Client, ns string) ([]string, error) {
	workloads, err := getWorkloads(ctx, kclient, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to get workloads: %s", err)
	}

	svcs, err := getSvcs(ctx, kclient, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to get svcs: %s", err)
	}

	return checkWorkloadsNoSvcs(svcs, workloads)
}

// map structure:
// - key: `<workload name>.<workload type>`, e.g.: zadig.deployment
// - value: a map[string]string, which is workload' labels
//
// Note:
// Currently we only support Deployment and StatefulSet workloads, if more workloads need to be supported,
// they need to be added here.
func getWorkloads(ctx context.Context, kclient client.Client, namespace string) (map[string]map[string]string, error) {
	deployments := &appsv1.DeploymentList{}
	err := kclient.List(ctx, deployments, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments in %s: %s", namespace, err)
	}

	statefulsets := &appsv1.StatefulSetList{}
	err = kclient.List(ctx, statefulsets, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list statefulsets in %s: %s", namespace, err)
	}

	res := map[string]map[string]string{}
	for _, d := range deployments.Items {
		res[fmt.Sprintf("%s.%s", d.Name, d.Kind)] = d.Spec.Template.GetLabels()
	}
	for _, s := range statefulsets.Items {
		res[fmt.Sprintf("%s.%s", s.Name, s.Kind)] = s.Spec.Template.GetLabels()
	}

	return res, nil
}

// map structure:
// - key: `<sic name>`, e.g.: zadig
// - value: a map[string]string, which is Service's selector
func getSvcs(ctx context.Context, kclient client.Client, namespace string) (map[string]map[string]string, error) {
	svcs := &corev1.ServiceList{}
	err := kclient.List(ctx, svcs, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list svcs in %s: %s", namespace, err)
	}

	res := map[string]map[string]string{}
	for _, s := range svcs.Items {
		res[s.Name] = s.Spec.Selector
	}

	return res, nil
}

// Structure of `svcs` and `workloads` is same as the above.
// Slice of `<workload name>.<workload type>` and error are returned.
func checkWorkloadsNoSvcs(svcs map[string]map[string]string, workloads map[string]map[string]string) ([]string, error) {
	for _, svcSelector := range svcs {
		for wName, wLabels := range workloads {
			if !isMapSubset(svcSelector, wLabels) {
				continue
			}

			delete(workloads, wName)

			// Note: Generally, a workload does not correspond to multiple services in a namespace.
			break
		}
	}

	workloadsName := make([]string, 0, len(workloads))
	for wname := range workloads {
		workloadsName = append(workloadsName, wname)
	}

	return workloadsName, nil
}

func isMapSubset(mapSubSet, mapSet map[string]string) bool {
	if len(mapSubSet) > len(mapSet) {
		return false
	}

	for k, v := range mapSubSet {
		if vv, found := mapSet[k]; !found || vv != v {
			return false
		}
	}

	return true
}

func ensureIstioLabel(ctx context.Context, kclient client.Client, ns string) error {
	nsObj := &corev1.Namespace{}
	err := kclient.Get(ctx, client.ObjectKey{
		Name: ns,
	}, nsObj)
	if err != nil {
		return fmt.Errorf("failed to query ns `%s`: %s", ns, err)
	}

	nsObj.Labels[zadigtypes.IstioLabelKeyInjection] = zadigtypes.IstioLabelValueInjection
	return kclient.Update(ctx, nsObj)
}

func checkIstioLabel(ctx context.Context, kclient client.Client, ns string) (bool, error) {
	nsObj := &corev1.Namespace{}
	err := kclient.Get(ctx, client.ObjectKey{
		Name: ns,
	}, nsObj)
	if err != nil {
		return false, fmt.Errorf("failed to query ns `%s`: %s", ns, err)
	}

	return nsObj.Labels[zadigtypes.IstioLabelKeyInjection] == zadigtypes.IstioLabelValueInjection, nil
}

func ensurePodsWithIsitoProxy(ctx context.Context, kclient client.Client, ns string) error {
	return restartPodsWithIstioProxy(ctx, kclient, ns, false)
}

func checkPodsWithIstioProxyAndReady(ctx context.Context, kclient client.Client, ns string) (allHaveIstioProxy, allPodsReady bool, err error) {
	pods := &corev1.PodList{}
	err = kclient.List(ctx, pods, client.InNamespace(ns))
	if err != nil {
		return false, false, fmt.Errorf("failed to query pods in ns `%s`: %s", ns, err)
	}

	allHaveIstioProxy = true
	allPodsReady = true
	for _, pod := range pods.Items {
		if !(pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodSucceeded) {
			allPodsReady = false
		}

		hasIstioProxy := false
		for _, container := range pod.Spec.Containers {
			if container.Name == istioProxyName {
				hasIstioProxy = true
				break
			}
		}
		if !hasIstioProxy {
			allHaveIstioProxy = false
		}

		if !allPodsReady && !allHaveIstioProxy {
			return false, false, nil
		}
	}

	return allHaveIstioProxy, allPodsReady, nil
}

func ensureVirtualServices(ctx context.Context, env *commonmodels.Product, kclient client.Client, istioClient versionedclient.Interface) error {
	svcs := &corev1.ServiceList{}
	err := kclient.List(ctx, svcs, client.InNamespace(env.Namespace))
	if err != nil {
		return fmt.Errorf("failed to list svcs in ns `%s`: %s", env.Namespace, err)
	}

	for _, svc := range svcs.Items {
		vsName := genVirtualServiceName(&svc)
		err := ensureVirtualService(ctx, kclient, istioClient, env, &svc, vsName)
		if err != nil {
			return fmt.Errorf("failed to ensure VirtualService `%s` in ns `%s`: %s", vsName, env.Namespace, err)
		}
	}

	return nil
}

func ensureVirtualService(ctx context.Context, kclient client.Client, istioClient versionedclient.Interface, env *commonmodels.Product, svc *corev1.Service, vsName string) error {
	vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(env.Namespace).Get(ctx, vsName, metav1.GetOptions{})
	if err == nil {
		log.Infof("Has found VirtualService `%s` in ns `%s` and don't recreate.", vsName, env.Namespace)
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to query VirtualService `%s` in ns `%s`: %s", vsName, env.Namespace, err)
	}

	matchedEnvs := []MatchedEnv{}
	if env.ShareEnv.Enable && env.ShareEnv.IsBase {
		subEnvs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:            env.ProductName,
			ShareEnvEnable:  zadigutil.GetBoolPointer(true),
			ShareEnvIsBase:  zadigutil.GetBoolPointer(false),
			ShareEnvBaseEnv: zadigutil.GetStrPointer(env.EnvName),
			Production:      zadigutil.GetBoolPointer(false),
		})
		if err != nil {
			return err
		}

		svcSelector := labels.SelectorFromSet(labels.Set(svc.Spec.Selector))
		for _, subEnv := range subEnvs {
			hasWorkload, err := doesSvcHasWorkload(ctx, subEnv.Namespace, svcSelector, kclient)
			if err != nil {
				return err
			}

			if !hasWorkload {
				continue
			}

			matchedEnvs = append(matchedEnvs, MatchedEnv{
				EnvName:   subEnv.EnvName,
				Namespace: subEnv.Namespace,
			})
		}
	}

	vsObj.Name = vsName

	if vsObj.Labels == nil {
		vsObj.Labels = map[string]string{}
	}
	vsObj.Labels[zadigtypes.ZadigLabelKeyGlobalOwner] = zadigtypes.Zadig

	routes := []*networkingv1alpha3.HTTPRoute{}
	for _, matchedEnv := range matchedEnvs {
		grayRoute := &networkingv1alpha3.HTTPRoute{
			Match: []*networkingv1alpha3.HTTPMatchRequest{
				&networkingv1alpha3.HTTPMatchRequest{
					Headers: map[string]*networkingv1alpha3.StringMatch{
						zadigMatchXEnv: &networkingv1alpha3.StringMatch{
							MatchType: &networkingv1alpha3.StringMatch_Exact{
								Exact: matchedEnv.EnvName,
							},
						},
					},
				},
			},
			Route: []*networkingv1alpha3.HTTPRouteDestination{
				&networkingv1alpha3.HTTPRouteDestination{
					Destination: &networkingv1alpha3.Destination{
						Host: fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, matchedEnv.Namespace),
					},
				},
			},
		}
		routes = append(routes, grayRoute)
	}
	routes = append(routes, &networkingv1alpha3.HTTPRoute{
		Route: []*networkingv1alpha3.HTTPRouteDestination{
			&networkingv1alpha3.HTTPRouteDestination{
				Destination: &networkingv1alpha3.Destination{
					Host: fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, env.Namespace),
				},
			},
		},
	})

	vsObj.Spec = networkingv1alpha3.VirtualService{
		Hosts: []string{svc.Name},
		Http:  routes,
	}
	_, err = istioClient.NetworkingV1alpha3().VirtualServices(env.Namespace).Create(ctx, vsObj, metav1.CreateOptions{})
	return err
}

func checkVirtualServicesDeployed(ctx context.Context, kclient client.Client, istioClient versionedclient.Interface, ns string) (bool, error) {
	svcs := &corev1.ServiceList{}
	err := kclient.List(ctx, svcs, client.InNamespace(ns))
	if err != nil {
		return false, fmt.Errorf("failed to list svcs in ns `%s`: %s", ns, err)
	}

	for _, svc := range svcs.Items {
		vsName := genVirtualServiceName(&svc)
		_, err := istioClient.NetworkingV1alpha3().VirtualServices(ns).Get(ctx, vsName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("failed to query VirtualService `%s` in ns `%s`: %s", vsName, ns, err)
		}
	}

	return true, nil
}

func ensureEnvoyFilter(ctx context.Context, istioClient versionedclient.Interface, clusterID, ns, name string, headerKeys []string) error {
	headerKeySet := sets.NewString(headerKeys...)
	envoyFilterObj, err := istioClient.NetworkingV1alpha3().EnvoyFilters(ns).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		log.Infof("Has found EnvoyFilter `%s` in ns `%s` and don't recreate.", name, istioNamespace)
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to query EnvoyFilter `%s` in ns `%s`: %s", name, istioNamespace, err)
	}

	storeCacheOperation, err := buildEnvoyStoreCacheOperation(headerKeySet.List())
	if err != nil {
		return fmt.Errorf("failed to build envoy operation of storing cache: %s", err)
	}

	getCacheOperation, err := buildEnvoyGetCacheOperation(headerKeySet.List())
	if err != nil {
		return fmt.Errorf("failed to build envoy operation of getting cache: %s", err)
	}

	var cacheServerAddr string
	var cacheServerPort int

	switch clusterID {
	case setting.LocalClusterID:
		cacheServerAddr = fmt.Sprintf("aslan.%s.svc.cluster.local", config.Namespace())
		cacheServerPort = 25000
	default:
		cacheServerAddr = "hub-agent.koderover-agent.svc.cluster.local"
		cacheServerPort = 80
	}

	clusterConfig, err := buildEnvoyClusterConfig(cacheServerAddr, cacheServerPort)
	if err != nil {
		return fmt.Errorf("failed to build envoy cluster config for `%s:%d`: %s", cacheServerAddr, cacheServerPort, err)
	}

	envoyFilterObj.Name = name

	if envoyFilterObj.Labels == nil {
		envoyFilterObj.Labels = map[string]string{}
	}
	envoyFilterObj.Labels[zadigtypes.ZadigLabelKeyGlobalOwner] = zadigtypes.Zadig

	envoyFilterObj.Spec = networkingv1alpha3.EnvoyFilter{
		ConfigPatches: []*networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
			&networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
				ApplyTo: networkingv1alpha3.EnvoyFilter_HTTP_FILTER,
				Match: &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
					Context: networkingv1alpha3.EnvoyFilter_SIDECAR_INBOUND,
					ObjectTypes: &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
						Listener: &networkingv1alpha3.EnvoyFilter_ListenerMatch{
							FilterChain: &networkingv1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
								Filter: &networkingv1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
									Name: envoyFilterNetworkHttpConnectionManager,
									SubFilter: &networkingv1alpha3.EnvoyFilter_ListenerMatch_SubFilterMatch{
										Name: envoyFilterHttpRouter,
									},
								},
							},
						},
					},
				},
				Patch: &networkingv1alpha3.EnvoyFilter_Patch{
					Operation: networkingv1alpha3.EnvoyFilter_Patch_INSERT_BEFORE,
					Value:     storeCacheOperation,
				},
			},
			&networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
				ApplyTo: networkingv1alpha3.EnvoyFilter_HTTP_FILTER,
				Match: &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
					Context: networkingv1alpha3.EnvoyFilter_SIDECAR_OUTBOUND,
					ObjectTypes: &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
						Listener: &networkingv1alpha3.EnvoyFilter_ListenerMatch{
							FilterChain: &networkingv1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
								Filter: &networkingv1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
									Name: envoyFilterNetworkHttpConnectionManager,
									SubFilter: &networkingv1alpha3.EnvoyFilter_ListenerMatch_SubFilterMatch{
										Name: envoyFilterHttpRouter,
									},
								},
							},
						},
					},
				},
				Patch: &networkingv1alpha3.EnvoyFilter_Patch{
					Operation: networkingv1alpha3.EnvoyFilter_Patch_INSERT_BEFORE,
					Value:     getCacheOperation,
				},
			},
			&networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
				ApplyTo: networkingv1alpha3.EnvoyFilter_CLUSTER,
				Patch: &networkingv1alpha3.EnvoyFilter_Patch{
					Operation: networkingv1alpha3.EnvoyFilter_Patch_ADD,
					Value:     clusterConfig,
				},
			},
		},
	}

	_, err = istioClient.NetworkingV1alpha3().EnvoyFilters(ns).Create(ctx, envoyFilterObj, metav1.CreateOptions{})
	return err
}
func buildEnvoyStoreCacheOperation1(keys []string) (*types.Struct, error) {
	inlineCode := `function envoy_on_request(request_handle)
  function split_str(s, delimiter)
    res = {}
    for match in (s..delimiter):gmatch("(.-)"..delimiter) do
      table.insert(res, match)
    end

    return res
  end

  local traceid = request_handle:headers():get("sw8")
  if traceid then
    arr = split_str(traceid, "-")
    traceid = arr[2]
  else
    traceid = request_handle:headers():get("x-request-id")
    if not traceid then
      traceid = request_handle:headers():get("x-b3-traceid")
    end
  end

  local env = request_handle:headers():get("x-env")
  local headers, body = request_handle:httpCall(
    "cache",
    {
      [":method"] = "POST",
      [":path"] = string.format("/api/cache/%s/%s", traceid, env),
      [":authority"] = "cache",
    },
    "",
    5000
  )
end
`
	data := map[string]interface{}{
		"name": "envoy.lua",
		"typed_config": map[string]string{
			"@type":      envoyFilterLua,
			"inlineCode": inlineCode,
		},
	}

	return buildEnvoyPatchValue(data)
}

func buildEnvoyGetCacheOperation1(keys []string) (*types.Struct, error) {
	inlineCode := `function envoy_on_request(request_handle)
  function split_str(s, delimiter)
    res = {}
    for match in (s..delimiter):gmatch("(.-)"..delimiter) do
      table.insert(res, match)
    end

    return res
  end

  local traceid = request_handle:headers():get("sw8")
  if traceid then
    arr = split_str(traceid, "-")
    traceid = arr[2]
  else
    traceid = request_handle:headers():get("x-request-id")
    if not traceid then
      traceid = request_handle:headers():get("x-b3-traceid")
    end
  end

  local headers, body = request_handle:httpCall(
    "cache",
    {
      [":method"] = "GET",
      [":path"] = string.format("/api/cache/%s", traceid),
      [":authority"] = "cache",
    },
    "",
    5000
  )

  request_handle:headers():add("x-env", headers["x-data"]);
end
`
	data := map[string]interface{}{
		"name": "envoy.lua",
		"typed_config": map[string]string{
			"@type":      envoyFilterLua,
			"inlineCode": inlineCode,
		},
	}

	return buildEnvoyPatchValue(data)
}

func buildEnvoyStoreCacheOperation(headerKeys []string) (*types.Struct, error) {
	inlineCodeStart := `function envoy_on_request(request_handle)
  function split_str(s, delimiter)
    res = {}
    for match in (s..delimiter):gmatch("(.-)"..delimiter) do
      table.insert(res, match)
    end

    return res
  end

  local traceid = request_handle:headers():get("sw8")
  if traceid then
    arr = split_str(traceid, "-")
    traceid = arr[2]
  else
    traceid = request_handle:headers():get("x-request-id")
    if not traceid then
      traceid = request_handle:headers():get("x-b3-traceid")
    end
  end

  local env = request_handle:headers():get("x-env")
  local headers, body = request_handle:httpCall(
    "cache",
    {
      [":method"] = "POST",
      [":path"] = string.format("/api/cache/%s/%s", traceid, env),
      [":authority"] = "cache",
    },
    "",
    5000
  )
`
	inlineCodeMid := ``
	for _, headerKey := range headerKeys {
		tmpInlineCodeMid := `
  local header_key = "%s"
  local key = traceid .. "-" .. header_key
  local value = request_handle:headers():get(header_key)
  local headers, body = request_handle:httpCall(
    "cache",
    {
      [":method"] = "POST",
      [":path"] = string.format("/api/cache/%%s/%%s", key, value),
      [":authority"] = "cache",
    },
    "",
    5000
  )
	`
		tmpInlineCodeMid = fmt.Sprintf(tmpInlineCodeMid, headerKey)
		inlineCodeMid += tmpInlineCodeMid
	}

	inlineCodeEnd := `end
`
	inlineCode := fmt.Sprintf(`%s%s%s`, inlineCodeStart, inlineCodeMid, inlineCodeEnd)

	data := map[string]interface{}{
		"name": "envoy.lua",
		"typed_config": map[string]string{
			"@type":      envoyFilterLua,
			"inlineCode": inlineCode,
		},
	}

	return buildEnvoyPatchValue(data)
}

func buildEnvoyGetCacheOperation(headerKeys []string) (*types.Struct, error) {
	inlineCodeStart := `function envoy_on_request(request_handle)
  function split_str(s, delimiter)
    res = {}
    for match in (s..delimiter):gmatch("(.-)"..delimiter) do
      table.insert(res, match)
    end

    return res
  end

  local traceid = request_handle:headers():get("sw8")
  if traceid then
    arr = split_str(traceid, "-")
    traceid = arr[2]
  else
    traceid = request_handle:headers():get("x-request-id")
    if not traceid then
      traceid = request_handle:headers():get("x-b3-traceid")
    end
  end

  local headers, body = request_handle:httpCall(
    "cache",
    {
      [":method"] = "GET",
      [":path"] = string.format("/api/cache/%s", traceid),
      [":authority"] = "cache",
    },
    "",
    5000
  )

  request_handle:headers():add("x-env", headers["x-data"]);
`

	inlineCodeMid := ``
	for _, headerKey := range headerKeys {
		tmpInlineCodeMid := `
  local header_key = "%s"
  local key = traceid .. "-" .. header_key
  local headers, body = request_handle:httpCall(
    "cache",
    {
      [":method"] = "GET",
      [":path"] = string.format("/api/cache/%%s", key),
      [":authority"] = "cache",
    },
    "",
    5000
  )

  request_handle:headers():add(header_key, headers["x-data"]);
	`
		tmpInlineCodeMid = fmt.Sprintf(tmpInlineCodeMid, headerKey)
		inlineCodeMid += tmpInlineCodeMid
	}

	inlineCodeEnd := `end
`
	inlineCode := fmt.Sprintf(`%s%s%s`, inlineCodeStart, inlineCodeMid, inlineCodeEnd)

	data := map[string]interface{}{
		"name": "envoy.lua",
		"typed_config": map[string]string{
			"@type":      envoyFilterLua,
			"inlineCode": inlineCode,
		},
	}

	return buildEnvoyPatchValue(data)
}

func buildEnvoyClusterConfig(cacheAddr string, port int) (*types.Struct, error) {
	data := map[string]interface{}{
		"name":            "cache",
		"type":            "STRICT_DNS",
		"connect_timeout": "3.0s",
		"lb_policy":       "ROUND_ROBIN",
		"load_assignment": EnvoyClusterConfigLoadAssignment{
			ClusterName: "cache",
			Endpoints: []EnvoyLBEndpoints{
				EnvoyLBEndpoints{
					LBEndpoints: []EnvoyEndpoints{
						EnvoyEndpoints{
							Endpoint: EnvoyEndpoint{
								Address: EnvoyAddress{
									SocketAddress: EnvoySocketAddress{
										Protocol:  "TCP",
										Address:   cacheAddr,
										PortValue: port,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return buildEnvoyPatchValue(data)
}

func buildEnvoyPatchValue(data map[string]interface{}) (*types.Struct, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %s", err)
	}

	val := &types.Struct{}
	err = protojson.Unmarshal(dataBytes, val)
	return val, err
}

func deleteGateways(ctx context.Context, kclient client.Client, istioClient versionedclient.Interface, ns string) error {
	zadigLabels := map[string]string{
		zadigtypes.ZadigLabelKeyGlobalOwner: zadigtypes.Zadig,
	}

	gwObjs, err := istioClient.NetworkingV1alpha3().Gateways(ns).List(ctx, metav1.ListOptions{
		LabelSelector: labels.FormatLabels(zadigLabels),
	})
	if err != nil {
		return fmt.Errorf("failed to list gateways in ns `%s`: %s", ns, err)
	}

	deleteOption := metav1.DeletePropagationBackground
	for _, gwObj := range gwObjs.Items {
		err := istioClient.NetworkingV1alpha3().Gateways(ns).Delete(ctx, gwObj.Name, metav1.DeleteOptions{
			PropagationPolicy: &deleteOption,
		})
		if err != nil {
			return fmt.Errorf("failed to delete gateways %s in ns `%s`: %s", gwObj.Name, ns, err)
		}
	}

	gatewaySvc := &corev1.Service{}
	err = kclient.Get(ctx, client.ObjectKey{
		Name:      "istio-ingressgateway",
		Namespace: "istio-system",
	}, gatewaySvc)
	if err != nil {
		return fmt.Errorf("failed to get istio default ingress gateway %s in ns %s, err: %w", "istio-ingressgateway", "istio-system", err)
	}
	newPorts := []corev1.ServicePort{}
	for _, port := range gatewaySvc.Spec.Ports {
		if !strings.HasPrefix(port.Name, zadigNamePrefix) {
			newPorts = append(newPorts, port)
		}
	}
	gatewaySvc.Spec.Ports = newPorts
	err = kclient.Update(ctx, gatewaySvc)
	if err != nil {
		return fmt.Errorf("failed to update istio ingress gateway service %s in namespace %s, err: %w", gatewaySvc.Name, gatewaySvc.Namespace, err)
	}

	return nil
}

func deleteVirtualServices(ctx context.Context, kclient client.Client, istioClient versionedclient.Interface, ns string) error {
	zadigLabels := map[string]string{
		zadigtypes.ZadigLabelKeyGlobalOwner: zadigtypes.Zadig,
	}

	vsObjs, err := istioClient.NetworkingV1alpha3().VirtualServices(ns).List(ctx, metav1.ListOptions{
		LabelSelector: labels.FormatLabels(zadigLabels),
	})
	if err != nil {
		return fmt.Errorf("failed to list VirtualServices in ns `%s`: %s", ns, err)
	}

	deleteOption := metav1.DeletePropagationBackground
	for _, vsObj := range vsObjs.Items {
		err := istioClient.NetworkingV1alpha3().VirtualServices(ns).Delete(ctx, vsObj.Name, metav1.DeleteOptions{
			PropagationPolicy: &deleteOption,
		})
		if err != nil {
			return fmt.Errorf("failed to delete VirtualService %s in ns `%s`: %s", vsObj.Name, ns, err)
		}
	}

	return nil
}

func removeIstioLabel(ctx context.Context, kclient client.Client, ns string) error {
	nsObj := &corev1.Namespace{}
	err := kclient.Get(ctx, client.ObjectKey{
		Name: ns,
	}, nsObj)
	if err != nil {
		return fmt.Errorf("failed to query ns `%s`: %s", ns, err)
	}

	delete(nsObj.Labels, zadigtypes.IstioLabelKeyInjection)
	return kclient.Update(ctx, nsObj)
}

func removePodsIstioProxy(ctx context.Context, kclient client.Client, ns string) error {
	return restartPodsWithIstioProxy(ctx, kclient, ns, true)
}

func genVirtualServiceName(svc *corev1.Service) string {
	return fmt.Sprintf("%s-%s", zadigNamePrefix, svc.Name)
}

func restartPodsWithIstioProxy(ctx context.Context, kclient client.Client, ns string, restartConditionHasIstioProxy bool) error {
	pods := &corev1.PodList{}
	err := kclient.List(ctx, pods, client.InNamespace(ns))
	if err != nil {
		return fmt.Errorf("failed to query pods in ns `%s`: %s", ns, err)
	}

	deleteOption := metav1.DeletePropagationBackground
	for _, pod := range pods.Items {
		hasIstioProxy := false
		for _, container := range pod.Spec.Containers {
			if container.Name == istioProxyName {
				hasIstioProxy = true
				break
			}
		}

		if hasIstioProxy == restartConditionHasIstioProxy {
			err := kclient.Delete(ctx, &pod, &client.DeleteOptions{
				PropagationPolicy: &deleteOption,
			})

			if err != nil {
				return fmt.Errorf("failed to delete Pod `%s` in ns `%s`: %s", pod.Name, ns, err)
			}
		}
	}

	return nil
}

func EnsureGrayEnvConfig(ctx context.Context, env *commonmodels.Product, kclient client.Client, istioClient versionedclient.Interface) error {
	opt := &commonrepo.ProductFindOptions{Name: env.ProductName, EnvName: env.ShareEnv.BaseEnv}
	baseEnv, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return fmt.Errorf("failed to find base env %s of product %s: %s", env.EnvName, env.ProductName, err)
	}

	baseNS := baseEnv.Namespace

	// 1. Deploy VirtualServices of the workloads in gray environment and update them in the base environment.
	err = ensureWorkloadsVirtualServiceInGrayAndBase(ctx, env, baseNS, kclient, istioClient)
	if err != nil {
		return fmt.Errorf("failed to ensure workloads VirtualService: %s", err)
	}

	// 2. Deploy K8s Services and VirtualServices of all workloads in the base environment to the gray environment.
	err = ensureDefaultK8sServiceAndVirtualServicesInGray(ctx, env, baseNS, kclient, istioClient)
	if err != nil {
		return fmt.Errorf("failed to ensure K8s Services and VirtualServices: %s", err)
	}

	return nil
}

func ensureDefaultK8sServiceAndVirtualServicesInGray(ctx context.Context, env *commonmodels.Product, baseNS string, kclient client.Client, istioClient versionedclient.Interface) error {
	svcsInBase := &corev1.ServiceList{}
	err := kclient.List(ctx, svcsInBase, client.InNamespace(baseNS))
	if err != nil {
		return fmt.Errorf("failed to list svcs in %s: %s", baseNS, err)
	}

	grayNS := env.Namespace
	for _, svcInBase := range svcsInBase.Items {
		err = ensureDefaultK8sServiceInGray(ctx, &svcInBase, grayNS, kclient)
		if err != nil {
			return err
		}

		err = ensureDefaultVirtualServiceInGray(ctx, &svcInBase, grayNS, baseNS, istioClient)
		if err != nil {
			return err
		}
	}

	return nil
}

func ensureDefaultK8sServiceInGray(ctx context.Context, baseSvc *corev1.Service, grayNS string, kclient client.Client) error {
	svcInGray := &corev1.Service{}
	err := kclient.Get(ctx, client.ObjectKey{
		Name:      baseSvc.Name,
		Namespace: grayNS,
	}, svcInGray)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	svcInGray.Name = baseSvc.Name
	svcInGray.Namespace = grayNS
	svcInGray.Labels = baseSvc.Labels
	svcInGray.Annotations = baseSvc.Annotations
	svcInGray.Spec.Selector = baseSvc.Spec.Selector

	ports := make([]corev1.ServicePort, len(baseSvc.Spec.Ports))
	for i, port := range baseSvc.Spec.Ports {
		ports[i] = corev1.ServicePort{
			Name:       port.Name,
			Protocol:   port.Protocol,
			Port:       port.Port,
			TargetPort: port.TargetPort,
		}
	}
	svcInGray.Spec.Ports = ports

	if svcInGray.Labels != nil {
		svcInGray.Labels[zadigtypes.ZadigLabelKeyGlobalOwner] = zadigtypes.Zadig
	}

	return kclient.Create(ctx, svcInGray)
}

func ensureDefaultVirtualServiceInGray(ctx context.Context, baseSvc *corev1.Service, grayNS, baseNS string, istioClient versionedclient.Interface) error {
	vsName := genVirtualServiceName(baseSvc)
	svcName := baseSvc.Name
	vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(grayNS).Get(ctx, vsName, metav1.GetOptions{})
	if err == nil {
		log.Infof("Has found VirtualService `%s` in ns `%s` and don't recreate.", vsName, grayNS)
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return err
	}

	vsObj.Name = vsName

	if vsObj.Labels == nil {
		vsObj.Labels = map[string]string{}
	}
	vsObj.Labels[zadigtypes.ZadigLabelKeyGlobalOwner] = zadigtypes.Zadig

	vsObj.Spec = networkingv1alpha3.VirtualService{
		Hosts: []string{svcName},
		Http: []*networkingv1alpha3.HTTPRoute{
			{
				Route: []*networkingv1alpha3.HTTPRouteDestination{
					{
						Destination: &networkingv1alpha3.Destination{
							Host: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, baseNS),
						},
					},
				},
			},
		},
	}
	_, err = istioClient.NetworkingV1alpha3().VirtualServices(grayNS).Create(ctx, vsObj, metav1.CreateOptions{})
	return err
}

// Note: Currently we have made an assumption that all the necessary K8s services exist in the current environment.
func ensureWorkloadsVirtualServiceInGrayAndBase(ctx context.Context, env *commonmodels.Product, baseNS string, kclient client.Client, istioClient versionedclient.Interface) error {
	svcs := &corev1.ServiceList{}
	grayNS := env.Namespace
	err := kclient.List(ctx, svcs, client.InNamespace(grayNS))
	if err != nil {
		return err
	}

	for _, svc := range svcs.Items {
		// If there is no workloads in the sub-environment, the service is not updated.
		hasWorkload, err := doesSvcHasWorkload(ctx, grayNS, labels.SelectorFromSet(labels.Set(svc.Spec.Selector)), kclient)
		if err != nil {
			return err
		}

		if !hasWorkload {
			continue
		}

		vsName := genVirtualServiceName(&svc)

		err = ensureVirtualServiceInGray(ctx, env.EnvName, vsName, svc.Name, grayNS, baseNS, istioClient)
		if err != nil {
			return err
		}

		err = ensureUpdateVirtualServiceInBase(ctx, env.EnvName, vsName, svc.Name, grayNS, baseNS, istioClient)
		if err != nil {
			return err
		}
	}

	return nil
}

func ensureVirtualServiceInGray(ctx context.Context, envName, vsName, svcName, grayNS, baseNS string, istioClient versionedclient.Interface) error {
	var isExisted bool

	vsObjInGray, err := istioClient.NetworkingV1alpha3().VirtualServices(grayNS).Get(ctx, vsName, metav1.GetOptions{})
	if err == nil {
		isExisted = true
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	vsObjInGray.Name = vsName
	vsObjInGray.Namespace = grayNS

	if vsObjInGray.Labels == nil {
		vsObjInGray.Labels = map[string]string{}
	}
	vsObjInGray.Labels[zadigtypes.ZadigLabelKeyGlobalOwner] = zadigtypes.Zadig

	vsObjInGray.Spec = networkingv1alpha3.VirtualService{
		Hosts: []string{svcName},
		Http: []*networkingv1alpha3.HTTPRoute{
			&networkingv1alpha3.HTTPRoute{
				Match: []*networkingv1alpha3.HTTPMatchRequest{
					&networkingv1alpha3.HTTPMatchRequest{
						Headers: map[string]*networkingv1alpha3.StringMatch{
							zadigMatchXEnv: &networkingv1alpha3.StringMatch{
								MatchType: &networkingv1alpha3.StringMatch_Exact{
									Exact: envName,
								},
							},
						},
					},
				},
				Route: []*networkingv1alpha3.HTTPRouteDestination{
					&networkingv1alpha3.HTTPRouteDestination{
						Destination: &networkingv1alpha3.Destination{
							Host: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, grayNS),
						},
					},
				},
			},
			&networkingv1alpha3.HTTPRoute{
				Route: []*networkingv1alpha3.HTTPRouteDestination{
					&networkingv1alpha3.HTTPRouteDestination{
						Destination: &networkingv1alpha3.Destination{
							Host: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, baseNS),
						},
					},
				},
			},
		},
	}

	if isExisted {
		_, err = istioClient.NetworkingV1alpha3().VirtualServices(grayNS).Update(ctx, vsObjInGray, metav1.UpdateOptions{})
	} else {
		_, err = istioClient.NetworkingV1alpha3().VirtualServices(grayNS).Create(ctx, vsObjInGray, metav1.CreateOptions{})
	}

	return err
}

func ensureUpdateVirtualServiceInBase(ctx context.Context, envName, vsName, svcName, grayNS, baseNS string, istioClient versionedclient.Interface) error {
	vsObjInBase, err := istioClient.NetworkingV1alpha3().VirtualServices(baseNS).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if vsObjInBase.Spec.Http == nil {
		vsObjInBase.Spec.Http = []*networkingv1alpha3.HTTPRoute{}
	}

	for _, vsHttp := range vsObjInBase.Spec.Http {
		if len(vsHttp.Match) == 0 {
			continue
		}

		for _, vsMatch := range vsHttp.Match {
			if len(vsMatch.Headers) == 0 {
				continue
			}

			matchValue, found := vsMatch.Headers[zadigMatchXEnv]
			if !found {
				continue
			}

			if matchValue.GetExact() == envName {
				return nil
			}
		}
	}

	grayRoute := &networkingv1alpha3.HTTPRoute{
		Match: []*networkingv1alpha3.HTTPMatchRequest{
			&networkingv1alpha3.HTTPMatchRequest{
				Headers: map[string]*networkingv1alpha3.StringMatch{
					zadigMatchXEnv: &networkingv1alpha3.StringMatch{
						MatchType: &networkingv1alpha3.StringMatch_Exact{
							Exact: envName,
						},
					},
				},
			},
		},
		Route: []*networkingv1alpha3.HTTPRouteDestination{
			&networkingv1alpha3.HTTPRouteDestination{
				Destination: &networkingv1alpha3.Destination{
					Host: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, grayNS),
				},
			},
		},
	}

	numRoutes := len(vsObjInBase.Spec.Http)
	if numRoutes == 0 {
		vsObjInBase.Spec.Http = append(vsObjInBase.Spec.Http, grayRoute)
	} else {
		routes := make([]*networkingv1alpha3.HTTPRoute, 1, numRoutes+1)
		routes[0] = grayRoute
		routes = append(routes, vsObjInBase.Spec.Http...)
		vsObjInBase.Spec.Http = routes
	}

	_, err = istioClient.NetworkingV1alpha3().VirtualServices(baseNS).Update(ctx, vsObjInBase, metav1.UpdateOptions{})
	return err
}

func EnsureDeleteShareEnvConfig(ctx context.Context, env *commonmodels.Product, istioClient versionedclient.Interface) error {
	if !env.ShareEnv.Enable {
		return nil
	}

	if env.ShareEnv.IsBase {
		// 1. Delete all associated subenvironments.
		err := ensureDeleteAssociatedEnvs(ctx, env)
		if err != nil {
			return err
		}

		// 2. If no environment is shared, delete EnvoyFilter.
		return ensureDeleteEnvoyFilter(ctx, env, istioClient)
	}

	// Updated the VirtualService configuration in the base environment.
	return ensureCleanRoutesInBase(ctx, env, istioClient)
}

func EnsureUpdateZadigService(ctx context.Context, env *commonmodels.Product, svcName string, kclient client.Client, istioClient versionedclient.Interface) error {
	if !env.ShareEnv.Enable {
		return nil
	}

	// Note: A Service may not be queried immediately after it is created.
	var err error
	svc := &corev1.Service{}
	for i := 0; i < 3; i++ {
		err = kclient.Get(ctx, client.ObjectKey{
			Name:      svcName,
			Namespace: env.Namespace,
		}, svc)
		if err == nil {
			break
		}

		log.Warnf("Failed to query Service %s in ns %s: %s", svcName, env.Namespace, err)
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("failed to query Service %s in ns %s: %s", svcName, env.Namespace, err)
	}

	return ensureUpdateZadigSerivce(ctx, env, svc, kclient, istioClient)
}

func EnsureDeleteZadigService(ctx context.Context, env *commonmodels.Product, svcName string, kclient client.Client, istioClient versionedclient.Interface) error {
	if !env.ShareEnv.Enable && !env.IstioGrayscale.Enable {
		return nil
	}

	svc := &corev1.Service{}
	err := kclient.Get(ctx, client.ObjectKey{
		Name:      svcName,
		Namespace: env.Namespace,
	}, svc)
	if err != nil {
		return fmt.Errorf("failed to get Service %s in ns %s, err: %s", svcName, env.Namespace, err)
	}

	if env.ShareEnv.Enable {
		return ensureDeleteZadigService(ctx, env, svc, kclient, istioClient)
	} else if env.IstioGrayscale.Enable {
		return kube.EnsureDeleteGrayscaleService(ctx, env, svc, kclient, istioClient)
	}
	return nil
}

func GetEnvServiceList(ctx context.Context, productName, baseEnvName string) ([][]string, error) {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: baseEnvName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query env `%s` in project `%s`: %s", baseEnvName, productName, err)
	}

	svcGroupNames := make([][]string, len(env.Services))
	for i, svcArr := range env.Services {
		svcGroupNames[i] = []string{}
		for _, svc := range svcArr {
			svcGroupNames[i] = append(svcGroupNames[i], svc.ServiceName)
		}
	}

	return svcGroupNames, nil
}

// Map of map[ServiceName][]string{EnvName} is returned.
func CheckServicesDeployedInSubEnvs(ctx context.Context, productName, envName string, services []string) (map[string][]string, error) {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query env %s of product %s: %s", envName, productName, err)
	}
	if !env.ShareEnv.Enable || !env.ShareEnv.IsBase {
		return nil, nil
	}

	envs, err := fetchSubEnvs(ctx, productName, env.ClusterID, envName)
	if err != nil {
		return nil, err
	}

	envsSvcs := make(map[string][]string, len(envs))
	for _, env := range envs {
		envsSvcs[env.EnvName] = getSvcInEnv(env)
	}

	svcsInSubEnvs := map[string][]string{}
	for _, svcName := range services {
		for envName, svcNames := range envsSvcs {
			if !zadigutil.InStringArray(svcName, svcNames) {
				continue
			}

			if _, found := svcsInSubEnvs[svcName]; !found {
				svcsInSubEnvs[svcName] = []string{}
			}
			svcsInSubEnvs[svcName] = append(svcsInSubEnvs[svcName], envName)
		}
	}

	return svcsInSubEnvs, nil
}

type GetPortalServiceResponse struct {
	DefaultGatewayAddress string                      `json:"default_gateway_address"`
	Servers               []SetupPortalServiceRequest `json:"servers"`
}

func GetPortalService(ctx context.Context, productName, envName, serviceName string) (GetPortalServiceResponse, error) {
	resp := GetPortalServiceResponse{}
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return resp, e.ErrGetPortalService.AddErr(fmt.Errorf("failed to query env %s of product %s: %s", envName, productName, err))
	}
	if !env.ShareEnv.Enable && !env.ShareEnv.IsBase {
		return resp, e.ErrGetPortalService.AddDesc("%s doesn't enable share environment or is not base environment")
	}

	ns := env.Namespace
	clusterID := env.ClusterID

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return resp, e.ErrGetPortalService.AddErr(fmt.Errorf("failed to get kube client: %s", err))
	}
	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return resp, e.ErrGetPortalService.AddErr(fmt.Errorf("failed to get rest config: %s", err))
	}
	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return resp, e.ErrGetPortalService.AddErr(fmt.Errorf("failed to new istio client: %s", err))
	}

	gatewayName := commonutil.GenIstioGatewayName(serviceName)
	resp.DefaultGatewayAddress, err = getDefaultIstioIngressGatewayAddress(ctx, serviceName, gatewayName, err, kclient)
	if err != nil {
		return resp, e.ErrGetPortalService.AddErr(fmt.Errorf("failed to get default istio ingress gateway address: %s", err))
	}

	resp.Servers, err = getIstioGatewayConfig(ctx, istioClient, ns, gatewayName)
	if err != nil {
		return resp, e.ErrGetPortalService.AddErr(fmt.Errorf("failed to get istio gateway config: %s", err))
	}

	return resp, nil
}

func getIstioGatewayConfig(ctx context.Context, istioClient *versionedclient.Clientset, ns string, gatewayName string) ([]SetupPortalServiceRequest, error) {
	servers := []SetupPortalServiceRequest{}
	gwObj, err := istioClient.NetworkingV1alpha3().Gateways(ns).Get(ctx, gatewayName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return servers, nil
		} else {
			return servers, fmt.Errorf("failed to get gateway %s in namespace %s, err: %w", gatewayName, ns, err)
		}
	}

	for _, server := range gwObj.Spec.Servers {
		if len(server.Hosts) == 0 {
			return servers, fmt.Errorf("can't find any host in istio gateway")
		}
		servers = append(servers, SetupPortalServiceRequest{
			Host:         server.Hosts[0],
			PortNumber:   server.Port.Number,
			PortProtocol: server.Port.Protocol,
		})
	}
	return servers, nil
}

func getDefaultIstioIngressGatewayAddress(ctx context.Context, serviceName, gatewayName string, err error, kclient client.Client) (string, error) {
	defaultGatewayAddress := ""
	gatewaySvc := &corev1.Service{}
	err = kclient.Get(ctx, client.ObjectKey{
		Name:      "istio-ingressgateway",
		Namespace: "istio-system",
	}, gatewaySvc)
	if err != nil {
		return "", fmt.Errorf("failed to get istio default ingress gateway %s in ns %s, err: %w", "istio-ingressgateway", "istio-system", err)
	}
	if len(gatewaySvc.Status.LoadBalancer.Ingress) == 0 {
		return "", e.ErrGetPortalService.AddDesc("istio default gateway's lb doesn't have ip address")
	}
	for _, ing := range gatewaySvc.Status.LoadBalancer.Ingress {
		if ing.IP != "" {
			defaultGatewayAddress = ing.IP
		}
		if ing.Hostname != "" {
			defaultGatewayAddress = ing.Hostname
		}
	}
	return defaultGatewayAddress, nil
}

type SetupPortalServiceRequest struct {
	Host         string `json:"host"`
	PortNumber   uint32 `json:"port_number"`
	PortProtocol string `json:"port_protocol"`
}

func SetupPortalService(ctx context.Context, productName, envName, serviceName string, servers []SetupPortalServiceRequest) error {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to query env %s of product %s: %s", envName, productName, err))
	}
	if !env.ShareEnv.Enable && !env.ShareEnv.IsBase {
		return e.ErrSetupPortalService.AddDesc("%s doesn't enable share environment or is not base environment")
	}

	templateProd, err := template.NewProductColl().Find(productName)
	if err != nil {
		return e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to find template product %s, err: %w", productName, err))
	}

	ns := env.Namespace
	clusterID := env.ClusterID

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to get kube client: %s", err))
	}
	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to get rest config: %s", err))
	}
	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to new istio client: %s", err))
	}
	gatewayName := commonutil.GenIstioGatewayName(serviceName)

	if len(servers) == 0 {
		// delete operation
		err = cleanIstioIngressGatewayService(ctx, err, istioClient, ns, gatewayName, kclient)
		if err != nil {
			return e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to clean istio ingress gateway service, err: %w", err))
		}
	} else {
		// 1. check istio ingress gateway whether has load balancing
		gatewaySvc, err := checkIstioIngressGatewayLB(ctx, err, kclient)
		if err != nil {
			return e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to check istio ingress gateway's load balancing, err: %w", err))
		}

		// 2. create gateway for the service
		err = createIstioGateway(ctx, istioClient, ns, gatewayName, servers)
		if err != nil {
			return e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to create gateway %s in namespace %s, err: %w", gatewayName, ns, err))
		}

		// 3. patch istio-ingressgateway service's port
		err = patchIstioIngressGatewayServicePort(ctx, gatewaySvc, servers, kclient)
		if err != nil {
			return e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to patch istio ingress gateway service's port, err: %w", err))
		}
	}

	// 4. change the related virtualservice
	svcs := []*corev1.Service{}
	deployType := templateProd.ProductFeature.GetDeployType()
	if deployType == setting.K8SDeployType {
		svcs, err = parseK8SProjectServices(ctx, env, serviceName, kclient, ns)
		if err != nil {
			return e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to parse k8s project services, err: %w", err))
		}
	} else if deployType == setting.HelmDeployType {
		svcs, err = parseHelmProjectServices(ctx, restConfig, env, envName, productName, serviceName, kclient, ns)
		if err != nil {
			return e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to parse helm project services, err: %w", err))
		}
	}

	for _, svc := range svcs {
		err = updateVirtualServiceForPortalService(ctx, svc, istioClient, ns, servers, gatewayName)
		if err != nil {
			return e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to update virtualservice for portal service, err: %w", err))
		}
	}

	return nil
}

func updateVirtualServiceForPortalService(ctx context.Context, svc *corev1.Service, istioClient *versionedclient.Clientset, ns string, servers []SetupPortalServiceRequest, gatewayName string) error {
	vsName := genVirtualServiceName(svc)
	vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(ns).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get vritualservice %s in namespace %s, err: %w", vsName, ns, err)
	}

	if len(servers) == 0 {
		// delete operation
		vsObj.Spec.Gateways = sets.NewString(vsObj.Spec.Gateways...).Delete(gatewayName).List()
		vsObj.Spec.Hosts = []string{svc.Name}
	} else {
		vsObj.Spec.Gateways = []string{gatewayName}
		vsObj.Spec.Hosts = []string{svc.Name}
		for _, server := range servers {
			vsObj.Spec.Hosts = append(vsObj.Spec.Hosts, server.Host)
		}
	}
	_, err = istioClient.NetworkingV1alpha3().VirtualServices(ns).Update(ctx, vsObj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update virtualservice %s in namespace %s, err: %v", vsName, ns, err)
	}
	return nil
}

func parseHelmProjectServices(ctx context.Context, restConfig *rest.Config, env *commonmodels.Product, envName string, productName string, serviceName string, kclient client.Client, ns string) ([]*corev1.Service, error) {
	svcs := []*corev1.Service{}
	helmClient, err := helmtool.NewClientFromRestConf(restConfig, env.Namespace)
	if err != nil {
		log.Errorf("[%s][%s] NewClientFromRestConf error: %s", envName, productName, err)
		return nil, fmt.Errorf("failed to init helm client, err: %s", err)
	}

	releaseName := serviceName
	isHelmChartDeploy := false
	if !isHelmChartDeploy {
		serviceMap := env.GetServiceMap()
		prodSvc, ok := serviceMap[serviceName]
		if !ok {
			return nil, fmt.Errorf("failed to find sercice %s in env %s", serviceName, envName)
		}

		revisionSvc, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
			ServiceName: serviceName,
			Revision:    prodSvc.Revision,
			ProductName: prodSvc.ProductName,
		}, env.Production)
		if err != nil {
			return nil, fmt.Errorf("failed to query template service %s/%s/%d, err: %w",
				productName, serviceName, prodSvc.Revision, err)
		}

		releaseName = zadigutil.GeneReleaseName(revisionSvc.GetReleaseNaming(), prodSvc.ProductName, env.Namespace, env.EnvName, prodSvc.ServiceName)
	}

	release, err := helmClient.GetRelease(releaseName)
	if err != nil {
		return nil, fmt.Errorf("failed to get release %s, err: %w", releaseName, err)
	}
	svcNames, err := util.GetSvcNamesFromManifest(release.Manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to get Service names from manifest, err: %w", err)
	}
	for _, svcName := range svcNames {
		svc := &corev1.Service{}
		err = kclient.Get(ctx, client.ObjectKey{
			Name:      svcName,
			Namespace: env.Namespace,
		}, svc)
		if err != nil {
			return nil, e.ErrSetupPortalService.AddErr(fmt.Errorf("failed to get service %s in namespace %s, err: %w", svcName, ns, err))
		}
		svcs = append(svcs, svc)
	}
	return svcs, nil
}

func parseK8SProjectServices(ctx context.Context, env *commonmodels.Product, serviceName string, kclient client.Client, ns string) ([]*corev1.Service, error) {
	svcs := []*corev1.Service{}
	prodSvc := env.GetServiceMap()[serviceName]
	if prodSvc == nil {
		return nil, fmt.Errorf("can't find %s in env %s", serviceName, env.EnvName)
	}
	yaml, err := kube.RenderEnvService(env, prodSvc.GetServiceRender(), prodSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to render env service, err: %w", err)
	}
	resources, err := kube.ManifestToUnstructured(yaml)
	if err != nil {
		return nil, fmt.Errorf("failed to convert yaml to Unstructured, manifest is\n%s\n, error: %v", yaml, err)
	}

	for _, resource := range resources {
		if resource.GetKind() == setting.Service {
			svc := &corev1.Service{}
			err = kclient.Get(ctx, client.ObjectKey{
				Name:      resource.GetName(),
				Namespace: ns,
			}, svc)
			if err != nil {
				return nil, fmt.Errorf("failed to get service %s in namespace %s, err: %w", resource.GetName(), ns, err)
			}
			svcs = append(svcs, svc)
		}
	}
	return svcs, nil
}

func patchIstioIngressGatewayServicePort(ctx context.Context, gatewaySvc *corev1.Service, servers []SetupPortalServiceRequest, kclient client.Client) error {
	newPorts := []corev1.ServicePort{}
	hasPortSet := sets.NewInt32()
	for _, port := range gatewaySvc.Spec.Ports {
		if !strings.HasPrefix(port.Name, zadigNamePrefix) {
			newPorts = append(newPorts, port)
			hasPortSet.Insert(port.Port)
		}
	}
	for _, server := range servers {
		if hasPortSet.Has(int32(server.PortNumber)) {
			continue
		}
		port := corev1.ServicePort{
			Name:     fmt.Sprintf("%s-%s-%d", zadigNamePrefix, "http", server.PortNumber),
			Protocol: "TCP",
			Port:     int32(server.PortNumber),
		}
		newPorts = append(newPorts, port)
	}
	gatewaySvc.Spec.Ports = newPorts
	err := kclient.Update(ctx, gatewaySvc)
	if err != nil {
		return fmt.Errorf("failed to update istio default ingress gateway service %s in namespace %s, err: %w", gatewaySvc.Name, gatewaySvc.Namespace, err)
	}
	return nil
}

func cleanIstioIngressGatewayService(ctx context.Context, err error, istioClient *versionedclient.Clientset, ns string, gatewayName string, kclient client.Client) error {
	err = istioClient.NetworkingV1alpha3().Gateways(ns).Delete(ctx, gatewayName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete gateway %s in namespace %s, err: %w", gatewayName, ns, err)
	}

	// remove ports which we added to istio ingress gateway svc
	gatewaySvc := &corev1.Service{}
	err = kclient.Get(ctx, client.ObjectKey{
		Name:      "istio-ingressgateway",
		Namespace: "istio-system",
	}, gatewaySvc)
	if err != nil {
		return fmt.Errorf("failed to get istio default ingress gateway %s in ns %s, err: %w", "istio-ingressgateway", "istio-system", err)
	}
	newPorts := []corev1.ServicePort{}
	for _, port := range gatewaySvc.Spec.Ports {
		if !strings.HasPrefix(port.Name, zadigNamePrefix) {
			newPorts = append(newPorts, port)
		}
	}
	gatewaySvc.Spec.Ports = newPorts
	err = kclient.Update(ctx, gatewaySvc)
	if err != nil {
		return fmt.Errorf("failed to update istio ingress gateway service %s in namespace %s, err: %w", gatewaySvc.Name, gatewaySvc.Namespace, err)
	}
	return nil
}

func checkIstioIngressGatewayLB(ctx context.Context, err error, kclient client.Client) (*corev1.Service, error) {
	gatewaySvc := &corev1.Service{}
	err = kclient.Get(ctx, client.ObjectKey{
		Name:      "istio-ingressgateway",
		Namespace: "istio-system",
	}, gatewaySvc)
	if err != nil {
		return nil, fmt.Errorf("failed to get istio default ingress gateway %s in ns %s, err: %w", "istio-ingressgateway", "istio-system", err)
	}
	if len(gatewaySvc.Status.LoadBalancer.Ingress) == 0 {
		return nil, fmt.Errorf("istio default gateway's lb doesn't have ip address or hostname")
	}
	return gatewaySvc, nil
}

func createIstioGateway(ctx context.Context, istioClient *versionedclient.Clientset, ns string, gatewayName string, servers []SetupPortalServiceRequest) error {
	isExisted := false
	gwObj, err := istioClient.NetworkingV1alpha3().Gateways(ns).Get(ctx, gatewayName, metav1.GetOptions{})
	if err == nil {
		isExisted = true
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get gateway %s in namespace %s, err: %w", gatewayName, ns, err)
	}

	gwObj.Name = gatewayName
	gwObj.Namespace = ns
	if gwObj.Labels == nil {
		gwObj.Labels = map[string]string{}
	}
	gwObj.Labels[zadigtypes.ZadigLabelKeyGlobalOwner] = zadigtypes.Zadig
	gwObj.Spec = networkingv1alpha3.Gateway{
		Selector: map[string]string{"istio": "ingressgateway"},
	}
	for _, server := range servers {
		serverObj := &networkingv1alpha3.Server{
			Hosts: []string{server.Host},
			Port: &networkingv1alpha3.Port{
				Name:     fmt.Sprintf("%s:%s:%d", server.Host, server.PortProtocol, server.PortNumber),
				Number:   server.PortNumber,
				Protocol: server.PortProtocol,
			},
		}
		gwObj.Spec.Servers = append(gwObj.Spec.Servers, serverObj)
	}

	if !isExisted {
		_, err = istioClient.NetworkingV1alpha3().Gateways(ns).Create(ctx, gwObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create gateway %s in namespace %s, err: %w", gatewayName, ns, err)
		}
	} else {
		_, err = istioClient.NetworkingV1alpha3().Gateways(ns).Update(ctx, gwObj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update gateway %s in namespace %s, err: %w", gatewayName, ns, err)
		}
	}
	return nil
}
