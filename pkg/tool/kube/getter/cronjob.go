/*
Copyright 2022 The KodeRover Authors.

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
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var CronJobGVK = schema.GroupVersionKind{
	Group:   "batch",
	Kind:    "CronJob",
	Version: "v1",
}

func ListCronJobs(ns string, selector labels.Selector, cl client.Client) ([]*batchv1.CronJob, error) {
	cjs := &batchv1.CronJobList{}
	err := ListResourceInCache(ns, selector, nil, cjs, cl)
	if err != nil {
		return nil, err
	}

	var res []*batchv1.CronJob
	for i := range cjs.Items {
		res = append(res, &cjs.Items[i])
	}
	return res, err
}

func ListCronJobsV1Beta(ns string, selector labels.Selector, cl client.Client) ([]*batchv1beta1.CronJob, error) {
	cjs := &batchv1beta1.CronJobList{}
	err := ListResourceInCache(ns, selector, nil, cjs, cl)
	if err != nil {
		return nil, err
	}

	var res []*batchv1beta1.CronJob
	for i := range cjs.Items {
		res = append(res, &cjs.Items[i])
	}
	return res, err
}

func GetCronJobYaml(ns, name string, cl client.Client, versionLessThan121 bool) ([]byte, bool, error) {
	gvk := CronJobGVK
	bytes, existed, err := GetResourceYamlInCache(ns, name, gvk, cl)
	if !versionLessThan121 {
		return bytes, existed, err
	}

	if existed && err == nil {
		return bytes, existed, err
	}

	gvk = schema.GroupVersionKind{
		Group:   "batch",
		Kind:    "CronJob",
		Version: "v1beta1",
	}
	return GetResourceYamlInCache(ns, name, gvk, cl)
}
