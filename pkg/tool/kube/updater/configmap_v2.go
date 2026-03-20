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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/kube/util"
)

// TODO: fix the whole function design

func DeleteConfigMapsV2(ctx context.Context, clusterID, namespace string, opts ...DeleteOption) error {
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

	deletePolicy := metav1.DeletePropagationForeground
	deleteOpts := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	if config.name != "" {
		err = c.CoreV1().ConfigMaps(namespace).Delete(ctx, config.name, deleteOpts)
		return util.IgnoreNotFoundError(err)
	}

	selector, err := labels.Parse(config.selector)
	if err != nil {
		return fmt.Errorf("failed to parse selector %q: %w", config.selector, err)
	}

	err = c.CoreV1().ConfigMaps(namespace).DeleteCollection(ctx, deleteOpts, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	return util.IgnoreNotFoundError(err)
}

func DeleteConfigMapV2(ctx context.Context, clusterID, namespace, name string) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	propagationPolicy := metav1.DeletePropagationForeground
	err = c.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	return util.IgnoreNotFoundError(err)
}

func CreateConfigMapV2(ctx context.Context, clusterID string, cm *corev1.ConfigMap) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	_, err = c.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create configmap %s/%s: %w", cm.Namespace, cm.Name, err)
	}

	return nil
}

func UpdateConfigMapV2(ctx context.Context, clusterID, namespace string, cm *corev1.ConfigMap) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	_, err = c.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update configmap %s/%s: %w", namespace, cm.Name, err)
	}

	return nil
}

// CreateOrPatchConfigMapV2 implements a 2-way merge patch for ConfigMap.
func CreateOrPatchConfigMapV2(ctx context.Context, clusterID, namespace, originalYAML, targetYAML string, resourceOverride bool) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	if resourceOverride {
		originalYAML = ""
	}

	targetJSON, err := yaml.YAMLToJSON([]byte(targetYAML))
	if err != nil {
		return fmt.Errorf("failed to convert target YAML to JSON: %w", err)
	}

	var targetObj corev1.ConfigMap
	if err := json.Unmarshal(targetJSON, &targetObj); err != nil {
		return fmt.Errorf("failed to unmarshal target JSON to ConfigMap: %w", err)
	}

	name := targetObj.GetName()
	if name == "" {
		return fmt.Errorf("configmap name cannot be empty in target YAML")
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

		var originalObj corev1.ConfigMap
		if err := json.Unmarshal(originalJSON, &originalObj); err == nil {
			originalObj.SetNamespace(namespace)
			originalJSONMutated, _ = json.Marshal(originalObj)
		} else {
			return fmt.Errorf("failed to unmarshal original JSON: %w", err)
		}
	}

	_, err = c.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, createErr := c.CoreV1().ConfigMaps(namespace).Create(ctx, &targetObj, metav1.CreateOptions{})
		if createErr != nil {
			return fmt.Errorf("failed to create configmap: %w", createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check configmap existence: %w", err)
	}

	if resourceOverride {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			existing, err := c.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get configmap for replace: %w", err)
			}
			targetObj.ResourceVersion = existing.ResourceVersion
			_, err = c.CoreV1().ConfigMaps(namespace).Update(ctx, &targetObj, metav1.UpdateOptions{})
			return err
		})
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(originalJSONMutated, targetJSONMutated, &corev1.ConfigMap{})
	if err != nil {
		return fmt.Errorf("failed to calculate 2-way merge patch: %w", err)
	}

	if string(patchBytes) == "{}" {
		return nil
	}

	_, err = c.CoreV1().ConfigMaps(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("configmap patch failed: %w", err)
	}

	return nil
}

func DeleteConfigMapsAndWaitV2(ctx context.Context, clusterID, namespace string, opts ...DeleteOption) error {
	config := &deleteConfig{}
	for _, opt := range opts {
		opt(config)
	}

	if config.selector == "" {
		return fmt.Errorf("must specify a selector for deletion to prevent accidental namespace wipeout")
	}

	selector, err := labels.Parse(config.selector)
	if err != nil {
		return fmt.Errorf("failed to parse selector %q: %w", config.selector, err)
	}

	cli, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	propagationPolicy := metav1.DeletePropagationForeground
	err = cli.CoreV1().ConfigMaps(namespace).DeleteCollection(ctx,
		metav1.DeleteOptions{PropagationPolicy: &propagationPolicy},
		metav1.ListOptions{LabelSelector: selector.String()},
	)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete configmaps matching %q in %s: %w", config.selector, namespace, err)
	}

	err = wait.PollUntilContextTimeout(ctx, time.Second, 60*time.Second, true, func(c context.Context) (done bool, err error) {
		list, listErr := cli.CoreV1().ConfigMaps(namespace).List(c, metav1.ListOptions{LabelSelector: selector.String()})
		if listErr != nil {
			return false, nil
		}
		return len(list.Items) == 0, nil
	})

	if err != nil {
		return fmt.Errorf("timeout waiting for configmaps matching %q in %s to be completely deleted: %w", config.selector, namespace, err)
	}

	return nil
}
