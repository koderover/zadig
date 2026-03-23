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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/kube/util"
)

func DeleteNamespaceV2(ctx context.Context, clusterID, name string) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	deletePolicy := metav1.DeletePropagationForeground
	err = c.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	return util.IgnoreNotFoundError(err)
}

// TODO: move the common labels into this function, leaving the additional label to the input.
func CreateNamespaceByNameV2(ctx context.Context, clusterID, ns string, nsLabels map[string]string) error {
	c, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ns,
			Labels: nsLabels,
		},
	}
	err = c.Create(ctx, namespace)
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", ns, err)
	}
	return nil
}

func UpdateNamespaceV2(ctx context.Context, clusterID, namespaceName string, mutationFunc func(ns *corev1.Namespace) error) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ns, err := c.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get live namespace: %w", err)
		}

		if err := mutationFunc(ns); err != nil {
			return fmt.Errorf("mutation failed or aborted: %w", err)
		}

		_, err = c.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
		return err
	})

	return err
}
