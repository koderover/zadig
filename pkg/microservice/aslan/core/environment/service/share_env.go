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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	types "github.com/golang/protobuf/ptypes/struct"
	helmclient "github.com/mittwald/go-helm-client"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/kube/util"
	"github.com/koderover/zadig/pkg/tool/log"
	zadigtypes "github.com/koderover/zadig/pkg/types"
	zadigutil "github.com/koderover/zadig/pkg/util"
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
	err = ensureEnvoyFilter(ctx, istioClient, clusterID, istioNamespace, zadigEnvoyFilter)
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

	// 3. Delete all VirtualServices delivered by the Zadig.
	err = deleteVirtualServices(ctx, kclient, istioClient, ns)
	if err != nil {
		return fmt.Errorf("failed to delete VirtualServices that Zadig created in ns `%s`: %s", ns, err)
	}

	// 4. Remove the `istio-injection=enabled` label of the namespace.
	err = removeIstioLabel(ctx, kclient, ns)
	if err != nil {
		return fmt.Errorf("failed to remove istio label on ns `%s`: %s", ns, err)
	}

	// 5. Restart the istio-Proxy injected Pods.
	err = removePodsIstioProxy(ctx, kclient, ns)
	if err != nil {
		return fmt.Errorf("failed to remove istio-proxy from pods in ns `%s`: %s", ns, err)
	}

	// 6. Update the environment configuration.
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

func ensureEnvoyFilter(ctx context.Context, istioClient versionedclient.Interface, clusterID, ns, name string) error {
	envoyFilterObj, err := istioClient.NetworkingV1alpha3().EnvoyFilters(ns).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		log.Infof("Has found EnvoyFilter `%s` in ns `%s` and don't recreate.", name, istioNamespace)
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to query EnvoyFilter `%s` in ns `%s`: %s", name, istioNamespace, err)
	}

	storeCacheOperation, err := buildEnvoyStoreCacheOperation()
	if err != nil {
		return fmt.Errorf("failed to build envoy operation of storing cache: %s", err)
	}

	getCacheOperation, err := buildEnvoyGetCacheOperation()
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

func buildEnvoyStoreCacheOperation() (*types.Struct, error) {
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

func buildEnvoyGetCacheOperation() (*types.Struct, error) {
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

	reader := bytes.NewReader(dataBytes)
	val := &types.Struct{}
	err = jsonpb.Unmarshal(reader, val)
	return val, err
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

func EnsureDeleteZadigService(ctx context.Context, env *commonmodels.Product, svcSelector labels.Selector, kclient client.Client, istioClient versionedclient.Interface) error {
	if !env.ShareEnv.Enable {
		return nil
	}

	svcList := &corev1.ServiceList{}
	err := kclient.List(ctx, svcList, &client.ListOptions{
		Namespace:     env.Namespace,
		LabelSelector: svcSelector,
	})
	if err != nil {
		return util.IgnoreNotFoundError(err)
	}

	if len(svcList.Items) != 1 {
		return fmt.Errorf("Length of svc list is not expected for env %s of product %s with selector %s. Expected 1 but got %d.", env.EnvName, env.ProductName, svcSelector.String(), len(svcList.Items))
	}
	svc := &svcList.Items[0]

	return ensureDeleteZadigService(ctx, env, svc, kclient, istioClient)
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

func EnsureZadigServiceByManifest(ctx context.Context, productName, namespace, manifest string) error {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:      productName,
		Namespace: namespace,
	})
	if err != nil {
		return fmt.Errorf("failed to query namespace %q in project %q: %s", namespace, productName, err)
	}

	if !env.ShareEnv.Enable {
		return nil
	}

	svcNames, err := util.GetSvcNamesFromManifest(manifest)
	if err != nil {
		return fmt.Errorf("failed to get Service names from manifest: %s", err)
	}

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %s", err)
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get rest config: %s", err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to get istio client: %s", err)
	}

	for _, svcName := range svcNames {
		err := EnsureUpdateZadigService(ctx, env, svcName, kclient, istioClient)
		if err != nil {
			return fmt.Errorf("failed to ensure Zadig Service for K8s Service %q in env %q of product %q: %s", svcName, env.EnvName, env.ProductName, err)
		}
	}

	return nil
}

func EnsureDeleteZadigServiceByHelmRelease(ctx context.Context, env *commonmodels.Product, releaseName string, helmClient helmclient.Client) error {
	if !env.ShareEnv.Enable {
		return nil
	}

	release, err := helmClient.GetRelease(releaseName)
	if err != nil {
		return fmt.Errorf("failed to get release %q in namespace %q: %s", releaseName, env.Namespace, err)
	}

	svcNames, err := util.GetSvcNamesFromManifest(release.Manifest)
	if err != nil {
		return fmt.Errorf("failed to get Service names from manifest: %s", err)
	}

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %s", err)
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get rest config: %s", err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to get istio client: %s", err)
	}

	for _, svcName := range svcNames {
		err = ensureDeleteZadigServiceBySvcName(ctx, env, svcName, kclient, istioClient)
		if err != nil {
			return fmt.Errorf("failed to ensure deleting Zadig service by service name %q for env %q of product %q: %s", svcName, env.EnvName, env.ProductName, err)
		}
	}

	return nil
}

func ensureDeleteZadigServiceBySvcName(ctx context.Context, env *commonmodels.Product, svcName string, kclient client.Client, istioClient versionedclient.Interface) error {
	svc := &corev1.Service{}
	err := kclient.Get(ctx, client.ObjectKey{
		Name:      svcName,
		Namespace: env.Namespace,
	}, svc)
	if err != nil {
		return fmt.Errorf("failed to find Service %q in namespace %q: %s", svcName, env.Namespace, err)
	}

	return ensureDeleteZadigService(ctx, env, svc, kclient, istioClient)
}

func EnsureDeletePreCreatedServices(ctx context.Context, productName, namespace string, chartSpec *helmclient.ChartSpec, helmClient *helmtool.HelmClient) error {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:      productName,
		Namespace: namespace,
	})
	if err != nil {
		return fmt.Errorf("failed to query namespace %q in project %q: %s", namespace, productName, err)
	}

	if !(env.ShareEnv.Enable && !env.ShareEnv.IsBase) {
		return nil
	}

	manifestBytes, err := helmClient.TemplateChart(chartSpec)
	if err != nil {
		return fmt.Errorf("failed template chart %q for release %q in namespace %q: %s", chartSpec.ChartName, chartSpec.ReleaseName, chartSpec.Namespace, err)
	}

	svcNames, err := util.GetSvcNamesFromManifest(string(manifestBytes))
	if err != nil {
		return fmt.Errorf("failed to get Service names from manifest: %s", err)
	}

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %s", err)
	}

	for _, svcName := range svcNames {
		err := ensureDeleteK8sService(ctx, namespace, svcName, kclient, true)
		if err != nil {
			return fmt.Errorf("failed to ensure delete existing K8s Service %q in namespace %q: %s", svcName, namespace, err)
		}
	}

	return nil
}
