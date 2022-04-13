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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
)

func CheckIstiod(ctx context.Context, clusterID string) (bool, error) {
	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return false, fmt.Errorf("failed to get kube client: %s", err)
	}

	namespaces := &corev1.NamespaceList{}
	err = kclient.List(ctx, namespaces, client.InNamespace(""))
	if err != nil {
		return false, fmt.Errorf("failed to list namespaces: %s", err)
	}

	deployment := &appsv1.Deployment{}
	for _, ns := range namespaces.Items {
		err = kclient.Get(ctx, client.ObjectKey{Name: IstiodDeployment, Namespace: ns.Name}, deployment)
		if err == nil {
			return true, nil
		}

		if apierrors.IsNotFound(err) {
			err = nil
			continue
		}

		err = fmt.Errorf("failed to search %s: %s", IstiodDeployment, err)
	}

	return false, err
}
