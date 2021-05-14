/*
Copyright 2021 The KodeRover Authors.

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

package watcher

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

var finishedStates = map[corev1.PodPhase]struct{}{corev1.PodSucceeded: {}, corev1.PodFailed: {}, corev1.PodUnknown: {}}

// waitPodUntilPhase waits until any pods in the list meet the desired phase
func waitPodUntilPhase(ctx context.Context, ns string, selector labels.Selector, desiredPhase corev1.PodPhase, clientset kubernetes.Interface) error {
	watcher, err := clientset.CoreV1().Pods(ns).Watch(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}
	defer watcher.Stop()

	_, err = WaitUntil(ctx, watcher, func(event watch.Event) (bool, error) {
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			return false, fmt.Errorf("obj <%v> is not a Pod", event.Object)
		}

		phase := pod.Status.Phase
		if _, ok := finishedStates[phase]; ok {
			return true, nil
		}

		return pod.Status.Phase == desiredPhase, nil
	})

	return err
}

// WaitUntilPodRunning waits until any pods in the list is running
func WaitUntilPodRunning(ctx context.Context, ns string, selector labels.Selector, clientset kubernetes.Interface) error {
	return waitPodUntilPhase(ctx, ns, selector, corev1.PodRunning, clientset)
}
