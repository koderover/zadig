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

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

type PVC struct {
	Name               string `json:"name"`
	StorageSizeInBytes int64  `json:"storage_size_in_bytes"`
}

func ListStorageClasses(ctx context.Context, clusterID string, scType types.StorageClassType) ([]string, error) {
	kclient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get kube client: %s", err)
	}

	scList := &storagev1.StorageClassList{}
	err = kclient.List(ctx, scList)
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
	kclient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
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

type DeploymentResp struct {
	Name       string           `json:"name"`
	Containers []*ContainerResp `json:"containers"`
}

type ContainerResp struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

func ListDeployments(ctx context.Context, clusterID, namespace string) ([]*DeploymentResp, error) {
	kclient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get kube client: %s", err)
	}

	deploymentList := &appsv1.DeploymentList{}
	err = kclient.List(ctx, deploymentList, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}

	resp := make([]*DeploymentResp, 0)
	for _, deployment := range deploymentList.Items {
		containerList := make([]*ContainerResp, 0)
		for _, container := range deployment.Spec.Template.Spec.Containers {
			containerList = append(containerList, &ContainerResp{
				Name:  container.Name,
				Image: container.Image,
			})
		}
		resp = append(resp, &DeploymentResp{
			Name:       deployment.Name,
			Containers: containerList,
		})
	}

	return resp, nil
}

type IstioVirtualServiceResp struct {
	Name  string   `json:"name"`
	Hosts []string `json:"hosts"`
}

func ListIstioVirtualServices(ctx context.Context, clusterID, namespace string) ([]*IstioVirtualServiceResp, error) {
	istioClient, err := clientmanager.NewKubeClientManager().GetIstioClientSet(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get kube client: %s", err)
	}

	vsList, err := istioClient.NetworkingV1alpha3().VirtualServices(namespace).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resp := make([]*IstioVirtualServiceResp, 0)

	for _, vs := range vsList.Items {
		resp = append(resp, &IstioVirtualServiceResp{
			Name:  vs.Name,
			Hosts: vs.Spec.Hosts,
		})
	}

	return resp, nil
}
