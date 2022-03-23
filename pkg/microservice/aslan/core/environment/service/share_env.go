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
	"errors"
	"fmt"
	"time"

	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/log"
)

var ErrNotImplemented = errors.New("not implemented")

const istioNamespace = "istio-system"
const istioProxyName = "istio-proxy"
const zadigEnvoyFilter = "zadig-share-env"

// Slice of `<workload name>.<workload type>` and error are returned.
func CheckWorkloadsK8sServices(ctx context.Context, envName, productName string) ([]string, error) {
	timeStart := time.Now()
	defer func() {
		log.Infof("[CheckWorkloadsK8sServices]Time consumed: %s", time.Since(timeStart))
	}()

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
	err = ensureVirtualServices(ctx, kclient, istioClient, ns)
	if err != nil {
		return fmt.Errorf("failed to ensure VirtualServices in namespace `%s`: %s", ns, err)
	}

	// 4. Ensure `EnvoyFilter` in istio namespace.
	err = ensureEnvoyFilter(ctx, istioClient, istioNamespace, zadigEnvoyFilter)
	if err != nil {
		return fmt.Errorf("failed to ensure EnvoyFilter in namespace `%s`: %s", istioNamespace, err)
	}

	return nil
}

func DisableBaseEnv(ctx context.Context, envName, productName string) error {

	return ErrNotImplemented
}

func CheckShareEnvReady(ctx context.Context, envName, productName string) (*ShareEnvReady, error) {
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
	isPodsHaveIstioProxyAndReady, err := checkPodsWithIstioProxyAndReady(ctx, kclient, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to check whether all pods in ns `%s` have istio-proxy and are ready: %s", ns, err)
	}

	res := &ShareEnvReady{
		Checks: ShareEnvReadyChecks{
			NamespaceHasIstioLabel:     isNamespaceHasIstioLabel,
			WorkloadsHaveK8sService:    isWorkloadsHaveNoK8sService,
			VirtualServicesDeployed:    isVirtualServicesDeployed,
			PodsHaveIstioProxyAndReady: isPodsHaveIstioProxyAndReady,
		},
	}
	res.CheckAndSetReady()

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

	nsObj.Labels["istio-injection"] = "enabled"
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

	return nsObj.Labels["istio-injection"] == "enabled", nil
}

func ensurePodsWithIsitoProxy(ctx context.Context, kclient client.Client, ns string) error {
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

		if !hasIstioProxy {
			err := kclient.Delete(ctx, &pod, &client.DeleteOptions{
				PropagationPolicy: &deleteOption,
			})

			if err != nil {
				return fmt.Errorf("failed to ensure Pod `%s` in ns `%s` to inject istio-proxy: %s", pod.Name, ns, err)
			}
		}
	}

	return nil
}

func checkPodsWithIstioProxyAndReady(ctx context.Context, kclient client.Client, ns string) (bool, error) {
	pods := &corev1.PodList{}
	err := kclient.List(ctx, pods, client.InNamespace(ns))
	if err != nil {
		return false, fmt.Errorf("failed to query pods in ns `%s`: %s", ns, err)
	}

	for _, pod := range pods.Items {
		if !(pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodSucceeded) {
			return false, nil
		}

		hasIstioProxy := false
		for _, container := range pod.Spec.Containers {
			if container.Name == istioProxyName {
				hasIstioProxy = true
				break
			}
		}
		if !hasIstioProxy {
			return false, nil
		}
	}

	return true, nil
}

func ensureVirtualServices(ctx context.Context, kclient client.Client, istioClient versionedclient.Interface, ns string) error {
	svcs := &corev1.ServiceList{}
	err := kclient.List(ctx, svcs, client.InNamespace(ns))
	if err != nil {
		return fmt.Errorf("failed to list svcs in ns `%s`: %s", ns, err)
	}

	for _, svc := range svcs.Items {
		err := ensureVirtualService(ctx, istioClient, ns, svc.Name)
		if err != nil {
			return fmt.Errorf("failed to ensure VirtualService `%s` in ns `%s`: %s", svc.Name, ns, err)
		}
	}

	return nil
}

func ensureVirtualService(ctx context.Context, istioClient versionedclient.Interface, ns, name string) error {
	vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(ns).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		log.Infof("Has found VirtualService `%s` in ns `%s` and don't recreate.", name, ns)
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to query VirtualService `%s` in ns `%s`: %s", name, ns, err)
	}

	vsObj.Name = name
	vsObj.Spec = networkingv1alpha3.VirtualService{
		Hosts: []string{name},
		Http: []*networkingv1alpha3.HTTPRoute{
			&networkingv1alpha3.HTTPRoute{
				Route: []*networkingv1alpha3.HTTPRouteDestination{
					&networkingv1alpha3.HTTPRouteDestination{
						Destination: &networkingv1alpha3.Destination{
							Host: fmt.Sprintf("%s.%s.svc.cluster.local", name, ns),
						},
					},
				},
			},
		},
	}
	_, err = istioClient.NetworkingV1alpha3().VirtualServices(ns).Create(ctx, vsObj, metav1.CreateOptions{})
	return err
}

func checkVirtualServicesDeployed(ctx context.Context, kclient client.Client, istioClient versionedclient.Interface, ns string) (bool, error) {
	svcs := &corev1.ServiceList{}
	err := kclient.List(ctx, svcs, client.InNamespace(ns))
	if err != nil {
		return false, fmt.Errorf("failed to list svcs in ns `%s`: %s", ns, err)
	}

	for _, svc := range svcs.Items {
		_, err := istioClient.NetworkingV1alpha3().VirtualServices(ns).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to query VirtualService `%s` in ns `%s`: %s", svc.Name, ns, err)
		}
	}

	return true, nil
}

func ensureEnvoyFilter(ctx context.Context, istioClient versionedclient.Interface, ns, name string) error {
	envoyFilterObj, err := istioClient.NetworkingV1alpha3().EnvoyFilters(ns).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		log.Infof("Has found EnvoyFilter `%s` in ns `%s` and don't recreate.", name, istioNamespace)
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to query EnvoyFilter `%s` in ns `%s`: %s", name, istioNamespace, err)
	}

	envoyFilterObj.Name = name
	envoyFilterObj.Spec = networkingv1alpha3.EnvoyFilter{
		ConfigPatches: []*networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{},
	}

	_, err = istioClient.NetworkingV1alpha3().EnvoyFilters(ns).Create(ctx, envoyFilterObj, metav1.CreateOptions{})
	return err
}
