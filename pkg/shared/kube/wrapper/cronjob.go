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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/util"
)

type CronJobItem interface {
	GetName() string
	ImageInfos() []string
	GetSchedule() string
	GetSuspend() bool
	GetActive() int
	GetLastSchedule() string
	GetAge() string
	GetCreationTime() time.Time
	GetAnnotations() map[string]string
}

type cronJob struct {
	*batchv1.CronJob
	CronJobBeta *v1beta1.CronJob
}

func CronJob(cj *batchv1.CronJob, cjBeta *v1beta1.CronJob) *cronJob {
	if cj == nil && cjBeta == nil {
		return nil
	}
	return &cronJob{
		CronJob:     cj,
		CronJobBeta: cjBeta,
	}
}

func (cj *cronJob) CronJobResource() *resource.CronJob {
	return &resource.CronJob{
		Name:         cj.GetName(),
		Labels:       cj.GetLabels(),
		Images:       cj.GetContainers(),
		CreateTime:   cj.GetCreationTime().Unix(),
		Suspend:      cj.GetSuspend(),
		Active:       cj.GetActive(),
		Schedule:     cj.GetSchedule(),
		LastSchedule: cj.GetLastSchedule(),
	}
}

func (cj *cronJob) GetName() string {
	if cj.CronJob != nil {
		return cj.Name
	}
	return cj.CronJobBeta.Name
}

func (cj *cronJob) GetLabels() map[string]string {
	if cj.CronJob != nil {
		return cj.CronJob.GetLabels()
	}
	return cj.CronJobBeta.GetLabels()
}

func (cj *cronJob) GetAnnotations() map[string]string {
	if cj.CronJob != nil {
		return cj.CronJob.GetAnnotations()
	}
	return cj.CronJobBeta.GetAnnotations()
}

func (cj *cronJob) ImageInfos() (images []string) {
	if cj.CronJob != nil {
		for _, v := range cj.CronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
			images = append(images, v.Image)
		}
	} else {
		for _, v := range cj.CronJobBeta.Spec.JobTemplate.Spec.Template.Spec.Containers {
			images = append(images, v.Image)
		}
	}
	return
}

func (cj *cronJob) GetContainers() []*resource.ContainerImage {
	containers := make([]*resource.ContainerImage, 0)
	if cj.CronJob != nil {
		for _, c := range cj.CronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
			containers = append(containers, &resource.ContainerImage{Name: c.Name, Image: c.Image, ImageName: util.ExtractImageName(c.Image)})
		}
	} else {
		for _, c := range cj.CronJobBeta.Spec.JobTemplate.Spec.Template.Spec.Containers {
			containers = append(containers, &resource.ContainerImage{Name: c.Name, Image: c.Image, ImageName: util.ExtractImageName(c.Image)})
		}
	}
	return containers
}

func (cj *cronJob) GetAge() string {
	if cj.CronJob != nil {
		return duration.HumanDuration(time.Now().Sub(cj.CreationTimestamp.Time))
	} else {
		return duration.HumanDuration(time.Now().Sub(cj.CronJobBeta.CreationTimestamp.Time))
	}
}

func (cj *cronJob) GetSchedule() string {
	if cj.CronJob != nil {
		return cj.Spec.Schedule
	}
	return cj.CronJobBeta.Spec.Schedule
}

func (cj *cronJob) GetSuspend() bool {
	if cj.CronJob != nil {
		return util.GetBoolFromPointer(cj.Spec.Suspend)
	}
	return util.GetBoolFromPointer(cj.CronJobBeta.Spec.Suspend)
}

func (cj *cronJob) GetActive() int {
	if cj.CronJob != nil {
		return len(cj.Status.Active)
	}
	return len(cj.CronJobBeta.Status.Active)
}

func (cj *cronJob) GetLastSchedule() string {
	if cj.CronJob != nil {
		if cj.CronJob.Status.LastScheduleTime != nil {
			return cj.Status.LastScheduleTime.String()
		}
	} else if cj.CronJobBeta.Status.LastScheduleTime != nil {
		return cj.CronJobBeta.Status.LastScheduleTime.String()
	}
	return ""
}

func (cj *cronJob) GetCreationTime() time.Time {
	if cj.CronJob != nil {
		return cj.CreationTimestamp.Time
	}
	return cj.CronJobBeta.CreationTimestamp.Time
}
