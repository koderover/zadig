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
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
)

var IngressGVK = schema.GroupVersionKind{
	Group:   "networking.k8s.io",
	Kind:    "Ingress",
	Version: "v1",
}

var IngressBetaGVK = schema.GroupVersionKind{
	Group:   "extensions",
	Kind:    "Ingress",
	Version: "v1beta1",
}

func GetExtensionsV1Beta1Ingress(namespace, name string, lister informers.SharedInformerFactory) (*extensionsv1beta1.Ingress, bool, error) {
	ret, err := lister.Extensions().V1beta1().Ingresses().Lister().Ingresses(namespace).Get(name)
	if err == nil {
		return ret, true, nil
	}
	return nil, false, err
}

func GetNetworkingV1Ingress(namespace, name string, lister informers.SharedInformerFactory) (*v1.Ingress, error) {
	return lister.Networking().V1().Ingresses().Lister().Ingresses(namespace).Get(name)
}

func GetUnstructuredIngress(namespace, name string, cl client.Client, clientset *kubernetes.Clientset) (*unstructured.Unstructured, bool, error) {
	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, false, err
	}
	gvk := IngressBetaGVK
	if !kubeclient.VersionLessThan122(version) {
		gvk = IngressGVK
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	found, err := GetResourceInCache(namespace, name, u, cl)
	if err != nil || !found {
		u = nil
	}
	return u, found, err
}

// ListExtensionsV1Beta1Ingresses gets the ingress (extensions/v1beta1) from the informer
func ListExtensionsV1Beta1Ingresses(selector labels.Selector, lister informers.SharedInformerFactory) ([]*extensionsv1beta1.Ingress, error) {
	if selector == nil {
		selector = labels.NewSelector()
	}
	return lister.Extensions().V1beta1().Ingresses().Lister().List(selector)
}

func ListNetworkingV1Ingress(selector labels.Selector, lister informers.SharedInformerFactory) ([]*v1.Ingress, error) {
	if selector == nil {
		selector = labels.NewSelector()
	}
	return lister.Networking().V1().Ingresses().Lister().List(selector)
}

func ListIngressesYaml(ns string, selector labels.Selector, cl client.Client) ([][]byte, error) {
	gvk := IngressBetaGVK
	return ListResourceYamlInCache(ns, selector, nil, gvk, cl)
}

func ListIngresses(namespace string, cl client.Client, lessThan122 bool) (*unstructured.UnstructuredList, error) {
	gvk := IngressBetaGVK
	if !lessThan122 {
		gvk = IngressGVK
	}
	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(gvk)

	err := ListResourceInCache(namespace, labels.Everything(), nil, u, cl)
	if err != nil {
		return u, err
	}
	return u, err
}

func GetIngressYaml(ns string, name string, cl client.Client, lessThan122 bool) ([]byte, bool, error) {
	gvk := IngressGVK
	bs, exist, err := GetResourceYamlInCache(ns, name, gvk, cl)
	if !lessThan122 {
		return bs, exist, err
	}
	if exist && err == nil {
		return bs, exist, err
	}
	gvk = IngressBetaGVK
	return GetResourceYamlInCache(ns, name, gvk, cl)
}

func GetIngressYamlFormat(ns string, name string, cl client.Client) ([]byte, bool, error) {
	gvk := IngressBetaGVK
	return GetResourceYamlInCacheFormat(ns, name, gvk, cl)
}
