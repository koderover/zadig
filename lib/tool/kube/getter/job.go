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

package getter

import (
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ListJobs(ns string, selector labels.Selector, cl client.Client) ([]*batchv1.Job, error) {
	jobs := &batchv1.JobList{}
	err := ListResourceInCache(ns, selector, nil, jobs, cl)
	if err != nil {
		return nil, err
	}

	var res []*batchv1.Job
	for i := range jobs.Items {
		res = append(res, &jobs.Items[i])
	}
	return res, err
}

func GetJob(ns, name string, cl client.Client) (*batchv1.Job, bool, error) {
	g := &batchv1.Job{}
	found, err := GetResourceInCache(ns, name, g, cl)
	if err != nil || !found {
		g = nil
	}

	return g, found, err
}
