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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/lib/tool/kube/getter"
)

func DeleteMatchingNamespace(ns string, selector labels.Selector, cl client.Client) error {
	namespace, found, err := getter.GetNamespace(ns, cl)
	if err != nil {
		return err
	} else if !found {
		return nil
	}

	if !selector.Matches(labels.Set(namespace.Labels)) {
		return nil
	}

	return DeleteNamespace(ns, cl)
}

func DeleteNamespace(ns string, cl client.Client) error {
	return deleteObjectWithDefaultOptions(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "",
			Name:      ns,
		},
	}, cl)
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
