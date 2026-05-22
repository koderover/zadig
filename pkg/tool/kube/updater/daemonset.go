/*
Copyright 2026 The KodeRover Authors.

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
package updater

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
)

func RestartDaemonSet(ctx context.Context, clusterID, namespace, name string) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		daemonSet, err := c.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get live daemonset: %w", err)
		}

		if daemonSet.Spec.Template.Annotations == nil {
			daemonSet.Spec.Template.Annotations = make(map[string]string)
		}
		daemonSet.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = metav1.Now().Format("2006-01-02T15:04:05Z07:00")

		_, err = c.AppsV1().DaemonSets(namespace).Update(ctx, daemonSet, metav1.UpdateOptions{})
		return err
	})
}

func UpdateDaemonSetImage(ctx context.Context, clusterID, namespace, daemonSetName, containerName, newImage string) error {
	return patchDaemonSetImage(ctx, clusterID, namespace, daemonSetName, containerName, newImage, "containers")
}

func UpdateDaemonSetInitImage(ctx context.Context, clusterID, namespace, daemonSetName, containerName, newImage string) error {
	return patchDaemonSetImage(ctx, clusterID, namespace, daemonSetName, containerName, newImage, "initContainers")
}

func patchDaemonSetImage(ctx context.Context, clusterID, namespace, daemonSetName, containerName, newImage, field string) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	patchPayload := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					field: []map[string]interface{}{
						{
							"name":  containerName,
							"image": newImage,
						},
					},
				},
			},
		},
	}

	patchBytes, err := json.Marshal(patchPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal image update patch payload: %w", err)
	}

	_, err = c.AppsV1().DaemonSets(namespace).Patch(
		ctx,
		daemonSetName,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to patch %s image for daemonset %s/%s: %w", field, namespace, daemonSetName, err)
	}

	return nil
}
