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
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var StatefulSetGVK = schema.GroupVersionKind{
	Group:   "apps",
	Kind:    "StatefulSet",
	Version: "v1",
}

func GetStatefulSet(ns, name string, cl client.Client) (*appsv1.StatefulSet, bool, error) {
	ss := &appsv1.StatefulSet{}
	found, err := GetResourceInCache(ns, name, ss, cl)
	if err != nil || !found {
		ss = nil
	}
	setStatefulSetGVK(ss)

	return ss, found, err
}

func ListStatefulSets(ns string, selector labels.Selector, cl client.Client) ([]*appsv1.StatefulSet, error) {
	ss := &appsv1.StatefulSetList{}
	err := ListResourceInCache(ns, selector, nil, ss, cl)
	if err != nil {
		return nil, err
	}

	var res []*appsv1.StatefulSet
	for i := range ss.Items {
		setStatefulSetGVK(&ss.Items[i])
		res = append(res, &ss.Items[i])
	}
	return res, err
}

func ListStatefulSetsWithCache(selector labels.Selector, lister informers.SharedInformerFactory) ([]*appsv1.StatefulSet, error) {
	if selector == nil {
		selector = labels.NewSelector()
	}
	return lister.Apps().V1().StatefulSets().Lister().List(selector)
}

func GetStatefulSetByNameWWithCache(name, namespace string, lister informers.SharedInformerFactory) (*appsv1.StatefulSet, error) {
	return lister.Apps().V1().StatefulSets().Lister().StatefulSets(namespace).Get(name)
}

func ListStatefulSetsYaml(ns string, selector labels.Selector, cl client.Client) ([][]byte, error) {
	return ListResourceYamlInCache(ns, selector, nil, StatefulSetGVK, cl)
}

func GetStatefulSetYaml(ns string, name string, cl client.Client) ([]byte, bool, error) {
	return GetResourceYamlInCache(ns, name, StatefulSetGVK, cl)
}

func GetStatefulSetYamlFormat(ns string, name string, cl client.Client) ([]byte, bool, error) {
	return GetResourceYamlInCacheFormat(ns, name, StatefulSetGVK, cl)
}

func setStatefulSetGVK(statefulSet *appsv1.StatefulSet) {
	if statefulSet == nil {
		return
	}
	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "StatefulSet",
		Version: "v1",
	}
	statefulSet.SetGroupVersionKind(gvk)
}
