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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/kube/util"
)

func DeleteJobsV2(ctx context.Context, clusterID, namespace string, opts ...DeleteOption) error {
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

	cl, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	propagationPolicy := metav1.DeletePropagationForeground
	deleteOpts := &client.DeleteAllOfOptions{
		DeleteOptions: client.DeleteOptions{PropagationPolicy: &propagationPolicy},
		ListOptions:   client.ListOptions{LabelSelector: selector, Namespace: namespace},
	}

	err = cl.DeleteAllOf(ctx, &batchv1.Job{}, deleteOpts)
	return util.IgnoreNotFoundError(err)
}

func CreateJobV2(ctx context.Context, clusterID string, job *batchv1.Job) error {
	cl, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	if err := util.CreateApplyAnnotation(job); err != nil {
		return fmt.Errorf("failed to create apply annotation: %w", err)
	}

	if err := cl.Create(ctx, job); err != nil {
		return fmt.Errorf("failed to create job %s/%s: %w", job.Namespace, job.Name, err)
	}

	return nil
}

func DeleteJobV2(ctx context.Context, clusterID, namespace, name string) error {
	cl, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	propagationPolicy := metav1.DeletePropagationForeground
	err = cl.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
	return util.IgnoreNotFoundError(err)
}

func DeleteJobAndWaitV2(ctx context.Context, clusterID, namespace, name string) error {
	cl, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	propagationPolicy := metav1.DeletePropagationForeground
	err = cl.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete job %s/%s: %w", namespace, name, err)
	}

	err = wait.PollUntilContextTimeout(ctx, time.Second, 60*time.Second, true, func(c context.Context) (done bool, err error) {
		fetched := &batchv1.Job{}
		errGet := cl.Get(c, client.ObjectKey{Namespace: namespace, Name: name}, fetched)
		if apierrors.IsNotFound(errGet) {
			return true, nil
		}
		if errGet != nil {
			return false, nil
		}
		return false, nil
	})

	if err != nil {
		return fmt.Errorf("timeout waiting for job %s/%s to be completely deleted: %w", namespace, name, err)
	}

	return nil
}

func DeleteJobsAndWaitV2(ctx context.Context, clusterID, namespace string, opts ...DeleteOption) error {
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

	cl, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	propagationPolicy := metav1.DeletePropagationForeground
	deleteOpts := &client.DeleteAllOfOptions{
		DeleteOptions: client.DeleteOptions{PropagationPolicy: &propagationPolicy},
		ListOptions:   client.ListOptions{LabelSelector: selector, Namespace: namespace},
	}

	err = cl.DeleteAllOf(ctx, &batchv1.Job{}, deleteOpts)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete jobs matching %q in %s: %w", config.selector, namespace, err)
	}

	err = wait.PollUntilContextTimeout(ctx, time.Second, 60*time.Second, true, func(c context.Context) (done bool, err error) {
		var list batchv1.JobList
		if err := cl.List(c, &list, &client.ListOptions{LabelSelector: selector, Namespace: namespace}); err != nil {
			return false, nil
		}
		return len(list.Items) == 0, nil
	})

	if err != nil {
		return fmt.Errorf("timeout waiting for jobs matching %q in %s to be completely deleted: %w", config.selector, namespace, err)
	}

	return nil
}
