/*
Copyright 2022 The KodeRover Authors.

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

func ListDaemonsets(ns string, selector labels.Selector, cl client.Client) ([]*appsv1.DaemonSet, error) {
	dss := &appsv1.DaemonSetList{}
	err := ListResourceInCache(ns, selector, nil, dss, cl)
	if err != nil {
		return nil, err
	}

	var res []*appsv1.DaemonSet
	for i := range dss.Items {
		res = append(res, &dss.Items[i])
	}
	return res, err
}

func GetDaemonSet(ns, name string, cl client.Client) (*appsv1.DaemonSet, bool, error) {
	ss := &appsv1.DaemonSet{}
	found, err := GetResourceInCache(ns, name, ss, cl)
	if err != nil || !found {
		ss = nil
	}
	return ss, found, err
}

func GetDaemonSetYaml(ns, name string, cl client.Client) ([]byte, bool, error) {
	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "DaemonSet",
		Version: "v1",
	}
	return GetResourceYamlInCache(ns, name, gvk, cl)
}
