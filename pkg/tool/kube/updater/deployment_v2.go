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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/kube/util"
)

func RestartDeploymentV2(ctx context.Context, clusterID, namespace, name string) error {
	c, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	deploy := &appsv1.Deployment{}
	deployKey := client.ObjectKey{Namespace: namespace, Name: name}
	if err := c.Get(ctx, deployKey, deploy); err != nil {
		return fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	selector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
	if err != nil {
		return fmt.Errorf("failed to parse deployment selector: %w", err)
	}

	deleteOpts := []client.DeleteAllOfOption{
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}

	pod := &corev1.Pod{}
	if err := c.DeleteAllOf(ctx, pod, deleteOpts...); err != nil {
		return fmt.Errorf("failed to delete pods for deployment %s/%s: %w", namespace, name, err)
	}

	return nil
}

func DeleteDeploymentV2(ctx context.Context, clusterID, namespace string, opts ...DeleteOption) error {
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

	if config.name != "" {
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      config.name,
			},
		}
	
		propagationPolicy := metav1.DeletePropagationBackground
		deleteOpts := &client.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		}
	
		err = c.Delete(ctx, deploy, deleteOpts)
		return util.IgnoreNotFoundError(err)
	}

	if config.selector != "" {
		selector, err := labels.Parse(config.selector)
		if err != nil {
			return fmt.Errorf("failed to parse selector %q: %w", config.selector, err)
		}

		deploy := &appsv1.Deployment{}

		propagationPolicy := metav1.DeletePropagationBackground
		deleteOpts := &client.DeleteAllOfOptions{
			DeleteOptions: client.DeleteOptions{PropagationPolicy: &propagationPolicy},
			ListOptions:   client.ListOptions{LabelSelector: selector, Namespace: namespace},
		}

		err = c.DeleteAllOf(ctx, deploy, deleteOpts)
		return util.IgnoreNotFoundError(err)
	}

	return nil
}

func DeleteDeploymentAndWaitV2(ctx context.Context, clusterID, namespace string, timeout time.Duration, opts ...DeleteOption) error {
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

	cli, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	propagationPolicy := metav1.DeletePropagationForeground
	deleteOpts := metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}

	if config.name != "" {
		err = cli.AppsV1().Deployments(namespace).Delete(ctx, config.name, deleteOpts)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("failed to initiate deletion for deployment %s/%s: %w", namespace, config.name, err)
		}

		err = wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(c context.Context) (done bool, err error) {
			_, errGet := cli.AppsV1().Deployments(namespace).Get(c, config.name, metav1.GetOptions{})
			if apierrors.IsNotFound(errGet) {
				return true, nil
			}
			if errGet != nil {
				return false, nil
			}
			return false, nil
		})

		if err != nil {
			return fmt.Errorf("timeout (%v) waiting for deployment %s/%s to be completely deleted: %w", timeout, namespace, config.name, err)
		}

		return nil
	}

	if config.selector != "" {
		err = cli.AppsV1().Deployments(namespace).DeleteCollection(ctx, deleteOpts, metav1.ListOptions{LabelSelector: config.selector})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("failed to initiate deletion for deployments matching %q in %s: %w", config.selector, namespace, err)
		}

		err = wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(c context.Context) (done bool, err error) {
			list, errList := cli.AppsV1().Deployments(namespace).List(c, metav1.ListOptions{LabelSelector: config.selector})
			if errList != nil {
				return false, nil
			}
			return len(list.Items) == 0, nil
		})

		if err != nil {
			return fmt.Errorf("timeout (%v) waiting for deployments matching %q in %s to be completely deleted: %w", timeout, config.selector, namespace, err)
		}

		return nil
	}

	return nil
}

// CreateOrPatchDeploymentV2 is used when the YAML is fully controlled by this system, it implements a 3-way merge patch for the deployment. 
// If we are simply editing the deployment, use UpdateDeploymentV2 instead.
func CreateOrPatchDeploymentV2(ctx context.Context, clusterID, namespace, originalYAML, targetYAML string, resourceOverride bool) error {
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

	var targetObj appsv1.Deployment
	if err := json.Unmarshal(targetJSON, &targetObj); err != nil {
		return fmt.Errorf("failed to unmarshal target JSON to Deployment: %w", err)
	}

	name := targetObj.GetName()
	if name == "" {
		return fmt.Errorf("deployment name cannot be empty in target YAML")
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

		var originalObj appsv1.Deployment
		if err := json.Unmarshal(originalJSON, &originalObj); err == nil {
			originalObj.SetNamespace(namespace)
			originalJSONMutated, _ = json.Marshal(originalObj)
		} else {
			return fmt.Errorf("failed to unmarshal original JSON: %w", err)
		}
	}

	_, err = c.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, createErr := c.AppsV1().Deployments(namespace).Create(ctx, &targetObj, metav1.CreateOptions{})
		if createErr != nil {
			return fmt.Errorf("failed to create deployment: %w", createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	if resourceOverride {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			existing, err := c.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get deployment for replace: %w", err)
			}
			targetObj.ResourceVersion = existing.ResourceVersion
			_, err = c.AppsV1().Deployments(namespace).Update(ctx, &targetObj, metav1.UpdateOptions{})
			return err
		})
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(originalJSONMutated, targetJSONMutated, &appsv1.Deployment{})
	if err != nil {
		return fmt.Errorf("failed to calculate 2-way merge patch: %w", err)
	}

	if string(patchBytes) == "{}" {
		return nil
	}

	_, err = c.AppsV1().Deployments(namespace).Patch(
		ctx,
		name,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("deployment patch failed: %w", err)
	}

	return nil
}

// UpdateDeploymentV2 takes the cluster and resource info to identify a resource, and use the mutation function to update the object then tries to patch the resource.
func UpdateDeploymentV2(ctx context.Context, clusterID, namespace, deploymentName string, mutationFunc func(deployment *appsv1.Deployment) error) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		deploy, err := c.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get live deployment: %w", err)
		}

		if err := mutationFunc(deploy); err != nil {
			return fmt.Errorf("mutation failed or aborted: %w", err)
		}

		_, err = c.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
		return err
	})

	return err
}

func UpdateDeploymentImageV2(ctx context.Context, clusterID, namespace, deploymentName, containerName, newImage string) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	patchPayload := map[string]interface{}{
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
	}

	patchBytes, err := json.Marshal(patchPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal image update patch payload: %w", err)
	}

	_, err = c.AppsV1().Deployments(namespace).Patch(
		ctx,
		deploymentName,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to patch image for deployment %s/%s: %w", namespace, deploymentName, err)
	}

	return nil
}

func UpdateDeploymentInitImageV2(ctx context.Context, clusterID, namespace, deploymentName, containerName, newImage string) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	patchPayload := map[string]interface{}{
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
	}

	patchBytes, err := json.Marshal(patchPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal image update patch payload: %w", err)
	}

	_, err = c.AppsV1().Deployments(namespace).Patch(
		ctx,
		deploymentName,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to patch init image for deployment %s/%s: %w", namespace, deploymentName, err)
	}

	return nil
}

func ScaleDeploymentV2(ctx context.Context, clusterID, namespace, name string, replicas int) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	patchBytes := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))

	_, err = c.AppsV1().Deployments(namespace).Patch(
		ctx,
		name,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to scale deployment %s/%s: %w", namespace, name, err)
	}

	return nil
}

func CreateDeploymentV2(ctx context.Context, clusterID, namespace string, deployment *appsv1.Deployment) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	deployment.SetNamespace(namespace)
	_, err = c.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment %s/%s: %w", namespace, deployment.Name, err)
	}

	return nil
}