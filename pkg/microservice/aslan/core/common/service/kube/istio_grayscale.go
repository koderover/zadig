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

package kube

import (
	"context"
	"fmt"
	"time"

	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	zadigtypes "github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util/boolptr"
)

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

func SetIstioGrayscaleWeight(ctx context.Context, envMap map[string]*commonmodels.Product, weightConfigs []commonmodels.IstioWeightConfig) error {
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

			vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(ns).Get(ctx, vsName, metav1.GetOptions{})
			if err == nil {
				isExisted = true
			}
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}

			vsObj, err = generateGrayscaleWeightVirtualService(ctx, envMap, vsName, ns, weightConfigs, false, &svc, kclient, vsObj)
			if err != nil {
				return fmt.Errorf("failed to generate VirtualService `%s` in ns `%s` for service %s: %s", vsName, env.Namespace, svc.Name, err)
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

func SetIstioGrayscaleHeaderMatch(ctx context.Context, envMap map[string]*commonmodels.Product, headerMatchConfigs []commonmodels.IstioHeaderMatchConfig) error {
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

			vsObj, err = generateGrayscaleHeaderMatchVirtualService(ctx, envMap, vsName, ns, baseNs, headerMatchConfigs, false, &svc, kclient, vsObj)
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

func generateGrayscaleWeightVirtualService(ctx context.Context, envMap map[string]*commonmodels.Product, vsName, ns string, weightConfigs []commonmodels.IstioWeightConfig, skipWorkloadCheck bool, svc *corev1.Service, kclient client.Client, vsObj *v1alpha3.VirtualService) (*v1alpha3.VirtualService, error) {
	matchKey := "x-env"
	svcName := svc.Name
	vsObj.Name = vsName
	vsObj.Namespace = ns

	baseEnv := ""

	if vsObj.Labels == nil {
		vsObj.Labels = map[string]string{}
	}
	vsObj.Labels[zadigtypes.ZadigLabelKeyGlobalOwner] = zadigtypes.Zadig

	httpRoutes := []*networkingv1alpha3.HTTPRoute{}
	for _, weightConfig := range weightConfigs {
		if envMap[weightConfig.Env] == nil {
			return nil, fmt.Errorf("env %s is not found", weightConfig.Env)
		}

		if baseEnv == "" {
			if envMap[weightConfig.Env].IstioGrayscale.IsBase {
				baseEnv = envMap[weightConfig.Env].EnvName
			} else {
				baseEnv = envMap[weightConfig.Env].IstioGrayscale.BaseEnv
			}
		}

		if !skipWorkloadCheck {
			// If there is no workloads in this environment, then service is not updated.
			hasWorkload, err := doesSvcHasWorkload(ctx, envMap[weightConfig.Env].Namespace, labels.SelectorFromSet(labels.Set(svc.Spec.Selector)), kclient)
			if err != nil {
				return nil, fmt.Errorf("failed to check if service %s has workload in env %s, err: %s", svcName, envMap[weightConfig.Env].EnvName, err)
			}

			if !hasWorkload {
				continue
			}
		}

		httpRoute := &networkingv1alpha3.HTTPRoute{
			Match: []*networkingv1alpha3.HTTPMatchRequest{
				{
					Headers: map[string]*networkingv1alpha3.StringMatch{
						matchKey: {
							MatchType: &networkingv1alpha3.StringMatch_Exact{
								Exact: weightConfig.Env,
							},
						},
					},
				},
			},
			Route: []*networkingv1alpha3.HTTPRouteDestination{
				{
					Destination: &networkingv1alpha3.Destination{
						Host: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, envMap[weightConfig.Env].Namespace),
					},
				},
			},
		}
		httpRoutes = append(httpRoutes, httpRoute)
	}

	weightSum := 0
	routes := []*networkingv1alpha3.HTTPRouteDestination{}
	for _, weightConfig := range weightConfigs {
		if envMap[weightConfig.Env] == nil {
			return nil, fmt.Errorf("env %s is not found", weightConfig.Env)
		}

		if !skipWorkloadCheck {
			// If there is no workloads in this environment, then service is not updated.
			hasWorkload, err := doesSvcHasWorkload(ctx, envMap[weightConfig.Env].Namespace, labels.SelectorFromSet(labels.Set(svc.Spec.Selector)), kclient)
			if err != nil {
				return nil, fmt.Errorf("failed to check if service %s has workload in env %s, err: %s", svcName, envMap[weightConfig.Env].EnvName, err)
			}

			if !hasWorkload {
				continue
			}
		}

		weightSum += int(weightConfig.Weight)
		route := &networkingv1alpha3.HTTPRouteDestination{
			Destination: &networkingv1alpha3.Destination{
				Host: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, envMap[weightConfig.Env].Namespace),
			},
			Weight: weightConfig.Weight,
			Headers: &networkingv1alpha3.Headers{
				Request: &networkingv1alpha3.Headers_HeaderOperations{
					Set: map[string]string{
						matchKey: weightConfig.Env,
					},
				},
			},
		}
		routes = append(routes, route)
	}
	if weightSum != 100 {
		return nil, fmt.Errorf("the sum of weight is not 100 for the service %s, the full-path grayscale can't work correctly", svcName)

		// @note code below is used when base env + grayscale envs > 2
		// log.Warnf("The sum of weight is not 100 for the service %s, the full-path grayscale may not work correctly", svcName)
		// if baseEnv == "" {
		// 	return nil, fmt.Errorf("base env is not found")
		// }

		// route := &networkingv1alpha3.HTTPRouteDestination{
		// 	Destination: &networkingv1alpha3.Destination{
		// 		Host: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, envMap[baseEnv].Namespace),
		// 	},
		// }
		// routes = append([]*networkingv1alpha3.HTTPRouteDestination{}, route)
	}
	httpRoutes = append(httpRoutes, &networkingv1alpha3.HTTPRoute{
		Route: routes,
	})

	hosts := sets.NewString(vsObj.Spec.Hosts...)
	hosts.Insert(svcName)
	vsObj.Spec = networkingv1alpha3.VirtualService{
		Gateways: vsObj.Spec.Gateways,
		Hosts:    hosts.List(),
		Http:     httpRoutes,
	}

	return vsObj, nil
}

func generateGrayscaleHeaderMatchVirtualService(ctx context.Context, envMap map[string]*commonmodels.Product, vsName, ns, baseNs string, headerMatchConfigs []commonmodels.IstioHeaderMatchConfig, skipWorkloadCheck bool, svc *corev1.Service, kclient client.Client, vsObj *v1alpha3.VirtualService) (*v1alpha3.VirtualService, error) {
	svcName := svc.Name
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

		if !skipWorkloadCheck {
			// If there is no workloads in this environment, then service is not updated.
			hasWorkload, err := doesSvcHasWorkload(ctx, envMap[headerMatchConfig.Env].Namespace, labels.SelectorFromSet(labels.Set(svc.Spec.Selector)), kclient)
			if err != nil {
				return nil, fmt.Errorf("failed to check if service %s has workload in env %s, err: %s", svcName, envMap[headerMatchConfig.Env].EnvName, err)
			}

			if !hasWorkload {
				continue
			}
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

	hosts := sets.NewString(vsObj.Spec.Hosts...)
	hosts.Insert(svcName)
	vsObj.Spec = networkingv1alpha3.VirtualService{
		Gateways: vsObj.Spec.Gateways,
		Hosts:    hosts.List(),
		Http:     httpRoutes,
	}

	return vsObj, nil
}

func ensureUpdateGrayscaleSerivce(ctx context.Context, curEnv *commonmodels.Product, svc *corev1.Service, kclient client.Client, istioClient versionedclient.Interface) error {
	vsName := genVirtualServiceName(svc)

	if curEnv.IstioGrayscale.IsBase {
		grayEnvs, err := commonutil.FetchGrayEnvs(ctx, curEnv.ProductName, curEnv.ClusterID, curEnv.EnvName)
		if err != nil {
			return fmt.Errorf("failed to fetch gray environments of %s/%s, err: %s", curEnv.ProductName, curEnv.EnvName, err)
		}

		envMap := map[string]*commonmodels.Product{}
		for _, grayEnv := range grayEnvs {
			envMap[grayEnv.EnvName] = grayEnv
		}
		envMap[curEnv.EnvName] = curEnv

		// 1. Create VirtualService in all of the base environments.
		err = ensureGrayscaleVirtualService(ctx, kclient, istioClient, curEnv, envMap, curEnv.IstioGrayscale, svc, vsName)
		if err != nil {
			return fmt.Errorf("failed to ensure VirtualService %s in env `%s` for svc %s, err: %w", vsName, curEnv.EnvName, svc.Name, err)
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
		grayEnvs, err := commonutil.FetchGrayEnvs(ctx, baseEnv.ProductName, baseEnv.ClusterID, baseEnv.EnvName)
		if err != nil {
			return fmt.Errorf("failed to fetch gray environments of %s/%s, err: %s", curEnv.ProductName, curEnv.EnvName, err)
		}
		envMap := map[string]*commonmodels.Product{}
		for _, grayEnv := range grayEnvs {
			envMap[grayEnv.EnvName] = grayEnv
		}
		envMap[curEnv.EnvName] = curEnv
		envMap[baseEnv.EnvName] = baseEnv

		// 1. Create VirtualService in the gray environment.
		err = ensureGrayscaleVirtualService(ctx, kclient, istioClient, curEnv, envMap, baseEnv.IstioGrayscale, svc, vsName)
		if err != nil {
			return fmt.Errorf("failed to ensure VirtualService %s in gray env `%s` for svc %s, err: %w", vsName, curEnv.EnvName, svc.Name, err)
		}
		// 2. Updated the VirtualService configuration in the base environment.
		err = ensureGrayscaleVirtualService(ctx, kclient, istioClient, baseEnv, envMap, baseEnv.IstioGrayscale, svc, vsName)
		if err != nil {
			return fmt.Errorf("failed to ensure VirtualService %s in base env `%s` for svc %s, err: %w", vsName, curEnv.EnvName, svc.Name, err)
		}

		return nil
	}
}

func EnsureDeleteGrayscaleService(ctx context.Context, env *commonmodels.Product, svc *corev1.Service, kclient client.Client, istioClient versionedclient.Interface) error {
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
			EnvName: env.IstioGrayscale.BaseEnv,
		})
		if err != nil {
			return err
		}

		// Update VirtualService Routes in the base environment.
		return ensureCleanGrayscaleRouteInBase(ctx, env.EnvName, env.Namespace, baseEnv.Namespace, svc.Name, vsName, istioClient)
	}
}

func ensureGrayscaleVirtualService(ctx context.Context, kclient client.Client, istioClient versionedclient.Interface, curEnv *commonmodels.Product, envMap map[string]*commonmodels.Product, istioGrayscaleConfig commonmodels.IstioGrayscale, svc *corev1.Service, vsName string) error {
	isExisted := false
	vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(curEnv.Namespace).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to query VirtualService `%s` in ns `%s`: %s", vsName, curEnv.Namespace, err)
	}
	if err == nil {
		isExisted = true
	}

	if istioGrayscaleConfig.GrayscaleStrategy == commonmodels.GrayscaleStrategyWeight {
		vsObj, err = generateGrayscaleWeightVirtualService(ctx, envMap, vsName, curEnv.Namespace, istioGrayscaleConfig.WeightConfigs, true, svc, kclient, vsObj)
	} else if istioGrayscaleConfig.GrayscaleStrategy == commonmodels.GrayscaleStrategyHeaderMatch {
		baseEnvName := curEnv.IstioGrayscale.BaseEnv
		if baseEnvName == "" {
			baseEnvName = curEnv.EnvName
		}
		baseNs := envMap[baseEnvName].Namespace
		vsObj, err = generateGrayscaleHeaderMatchVirtualService(ctx, envMap, vsName, curEnv.Namespace, baseNs, istioGrayscaleConfig.HeaderMatchConfigs, true, svc, kclient, vsObj)
	} else {
		return ensureGrayscaleDefaultVirtualService(ctx, kclient, istioClient, curEnv, svc, vsName)
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

func ensureCleanGrayscaleRouteInBase(ctx context.Context, envName, ns, baseNS, svcName, vsName string, istioClient versionedclient.Interface) error {
	vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(baseNS).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil {
		log.Warnf("Failed to query VirtualService %s releated to env %s in namesapce %s: %s. It may be not an exception and skip.", vsName, envName, baseNS, err)
		return nil
	}

	// Note: DeepCopy is used to avoid unpredictable results in concurrent operations.
	baseVS := vsObj.DeepCopy()
	host := fmt.Sprintf("%s.%s.svc.cluster.local", svcName, ns)

	if len(baseVS.Spec.Http) == 0 {
		return nil
	}

	newHttpRoutes := []*networkingv1alpha3.HTTPRoute{}
	for _, httpRoute := range baseVS.Spec.Http {
		if len(httpRoute.Match) != 0 {
			needToClean := false
			for _, route := range httpRoute.Route {
				if route.Destination.Host == host {
					needToClean = true
					break
				}
			}
			if needToClean {
				continue
			}
			newHttpRoutes = append(newHttpRoutes, httpRoute)
		} else {
			weightSum := 0
			for _, route := range httpRoute.Route {
				if route.Destination.Host == host {
					continue
				}
				weightSum += int(route.Weight)
			}

			if weightSum != 100 {
				baseHost := fmt.Sprintf("%s.%s.svc.cluster.local", svcName, baseNS)
				httpRoute.Route = []*networkingv1alpha3.HTTPRouteDestination{
					{
						Destination: &networkingv1alpha3.Destination{
							Host: baseHost,
						},
					},
				}
			}
			newHttpRoutes = append(newHttpRoutes, httpRoute)
		}
	}

	log.Infof("Begin to clean route svc %s in base ns %s for VirtualService %s.", svcName, baseNS, vsName)

	baseVS.Spec.Http = newHttpRoutes
	_, err = istioClient.NetworkingV1alpha3().VirtualServices(baseNS).Update(ctx, baseVS, metav1.UpdateOptions{})
	return err
}
