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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/kube/util"
)

func DeleteSecretsV2(ctx context.Context, clusterID, namespace string, opts ...DeleteOption) error {
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

	cl, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	if config.name != "" {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      config.name,
			},
		}
		err = cl.Delete(ctx, svc)
		return util.IgnoreNotFoundError(err)
	}

	if config.selector != "" {
		selector, err := labels.Parse(config.selector)
		if err != nil {
			return fmt.Errorf("failed to parse selector %q: %w", config.selector, err)
		}

		deploy := &corev1.Secret{}

		propagationPolicy := metav1.DeletePropagationBackground
		deleteOpts := &client.DeleteAllOfOptions{
			DeleteOptions: client.DeleteOptions{PropagationPolicy: &propagationPolicy},
			ListOptions:   client.ListOptions{LabelSelector: selector, Namespace: namespace},
		}

		err = cl.DeleteAllOf(ctx, deploy, deleteOpts)
		return util.IgnoreNotFoundError(err)
	}

	return fmt.Errorf("must specify either a name or a selector for deletion of the service to prevent accidental namespace wipeout")
}

func UpdateOrCreateSecretV2(ctx context.Context, clusterID string, s *corev1.Secret) error {
	cl, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	if err := util.CreateApplyAnnotation(s); err != nil {
		return fmt.Errorf("failed to create apply annotation: %w", err)
	}

	err = cl.Update(ctx, s)
	if err == nil {
		return nil
	}
	if apierrors.IsNotFound(err) {
		if createErr := cl.Create(ctx, s); createErr != nil {
			return fmt.Errorf("failed to create secret %s/%s: %w", s.Namespace, s.Name, createErr)
		}
		return nil
	}
	return fmt.Errorf("failed to update secret %s/%s: %w", s.Namespace, s.Name, err)
}
