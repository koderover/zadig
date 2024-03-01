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
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ListEvents(ns string, selector fields.Selector, cl client.Reader) ([]*corev1.Event, error) {
	es := &corev1.EventList{}
	err := ListResourceInCache(ns, nil, selector, es, cl)
	if err != nil {
		return nil, err
	}

	var res []*corev1.Event
	for i := range es.Items {
		res = append(res, &es.Items[i])
	}
	return res, err
}
