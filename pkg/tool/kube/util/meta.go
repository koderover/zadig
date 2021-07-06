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

package util

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func GetGroupVersionKind(obj runtime.Object) (*schema.GroupVersionKind, error) {
	t, err := meta.TypeAccessor(obj)
	if err != nil {
		return nil, err
	}

	gvk := schema.FromAPIVersionAndKind(t.GetAPIVersion(), t.GetKind())

	return &gvk, nil
}

func SetGroupVersionKind(obj runtime.Object, gvk *schema.GroupVersionKind) (runtime.Object, error) {
	t, err := meta.TypeAccessor(obj)
	if err != nil {
		return nil, err
	}

	t.SetAPIVersion(gvk.GroupVersion().String())
	t.SetKind(gvk.Kind)

	return obj, nil
}
