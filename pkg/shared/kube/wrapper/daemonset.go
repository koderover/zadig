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

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/kube/resource"
	"github.com/koderover/zadig/v2/pkg/util"
)

// daemonSet is the wrapper for appsv1.DaemonSet type.
type daemonSet struct {
	*appsv1.DaemonSet
}

func DaemonSet(d *appsv1.DaemonSet) *daemonSet {
	if d == nil {
		return nil
	}

	return &daemonSet{
		DaemonSet: d,
	}
}

// Unwrap returns the appsv1.DaemonSet object.
func (d *daemonSet) Unwrap() *appsv1.DaemonSet {
	return d.DaemonSet
}

func (d *daemonSet) Ready() bool {
	return d.Status.ObservedGeneration >= d.Generation &&
		d.Status.DesiredNumberScheduled == d.Status.UpdatedNumberScheduled &&
		d.Status.DesiredNumberScheduled == d.Status.NumberReady &&
		d.Status.NumberUnavailable == 0
}

func (d *daemonSet) ImageInfos() (images []string) {
	for _, v := range d.Spec.Template.Spec.Containers {
		images = append(images, v.Image)
	}
	return
}

func (d *daemonSet) WorkloadResource(pods []*corev1.Pod) *resource.Workload {
	wl := &resource.Workload{
		Name:     d.Name,
		Type:     setting.DaemonSet,
		Replicas: d.Status.NumberReady,
		Pods:     make([]*resource.Pod, 0, len(pods)),
	}

	for _, c := range d.Spec.Template.Spec.Containers {
		wl.Images = append(wl.Images, resource.ContainerImage{Name: c.Name, Image: c.Image, ImageName: util.ExtractImageName(c.Image)})
	}

	for _, p := range pods {
		wl.Pods = append(wl.Pods, Pod(p).Resource())
	}
	return wl
}

func (d *daemonSet) Images() (images []string) {
	for _, v := range d.Spec.Template.Spec.Containers {
		images = append(images, v.Name)
	}
	return
}

func (d *daemonSet) GetKind() string {
	return d.Kind
}

func (d *daemonSet) GetContainers() []*resource.ContainerImage {
	containers := make([]*resource.ContainerImage, 0, len(d.Spec.Template.Spec.Containers))
	for _, c := range d.Spec.Template.Spec.Containers {
		containers = append(containers, &resource.ContainerImage{Name: c.Name, Image: c.Image, ImageName: util.ExtractImageName(c.Image)})
	}
	return containers
}
