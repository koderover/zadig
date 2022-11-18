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
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/util"
)

// pod is the wrapper for corev1.Pod type.
type pod struct {
	*corev1.Pod
}

func Pod(w *corev1.Pod) *pod {
	if w == nil {
		return nil
	}

	return &pod{
		Pod: w,
	}
}

// Unwrap returns the corev1.Pod object.
func (w *pod) Unwrap() *corev1.Pod {
	return w.Pod
}

// Finished means the pod is finished and closed, usually it is a job pod
func (w *pod) Finished() bool {
	p := w.phase()
	return p == corev1.PodSucceeded || p == corev1.PodFailed
}

func (w *pod) Pending() bool {
	return w.phase() == corev1.PodPending
}

// Succeeded means the pod is succeeded and closed, usually it is a job pod
func (w *pod) Succeeded() bool {
	return w.phase() == corev1.PodSucceeded
}

// Failed means the pod is failed and closed, usually it is a job pod
func (w *pod) Failed() bool {
	return w.phase() == corev1.PodFailed
}

func (w *pod) phase() corev1.PodPhase {
	return w.Pod.Status.Phase
}

func (w *pod) Phase() string {
	return string(w.phase())
}

// Ready indicates that the pod is ready for traffic.
func (w *pod) Ready() bool {
	cs := w.Status.Conditions
	for _, c := range cs {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// For a Pod that uses custom conditions, that Pod is evaluated to be ready only when both the following statements apply:
// All containers in the Pod are ready.
// All conditions specified in readinessGates are True.
// When a Pod's containers are Ready but at least one custom condition is missing or False, the kubelet sets the Pod's condition to ContainersReady.
// https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-readiness-status
func (w *pod) ContainersReady() (bool, string) {
	cs := w.Status.Conditions

	for _, c := range cs {
		if c.Type == corev1.ContainersReady {
			if c.Status == corev1.ConditionTrue {
				return true, ""
			} else {
				return false, c.Message
			}
		}
	}
	return false, ""
}

func (w *pod) Containers() []string {
	var cs []string
	for _, c := range w.Spec.Containers {
		cs = append(cs, c.Image)
	}

	return cs
}

func (w *pod) ContainerNames() []string {
	var cs []string
	for _, c := range w.Spec.Containers {
		cs = append(cs, c.Name)
	}

	return cs
}

func (w *pod) Resource() *resource.Pod {
	containersReady, containersMessage := w.ContainersReady()
	p := &resource.Pod{
		Name:              w.Name,
		Status:            string(w.Status.Phase),
		Age:               util.Age(w.CreationTimestamp.Unix()),
		CreateTime:        w.CreationTimestamp.Unix(),
		IP:                w.Status.PodIP,
		Labels:            w.Labels,
		PodReady:          w.Ready(),
		ContainersReady:   containersReady,
		ContainersMessage: containersMessage,
		ContainerStatuses: []resource.Container{},
		NodeName:          w.Spec.NodeName,
		HostIP:            w.Status.HostIP,
		Succeed:           w.Succeeded(),
		Ready:             w.Ready(),
	}
	if len(w.OwnerReferences) > 0 {
		p.Kind = w.OwnerReferences[0].Kind
	}

	containersStatus := []corev1.ContainerStatus{}
	containersStatus = append(containersStatus, w.Status.ContainerStatuses...)
	if CheckEphemeralContainerStatusFieldExist(&w.Status) {
		for _, status := range w.Status.EphemeralContainerStatuses {
			status.Ready = true
			containersStatus = append(containersStatus, status)
		}
	}

	for _, container := range containersStatus {
		cs := resource.Container{
			Name:         container.Name,
			RestartCount: container.RestartCount,
			Ready:        container.Ready,
		}

		if container.State.Running != nil {
			cs.Status = "running"
			cs.StartedAt = container.State.Running.StartedAt.Unix()
		}

		if container.State.Waiting != nil {
			cs.Status = "waiting"
			cs.Message = container.State.Waiting.Message
			cs.Reason = container.State.Waiting.Reason
		}

		if container.State.Terminated != nil {
			cs.Status = "terminated"
			cs.Message = container.State.Terminated.Message

			if container.State.Terminated.ExitCode != 0 {
				cs.Message += fmt.Sprintf("exit code (%d)", container.State.Terminated.ExitCode)
			}

			cs.Reason = container.State.Terminated.Reason
			cs.StartedAt = container.State.Terminated.StartedAt.Unix()
			cs.FinishedAt = container.State.Terminated.FinishedAt.Unix()
		}

		// 如果镜像hash一致，但是tag不同，kube list pod会随机拿一个容器和镜像名称返回，会导致跟实际更新的镜像tag不一致
		// 暂时用 w.Spec.Containers 来获取最新的镜像名称
		// TODO: 问题未修复
		for _, specContainer := range w.Spec.Containers {
			if specContainer.Name == container.Name {
				cs.Image = specContainer.Image
				break
			}
		}

		if CheckEphemeralContainerFieldExist(&w.Spec) {
			for _, specContainer := range w.Spec.EphemeralContainers {
				if specContainer.Name == container.Name {
					cs.Image = specContainer.Image
					break
				}
			}
		}
		p.ContainerStatuses = append(p.ContainerStatuses, cs)
	}

	// Note: Seems that in K8s versions [v1.16, v1.22], EphemeralContainerStatuses exist but are empty while in K8s versions
	// [v1.23, ], EphemeralContainerStatuses exist and are not empty.
	if CheckEphemeralContainerStatusFieldExist(&w.Status) && len(w.Status.EphemeralContainerStatuses) == 0 &&
		CheckEphemeralContainerFieldExist(&w.Spec) {
		for _, container := range w.Spec.EphemeralContainers {
			cs := resource.Container{
				Name:         container.Name,
				Image:        container.Image,
				RestartCount: 0,
				Status:       "running",
				Ready:        true,
			}
			p.ContainerStatuses = append(p.ContainerStatuses, cs)
		}
	}

	if w.DeletionTimestamp != nil {
		p.Status = "Terminating"
	}

	if p.Status == "Running" {
		for _, status := range p.ContainerStatuses {
			if status.Status != "running" {
				p.Status = "Unstable"
				break
			}
		}
	}

	if CheckEphemeralContainerFieldExist(&w.Spec) && len(w.Spec.EphemeralContainers) > 0 {
		p.EnableDebugContainer = true
	}

	return p
}
