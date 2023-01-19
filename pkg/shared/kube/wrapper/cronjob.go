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
	"k8s.io/apimachinery/pkg/util/duration"
)

type cronJob struct {
	*batchv1.CronJob
}

func CronJob(cj *batchv1.CronJob) *cronJob {
	if cj == nil {
		return nil
	}
	return &cronJob{
		CronJob: cj,
	}
}

func (cj *cronJob) ImageInfos() (images []string) {
	for _, v := range cj.Spec.JobTemplate.Spec.Template.Spec.Containers {
		images = append(images, v.Image)
	}
	return
}

func (cj *cronJob) GetAge() string {
	return duration.HumanDuration(time.Now().Sub(cj.CreationTimestamp.Time))
}
