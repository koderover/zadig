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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubectl/pkg/util"
)

var metadataAccessor = meta.NewAccessor()

func CreateApplyAnnotation(obj runtime.Object) error {
	return util.CreateApplyAnnotation(obj, unstructured.UnstructuredJSONScheme)
}

func GetModifiedConfiguration(obj runtime.Object) ([]byte, error) {
	return util.GetModifiedConfiguration(obj, true, unstructured.UnstructuredJSONScheme)
}

// GetOriginalConfiguration retrieves the original configuration of the object
// from the annotation, or nil if no annotation was found.
func GetOriginalConfiguration(obj runtime.Object) ([]byte, error) {
	annots, err := metadataAccessor.Annotations(obj)
	if err != nil {
		return nil, err
	}

	if annots == nil {
		return nil, nil
	}

	original, ok := annots[corev1.LastAppliedConfigAnnotation]
	if !ok {
		return nil, nil
	}

	return []byte(original), nil
}
