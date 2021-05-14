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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/koderover/zadig/lib/internal/kube/resource"
	"github.com/koderover/zadig/lib/setting"
)

// deployment is the wrapper for appsv1.Deployment type.
type deployment struct {
	*appsv1.Deployment
}

func Deployment(w *appsv1.Deployment) *deployment {
	if w == nil {
		return nil
	}

	return &deployment{
		Deployment: w,
	}
}

// Unwrap returns the appsv1.Deployment object.
func (w *deployment) Unwrap() *appsv1.Deployment {
	return w.Deployment
}

func (w *deployment) Ready() bool {
	return w.Status.Replicas == w.Status.AvailableReplicas
}

func (w *deployment) WorkloadResource(pods []*corev1.Pod) *resource.Workload {
	wl := &resource.Workload{
		Name:     w.Name,
		Type:     setting.Deployment,
		Replicas: *w.Spec.Replicas,
		Pods:     make([]*resource.Pod, 0, len(pods)),
	}

	for _, c := range w.Spec.Template.Spec.Containers {
		wl.Images = append(wl.Images, resource.ContainerImage{Name: c.Name, Image: c.Image})
	}

	for _, p := range pods {
		wl.Pods = append(wl.Pods, Pod(p).Resource())
	}

	return wl
}
