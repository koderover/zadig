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

// namespace is the wrapper for corev1.Namespace type.
type namespace struct {
	*corev1.Namespace
}

func Namespace(w *corev1.Namespace) *namespace {
	if w == nil {
		return nil
	}

	return &namespace{
		Namespace: w,
	}
}

// Unwrap returns the corev1.Namespace object.
func (w *namespace) Unwrap() *corev1.Namespace {
	return w.Namespace
}

func (w *namespace) phase() corev1.NamespacePhase {
	return w.Status.Phase
}

func (w *namespace) Terminating() bool {
	return w.phase() == corev1.NamespaceTerminating
}

func (w *namespace) Resource() *resource.Namespace {
	return &resource.Namespace{
		Name:   w.Name,
		Status: string(w.Status.Phase),
		Age:    util.Age(w.CreationTimestamp.Unix()),
		Labels: w.Labels,
	}
}
