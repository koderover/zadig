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

	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/util/duration"
)

type cronJobV1Beta struct {
	*batchv1beta1.CronJob
}

func CronJobV1Beta(cj *batchv1beta1.CronJob) *cronJobV1Beta {
	if cj == nil {
		return nil
	}
	return &cronJobV1Beta{
		CronJob: cj,
	}
}

func (cj *cronJobV1Beta) ImageInfos() (images []string) {
	for _, v := range cj.Spec.JobTemplate.Spec.Template.Spec.Containers {
		images = append(images, v.Image)
	}
	return
}

func (cj *cronJobV1Beta) GetAge() string {
	return duration.HumanDuration(time.Now().Sub(cj.CreationTimestamp.Time))
}
