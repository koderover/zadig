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
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var CronJobGVK = schema.GroupVersionKind{
	Group:   "batch",
	Kind:    "CronJob",
	Version: "v1",
}

var CronJobV1BetaGVK = schema.GroupVersionKind{
	Group:   "batch",
	Kind:    "CronJob",
	Version: "v1beta1",
}

func ListCronJobsWithCache(selector labels.Selector, lister informers.SharedInformerFactory, versionLessThan121 bool) ([]*batchv1.CronJob, []*batchv1beta1.CronJob, error) {
	if selector == nil {
		selector = labels.NewSelector()
	}
	var cronJobs []*batchv1.CronJob
	var cronJobsBetas []*batchv1beta1.CronJob
	var err error
	if !versionLessThan121 {
		cronJobs, err = lister.Batch().V1().CronJobs().Lister().List(selector)
	} else {
		cronJobsBetas, err = lister.Batch().V1beta1().CronJobs().Lister().List(selector)
	}
	return cronJobs, cronJobsBetas, err
}

func ListCronJobs(ns string, selector labels.Selector, cl client.Client, versionLessThan121 bool) ([]*batchv1.CronJob, []*batchv1beta1.CronJob, error) {
	var cronJobs []*batchv1.CronJob
	var cronJobsBetas []*batchv1beta1.CronJob
	var err error
	if !versionLessThan121 {
		cronJobs, err = ListCronJobsV1(ns, selector, cl)
	} else {
		cronJobsBetas, err = ListCronJobsV1Beta(ns, selector, cl)
	}
	return cronJobs, cronJobsBetas, err
}

func ListCronJobsV1(ns string, selector labels.Selector, cl client.Client) ([]*batchv1.CronJob, error) {
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

// GetCronJobYaml if k8s version higher than 1.21, only batch/v1 is supported, or we will fetch batch/v1beta1
func GetCronJobYaml(ns, name string, cl client.Client, versionLessThan121 bool) ([]byte, bool, error) {
	gvk := CronJobGVK
	bytes, existed, err := GetResourceYamlInCache(ns, name, gvk, cl)
	if !versionLessThan121 {
		return bytes, existed, err
	}

	if existed && err == nil {
		return bytes, existed, nil
	}

	return GetResourceYamlInCache(ns, name, CronJobV1BetaGVK, cl)
}

func GetCronJob(ns, name string, cl client.Client, versionLessThan121 bool) (*batchv1.CronJob, *batchv1beta1.CronJob, bool, error) {
	cron := &batchv1.CronJob{}
	cronBeta := &batchv1beta1.CronJob{}

	if !versionLessThan121 {
		existed, err := GetResourceInCache(ns, name, cron, cl)
		cron.SetGroupVersionKind(CronJobGVK)
		return cron, nil, existed, err
	}

	existed, err := GetResourceInCache(ns, name, cronBeta, cl)
	cronBeta.SetGroupVersionKind(CronJobGVK)
	return nil, cronBeta, existed, err

}
