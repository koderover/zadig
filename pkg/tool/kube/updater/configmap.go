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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/tool/kube/util"
)

func DeleteConfigMaps(namespace string, selector labels.Selector, clientset *kubernetes.Clientset) error {
	deletePolicy := metav1.DeletePropagationForeground
	err := clientset.CoreV1().ConfigMaps(namespace).DeleteCollection(
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

func UpdateConfigMap(namespace string, cm *corev1.ConfigMap, clientset *kubernetes.Clientset) error {
	_, err := clientset.CoreV1().ConfigMaps(namespace).Update(
		context.TODO(),
		cm,
		metav1.UpdateOptions{},
	)
	return err
}

func DeleteConfigMap(ns, name string, cl client.Client) error {
	return deleteObjectWithDefaultOptions(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}, cl)
}

func CreateConfigMap(cm *corev1.ConfigMap, cl client.Client) error {
	return createObjectNeverAnnotation(cm, cl)
}

func DeleteConfigMapsAndWait(ns string, selector labels.Selector, cl client.Client) error {
	gvk := schema.GroupVersionKind{
		Group:   "",
		Kind:    "ConfigMap",
		Version: "v1",
	}
	return deleteObjectsAndWait(ns, selector, &corev1.ConfigMap{}, gvk, cl)
}
