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

package kube

import (
	"context"
	"fmt"
	"time"

	helmclient "github.com/mittwald/go-helm-client"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/kube/util"
	"github.com/koderover/zadig/pkg/tool/log"
	zadigtypes "github.com/koderover/zadig/pkg/types"
	zadigutil "github.com/koderover/zadig/pkg/util"
)

const zadigNamePrefix = "zadig"
const zadigMatchXEnv = "x-env"

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

func genVirtualServiceName(svc *corev1.Service) string {
	return fmt.Sprintf("%s-%s", zadigNamePrefix, svc.Name)
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
		err := EnsureDeleteK8sService(ctx, namespace, svcName, kclient, true)
		if err != nil {
			return fmt.Errorf("failed to ensure delete existing K8s Service %q in namespace %q: %s", svcName, namespace, err)
		}
	}

	return nil
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
