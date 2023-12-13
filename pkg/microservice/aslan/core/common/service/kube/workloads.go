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

package kube

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/client-go/kubernetes"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func FetchSelectedWorkloads(namespace string, Resource []*WorkloadResource, kubeclient crClient.Client, clientSet *kubernetes.Clientset) ([]*appsv1.Deployment, []*appsv1.StatefulSet,
	[]*batchv1.CronJob, []*batchv1beta1.CronJob, error) {
	var deployments []*appsv1.Deployment
	var statefulSets []*appsv1.StatefulSet
	var cronJobs []*batchv1.CronJob
	var betaCronJobs []*batchv1beta1.CronJob

	k8sServerVersion, err := clientSet.Discovery().ServerVersion()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	for _, item := range Resource {
		switch item.Type {
		case setting.Deployment:
			deploy, deployExists, err := getter.GetDeployment(namespace, item.Name, kubeclient)
			if deployExists && err == nil {
				deployments = append(deployments, deploy)
			}
			if err != nil {
				log.Errorf("failed to fetch deployment %s, error: %v", item.Name, err)
			}
		case setting.StatefulSet:
			sts, stsExists, err := getter.GetStatefulSet(namespace, item.Name, kubeclient)
			if stsExists && err == nil {
				statefulSets = append(statefulSets, sts)
			}
			if err != nil {
				log.Errorf("failed to fetch statefulset %s, error: %v", item.Name, err)
			}
		case setting.CronJob:
			cronjob, cronjobBeta, cronjobExists, err := getter.GetCronJob(namespace, item.Name, kubeclient, client.VersionLessThan121(k8sServerVersion))
			if err != nil {
				log.Errorf("failed to fetch cronjob %s, error: %v", item.Name, err)
			}
			if cronjob != nil && cronjobExists {
				cronJobs = append(cronJobs, cronjob)
			}
			if cronjobBeta != nil && cronjobExists {
				betaCronJobs = append(betaCronJobs, cronjobBeta)
			}
		}
	}
	return deployments, statefulSets, cronJobs, betaCronJobs, nil
}
