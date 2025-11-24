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
	"encoding/json"
	"fmt"
	"time"

	types "github.com/golang/protobuf/ptypes/struct"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"google.golang.org/protobuf/encoding/protojson"
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
	"github.com/koderover/zadig/v2/pkg/setting"
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

		kclient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
		if err != nil {
			return fmt.Errorf("failed to get kube client: %s", err)
		}

		istioClient, err := clientmanager.NewKubeClientManager().GetIstioClientSet(clusterID)
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
			vsName := GenVirtualServiceName(&svc)

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

		kclient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
		if err != nil {
			return fmt.Errorf("failed to get kube client: %s", err)
		}

		istioClient, err := clientmanager.NewKubeClientManager().GetIstioClientSet(clusterID)
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
			vsName := GenVirtualServiceName(&svc)

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

	// basic http routes
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
				log.Warnf("service %s has no workload in env %s, selector: %v, skip", svcName, weightConfig.Env, svc.Spec.Selector)
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

	// weight http routes
	weightSum := 0
	configuredEnvSet := sets.NewString()
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
				log.Warnf("service %s has no workload in env %s, selector: %v, skip", svcName, weightConfig.Env, svc.Spec.Selector)
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

		configuredEnvSet.Insert(weightConfig.Env)
	}
	if configuredEnvSet.Len() >= 2 {
		if weightSum != 100 {
			return nil, fmt.Errorf("the sum of weight is not 100 for the service %s, the full-path grayscale can't work correctly", svcName)
		}
	} else {
		if baseEnv == "" {
			return nil, fmt.Errorf("base env is not found")
		}
		route := &networkingv1alpha3.HTTPRouteDestination{
			Destination: &networkingv1alpha3.Destination{
				Host: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, envMap[baseEnv].Namespace),
			},
		}
		routes = append([]*networkingv1alpha3.HTTPRouteDestination{}, route)
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
				log.Warnf("service %s has no workload in env %s, selector: %v, skip", svcName, headerMatchConfig.Env, svc.Spec.Selector)
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
	vsName := GenVirtualServiceName(svc)

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
	vsName := GenVirtualServiceName(svc)

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

		vsName := GenVirtualServiceName(svc)
		err := ensureGrayscaleDefaultVirtualService(ctx, kclient, istioClient, env, svc, vsName)
		if err != nil {
			return fmt.Errorf("failed to ensure grayscale default VirtualService %s in env %s for svc %s, err: %w",
				vsName, env.EnvName, svc.Name, err)
		}

		err = EnsureDefaultK8sServiceInGray(ctx, svc, env.Namespace, kclient)
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

func DeleteEnvoyFilter(ctx context.Context, istioClient versionedclient.Interface, istioNamespace, name string) error {
	_, err := istioClient.NetworkingV1alpha3().EnvoyFilters(istioNamespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		log.Infof("EnvoyFilter %s is not found in ns `%s`. Skip.", name, setting.IstioNamespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to query EnvoyFilter %s in ns `%s`: %s", name, setting.IstioNamespace, err)
	}

	deleteOption := metav1.DeletePropagationBackground
	return istioClient.NetworkingV1alpha3().EnvoyFilters(setting.IstioNamespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &deleteOption,
	})
}

func EnsureEnvoyFilter(ctx context.Context, istioClient versionedclient.Interface, clusterID, ns, name string, headerKeys []string) error {
	headerKeySet := sets.NewString(headerKeys...)
	envoyFilterObj, err := istioClient.NetworkingV1alpha3().EnvoyFilters(ns).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		log.Infof("Has found EnvoyFilter `%s` in ns `%s` and don't recreate.", name, setting.IstioNamespace)
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to query EnvoyFilter `%s` in ns `%s`: %s", name, setting.IstioNamespace, err)
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
									Name: setting.EnvoyFilterNetworkHttpConnectionManager,
									SubFilter: &networkingv1alpha3.EnvoyFilter_ListenerMatch_SubFilterMatch{
										Name: setting.EnvoyFilterHttpRouter,
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
									Name: setting.EnvoyFilterNetworkHttpConnectionManager,
									SubFilter: &networkingv1alpha3.EnvoyFilter_ListenerMatch_SubFilterMatch{
										Name: setting.EnvoyFilterHttpRouter,
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

func reGenerateEnvoyFilter(ctx context.Context, clusterID string, headerKeys []string) error {
	istioClient, err := clientmanager.NewKubeClientManager().GetIstioClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to new istio client: %s", err)
	}

	err = DeleteEnvoyFilter(ctx, istioClient, setting.IstioNamespace, setting.ZadigEnvoyFilter)
	if err != nil {
		return fmt.Errorf("failed to delete EnvoyFilter: %s", err)
	}

	err = EnsureEnvoyFilter(ctx, istioClient, clusterID, setting.IstioNamespace, setting.ZadigEnvoyFilter, headerKeys)
	if err != nil {
		return fmt.Errorf("failed to ensure EnvoyFilter in namespace `%s`: %s", setting.IstioNamespace, err)
	}

	return nil
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

  -- Extract trace ID from various tracing headers
  local function extract_trace_id(headers)
    -- W3C traceparent format: 00-{trace-id}-{parent-id}-{trace-flags}
    local traceparent = headers:get("traceparent")
    if traceparent then
      local parts = split_str(traceparent, "-")
      if #parts >= 2 then
        return parts[2]
      end
    end

    -- SkyWalking format: sw8
    local sw8 = headers:get("sw8")
    if sw8 then
      local parts = split_str(sw8, "-")
      if #parts >= 2 then
        return parts[2]
      end
    end

    -- Zipkin B3 format
    local b3_traceid = headers:get("x-b3-traceid")
    if b3_traceid then
      return b3_traceid
    end

    -- Generic request ID fallback
    local request_id = headers:get("x-request-id")
    if request_id then
      return request_id
    end

    return nil
  end

  local traceid = extract_trace_id(request_handle:headers())

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
			"@type":      setting.EnvoyFilterLua,
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

  -- Extract trace ID from various tracing headers
  local function extract_trace_id(headers)
    -- W3C traceparent format: 00-{trace-id}-{parent-id}-{trace-flags}
    local traceparent = headers:get("traceparent")
    if traceparent then
      local parts = split_str(traceparent, "-")
      if #parts >= 2 then
        return parts[2]
      end
    end

    -- SkyWalking format: sw8
    local sw8 = headers:get("sw8")
    if sw8 then
      local parts = split_str(sw8, "-")
      if #parts >= 2 then
        return parts[2]
      end
    end

    -- Zipkin B3 format
    local b3_traceid = headers:get("x-b3-traceid")
    if b3_traceid then
      return b3_traceid
    end

    -- Generic request ID fallback
    local request_id = headers:get("x-request-id")
    if request_id then
      return request_id
    end

    return nil
  end

  local traceid = extract_trace_id(request_handle:headers())

  -- Only add x-env header if not already present
  if not request_handle:headers():get("x-env") then
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

    if headers and headers["x-data"] then
      request_handle:headers():add("x-env", headers["x-data"])
    end
  end
`

	inlineCodeMid := ``
	for _, headerKey := range headerKeys {
		tmpInlineCodeMid := `
  -- Only add header if not already present
  local header_key = "%s"
  if not request_handle:headers():get(header_key) then
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

    if headers and headers["x-data"] then
      request_handle:headers():add(header_key, headers["x-data"])
    end
  end
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
			"@type":      setting.EnvoyFilterLua,
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

	grayEnvs, err := commonutil.FetchGrayEnvs(ctx, baseEnv.ProductName, baseEnv.ClusterID, baseEnv.EnvName)
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
		err = SetIstioGrayscaleWeight(context.TODO(), envMap, req.WeightConfigs)
		if err != nil {
			return fmt.Errorf("failed to set istio grayscale weight, err: %w", err)
		}
	} else if req.GrayscaleStrategy == commonmodels.GrayscaleStrategyHeaderMatch {
		err = SetIstioGrayscaleHeaderMatch(context.TODO(), envMap, req.HeaderMatchConfigs)
		if err != nil {
			return fmt.Errorf("failed to set istio grayscale weight, err: %w", err)
		}

		headerKeys := []string{}
		for _, headerMatchConfig := range req.HeaderMatchConfigs {
			for _, headerMatch := range headerMatchConfig.HeaderMatchs {
				headerKeys = append(headerKeys, headerMatch.Key)
			}
		}

		err = reGenerateEnvoyFilter(ctx, baseEnv.ClusterID, headerKeys)
		if err != nil {
			return fmt.Errorf("failed to re-generate envoy filter, err: %w", err)
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
