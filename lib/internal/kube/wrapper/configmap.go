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

package wrapper

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/koderover/zadig/lib/internal/kube/resource"
	"github.com/koderover/zadig/lib/util"
)

// configMap is the wrapper for corev1.ConfigMap type.
type configMap struct {
	*corev1.ConfigMap
}

func ConfigMap(w *corev1.ConfigMap) *configMap {
	if w == nil {
		return nil
	}

	return &configMap{
		ConfigMap: w,
	}
}

// Unwrap returns the corev1.ConfigMap object.
func (w *configMap) Unwrap() *corev1.ConfigMap {
	return w.ConfigMap
}

func (w *configMap) Resource() *resource.ConfigMap {
	return &resource.ConfigMap{
		Name:       w.Name,
		Age:        util.Age(w.CreationTimestamp.Unix()),
		Labels:     w.Labels,
		Data:       w.Data,
		CreateTime: w.CreationTimestamp.Unix(),
	}
}
