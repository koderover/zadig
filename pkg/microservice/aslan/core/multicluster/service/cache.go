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

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/types"
)

type PVC struct {
	Name               string `json:"name"`
	StorageSizeInBytes int64  `json:"storage_size_in_bytes"`
}

func ListStorageClasses(ctx context.Context, clusterID string, scType types.StorageClassType) ([]string, error) {
	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get kube client: %s", err)
	}

	var namespace string
	switch clusterID {
	case setting.LocalClusterID:
		// For cluster-level resources, we need to explicitly configure the namespace to be empty.
		namespace = ""
	default:
		namespace = setting.AttachedClusterNamespace
	}

	scList := &storagev1.StorageClassList{}
	err = kclient.List(ctx, scList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list storageclasses: %s", err)
	}

	storageclasses := make([]string, 0, len(scList.Items))
	for _, item := range scList.Items {
		if scType != types.StorageClassAll && !StorageProvisioner(item.Provisioner).IsNFS() {
			continue
		}

		storageclasses = append(storageclasses, item.Name)
	}

	return storageclasses, nil
}

func ListPVCs(ctx context.Context, clusterID, namespace string) ([]PVC, error) {
	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get kube client: %s", err)
	}

	switch clusterID {
	case setting.LocalClusterID:
		// For now caller may not know exactly which namespace Zadig is deployed to, so a correction is made here.
		namespace = config.Namespace()
	default:
		if namespace != setting.AttachedClusterNamespace {
			return nil, fmt.Errorf("invalid namespace in attached cluster: %s. Valid: %s", namespace, setting.AttachedClusterNamespace)
		}
	}

	pvcList := &corev1.PersistentVolumeClaimList{}
	err = kclient.List(ctx, pvcList, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}

	pvcs := make([]PVC, 0, len(pvcList.Items))
	for _, item := range pvcList.Items {
		// Don't support `corev1.PersistentVolumeBlock` because the PVC would be mounted on multiple nodes.
		if item.Spec.VolumeMode != nil && *item.Spec.VolumeMode == corev1.PersistentVolumeBlock {
			continue
		}

		pvcs = append(pvcs, PVC{
			Name:               item.Name,
			StorageSizeInBytes: item.Spec.Resources.Requests.Storage().Value(),
		})
	}

	return pvcs, nil
}
