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

// service does not support deleteCollection
// see here for details: https://github.com/kubernetes/kubernetes/issues/68468#issuecomment-419981870
func DeleteServices(ns string, selector labels.Selector, cl client.Client) error {
	services, err := getter.ListServices(ns, selector, cl)
	if err != nil {
		return err
	}

	var lastErr error
	for _, service := range services {
		err := DeleteService(ns, service.Name, cl)
		if err != nil {
			lastErr = err
		}
	}

	return lastErr
}

func DeleteService(ns, name string, cl client.Client) error {
	deletePolicy := metav1.DeletePropagationForeground
	return deleteObject(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}, cl, &client.DeleteOptions{PropagationPolicy: &deletePolicy})
}
