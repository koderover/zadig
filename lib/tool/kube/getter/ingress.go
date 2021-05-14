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
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetIngress(ns, name string, cl client.Client) (*extensionsv1beta1.Ingress, bool, error) {
	g := &extensionsv1beta1.Ingress{}
	found, err := GetResourceInCache(ns, name, g, cl)
	if err != nil || !found {
		g = nil
	}

	return g, found, err
}

func ListIngresses(ns string, selector labels.Selector, cl client.Client) ([]*extensionsv1beta1.Ingress, error) {
	// ingress is moved to networking.k8s.io/v1beta1 from extensions/v1beta1.
	is := &extensionsv1beta1.IngressList{}
	err := ListResourceInCache(ns, selector, nil, is, cl)
	if err != nil {
		return nil, err
	}

	var res []*extensionsv1beta1.Ingress
	for i := range is.Items {
		res = append(res, &is.Items[i])
	}
	return res, err
}

func ListIngressesYaml(ns string, selector labels.Selector, cl client.Client) ([][]byte, error) {
	gvk := schema.GroupVersionKind{
		Group:   "extensions",
		Kind:    "Ingress",
		Version: "v1beta1",
	}
	return ListResourceYamlInCache(ns, selector, nil, gvk, cl)
}
