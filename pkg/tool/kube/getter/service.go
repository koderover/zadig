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
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ServiceGVK = schema.GroupVersionKind{
	Group:   "",
	Kind:    "Service",
	Version: "v1",
}

func GetService(ns, name string, cl client.Client) (*corev1.Service, bool, error) {
	svc := &corev1.Service{}
	found, err := GetResourceInCache(ns, name, svc, cl)
	if err != nil || !found {
		svc = nil
	}
	setServiceGVK(svc)

	return svc, found, err
}

func ListServices(ns string, selector labels.Selector, cl client.Client) ([]*corev1.Service, error) {
	ss := &corev1.ServiceList{}
	err := ListResourceInCache(ns, selector, nil, ss, cl)
	if err != nil {
		return nil, err
	}

	var res []*corev1.Service
	for i := range ss.Items {
		setServiceGVK(&ss.Items[i])
		res = append(res, &ss.Items[i])
	}
	return res, err
}

func ListServicesWithCache(selector labels.Selector, lister informers.SharedInformerFactory) ([]*corev1.Service, error) {
	if selector == nil {
		selector = labels.NewSelector()
	}
	return lister.Core().V1().Services().Lister().List(selector)
}

func GetServiceByNameFromCache(name, namespace string, lister informers.SharedInformerFactory) (*corev1.Service, error) {
	return lister.Core().V1().Services().Lister().Services(namespace).Get(name)
}

func ListServicesYaml(ns string, selector labels.Selector, cl client.Client) ([][]byte, error) {
	return ListResourceYamlInCache(ns, selector, nil, ServiceGVK, cl)
}

func GetServiceYaml(ns string, name string, cl client.Client) ([]byte, bool, error) {
	return GetResourceYamlInCache(ns, name, ServiceGVK, cl)
}

func GetServiceYamlFormat(ns string, name string, cl client.Client) ([]byte, bool, error) {
	return GetResourceYamlInCacheFormat(ns, name, ServiceGVK, cl)
}

func setServiceGVK(service *corev1.Service) {
	if service == nil {
		return
	}
	gvk := schema.GroupVersionKind{
		Group:   "",
		Kind:    "Service",
		Version: "v1",
	}
	service.SetGroupVersionKind(gvk)
}
