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
	"k8s.io/client-go/informers"
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

// ListIngresses gets the ingress (extensions/v1beta1) from the informer
func ListIngresses(selector labels.Selector, lister informers.SharedInformerFactory) ([]*extensionsv1beta1.Ingress, error) {
	if selector == nil {
		selector = labels.NewSelector()
	}
	return lister.Extensions().V1beta1().Ingresses().Lister().List(selector)
}

func ListIngressesYaml(ns string, selector labels.Selector, cl client.Client) ([][]byte, error) {
	gvk := schema.GroupVersionKind{
		Group:   "extensions",
		Kind:    "Ingress",
		Version: "v1beta1",
	}
	return ListResourceYamlInCache(ns, selector, nil, gvk, cl)
}
