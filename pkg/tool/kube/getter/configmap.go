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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ConfigMapGVK = schema.GroupVersionKind{
	Group:   "",
	Kind:    "ConfigMap",
	Version: "v1",
}

func ListConfigMaps(ns string, selector labels.Selector, cl client.Client) ([]*corev1.ConfigMap, error) {
	l := &corev1.ConfigMapList{}
	gvk := schema.GroupVersionKind{
		Group:   "core",
		Kind:    "ConfigMap",
		Version: "v1",
	}
	l.SetGroupVersionKind(gvk)
	err := ListResourceInCache(ns, selector, nil, l, cl)
	if err != nil {
		return nil, err
	}

	var res []*corev1.ConfigMap
	for i := range l.Items {
		setConfigMapGVK(&l.Items[i])
		res = append(res, &l.Items[i])
	}
	return res, err
}

func GetConfigMap(ns, name string, cl client.Client) (*corev1.ConfigMap, bool, error) {
	g := &corev1.ConfigMap{}
	found, err := GetResourceInCache(ns, name, g, cl)
	if err != nil || !found {
		g = nil
	}
	setConfigMapGVK(g)

	return g, found, err
}

func ListConfigMapsYaml(ns string, selector labels.Selector, cl client.Client) ([][]byte, error) {
	return ListResourceYamlInCache(ns, selector, nil, ConfigMapGVK, cl)
}

func GetConfigMapYaml(ns string, name string, cl client.Client) ([]byte, bool, error) {
	return GetResourceYamlInCache(ns, name, ConfigMapGVK, cl)
}

func GetConfigMapYamlFormat(ns string, name string, cl client.Client) ([]byte, bool, error) {
	return GetResourceYamlInCacheFormat(ns, name, ConfigMapGVK, cl)
}

func setConfigMapGVK(configMap *corev1.ConfigMap) {
	if configMap == nil {
		return
	}
	gvk := schema.GroupVersionKind{
		Group:   "",
		Kind:    "ConfigMap",
		Version: "v1",
	}
	configMap.SetGroupVersionKind(gvk)
}
