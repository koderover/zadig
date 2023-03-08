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
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ReplicaSetGVK = schema.GroupVersionKind{
	Group:   "apps",
	Kind:    "ReplicaSet",
	Version: "v1",
}

func ListReplicaSets(ns string, selector labels.Selector, cl client.Client) ([]*appsv1.ReplicaSet, error) {
	ss := &appsv1.ReplicaSetList{}
	err := ListResourceInCache(ns, selector, nil, ss, cl)
	if err != nil {
		return nil, err
	}

	var res []*appsv1.ReplicaSet
	for i := range ss.Items {
		setReplicaSetGVK(&ss.Items[i])
		res = append(res, &ss.Items[i])
	}
	return res, err
}

func setReplicaSetGVK(replicaSet *appsv1.ReplicaSet) {
	if replicaSet == nil {
		return
	}
	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "ReplicaSet",
		Version: "v1",
	}
	replicaSet.SetGroupVersionKind(gvk)
}
