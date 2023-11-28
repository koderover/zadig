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
	"context"
	"fmt"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/log"
	zadigtypes "github.com/koderover/zadig/pkg/types"
	zadigutil "github.com/koderover/zadig/pkg/util"
	"github.com/koderover/zadig/pkg/util/boolptr"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func EnableIstioGrayscale(ctx context.Context, envName, productName string) error {
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

	// 3. Ensure `EnvoyFilter` in istio namespace.
	err = ensureEnvoyFilter(ctx, istioClient, clusterID, istioNamespace, zadigEnvoyFilter)
	if err != nil {
		return fmt.Errorf("failed to ensure EnvoyFilter in namespace `%s`: %s", istioNamespace, err)
	}

	// 4. Update the environment configuration.
	return ensureIstioGrayConfig(ctx, prod)
}

// @todo disable istio grayscale
func DisableIstioGrayscale(ctx context.Context, envName, productName string) error {
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

func CheckIstioGrayscaleReady(ctx context.Context, envName, op, productName string) (*IstioGrayscaleReady, error) {
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

	// 3. Check whether all Pods have istio-proxy and are ready.
	allHaveIstioProxy, allPodsReady, err := checkPodsWithIstioProxyAndReady(ctx, kclient, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to check whether all pods in ns `%s` have istio-proxy and are ready: %s", ns, err)
	}

	res := &IstioGrayscaleReady{
		Checks: IstioGrayscaleChecks{
			NamespaceHasIstioLabel:  isNamespaceHasIstioLabel,
			WorkloadsHaveK8sService: isWorkloadsHaveNoK8sService,
			PodsHaveIstioProxy:      allHaveIstioProxy,
			WorkloadsReady:          allPodsReady,
		},
	}
	res.CheckAndSetReady(shareEnvOp)

	return res, nil
}

// Used when create a gray environment
func EnsureFullPathGrayScaleConfig(ctx context.Context, env *commonmodels.Product, kclient client.Client, istioClient versionedclient.Interface) error {
	opt := &commonrepo.ProductFindOptions{Name: env.ProductName, EnvName: env.IstioGrayscale.BaseEnv, Production: boolptr.True()}
	baseEnv, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return fmt.Errorf("failed to find base env %s of product %s: %s", env.IstioGrayscale.BaseEnv, env.ProductName, err)
	}

	baseNS := baseEnv.Namespace

	// Deploy K8s Services and VirtualServices of all workloads in the base environment to the gray environment.
	err = ensureDefaultK8sServiceAndVirtualServicesInGray(ctx, env, baseNS, kclient, istioClient)
	if err != nil {
		return fmt.Errorf("failed to ensure K8s Services and VirtualServices: %s", err)
	}

	return nil
}

func GetIstioGrayscaleConfig(ctx context.Context, envName, productName string) (commonmodels.IstioGrayscale, error) {
	resp := commonmodels.IstioGrayscale{}
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: boolptr.True()}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return resp, fmt.Errorf("failed to query env `%s` in project `%s`: %s", envName, productName, err)
	}
	if !prod.IstioGrayscale.IsBase {
		return resp, fmt.Errorf("cannot get istio grayscale config from gray environment %s/%s", productName, envName)
	}

	return prod.IstioGrayscale, nil
}

// Used when add service to env
func EnsureUpdateGrayscaleService(ctx context.Context, env *commonmodels.Product, svcName string, kclient client.Client, istioClient versionedclient.Interface) error {
	if !env.IstioGrayscale.Enable {
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

	return ensureUpdateGrayscaleSerivce(ctx, env, svc, kclient, istioClient)
}

type SetIstioGrayscaleConfigRequest struct {
	GrayscaleStrategy  commonmodels.GrayscaleStrategyType    `bson:"grayscale_strategy" json:"grayscale_strategy"`
	WeightConfigs      []commonmodels.IstioWeightConfig      `bson:"weight_configs" json:"weight_configs"`
	HeaderMatchConfigs []commonmodels.IstioHeaderMatchConfig `bson:"header_match_configs" json:"header_match_configs"`
}

func SetIstioGrayscaleConfig(ctx context.Context, envName, productName string, req SetIstioGrayscaleConfigRequest) error {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: boolptr.True()}
	baseEnv, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return fmt.Errorf("failed to query env `%s` in project `%s`: %s", envName, productName, err)
	}
	if !baseEnv.IstioGrayscale.IsBase {
		return fmt.Errorf("cannot set istio grayscale config for gray environment")
	}
	if baseEnv.IsSleeping() {
		return fmt.Errorf("Environment %s is sleeping", baseEnv.EnvName)
	}

	grayEnvs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:                  baseEnv.ProductName,
		IstioGrayscaleEnable:  zadigutil.GetBoolPointer(true),
		IstioGrayscaleIsBase:  zadigutil.GetBoolPointer(false),
		IstioGrayscaleBaseEnv: zadigutil.GetStrPointer(baseEnv.EnvName),
		Production:            zadigutil.GetBoolPointer(true),
	})
	if err != nil {
		return fmt.Errorf("failed to list gray environments of %s/%s: %s", baseEnv.ProductName, baseEnv.EnvName, err)
	}

	envMap := map[string]*commonmodels.Product{
		baseEnv.EnvName: baseEnv,
	}
	for _, env := range grayEnvs {
		if env.IsSleeping() {
			return fmt.Errorf("Environment %s is sleeping", baseEnv.EnvName)
		}
		envMap[env.EnvName] = env
	}

	if req.GrayscaleStrategy == commonmodels.GrayscaleStrategyWeight {
		err = setIstioGrayscaleWeight(context.TODO(), envMap, req.WeightConfigs)
		if err != nil {
			return fmt.Errorf("failed to set istio grayscale weight, err: %w", err)
		}
	} else if req.GrayscaleStrategy == commonmodels.GrayscaleStrategyHeaderMatch {
		err = setIstioGrayscaleHeaderMatch(context.TODO(), envMap, req.HeaderMatchConfigs)
		if err != nil {
			return fmt.Errorf("failed to set istio grayscale weight, err: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported grayscale strategy type: %s", req.GrayscaleStrategy)
	}

	baseEnv.IstioGrayscale.GrayscaleStrategy = req.GrayscaleStrategy
	baseEnv.IstioGrayscale.WeightConfigs = req.WeightConfigs
	baseEnv.IstioGrayscale.HeaderMatchConfigs = req.HeaderMatchConfigs

	err = commonrepo.NewProductColl().UpdateIstioGrayscale(baseEnv.EnvName, baseEnv.ProductName, baseEnv.IstioGrayscale)
	if err != nil {
		return fmt.Errorf("failed to update istio grayscale config of %s/%s environment: %s", baseEnv.ProductName, baseEnv.EnvName, err)
	}

	return nil
}

func setIstioGrayscaleWeight(ctx context.Context, envMap map[string]*commonmodels.Product, weightConfigs []commonmodels.IstioWeightConfig) error {
	for _, env := range envMap {
		ns := env.Namespace
		clusterID := env.ClusterID

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

		svcs := &corev1.ServiceList{}
		err = kclient.List(ctx, svcs, client.InNamespace(ns))
		if err != nil {
			return err
		}

		for _, svc := range svcs.Items {
			// If there is no workloads in this environment, then service is not updated.
			hasWorkload, err := doesSvcHasWorkload(ctx, ns, labels.SelectorFromSet(labels.Set(svc.Spec.Selector)), kclient)
			if err != nil {
				return err
			}

			if !hasWorkload {
				continue
			}

			isExisted := false
			vsName := genVirtualServiceName(&svc)
			svcName := svc.Name

			vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(ns).Get(ctx, vsName, metav1.GetOptions{})
			if err == nil {
				isExisted = true
			}
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}

			vsObj, err = generateGrayscaleWeightVirtualService(ctx, envMap, svcName, vsName, ns, weightConfigs, vsObj)
			if err != nil {
				return fmt.Errorf("failed to generate VirtualService `%s` in ns `%s` for service %s: %s", vsName, env.Namespace, svcName, err)
			}

			if isExisted {
				_, err = istioClient.NetworkingV1alpha3().VirtualServices(ns).Update(ctx, vsObj, metav1.UpdateOptions{})
			} else {
				_, err = istioClient.NetworkingV1alpha3().VirtualServices(ns).Create(ctx, vsObj, metav1.CreateOptions{})
			}
			if err != nil {
				return fmt.Errorf("failed to create or update virtual service %s in ns %s: %s", vsName, ns, err)
			}
		}
	}

	return nil
}

func setIstioGrayscaleHeaderMatch(ctx context.Context, envMap map[string]*commonmodels.Product, headerMatchConfigs []commonmodels.IstioHeaderMatchConfig) error {
	for _, env := range envMap {
		ns := env.Namespace
		clusterID := env.ClusterID
		baseEnvName := env.IstioGrayscale.BaseEnv
		if baseEnvName == "" {
			baseEnvName = env.EnvName
		}
		baseNs := envMap[baseEnvName].Namespace

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

		svcs := &corev1.ServiceList{}
		err = kclient.List(ctx, svcs, client.InNamespace(ns))
		if err != nil {
			return err
		}

		for _, svc := range svcs.Items {
			// If there is no workloads in this environment, then service is not updated.
			hasWorkload, err := doesSvcHasWorkload(ctx, ns, labels.SelectorFromSet(labels.Set(svc.Spec.Selector)), kclient)
			if err != nil {
				return err
			}

			if !hasWorkload {
				continue
			}

			isExisted := false
			svcName := svc.Name
			vsName := genVirtualServiceName(&svc)

			vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(ns).Get(ctx, vsName, metav1.GetOptions{})
			if err == nil {
				isExisted = true
			}
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}

			vsObj, err = generateGrayscaleHeaderMatchVirtualService(ctx, envMap, svcName, vsName, ns, baseNs, headerMatchConfigs, vsObj)
			if err != nil {
				return fmt.Errorf("failed to generate VirtualService `%s` in ns `%s` for service %s: %s", vsName, env.Namespace, svcName, err)
			}

			if isExisted {
				_, err = istioClient.NetworkingV1alpha3().VirtualServices(ns).Update(ctx, vsObj, metav1.UpdateOptions{})
			} else {
				_, err = istioClient.NetworkingV1alpha3().VirtualServices(ns).Create(ctx, vsObj, metav1.CreateOptions{})
			}
			if err != nil {
				return fmt.Errorf("failed to create or update virtual service %s in ns %s: %s", vsName, ns, err)
			}
		}
	}

	return nil
}

func generateGrayscaleWeightVirtualService(ctx context.Context, envMap map[string]*commonmodels.Product, svcName, vsName, ns string, weightConfigs []commonmodels.IstioWeightConfig, vsObj *v1alpha3.VirtualService) (*v1alpha3.VirtualService, error) {
	vsObj.Name = vsName
	vsObj.Namespace = ns

	if vsObj.Labels == nil {
		vsObj.Labels = map[string]string{}
	}
	vsObj.Labels[zadigtypes.ZadigLabelKeyGlobalOwner] = zadigtypes.Zadig

	httpRouteDestinations := []*networkingv1alpha3.HTTPRouteDestination{}
	for _, weightConfig := range weightConfigs {
		if envMap[weightConfig.Env] == nil {
			return nil, fmt.Errorf("env %s is not found", weightConfig.Env)
		}

		httpRouteDestination := &networkingv1alpha3.HTTPRouteDestination{
			Destination: &networkingv1alpha3.Destination{
				Host: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, envMap[weightConfig.Env].Namespace),
			},
			Weight: weightConfig.Weight,
		}
		httpRouteDestinations = append(httpRouteDestinations, httpRouteDestination)
	}

	vsObj.Spec = networkingv1alpha3.VirtualService{
		Hosts: []string{svcName},
		Http: []*networkingv1alpha3.HTTPRoute{
			{
				Route: httpRouteDestinations,
			},
		},
	}

	return vsObj, nil
}

func generateGrayscaleHeaderMatchVirtualService(ctx context.Context, envMap map[string]*commonmodels.Product, svcName, vsName, ns, baseNs string, headerMatchConfigs []commonmodels.IstioHeaderMatchConfig, vsObj *v1alpha3.VirtualService) (*v1alpha3.VirtualService, error) {
	vsObj.Name = vsName
	vsObj.Namespace = ns

	if vsObj.Labels == nil {
		vsObj.Labels = map[string]string{}
	}
	vsObj.Labels[zadigtypes.ZadigLabelKeyGlobalOwner] = zadigtypes.Zadig

	configedEnvSet := sets.NewString()
	httpRoutes := []*networkingv1alpha3.HTTPRoute{}
	for _, headerMatchConfig := range headerMatchConfigs {
		if envMap[headerMatchConfig.Env] == nil {
			return nil, fmt.Errorf("env %s is not found", headerMatchConfig.Env)
		}
		configedEnvSet.Insert(headerMatchConfig.Env)

		header := map[string]*networkingv1alpha3.StringMatch{}
		for _, headerMatch := range headerMatchConfig.HeaderMatchs {
			if headerMatch.Match == commonmodels.StringMatchPrefix {
				header[headerMatch.Key] = &networkingv1alpha3.StringMatch{
					MatchType: &networkingv1alpha3.StringMatch_Prefix{
						Prefix: headerMatch.Value,
					},
				}
			} else if headerMatch.Match == commonmodels.StringMatchExact {
				header[headerMatch.Key] = &networkingv1alpha3.StringMatch{
					MatchType: &networkingv1alpha3.StringMatch_Exact{
						Exact: headerMatch.Value,
					},
				}
			} else if headerMatch.Match == commonmodels.StringMatchRegex {
				header[headerMatch.Key] = &networkingv1alpha3.StringMatch{
					MatchType: &networkingv1alpha3.StringMatch_Regex{
						Regex: headerMatch.Value,
					},
				}
			} else {
				return nil, fmt.Errorf("unsupported header match type: %s", headerMatch.Match)
			}
		}
		httpRoutes = append(httpRoutes, &networkingv1alpha3.HTTPRoute{
			Match: []*networkingv1alpha3.HTTPMatchRequest{
				{
					Headers: header,
				},
			},
			Route: []*networkingv1alpha3.HTTPRouteDestination{
				{
					Destination: &networkingv1alpha3.Destination{
						Host: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, envMap[headerMatchConfig.Env].Namespace),
					},
				},
			},
		})
	}

	for _, env := range envMap {
		if !configedEnvSet.Has(env.EnvName) && env.IstioGrayscale.Enable && env.IstioGrayscale.IsBase {
			httpRoutes = append(httpRoutes, &networkingv1alpha3.HTTPRoute{
				Route: []*networkingv1alpha3.HTTPRouteDestination{
					{
						Destination: &networkingv1alpha3.Destination{
							Host: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, baseNs),
						},
					},
				},
			})
			break
		}
	}

	vsObj.Spec = networkingv1alpha3.VirtualService{
		Hosts: []string{svcName},
		Http:  httpRoutes,
	}

	return vsObj, nil
}

func ensureUpdateGrayscaleSerivce(ctx context.Context, curEnv *commonmodels.Product, svc *corev1.Service, kclient client.Client, istioClient versionedclient.Interface) error {
	vsName := genVirtualServiceName(svc)

	grayEnvs, err := fetchGrayEnvs(ctx, curEnv.ProductName, curEnv.ClusterID, curEnv.EnvName)
	if err != nil {
		return fmt.Errorf("failed to fetch gray environments of %s/%s, err: %s", curEnv.ProductName, curEnv.EnvName, err)
	}

	envMap := map[string]*commonmodels.Product{}
	for _, grayEnv := range grayEnvs {
		envMap[grayEnv.EnvName] = grayEnv
	}
	envMap[curEnv.EnvName] = curEnv

	if curEnv.IstioGrayscale.IsBase {
		// 1. Create VirtualService in all of the base environments.
		err = ensureGrayscaleVirtualService(ctx, kclient, istioClient, curEnv, envMap, svc, vsName)
		if err != nil {
			return fmt.Errorf("failed to ensure VirtualService %s in env `%s` for svc %s, err: %w", vsName, curEnv.EnvName, svc, err)
		}

		// 2. Create Default Service in all of the gray environments.
		ensureServicesInAllGrayEnvs(ctx, curEnv, grayEnvs, svc, kclient, istioClient)
		if err != nil {
			return fmt.Errorf("failed to ensure service %s in all gray envs, err: %w", svc.Name, err)
		}
		return nil
	} else {
		baseEnv, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:    curEnv.ProductName,
			EnvName: curEnv.IstioGrayscale.BaseEnv,
		})
		if err != nil {
			return fmt.Errorf("failed to find base env %s of product %s, err: %w", curEnv.IstioGrayscale.BaseEnv, curEnv.ProductName, err)
		}
		envMap[baseEnv.EnvName] = baseEnv

		// 1. Create VirtualService in the gray environment.
		err = ensureGrayscaleVirtualService(ctx, kclient, istioClient, curEnv, envMap, svc, vsName)
		if err != nil {
			return fmt.Errorf("failed to ensure VirtualService %s in env `%s` for svc %s, err: %w", vsName, curEnv.EnvName, svc, err)
		}
		// 2. Updated the VirtualService configuration in the base environment.
		err = ensureGrayscaleVirtualService(ctx, kclient, istioClient, baseEnv, envMap, svc, vsName)
		if err != nil {
			return fmt.Errorf("failed to ensure VirtualService %s in env `%s` for svc %s, err: %w", vsName, curEnv.EnvName, svc, err)
		}

		return nil
		// return ensureUpdateVirtualServiceInBase(ctx, curEnv.EnvName, vsName, svc.Name, curEnv.Namespace, baseEnv.Namespace, istioClient)
	}
}

func ensureDeleteGrayscaleService(ctx context.Context, env *commonmodels.Product, svc *corev1.Service, kclient client.Client, istioClient versionedclient.Interface) error {
	vsName := genVirtualServiceName(svc)

	// Delete VirtualService in the current environment.
	err := ensureDeleteVirtualService(ctx, env, vsName, istioClient)
	if err != nil {
		return err
	}

	if env.IstioGrayscale.IsBase {
		// Delete VirtualService and K8s Service in all of the sub environments if there're no specific workloads.
		return ensureDeleteServiceInAllSubEnvs(ctx, env, svc, kclient, istioClient)
	} else {
		baseEnv, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:    env.ProductName,
			EnvName: env.ShareEnv.BaseEnv,
		})
		if err != nil {
			return err
		}

		// Update VirtualService Routes in the base environment.
		return ensureCleanRouteInBase(ctx, env.EnvName, baseEnv.Namespace, vsName, istioClient)
	}
}

func ensureGrayscaleVirtualService(ctx context.Context, kclient client.Client, istioClient versionedclient.Interface, curEnv *commonmodels.Product, envMap map[string]*commonmodels.Product, svc *corev1.Service, vsName string) error {
	isExisted := false
	vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(curEnv.Namespace).Get(ctx, vsName, metav1.GetOptions{})
	if err == nil {
		isExisted = true
		// log.Infof("Has found VirtualService `%s` in ns `%s` and don't recreate.", vsName, curEnv.Namespace)
		// return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to query VirtualService `%s` in ns `%s`: %s", vsName, curEnv.Namespace, err)
	}

	if curEnv.IstioGrayscale.GrayscaleStrategy == commonmodels.GrayscaleStrategyWeight {
		vsObj, err = generateGrayscaleWeightVirtualService(ctx, envMap, svc.Name, vsName, curEnv.Namespace, curEnv.IstioGrayscale.WeightConfigs, vsObj)
	} else if curEnv.IstioGrayscale.GrayscaleStrategy == commonmodels.GrayscaleStrategyHeaderMatch {
		baseNs := curEnv.IstioGrayscale.BaseEnv
		vsObj, err = generateGrayscaleHeaderMatchVirtualService(ctx, envMap, svc.Name, vsName, curEnv.Namespace, baseNs, curEnv.IstioGrayscale.HeaderMatchConfigs, vsObj)
	} else {
		return fmt.Errorf("unsupported grayscale strategy type: %s", curEnv.IstioGrayscale.GrayscaleStrategy)
	}
	if err != nil {
		return fmt.Errorf("failed to generate VirtualService `%s` in ns `%s` for service %s: %s", vsName, curEnv.Namespace, svc.Name, err)
	}

	if isExisted {
		_, err = istioClient.NetworkingV1alpha3().VirtualServices(curEnv.Namespace).Update(ctx, vsObj, metav1.UpdateOptions{})
	} else {
		_, err = istioClient.NetworkingV1alpha3().VirtualServices(curEnv.Namespace).Create(ctx, vsObj, metav1.CreateOptions{})
	}
	return err
}

func ensureServicesInAllGrayEnvs(ctx context.Context, env *commonmodels.Product, grayEnvs []*commonmodels.Product, svc *corev1.Service, kclient client.Client, istioClient versionedclient.Interface) error {
	for _, env := range grayEnvs {
		log.Infof("Begin to ensure Services in gray env %s of prouduct %s.", env.EnvName, env.ProductName)

		vsName := genVirtualServiceName(svc)
		err := ensureGrayscaleDefaultVirtualService(ctx, kclient, istioClient, env, svc, vsName)
		if err != nil {
			return fmt.Errorf("failed to ensure grayscale default VirtualService %s in env %s for svc %s, err: %w",
				vsName, env.EnvName, svc.Name, err)
		}

		err = ensureDefaultK8sServiceInGray(ctx, svc, env.Namespace, kclient)
		if err != nil {
			return fmt.Errorf("failed to ensure grayscale default service %s in env %s, err: %w",
				svc.Name, env.EnvName, err)
		}
	}

	return nil
}

func ensureGrayscaleDefaultVirtualService(ctx context.Context, kclient client.Client, istioClient versionedclient.Interface, env *commonmodels.Product, svc *corev1.Service, vsName string) error {
	vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(env.Namespace).Get(ctx, vsName, metav1.GetOptions{})
	if err == nil {
		log.Infof("Has found VirtualService `%s` in ns `%s` and don't recreate.", vsName, env.Namespace)
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to query VirtualService `%s` in ns `%s`: %s", vsName, env.Namespace, err)
	}

	vsObj.Name = vsName
	if vsObj.Labels == nil {
		vsObj.Labels = map[string]string{}
	}
	vsObj.Labels[zadigtypes.ZadigLabelKeyGlobalOwner] = zadigtypes.Zadig

	routes := []*networkingv1alpha3.HTTPRoute{}
	routes = append(routes, &networkingv1alpha3.HTTPRoute{
		Route: []*networkingv1alpha3.HTTPRouteDestination{
			{
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
