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

	"github.com/koderover/zadig/pkg/tool/kube/util"
)

func DeletePersistentVolumeClaims(namespace string, selector labels.Selector, clientset *kubernetes.Clientset) error {
	deletePolicy := metav1.DeletePropagationForeground
	err := clientset.CoreV1().PersistentVolumeClaims(namespace).DeleteCollection(
		context.TODO(),
		metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		},
		metav1.ListOptions{
			LabelSelector: selector.String(),
		},
	)

	return util.IgnoreNotFoundError(err)
}

func DeletePvcWithName(namespace, name string, clientset *kubernetes.Clientset) error {
	deletePolicy := metav1.DeletePropagationForeground
	return clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(
		context.TODO(), name,
		metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		},
	)
}

func CreatePvc(namespace string, pvc *corev1.PersistentVolumeClaim, clientset *kubernetes.Clientset) error {
	_, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), pvc,
		metav1.CreateOptions{})
	return err
}

func UpdatePvc(namespace string, pvc *corev1.PersistentVolumeClaim, clientset *kubernetes.Clientset) error {
	_, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Update(context.TODO(), pvc,
		metav1.UpdateOptions{})
	return err
}
