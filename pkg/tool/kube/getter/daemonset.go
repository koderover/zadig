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
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var DaemonSetGVK = schema.GroupVersionKind{
	Group:   "apps",
	Kind:    "DaemonSet",
	Version: "v1",
}

func GetDaemonSet(ns, name string, cl client.Client) (*appsv1.DaemonSet, bool, error) {
	daemonSet := &appsv1.DaemonSet{}
	found, err := GetResourceInCache(ns, name, daemonSet, cl)
	if err != nil || !found {
		daemonSet = nil
	}
	setDaemonSetGVK(daemonSet)
	return daemonSet, found, err
}

func ListDaemonSets(ns string, selector labels.Selector, cl client.Client) ([]*appsv1.DaemonSet, error) {
	daemonSetList := &appsv1.DaemonSetList{}
	err := ListResourceInCache(ns, selector, nil, daemonSetList, cl)
	if err != nil {
		return nil, err
	}

	daemonSets := make([]*appsv1.DaemonSet, 0, len(daemonSetList.Items))
	for i := range daemonSetList.Items {
		setDaemonSetGVK(&daemonSetList.Items[i])
		daemonSets = append(daemonSets, &daemonSetList.Items[i])
	}
	return daemonSets, err
}

func ListDaemonSetsWithCache(selector labels.Selector, lister informers.SharedInformerFactory) ([]*appsv1.DaemonSet, error) {
	if selector == nil {
		selector = labels.NewSelector()
	}
	return lister.Apps().V1().DaemonSets().Lister().List(selector)
}

func GetDaemonSetByNameWithCache(name, namespace string, lister informers.SharedInformerFactory) (*appsv1.DaemonSet, error) {
	return lister.Apps().V1().DaemonSets().Lister().DaemonSets(namespace).Get(name)
}

func ListDaemonSetsYaml(ns string, selector labels.Selector, cl client.Client) ([][]byte, error) {
	return ListResourceYamlInCache(ns, selector, nil, DaemonSetGVK, cl)
}

func GetDaemonSetYaml(ns, name string, cl client.Client) ([]byte, bool, error) {
	return GetResourceYamlInCache(ns, name, DaemonSetGVK, cl)
}

func GetDaemonSetYamlFormat(ns string, name string, cl client.Client) ([]byte, bool, error) {
	return GetResourceYamlInCacheFormat(ns, name, DaemonSetGVK, cl)
}

func setDaemonSetGVK(daemonSet *appsv1.DaemonSet) {
	if daemonSet == nil {
		return
	}
	daemonSet.SetGroupVersionKind(DaemonSetGVK)
}
