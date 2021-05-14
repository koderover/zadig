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
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// GetResourceInCache gets a specific Kubernetes object in local cache, object can be any types
// which are registered by "scheme.AddToScheme()".
// Return true if object is found, false if not, or an error if something bad happened.
func GetResourceInCache(ns, name string, obj client.Object, cl client.Reader) (bool, error) {
	err := cl.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: name}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// ListResourceInCache gets a set of specific Kubernetes object in local cache, object can be any types
// which are registered by "scheme.AddToScheme()".
// Important: fieldSelector must be nil or contain just one kv pair, otherwise, controller-runtime will return error.
func ListResourceInCache(ns string, selector labels.Selector, fieldSelector fields.Selector, obj client.ObjectList, cl client.Reader) error {
	opts := []client.ListOption{client.InNamespace(ns)}
	if selector != nil && !selector.Empty() {
		opts = append(opts, client.MatchingLabelsSelector{Selector: selector})
	}

	if fieldSelector != nil && !fieldSelector.Empty() {
		opts = append(opts, client.MatchingFieldsSelector{Selector: fieldSelector})
	}

	return cl.List(context.TODO(), obj, opts...)
}

// ListUnstructuredResourceInCache gets a set of specific Kubernetes object in local cache, and return a representation in yaml format.
func ListUnstructuredResourceInCache(ns string, selector labels.Selector, fieldSelector fields.Selector, gvk schema.GroupVersionKind, cl client.Reader) ([]*unstructured.Unstructured, error) {
	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(gvk)
	err := ListResourceInCache(ns, selector, fieldSelector, u, cl)
	if err != nil {
		return nil, err
	}

	var res []*unstructured.Unstructured
	for i := range u.Items {
		res = append(res, &u.Items[i])
	}
	return res, err
}

// GetResourceJsonInCache gets a specific Kubernetes object in local cache, and return a representation in json format.
// Return true if object is found, false if not, or an error if something bad happened.
func GetResourceJsonInCache(ns, name string, gvk schema.GroupVersionKind, cl client.Reader) ([]byte, bool, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)

	found, err := GetResourceInCache(ns, name, u, cl)
	if err != nil || !found {
		return nil, false, err
	}

	data, err := u.MarshalJSON()
	if err != nil {
		return nil, false, err
	}

	return data, true, nil
}

// GetResourceYamlInCache gets a specific Kubernetes object in local cache, and return a representation in yaml format.
// Return true if object is found, false if not, or an error if something bad happened.
func GetResourceYamlInCache(ns, name string, gvk schema.GroupVersionKind, cl client.Reader) ([]byte, bool, error) {
	d, found, err := GetResourceJsonInCache(ns, name, gvk, cl)
	if err != nil || !found || len(d) == 0 {
		return nil, false, err
	}

	data, err := yaml.JSONToYAML(d)
	if err != nil {
		return nil, false, err
	}

	return data, true, nil
}

// ListResourceJsonInCache gets a set of specific Kubernetes object in local cache, and return a representation in json format.
func ListResourceJsonInCache(ns string, selector labels.Selector, fieldSelector fields.Selector, gvk schema.GroupVersionKind, cl client.Reader) ([][]byte, error) {
	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(gvk)

	err := ListResourceInCache(ns, selector, fieldSelector, u, cl)
	if err != nil {
		return nil, err
	}

	var res [][]byte
	for _, item := range u.Items {
		data, err := item.MarshalJSON()
		if err != nil {
			return nil, err
		}
		res = append(res, data)
	}

	return res, nil
}

// ListResourceYamlInCache gets a set of specific Kubernetes object in local cache, and return a representation in yaml format.
func ListResourceYamlInCache(ns string, selector labels.Selector, fieldSelector fields.Selector, gvk schema.GroupVersionKind, cl client.Reader) ([][]byte, error) {
	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(gvk)

	err := ListResourceInCache(ns, selector, fieldSelector, u, cl)
	if err != nil {
		return nil, err
	}

	var res [][]byte
	for _, item := range u.Items {
		data, err := item.MarshalJSON()
		if err != nil {
			return nil, err
		}
		data, err = yaml.JSONToYAML(data)
		if err != nil {
			return nil, err
		}
		res = append(res, data)
	}

	return res, nil
}
