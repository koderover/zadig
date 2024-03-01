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

package updater

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DeleteServices(namespace string, selector labels.Selector, clientset *kubernetes.Clientset) error {
	services, err := clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}

	var lastErr error
	deletePolicy := metav1.DeletePropagationForeground
	for _, svc := range services.Items {
		err := clientset.CoreV1().Services(namespace).Delete(
			context.TODO(),
			svc.Name,
			metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			},
		)
		if err != nil {
		}
		lastErr = err
	}

	return lastErr
}

func CreateOrPatchService(s *corev1.Service, cl client.Client) error {
	return createOrPatchObject(s, cl)
}

func DeleteService(ns, name string, cl client.Client) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
	return deleteObject(service, cl)
}
