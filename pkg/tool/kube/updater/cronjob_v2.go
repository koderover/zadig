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

	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/kube/util"
)

func DeleteCronJobsV2(ctx context.Context, clusterID, namespace string, opts ...DeleteOption) error {
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

	version, err := c.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get server version: %w", err)
	}

	deletePolicy := metav1.DeletePropagationForeground
	deleteOpts := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	if config.name != "" {
		if kubeclient.VersionLessThan121(version) {
			err = c.BatchV1beta1().CronJobs(namespace).Delete(ctx, config.name, deleteOpts)
		} else {
			err = c.BatchV1().CronJobs(namespace).Delete(ctx, config.name, deleteOpts)
		}
		return util.IgnoreNotFoundError(err)
	}

	selector, err := labels.Parse(config.selector)
	if err != nil {
		return fmt.Errorf("failed to parse selector %q: %w", config.selector, err)
	}

	listOpts := metav1.ListOptions{
		LabelSelector: selector.String(),
	}

	if kubeclient.VersionLessThan121(version) {
		err = c.BatchV1beta1().CronJobs(namespace).DeleteCollection(ctx, deleteOpts, listOpts)
	} else {
		err = c.BatchV1().CronJobs(namespace).DeleteCollection(ctx, deleteOpts, listOpts)
	}
	return util.IgnoreNotFoundError(err)
}

// CreateOrPatchCronJobV2 implements a 2-way merge patch for CronJob, similar to CreateOrPatchDeploymentV2.
// On clusters < 1.21, it falls back to batch/v1beta1 API.
func CreateOrPatchCronJobV2(ctx context.Context, clusterID, namespace, originalYAML, targetYAML string) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	version, err := c.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get server version: %w", err)
	}

	if kubeclient.VersionLessThan121(version) {
		return createOrPatchCronJobBeta(ctx, c, namespace, originalYAML, targetYAML)
	}
	return createOrPatchCronJobV1(ctx, c, namespace, originalYAML, targetYAML)
}

func createOrPatchCronJobV1(ctx context.Context, c *kubernetes.Clientset, namespace, originalYAML, targetYAML string) error {
	targetJSON, err := yaml.YAMLToJSON([]byte(targetYAML))
	if err != nil {
		return fmt.Errorf("failed to convert target YAML to JSON: %w", err)
	}

	var targetObj batchv1.CronJob
	if err := json.Unmarshal(targetJSON, &targetObj); err != nil {
		return fmt.Errorf("failed to unmarshal target JSON to CronJob: %w", err)
	}

	name := targetObj.GetName()
	if name == "" {
		return fmt.Errorf("cronjob name cannot be empty in target YAML")
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

		var originalObj batchv1.CronJob
		if err := json.Unmarshal(originalJSON, &originalObj); err == nil {
			originalObj.SetNamespace(namespace)
			originalJSONMutated, _ = json.Marshal(originalObj)
		} else {
			return fmt.Errorf("failed to unmarshal original JSON: %w", err)
		}
	}

	_, err = c.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, createErr := c.BatchV1().CronJobs(namespace).Create(ctx, &targetObj, metav1.CreateOptions{})
		if createErr != nil {
			return fmt.Errorf("failed to create cronjob: %w", createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check cronjob existence: %w", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(originalJSONMutated, targetJSONMutated, &batchv1.CronJob{})
	if err != nil {
		return fmt.Errorf("failed to calculate 2-way merge patch: %w", err)
	}

	if string(patchBytes) == "{}" {
		return nil
	}

	_, err = c.BatchV1().CronJobs(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("cronjob patch failed: %w", err)
	}

	return nil
}

func createOrPatchCronJobBeta(ctx context.Context, c *kubernetes.Clientset, namespace, originalYAML, targetYAML string) error {
	targetJSON, err := yaml.YAMLToJSON([]byte(targetYAML))
	if err != nil {
		return fmt.Errorf("failed to convert target YAML to JSON: %w", err)
	}

	var targetObj batchv1beta1.CronJob
	if err := json.Unmarshal(targetJSON, &targetObj); err != nil {
		return fmt.Errorf("failed to unmarshal target JSON to CronJob: %w", err)
	}

	name := targetObj.GetName()
	if name == "" {
		return fmt.Errorf("cronjob name cannot be empty in target YAML")
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

		var originalObj batchv1beta1.CronJob
		if err := json.Unmarshal(originalJSON, &originalObj); err == nil {
			originalObj.SetNamespace(namespace)
			originalJSONMutated, _ = json.Marshal(originalObj)
		} else {
			return fmt.Errorf("failed to unmarshal original JSON: %w", err)
		}
	}

	_, err = c.BatchV1beta1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, createErr := c.BatchV1beta1().CronJobs(namespace).Create(ctx, &targetObj, metav1.CreateOptions{})
		if createErr != nil {
			return fmt.Errorf("failed to create cronjob (v1beta1): %w", createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check cronjob existence: %w", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(originalJSONMutated, targetJSONMutated, &batchv1beta1.CronJob{})
	if err != nil {
		return fmt.Errorf("failed to calculate 2-way merge patch: %w", err)
	}

	if string(patchBytes) == "{}" {
		return nil
	}

	_, err = c.BatchV1beta1().CronJobs(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("cronjob (v1beta1) patch failed: %w", err)
	}

	return nil
}

func UpdateCronJobImageV2(ctx context.Context, clusterID, namespace, name, containerName, newImage string) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	version, err := c.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get server version: %w", err)
	}

	patchPayload := map[string]interface{}{
		"spec": map[string]interface{}{
			"jobTemplate": map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name":  containerName,
									"image": newImage,
								},
							},
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

	if kubeclient.VersionLessThan121(version) {
		_, err = c.BatchV1beta1().CronJobs(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	} else {
		_, err = c.BatchV1().CronJobs(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}
	if err != nil {
		return fmt.Errorf("failed to patch image for cronjob %s/%s: %w", namespace, name, err)
	}

	return nil
}

func UpdateCronJobInitImageV2(ctx context.Context, clusterID, namespace, name, containerName, newImage string) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	version, err := c.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get server version: %w", err)
	}

	patchPayload := map[string]interface{}{
		"spec": map[string]interface{}{
			"jobTemplate": map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"initContainers": []map[string]interface{}{
								{
									"name":  containerName,
									"image": newImage,
								},
							},
						},
					},
				},
			},
		},
	}

	patchBytes, err := json.Marshal(patchPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal init image update patch payload: %w", err)
	}

	if kubeclient.VersionLessThan121(version) {
		_, err = c.BatchV1beta1().CronJobs(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	} else {
		_, err = c.BatchV1().CronJobs(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}
	if err != nil {
		return fmt.Errorf("failed to patch init image for cronjob %s/%s: %w", namespace, name, err)
	}

	return nil
}

func SuspendCronJobV2(ctx context.Context, clusterID, namespace, name string) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	version, err := c.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get server version: %w", err)
	}

	patchBytes := []byte(`{"spec":{"suspend":true}}`)

	if kubeclient.VersionLessThan121(version) {
		_, err = c.BatchV1beta1().CronJobs(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	} else {
		_, err = c.BatchV1().CronJobs(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}
	if err != nil {
		return fmt.Errorf("failed to suspend cronjob %s/%s: %w", namespace, name, err)
	}

	return nil
}

func ResumeCronJobV2(ctx context.Context, clusterID, namespace, name string) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	version, err := c.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get server version: %w", err)
	}

	patchBytes := []byte(`{"spec":{"suspend":false}}`)

	if kubeclient.VersionLessThan121(version) {
		_, err = c.BatchV1beta1().CronJobs(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	} else {
		_, err = c.BatchV1().CronJobs(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}
	if err != nil {
		return fmt.Errorf("failed to resume cronjob %s/%s: %w", namespace, name, err)
	}

	return nil
}
