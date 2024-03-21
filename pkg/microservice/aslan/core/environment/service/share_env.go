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
	"fmt"
	"strings"
	"time"

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
	err = kube.EnsureEnvoyFilter(ctx, istioClient, clusterID, setting.IstioNamespace, setting.ZadigEnvoyFilter, nil)
	if err != nil {
		return fmt.Errorf("failed to ensure EnvoyFilter in namespace `%s`: %s", setting.IstioNamespace, err)
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
			if container.Name == setting.IstioProxyName {
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
		vsName := kube.GenVirtualServiceName(&svc)
		err := kube.EnsureVirtualService(ctx, kclient, istioClient, env, &svc, vsName)
		if err != nil {
			return fmt.Errorf("failed to ensure VirtualService `%s` in ns `%s`: %s", vsName, env.Namespace, err)
		}
	}

	return nil
}

func checkVirtualServicesDeployed(ctx context.Context, kclient client.Client, istioClient versionedclient.Interface, ns string) (bool, error) {
	svcs := &corev1.ServiceList{}
	err := kclient.List(ctx, svcs, client.InNamespace(ns))
	if err != nil {
		return false, fmt.Errorf("failed to list svcs in ns `%s`: %s", ns, err)
	}

	for _, svc := range svcs.Items {
		vsName := kube.GenVirtualServiceName(&svc)
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
			if container.Name == setting.IstioProxyName {
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

	return kube.EnsureUpdateZadigSerivce(ctx, env, svc, kclient, istioClient)
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
		return kube.EnsureDeleteZadigService(ctx, env, svc, kclient, istioClient)
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

	envs, err := kube.FetchSubEnvs(ctx, productName, env.ClusterID, envName)
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
		return "", e.ErrGetPortalService.AddDesc("istio default gateway's lb doesn't have ip address or hostname")
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
	isExisted := false
	vsName := kube.GenVirtualServiceName(svc)
	vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(ns).Get(ctx, vsName, metav1.GetOptions{})
	if err == nil {
		isExisted = true
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	vsObj.Name = vsName
	vsObj.Namespace = ns

	if vsObj.Labels == nil {
		vsObj.Labels = map[string]string{}
	}
	vsObj.Labels[zadigtypes.ZadigLabelKeyGlobalOwner] = zadigtypes.Zadig

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

	if isExisted {
		_, err = istioClient.NetworkingV1alpha3().VirtualServices(ns).Update(ctx, vsObj, metav1.UpdateOptions{})
	} else {
		vsObj.Spec.Http = append(vsObj.Spec.Http, &networkingv1alpha3.HTTPRoute{
			Route: []*networkingv1alpha3.HTTPRouteDestination{
				{
					Destination: &networkingv1alpha3.Destination{
						Host: fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, ns),
					},
				},
			},
		})
		_, err = istioClient.NetworkingV1alpha3().VirtualServices(ns).Create(ctx, vsObj, metav1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf("failed to create or update virtual service %s in ns %s: %s", vsName, ns, err)
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
