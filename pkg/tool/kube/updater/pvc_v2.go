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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/kube/util"
)

func DeletePVCV2(ctx context.Context, clusterID, namespace string, opts ...DeleteOption) error {
	config := &deleteConfig{}
	for _, opt := range opts {
		opt(config)
	}

	if config.name == "" && config.selector == "" {
		return fmt.Errorf("must specify either a name or a selector for deletion to prevent accidental namespace wipeout")
	}
	if config.name != "" && config.selector != "" {
		return fmt.Errorf("cannot specify both name and selector simultaneously")
	}

	c, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	propagationPolicy := metav1.DeletePropagationForeground

	if config.name != "" {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      config.name,
			},
		}
		deleteOpts := &client.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		}
		err = c.Delete(ctx, pvc, deleteOpts)
		return util.IgnoreNotFoundError(err)
	}

	if config.selector != "" {
		selector, err := labels.Parse(config.selector)
		if err != nil {
			return fmt.Errorf("failed to parse selector %q: %w", config.selector, err)
		}

		pvc := &corev1.PersistentVolumeClaim{}
		delAllOfOpts := &client.DeleteAllOfOptions{
			DeleteOptions: client.DeleteOptions{PropagationPolicy: &propagationPolicy},
			ListOptions:   client.ListOptions{LabelSelector: selector, Namespace: namespace},
		}

		err = c.DeleteAllOf(ctx, pvc, delAllOfOpts)
		return util.IgnoreNotFoundError(err)
	}

	return nil
}

func CreatePVCV2(ctx context.Context, clusterID, namespace string, pvc *corev1.PersistentVolumeClaim) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	pvc.SetNamespace(namespace)
	_, err = c.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PVC %s/%s: %w", namespace, pvc.Name, err)
	}
	return nil
}

func UpdatePvcV2(ctx context.Context, clusterID, namespace, pvcName string, mutationFunc func(pvc *corev1.PersistentVolumeClaim) error) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pvc, err := c.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get live PVC: %w", err)
		}

		before := pvc.DeepCopy()

		if err := mutationFunc(pvc); err != nil {
			return fmt.Errorf("mutation failed or aborted: %w", err)
		}

		if equality.Semantic.DeepEqual(before, pvc) {
			return nil
		}

		_, err = c.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, pvc, metav1.UpdateOptions{})
		return err
	})

	return err
}
