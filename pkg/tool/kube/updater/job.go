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

package updater

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DeleteJobs(ns string, selector labels.Selector, cl client.Client) error {
	return deleteObjectsWithDefaultOptions(ns, selector, &batchv1.Job{}, cl)
}

func CreateJob(job *batchv1.Job, cl client.Client) error {
	return createObject(job, cl)
}

func DeleteJob(ns, name string, cl client.Client) error {
	return deleteObjectWithDefaultOptions(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}, cl)
}

func DeleteJobAndWait(ns, name string, cl client.Client) error {
	return deleteObjectAndWait(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}, cl)
}

func DeleteJobsAndWait(ns string, selector labels.Selector, cl client.Client) error {
	gvk := schema.GroupVersionKind{
		Group:   "batch",
		Kind:    "Job",
		Version: "v1",
	}
	return deleteObjectsAndWait(ns, selector, &batchv1.Job{}, gvk, cl)
}
