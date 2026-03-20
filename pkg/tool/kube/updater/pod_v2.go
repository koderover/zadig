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
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/kube/util"
)

func DeletePodsV2(ctx context.Context, clusterID, namespace string, opts ...DeleteOption) error {
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
	deleteOpts := &client.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}

	if config.name != "" {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      config.name,
			},
		}
		err = c.Delete(ctx, pod, deleteOpts)
		return util.IgnoreNotFoundError(err)
	}

	if config.selector != "" {
		selector, err := labels.Parse(config.selector)
		if err != nil {
			return fmt.Errorf("failed to parse selector %q: %w", config.selector, err)
		}

		pod := &corev1.Pod{}
		delAllOfOpts := &client.DeleteAllOfOptions{
			DeleteOptions: client.DeleteOptions{PropagationPolicy: &propagationPolicy},
			ListOptions:   client.ListOptions{LabelSelector: selector, Namespace: namespace},
		}

		err = c.DeleteAllOf(ctx, pod, delAllOfOpts)
		return util.IgnoreNotFoundError(err)
	}

	return nil
}

func UpdatePodV2(ctx context.Context, clusterID, namespace, name string, mutationFunc func(pod *corev1.Pod) error) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pod, err := c.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get live pod: %w", err)
		}

		if err := mutationFunc(pod); err != nil {
			return fmt.Errorf("mutation failed or aborted: %w", err)
		}

		_, err = c.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{})
		return err
	})

	return err
}

func CreateOrPatchPodV2(ctx context.Context, clusterID, namespace, originalYAML, targetYAML string, resourceOverride bool) error {
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

	var targetObj corev1.Pod
	if err := json.Unmarshal(targetJSON, &targetObj); err != nil {
		return fmt.Errorf("failed to unmarshal target JSON to Pod: %w", err)
	}

	name := targetObj.GetName()
	if name == "" {
		return fmt.Errorf("pod name cannot be empty in target YAML")
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

		var originalObj corev1.Pod
		if err := json.Unmarshal(originalJSON, &originalObj); err == nil {
			originalObj.SetNamespace(namespace)
			originalJSONMutated, _ = json.Marshal(originalObj)
		} else {
			return fmt.Errorf("failed to unmarshal original JSON: %w", err)
		}
	}

	_, err = c.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, createErr := c.CoreV1().Pods(namespace).Create(ctx, &targetObj, metav1.CreateOptions{})
		if createErr != nil {
			return fmt.Errorf("failed to create pod: %w", createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check pod existence: %w", err)
	}

	if resourceOverride {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			existing, err := c.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get pod for replace: %w", err)
			}
			targetObj.ResourceVersion = existing.ResourceVersion
			_, err = c.CoreV1().Pods(namespace).Update(ctx, &targetObj, metav1.UpdateOptions{})
			return err
		})
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(originalJSONMutated, targetJSONMutated, &corev1.Pod{})
	if err != nil {
		return fmt.Errorf("failed to calculate 2-way merge patch: %w", err)
	}

	if string(patchBytes) == "{}" {
		return nil
	}

	_, err = c.CoreV1().Pods(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("pod patch failed: %w", err)
	}

	return nil
}
