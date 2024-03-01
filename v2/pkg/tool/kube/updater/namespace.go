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
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DeleteNamespace(name string, clientset *kubernetes.Clientset) error {
	deletePolicy := metav1.DeletePropagationForeground
	return clientset.CoreV1().Namespaces().Delete(context.TODO(), name, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
}

func CreateNamespace(ns *corev1.Namespace, cl client.Client) error {
	return createObject(ns, cl)
}

func CreateNamespaceByName(ns string, labels map[string]string, cl client.Client) error {
	n := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "",
			Name:      ns,
			Labels:    labels,
		},
	}
	return CreateNamespace(n, cl)
}

func UpdateNamespace(ns *corev1.Namespace, cl client.Client) error {
	return updateObject(ns, cl)
}
