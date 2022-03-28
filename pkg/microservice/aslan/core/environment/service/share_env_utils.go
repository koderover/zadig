/*
Copyright 2022 The KodeRover Authors.

.Licensed under the Apache License, Version 2.0 (the "License");
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

	"go.uber.org/zap"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
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

func ensureDeleteAssociatedEnvs(ctx context.Context, baseEnvName string) error {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		ShareEnvEnable:  getBoolPointer(true),
		ShareEnvIsBase:  getBoolPointer(false),
		ShareEnvBaseEnv: getStrPointer(baseEnvName),
	})
	if err != nil {
		return err
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}

	for _, env := range envs {
		err := DeleteProduct("system", env.EnvName, env.ProductName, "", logger.Sugar())
		if err != nil {
			return err
		}
	}

	return nil
}

func ensureDeleteEnvoyFilter(ctx context.Context, baseEnv *commonmodels.Product, istioClient versionedclient.Interface) error {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		ShareEnvEnable: getBoolPointer(true),
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

func getBoolPointer(data bool) *bool {
	return &data
}

func getStrPointer(data string) *string {
	return &data
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
	baseVS, err := istioClient.NetworkingV1alpha3().VirtualServices(baseNS).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil {
		return err
	}

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

	httpRoutes := make([]*networkingv1alpha3.HTTPRoute, 0, len(baseVS.Spec.Http)-1)
	httpRoutes = append(httpRoutes, baseVS.Spec.Http[:location]...)
	httpRoutes = append(httpRoutes, baseVS.Spec.Http[location+1:]...)
	baseVS.Spec.Http = httpRoutes

	_, err = istioClient.NetworkingV1alpha3().VirtualServices(baseNS).Update(ctx, baseVS, metav1.UpdateOptions{})
	return err
}
