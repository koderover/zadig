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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/kube/util"
)

func DeleteServiceAccountsV2(ctx context.Context, clusterID, namespace string, opts ...DeleteOption) error {
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

	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	propagationPolicy := metav1.DeletePropagationForeground
	deleteOpts := metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}

	if config.name != "" {
		err = c.CoreV1().ServiceAccounts(namespace).Delete(ctx, config.name, deleteOpts)
		return util.IgnoreNotFoundError(err)
	}

	selector, err := labels.Parse(config.selector)
	if err != nil {
		return fmt.Errorf("failed to parse selector %q: %w", config.selector, err)
	}

	err = c.CoreV1().ServiceAccounts(namespace).DeleteCollection(ctx, deleteOpts, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	return util.IgnoreNotFoundError(err)
}

func CreateServiceAccountV2(ctx context.Context, clusterID, namespace string, sa *corev1.ServiceAccount) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	sa.SetNamespace(namespace)
	_, err = c.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service account %s/%s: %w", namespace, sa.Name, err)
	}

	return nil
}

// CreateOrPatchServiceAccountV2 implements a 2-way merge patch for ServiceAccount.
func CreateOrPatchServiceAccountV2(ctx context.Context, clusterID, namespace, originalYAML, targetYAML string) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	targetJSON, err := yaml.YAMLToJSON([]byte(targetYAML))
	if err != nil {
		return fmt.Errorf("failed to convert target YAML to JSON: %w", err)
	}

	var targetObj corev1.ServiceAccount
	if err := json.Unmarshal(targetJSON, &targetObj); err != nil {
		return fmt.Errorf("failed to unmarshal target JSON to ServiceAccount: %w", err)
	}

	name := targetObj.GetName()
	if name == "" {
		return fmt.Errorf("service account name cannot be empty in target YAML")
	}

	targetObj.SetNamespace(namespace)
	targetJSONMutated, err := json.Marshal(targetObj)
	if err != nil {
		return fmt.Errorf("failed to re-marshal mutated target object: %w", err)
	}

	originalJSONMutated := []byte("{}")
	if originalYAML != "" {
		originalJSON, err := yaml.YAMLToJSON([]byte(originalYAML))
		if err != nil {
			return fmt.Errorf("failed to convert original YAML to JSON: %w", err)
		}

		var originalObj corev1.ServiceAccount
		if err := json.Unmarshal(originalJSON, &originalObj); err == nil {
			originalObj.SetNamespace(namespace)
			originalJSONMutated, _ = json.Marshal(originalObj)
		} else {
			return fmt.Errorf("failed to unmarshal original JSON: %w", err)
		}
	}

	_, err = c.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, createErr := c.CoreV1().ServiceAccounts(namespace).Create(ctx, &targetObj, metav1.CreateOptions{})
		if createErr != nil {
			return fmt.Errorf("failed to create service account: %w", createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check service account existence: %w", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(originalJSONMutated, targetJSONMutated, &corev1.ServiceAccount{})
	if err != nil {
		return fmt.Errorf("failed to calculate 2-way merge patch: %w", err)
	}

	if string(patchBytes) == "{}" {
		return nil
	}

	_, err = c.CoreV1().ServiceAccounts(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("service account patch failed: %w", err)
	}

	return nil
}
