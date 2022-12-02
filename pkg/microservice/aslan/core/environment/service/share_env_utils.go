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

	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/kube/util"
	"github.com/koderover/zadig/pkg/tool/log"
	zadiglabels "github.com/koderover/zadig/pkg/types"
	zadigutil "github.com/koderover/zadig/pkg/util"
)

func ensureBaseEnvConfig(ctx context.Context, baseEnv *commonmodels.Product) error {
	if baseEnv.ShareEnv.Enable && baseEnv.ShareEnv.IsBase {
		return nil
	}

	baseEnv.ShareEnv = commonmodels.ProductShareEnv{
		Enable: true,
		IsBase: true,
	}

	return commonrepo.NewProductColl().Update(baseEnv)
}

func ensureDisableBaseEnvConfig(ctx context.Context, baseEnv *commonmodels.Product) error {
	if !baseEnv.ShareEnv.Enable && !baseEnv.ShareEnv.IsBase {
		return nil
	}

	baseEnv.ShareEnv = commonmodels.ProductShareEnv{
		Enable: false,
		IsBase: false,
	}

	return commonrepo.NewProductColl().Update(baseEnv)
}

func ensureDeleteAssociatedEnvs(ctx context.Context, baseProduct *commonmodels.Product) error {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:            baseProduct.ProductName,
		ShareEnvEnable:  zadigutil.GetBoolPointer(true),
		ShareEnvIsBase:  zadigutil.GetBoolPointer(false),
		ShareEnvBaseEnv: zadigutil.GetStrPointer(baseProduct.EnvName),
	})
	if err != nil {
		log.Error(err)
		return err
	}

	logger := log.SugaredLogger()
	for _, env := range envs {
		err := DeleteProduct("system", env.EnvName, env.ProductName, "", true, logger)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}

func ensureDeleteEnvoyFilter(ctx context.Context, baseEnv *commonmodels.Product, istioClient versionedclient.Interface) error {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		ShareEnvEnable: zadigutil.GetBoolPointer(true),
	})
	if err != nil {
		return fmt.Errorf("failed to list products which enable env sharing: %s", err)
	}

	needDeleteEnvoyFilter := true
	for _, env := range envs {
		if env.ProductName == baseEnv.ProductName && env.EnvName == baseEnv.EnvName {
			continue
		}

		needDeleteEnvoyFilter = false
	}

	if !needDeleteEnvoyFilter {
		return nil
	}

	return deleteEnvoyFilter(ctx, istioClient, istioNamespace, zadigEnvoyFilter)
}

func ensureCleanRoutesInBase(ctx context.Context, grayEnv *commonmodels.Product, istioClient versionedclient.Interface) error {
	baseEnv, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    grayEnv.ProductName,
		EnvName: grayEnv.ShareEnv.BaseEnv,
	})
	if err != nil {
		log.Error(err)
		return err
	}

	grayNS := grayEnv.Namespace
	vsList, err := istioClient.NetworkingV1alpha3().VirtualServices(grayNS).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list VirtualServices for env %s of product %s: %s", grayEnv.EnvName, grayEnv.ProductName, err)
	}

	for _, vsInGray := range vsList.Items {
		err = ensureCleanRouteInBase(ctx, grayEnv.EnvName, baseEnv.Namespace, vsInGray.Name, istioClient)
		if err != nil {
			return fmt.Errorf("failed to clean route in base env: %s", err)
		}
	}

	return nil
}

func deleteEnvoyFilter(ctx context.Context, istioClient versionedclient.Interface, istioNamespace, name string) error {
	_, err := istioClient.NetworkingV1alpha3().EnvoyFilters(istioNamespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		log.Infof("EnvoyFilter %s is not found in ns `%s`. Skip.", name, istioNamespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to query EnvoyFilter %s in ns `%s`: %s", name, istioNamespace, err)
	}

	deleteOption := metav1.DeletePropagationBackground
	return istioClient.NetworkingV1alpha3().EnvoyFilters(istioNamespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &deleteOption,
	})
}

func ensureCleanRouteInBase(ctx context.Context, envName, baseNS, vsName string, istioClient versionedclient.Interface) error {
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
	envs, err := fetchSubEnvs(ctx, env.ProductName, env.ClusterID, env.EnvName)
	if err != nil {
		return err
	}

	vsName := genVirtualServiceName(svc)
	for _, env := range envs {
		log.Infof("Begin to ensure Services in subenv %s of prouduct %s.", env.EnvName, env.ProductName)

		err = ensureVirtualService(ctx, kclient, istioClient, env, svc, vsName)
		if err != nil {
			return err
		}

		err = ensureDefaultK8sServiceInGray(ctx, svc, env.Namespace, kclient)
		if err != nil {
			return err
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
	envs, err := fetchSubEnvs(ctx, baseEnv.ProductName, baseEnv.ClusterID, baseEnv.EnvName)
	if err != nil {
		return err
	}

	// Note: Don't delete VirtualService and K8s Service if there's selected pods.
	vsName := genVirtualServiceName(svc)
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

		err = ensureDeleteK8sService(ctx, env.Namespace, svc.Name, kclient, true)
		if err != nil {
			return fmt.Errorf("failed to delete K8s Service %s in env %s of product: %s: %s", svc.Name, env.EnvName, env.ProductName, err)
		}
	}

	return nil
}

func ensureDeleteK8sService(ctx context.Context, ns, svcName string, kclient client.Client, systemCreatedOnly bool) error {
	svc := &corev1.Service{}
	err := kclient.Get(ctx, client.ObjectKey{
		Name:      svcName,
		Namespace: ns,
	}, svc)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if systemCreatedOnly && !(svc.Labels != nil && svc.Labels[zadiglabels.ZadigLabelKeyGlobalOwner] == zadiglabels.Zadig) {
		return nil
	}

	deleteOption := metav1.DeletePropagationBackground
	return kclient.Delete(ctx, svc, &client.DeleteOptions{
		PropagationPolicy: &deleteOption,
	})
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

func fetchSubEnvs(ctx context.Context, productName, clusterID, baseEnvName string) ([]*commonmodels.Product, error) {
	return commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:            productName,
		ClusterID:       clusterID,
		ShareEnvEnable:  zadigutil.GetBoolPointer(true),
		ShareEnvIsBase:  zadigutil.GetBoolPointer(false),
		ShareEnvBaseEnv: zadigutil.GetStrPointer(baseEnvName),
	})
}

func getSvcInEnv(env *commonmodels.Product) []string {
	svcs := []string{}
	for _, svcGroup := range env.Services {
		for _, svc := range svcGroup {
			svcs = append(svcs, svc.ServiceName)
		}
	}

	return svcs
}

func ensureUpdateZadigSerivce(ctx context.Context, env *commonmodels.Product, svc *corev1.Service, kclient client.Client, istioClient versionedclient.Interface) error {
	vsName := genVirtualServiceName(svc)

	if env.ShareEnv.IsBase {
		// 1. Create VirtualService in the base environment.
		err := ensureVirtualService(ctx, kclient, istioClient, env, svc, vsName)
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

func ensureDeleteZadigService(ctx context.Context, env *commonmodels.Product, svc *corev1.Service, kclient client.Client, istioClient versionedclient.Interface) error {
	vsName := genVirtualServiceName(svc)

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
	return ensureCleanRouteInBase(ctx, env.EnvName, baseEnv.Namespace, vsName, istioClient)
}
