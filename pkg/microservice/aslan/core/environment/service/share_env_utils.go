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

	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	zadigutil "github.com/koderover/zadig/v2/pkg/util"
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
		Production:      zadigutil.GetBoolPointer(false),
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
	shareSubEnvs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		ShareEnvEnable: zadigutil.GetBoolPointer(true),
		Production:     zadigutil.GetBoolPointer(false),
	})
	if err != nil {
		return fmt.Errorf("failed to list products which enable env sharing: %s", err)
	}
	grayscaleGrayEnvs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		IstioGrayscaleEnable: zadigutil.GetBoolPointer(true),
		Production:           zadigutil.GetBoolPointer(true),
	})
	if err != nil {
		return fmt.Errorf("failed to list products which enable istio grayscale: %s", err)
	}

	envs := shareSubEnvs
	envs = append(envs, grayscaleGrayEnvs...)

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

	return kube.DeleteEnvoyFilter(ctx, istioClient, setting.IstioNamespace, setting.ZadigEnvoyFilter)
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
		err = kube.EnsureCleanRouteInBase(ctx, grayEnv.EnvName, baseEnv.Namespace, vsInGray.Name, istioClient)
		if err != nil {
			return fmt.Errorf("failed to clean route in base env: %s", err)
		}
	}

	return nil
}

func ensureDeleteGateway(ctx context.Context, env *commonmodels.Product, gwName string, istioClient versionedclient.Interface) error {
	_, err := istioClient.NetworkingV1alpha3().Gateways(env.Namespace).Get(ctx, gwName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		log.Warnf("Failed to find gateway %s for env %s of product %s. Don't try to delete it.", gwName, env.EnvName, env.ProductName)
		return nil
	}
	if err != nil {
		return err
	}

	deleteOption := metav1.DeletePropagationBackground
	return istioClient.NetworkingV1alpha3().Gateways(env.Namespace).Delete(ctx, gwName, metav1.DeleteOptions{
		PropagationPolicy: &deleteOption,
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
