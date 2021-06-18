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

package getter

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetNamespace(ns string, cl client.Client) (*corev1.Namespace, bool, error) {
	g := &corev1.Namespace{}
	found, err := GetResourceInCache("", ns, g, cl)
	if err != nil || !found {
		g = nil
	}

	return g, found, err
}

func ListNamespaces(cl client.Reader) ([]*corev1.Namespace, error) {
	l := &corev1.NamespaceList{}
	err := ListResourceInCache("", nil, nil, l, cl)
	if err != nil {
		return nil, err
	}

	var res []*corev1.Namespace
	for i := range l.Items {
		res = append(res, &l.Items[i])
	}
	return res, err
}
