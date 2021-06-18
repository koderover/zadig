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
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetPod(ns, name string, cl client.Client) (*corev1.Pod, bool, error) {
	p := &corev1.Pod{}
	found, err := GetResourceInCache(ns, name, p, cl)
	if err != nil || !found {
		p = nil
	}

	return p, found, err
}

func ListPods(ns string, selector labels.Selector, cl client.Client) ([]*corev1.Pod, error) {
	ps := &corev1.PodList{}
	err := ListResourceInCache(ns, selector, nil, ps, cl)
	if err != nil {
		return nil, err
	}

	var res []*corev1.Pod
	for i := range ps.Items {
		res = append(res, &ps.Items[i])
	}
	return res, err
}
