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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/util"
)

// job is the wrapper for batchv1.Job type.
type job struct {
	*batchv1.Job
}

func Job(w *batchv1.Job) *job {
	if w == nil {
		return nil
	}

	return &job{
		Job: w,
	}
}

// Unwrap returns the batchv1.Job object.
func (w *job) Unwrap() *batchv1.Job {
	return w.Job
}

func (w *job) Complete() bool {
	for _, c := range w.Status.Conditions {
		if c.Type == batchv1.JobComplete {
			return c.Status == corev1.ConditionTrue
		}
	}

	return false
}

func (w *job) Resource() *resource.Job {
	return &resource.Job{
		Name:       w.Name,
		Age:        util.Age(w.CreationTimestamp.Unix()),
		CreateTime: w.CreationTimestamp.Unix(),
		Labels:     w.Labels,
		Active:     w.Status.Active,
		Succeeded:  w.Status.Succeeded,
		Failed:     w.Status.Failed,
		Containers: append(w.Spec.Template.Spec.Containers, w.Spec.Template.Spec.InitContainers...),
	}
}

func (job *job) ImageInfos() (images []string) {
	for _, v := range job.Spec.Template.Spec.Containers {
		images = append(images, v.Image)
	}
	return
}

func (job *job) GetDuration() string {
	if job.Status.StartTime != nil && job.Status.CompletionTime != nil {
		if job.Status.CompletionTime != nil {
			return duration.HumanDuration(job.Status.CompletionTime.Sub(job.Status.StartTime.Time))
		} else {
			return duration.HumanDuration(time.Now().Sub(job.Status.StartTime.Time))
		}
	}
	return ""
}

func (job *job) GetAge() string {
	return duration.HumanDuration(time.Now().Sub(job.CreationTimestamp.Time))
}

func (job *job) WorkloadResource(pods []*corev1.Pod) *resource.Workload {
	wl := &resource.Workload{
		Name:     job.Name,
		Type:     setting.Job,
		Replicas: job.Status.Active,
		Pods:     make([]*resource.Pod, 0, len(pods)),
	}

	for _, c := range job.Spec.Template.Spec.Containers {
		wl.Images = append(wl.Images, resource.ContainerImage{Name: c.Name, Image: c.Image})
	}

	for _, p := range pods {
		wl.Pods = append(wl.Pods, Pod(p).Resource())
	}
	return wl
}
