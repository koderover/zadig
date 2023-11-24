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

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ensureUpdateIstioGrayscaleSerivce(ctx context.Context, env *commonmodels.Product, svc *corev1.Service, kclient client.Client, istioClient versionedclient.Interface) error {
	vsName := genVirtualServiceName(svc)

	if env.IstioGrayscale.IsBase {
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
		EnvName: env.IstioGrayscale.BaseEnv,
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
