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
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ListPvcs(ns string, selector fields.Selector, cl client.Reader) ([]*corev1.PersistentVolumeClaim, error) {
	pvcList := &corev1.PersistentVolumeClaimList{}
	gvk := schema.GroupVersionKind{
		Group:   "core",
		Kind:    "PersistentVolumeClaim",
		Version: "v1",
	}
	pvcList.SetGroupVersionKind(gvk)
	err := ListResourceInCache(ns, nil, selector, pvcList, cl)
	if err != nil {
		return nil, err
	}

	var res []*corev1.PersistentVolumeClaim
	for i := range pvcList.Items {
		setPVCGVK(&pvcList.Items[i])
		res = append(res, &pvcList.Items[i])
	}
	return res, err
}

func GetPVCYaml(ns string, name string, cl client.Client) ([]byte, bool, error) {
	gvk := schema.GroupVersionKind{
		Group:   "",
		Kind:    "PersistentVolumeClaim",
		Version: "v1",
	}
	return GetResourceYamlInCache(ns, name, gvk, cl)
}

func GetPvc(ns, name string, cl client.Client) (*corev1.PersistentVolumeClaim, bool, error) {
	g := &corev1.PersistentVolumeClaim{}
	found, err := GetResourceInCache(ns, name, g, cl)
	if err != nil || !found {
		g = nil
	}
	setPVCGVK(g)

	return g, found, err
}

func GetPVCYamlFormat(ns string, name string, cl client.Client) ([]byte, bool, error) {
	gvk := schema.GroupVersionKind{
		Group:   "",
		Kind:    "PersistentVolumeClaim",
		Version: "v1",
	}
	return GetResourceYamlInCacheFormat(ns, name, gvk, cl)
}

func setPVCGVK(pvc *corev1.PersistentVolumeClaim) {
	if pvc == nil {
		return
	}
	gvk := schema.GroupVersionKind{
		Group:   "",
		Kind:    "PersistentVolumeClaim",
		Version: "v1",
	}
	pvc.SetGroupVersionKind(gvk)
}
