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

	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/kube/util"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	zadigutil "github.com/koderover/zadig/v2/pkg/util"
)

func EnsureCleanRouteInBase(ctx context.Context, envName, baseNS, vsName string, istioClient versionedclient.Interface) error {
	vsObj, err := istioClient.NetworkingV1alpha3().VirtualServices(baseNS).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil {
		log.Warnf("Failed to query VirtualService %s releated to env %s in namesapce %s: %s. It may be not an exception and skip.", vsName, envName, baseNS, err)
		return nil
	}

	// Note: DeepCopy is used to avoid unpredictable results in concurrent operations.
	baseVS := vsObj.DeepCopy()

	if len(baseVS.Spec.Http) == 0 {
		return nil
	}

	var needClean bool
	var location int
	for i, vsHttp := range baseVS.Spec.Http {
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

			if matchValue.GetExact() != envName {
				continue
			}

			needClean = true
			break
		}

		if !needClean {
			continue
		}

		location = i
		break
	}

	if !needClean {
		return nil
	}

	log.Infof("Begin to clean route env=%s in base ns %s for VirtualService %s.", envName, baseNS, vsName)

	httpRoutes := make([]*networkingv1alpha3.HTTPRoute, 0, len(baseVS.Spec.Http)-1)
	httpRoutes = append(httpRoutes, baseVS.Spec.Http[:location]...)
	httpRoutes = append(httpRoutes, baseVS.Spec.Http[location+1:]...)
	baseVS.Spec.Http = httpRoutes

	_, err = istioClient.NetworkingV1alpha3().VirtualServices(baseNS).Update(ctx, baseVS, metav1.UpdateOptions{})
	return err
}

func ensureServicesInAllSubEnvs(ctx context.Context, env *commonmodels.Product, svc *corev1.Service, kclient client.Client, istioClient versionedclient.Interface) error {
	envs, err := FetchSubEnvs(ctx, env.ProductName, env.ClusterID, env.EnvName)
	if err != nil {
		return err
	}

	vsName := GenVirtualServiceName(svc)
	for _, env := range envs {
		log.Infof("Begin to ensure Services in subenv %s of prouduct %s.", env.EnvName, env.ProductName)

		err = EnsureVirtualService(ctx, kclient, istioClient, env, svc, vsName)
		if err != nil {
			return fmt.Errorf("failed to ensure virtual service, err: %w", err)
		}

		err = EnsureDefaultK8sServiceInGray(ctx, svc, env.Namespace, kclient)
		if err != nil {
			return fmt.Errorf("failed to ensure default k8s service in gray, err: %w", err)
		}
	}

	return nil
}

func ensureDeleteVirtualService(ctx context.Context, env *commonmodels.Product, vsName string, istioClient versionedclient.Interface) error {
	_, err := istioClient.NetworkingV1alpha3().VirtualServices(env.Namespace).Get(ctx, vsName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		log.Warnf("Failed to find VirtualService %s for env %s of product %s. Don't try to delete it.", vsName, env.EnvName, env.ProductName)
		return nil
	}
	if err != nil {
		return err
	}

	deleteOption := metav1.DeletePropagationBackground
	return istioClient.NetworkingV1alpha3().VirtualServices(env.Namespace).Delete(ctx, vsName, metav1.DeleteOptions{
		PropagationPolicy: &deleteOption,
	})
}

func ensureDeleteServiceInAllSubEnvs(ctx context.Context, baseEnv *commonmodels.Product, svc *corev1.Service, kclient client.Client, istioClient versionedclient.Interface) error {
	envs, err := FetchSubEnvs(ctx, baseEnv.ProductName, baseEnv.ClusterID, baseEnv.EnvName)
	if err != nil {
		return err
	}

	// Note: Don't delete VirtualService and K8s Service if there's selected pods.
	vsName := GenVirtualServiceName(svc)
	workloadSelector := labels.SelectorFromSet(labels.Set(svc.Spec.Selector))
	for _, env := range envs {
		hasWorkload, err := doesSvcHasWorkload(ctx, env.Namespace, workloadSelector, kclient)
		if err != nil {
			return err
		}
		if hasWorkload {
			continue
		}

		err = ensureDeleteVirtualService(ctx, env, vsName, istioClient)
		if err != nil {
			return fmt.Errorf("failed to delete VirtualService %s in env %s of product %s: %s", vsName, env.EnvName, env.ProductName, err)
		}

		err = EnsureDeleteK8sService(ctx, env.Namespace, svc.Name, kclient, true)
		if err != nil {
			return fmt.Errorf("failed to delete K8s Service %s in env %s of product: %s: %s", svc.Name, env.EnvName, env.ProductName, err)
		}
	}

	return nil
}

func doesSvcHasWorkload(ctx context.Context, ns string, svcSelector labels.Selector, kclient client.Client) (bool, error) {
	podList := &corev1.PodList{}
	err := kclient.List(ctx, podList, &client.ListOptions{
		Namespace:     ns,
		LabelSelector: svcSelector,
	})
	if err != nil {
		return false, util.IgnoreNotFoundError(err)
	}

	if len(podList.Items) == 0 {
		return false, nil
	}

	return true, nil
}

func FetchSubEnvs(ctx context.Context, productName, clusterID, baseEnvName string) ([]*commonmodels.Product, error) {
	return commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:            productName,
		ClusterID:       clusterID,
		ShareEnvEnable:  zadigutil.GetBoolPointer(true),
		ShareEnvIsBase:  zadigutil.GetBoolPointer(false),
		ShareEnvBaseEnv: zadigutil.GetStrPointer(baseEnvName),
		Production:      zadigutil.GetBoolPointer(false),
	})
}

func EnsureUpdateZadigSerivce(ctx context.Context, env *commonmodels.Product, svc *corev1.Service, kclient client.Client, istioClient versionedclient.Interface) error {
	vsName := GenVirtualServiceName(svc)

	if env.ShareEnv.IsBase {
		// 1. Create VirtualService in the base environment.
		err := EnsureVirtualService(ctx, kclient, istioClient, env, svc, vsName)
		if err != nil {
			return err
		}

		// 2. Create VirtualService in all of the sub environments.
		return ensureServicesInAllSubEnvs(ctx, env, svc, kclient, istioClient)
	}

	baseEnv, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    env.ProductName,
		EnvName: env.ShareEnv.BaseEnv,
	})
	if err != nil {
		return err
	}

	// 1. Create VirtualService in the sub environment.
	err = ensureVirtualServiceInGray(ctx, env.EnvName, vsName, svc.Name, env.Namespace, baseEnv.Namespace, istioClient)
	if err != nil {
		return err
	}

	// 2. Updated the VirtualService configuration in the base environment.
	return ensureUpdateVirtualServiceInBase(ctx, env.EnvName, vsName, svc.Name, env.Namespace, baseEnv.Namespace, istioClient)
}

func EnsureDeleteZadigService(ctx context.Context, env *commonmodels.Product, svc *corev1.Service, kclient client.Client, istioClient versionedclient.Interface) error {
	vsName := GenVirtualServiceName(svc)

	// Delete VirtualService in the current environment.
	err := ensureDeleteVirtualService(ctx, env, vsName, istioClient)
	if err != nil {
		return err
	}

	if env.ShareEnv.IsBase {
		// Delete VirtualService and K8s Service in all of the sub environments if there're no specific workloads.
		return ensureDeleteServiceInAllSubEnvs(ctx, env, svc, kclient, istioClient)
	}

	baseEnv, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    env.ProductName,
		EnvName: env.ShareEnv.BaseEnv,
	})
	if err != nil {
		return err
	}

	// Update VirtualService Routes in the base environment.
	return EnsureCleanRouteInBase(ctx, env.EnvName, baseEnv.Namespace, vsName, istioClient)
}
