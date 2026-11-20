/*
Copyright 2026 The KodeRover Authors.

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

package service

import (
	"go.uber.org/zap"
	"k8s.io/client-go/informers"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalresource "github.com/koderover/zadig/v2/pkg/shared/kube/resource"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
)

func GetServiceRuntimeResources(resources []*commonmodels.ServiceResource, namespace string, cronJobVersionLessThan121 *bool, informer informers.SharedInformerFactory, logger *zap.SugaredLogger) ([]*internalresource.Workload, []*internalresource.CronJob) {
	workloads := make([]*internalresource.Workload, 0)
	cronJobs := make([]*internalresource.CronJob, 0)
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		switch resource.Kind {
		case setting.Deployment:
			deployment, err := getter.GetDeploymentByNameWithCache(resource.Name, namespace, informer)
			if err != nil {
				logger.Warnf("failed to get deployment %s/%s: %s", namespace, resource.Name, err)
				continue
			}
			if workload := GetDeploymentWorkloadResource(deployment, informer, logger); workload != nil {
				workloads = append(workloads, workload)
			}
		case setting.DaemonSet:
			daemonSet, err := getter.GetDaemonSetByNameWithCache(resource.Name, namespace, informer)
			if err != nil {
				logger.Warnf("failed to get daemonset %s/%s: %s", namespace, resource.Name, err)
				continue
			}
			if workload := GetDaemonSetWorkloadResource(daemonSet, informer, logger); workload != nil {
				workloads = append(workloads, workload)
			}
		case setting.StatefulSet:
			statefulSet, err := getter.GetStatefulSetByNameWWithCache(resource.Name, namespace, informer)
			if err != nil {
				logger.Warnf("failed to get statefulset %s/%s: %s", namespace, resource.Name, err)
				continue
			}
			if workload := getStatefulSetWorkloadResource(statefulSet, informer, logger); workload != nil {
				workloads = append(workloads, workload)
			}
		case setting.Job:
			job, err := getter.GetJobByNameWithCache(resource.Name, namespace, informer)
			if err != nil {
				logger.Warnf("failed to get job %s/%s: %s", namespace, resource.Name, err)
				continue
			}
			if workload := getJobWorkloadResource(job, informer, logger); workload != nil {
				workloads = append(workloads, workload)
			}
		case setting.CronJob:
			if cronJobVersionLessThan121 == nil {
				continue
			}
			cronJob, cronJobBeta, err := getter.GetCronJobByNameWithCache(resource.Name, namespace, informer, *cronJobVersionLessThan121)
			if err != nil {
				logger.Warnf("failed to get cronjob %s/%s: %s", namespace, resource.Name, err)
				continue
			}
			if cronJobResource := getCronJobWorkLoadResource(cronJob, cronJobBeta, informer, logger); cronJobResource != nil {
				cronJobs = append(cronJobs, cronJobResource)
			}
		}
	}
	return workloads, cronJobs
}
