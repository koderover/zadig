/*
Copyright 2023 The KodeRover Authors.

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

	"github.com/koderover/zadig/pkg/shared/kube/resource"
)

// deployment is the wrapper for appsv1.Deployment type.
type daemonset struct {
	*appsv1.DaemonSet
}

func Daemenset(d *appsv1.DaemonSet) *daemonset {
	if d == nil {
		return nil
	}

	return &daemonset{
		DaemonSet: d,
	}
}

// Unwrap returns the appsv1.DaemonSet object.
func (d *daemonset) Unwrap() *appsv1.DaemonSet {
	return d.DaemonSet
}

func (d *daemonset) ImageInfos() (images []string) {
	for _, v := range d.Spec.Template.Spec.Containers {
		images = append(images, v.Image)
	}
	return
}

func (d *daemonset) WorkloadResource(pods []*corev1.Pod) *resource.Workload {
	wl := &resource.Workload{
		Name:     d.Name,
		Type:     "DaemonSet",
		Replicas: d.Status.NumberReady,
		Pods:     make([]*resource.Pod, 0, len(pods)),
	}

	for _, c := range d.Spec.Template.Spec.Containers {
		wl.Images = append(wl.Images, resource.ContainerImage{Name: c.Name, Image: c.Image})
	}

	for _, p := range pods {
		wl.Pods = append(wl.Pods, Pod(p).Resource())
	}
	return wl
}
