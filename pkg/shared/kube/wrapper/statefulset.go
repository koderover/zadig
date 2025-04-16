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
	"github.com/koderover/zadig/v2/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/kube/resource"
)

// statefulSet is the wrapper for appsv1.StatefulSet type.
type statefulSet struct {
	*appsv1.StatefulSet
}

func StatefulSet(w *appsv1.StatefulSet) *statefulSet {
	if w == nil {
		return nil
	}

	return &statefulSet{
		StatefulSet: w,
	}
}

// Unwrap returns the appsv1.StatefulSet object.
func (w *statefulSet) Unwrap() *appsv1.StatefulSet {
	return w.StatefulSet
}

func (w *statefulSet) Ready() bool {
	return w.Status.Replicas == w.Status.AvailableReplicas
}

func (w *statefulSet) WorkloadResource(pods []*corev1.Pod) *resource.Workload {
	wl := &resource.Workload{
		Name:     w.Name,
		Type:     setting.StatefulSet,
		Replicas: *w.Spec.Replicas,
		Pods:     make([]*resource.Pod, 0, len(pods)),
	}

	// Note: EphemeralContainers only belong to Pods and do not exist in Deployment Spec.
	for _, c := range w.Spec.Template.Spec.Containers {
		wl.Images = append(wl.Images, resource.ContainerImage{Name: c.Name, Image: c.Image, ImageName: util.ExtractImageName(c.Image)})
	}

	for _, p := range pods {
		wl.Pods = append(wl.Pods, Pod(p).Resource())
	}

	return wl
}

func (w *statefulSet) Images() (images []string) {
	for _, v := range w.Spec.Template.Spec.Containers {
		images = append(images, v.Name)
	}
	return
}

func (w *statefulSet) ImageInfos() (images []string) {
	for _, v := range w.Spec.Template.Spec.Containers {
		images = append(images, v.Image)
	}
	return
}

func (w *statefulSet) GetKind() string {
	return w.Kind
}

func (w *statefulSet) GetContainers() []*resource.ContainerImage {
	containers := make([]*resource.ContainerImage, 0, len(w.Spec.Template.Spec.Containers))
	for _, c := range w.Spec.Template.Spec.Containers {
		containers = append(containers, &resource.ContainerImage{Name: c.Name, Image: c.Image, ImageName: util.ExtractImageName(c.Image)})
	}
	return containers
}
