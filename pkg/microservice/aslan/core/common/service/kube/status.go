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

package kube

import (
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
)

func GetSelectedPodsInfo(selector labels.Selector, informer informers.SharedInformerFactory, currentImages []string, workloadType string, workloadUID types.UID, log *zap.SugaredLogger) (string, string, []string) {
	pods, err := getter.ListPodsWithCache(selector, informer)
	if err != nil {
		return setting.PodError, setting.PodNotReady, nil
	}
	if workloadType == setting.Deployment {
		replicaSets, err := informer.Apps().V1().ReplicaSets().Lister().List(selector)
		if err != nil {
			return setting.PodError, setting.PodNotReady, nil
		}
		pods = filterDeploymentPodsByOwnerUID(workloadUID, replicaSets, pods)
	} else {
		pods = FilterPodsByControllerUID(pods, workloadUID)
	}

	if len(pods) == 0 {
		return setting.PodNonStarted, setting.PodNotReady, currentImages
	}

	imageSet := sets.NewString()
	for _, pod := range pods {
		ipod := wrapper.Pod(pod)
		imageSet.Insert(ipod.Containers()...)
	}
	images := imageSet.List()

	ready := setting.PodReady

	succeededPods := 0
	for _, pod := range pods {
		iPod := wrapper.Pod(pod)

		if iPod.Succeeded() {
			succeededPods++
			continue
		}

		if !iPod.Ready() {
			return setting.PodUnstable, setting.PodNotReady, images
		}
	}

	if len(pods) == succeededPods {
		return string(corev1.PodSucceeded), setting.JobReady, images
	}

	return setting.PodRunning, ready, images
}

// FilterPodsByControllerUID removes pods that only match a workload's labels
// but are controlled by another workload.
func FilterPodsByControllerUID(pods []*corev1.Pod, controllerUID types.UID) []*corev1.Pod {
	if controllerUID == "" {
		return []*corev1.Pod{}
	}
	ret := make([]*corev1.Pod, 0, len(pods))
	for _, pod := range pods {
		if pod == nil {
			continue
		}
		controller := metav1.GetControllerOf(pod)
		if controller != nil && controller.UID == controllerUID {
			ret = append(ret, pod)
		}
	}
	return ret
}

// FilterJobsByControllerUID removes jobs that have the same labels or name
// pattern but are controlled by another CronJob.
func FilterJobsByControllerUID(jobs []*batchv1.Job, controllerUID types.UID) []*batchv1.Job {
	if controllerUID == "" {
		return []*batchv1.Job{}
	}
	ret := make([]*batchv1.Job, 0, len(jobs))
	for _, job := range jobs {
		if job == nil {
			continue
		}
		controller := metav1.GetControllerOf(job)
		if controller != nil && controller.UID == controllerUID {
			ret = append(ret, job)
		}
	}
	return ret
}

// FilterDeploymentPodsByOwner validates the complete
// Pod -> ReplicaSet -> Deployment controller chain. A pod's direct controller
// is its ReplicaSet, so checking only the Deployment UID is insufficient.
func FilterDeploymentPodsByOwner(deployment *appsv1.Deployment, replicaSets []*appsv1.ReplicaSet, pods []*corev1.Pod) []*corev1.Pod {
	if deployment == nil {
		return []*corev1.Pod{}
	}
	return filterDeploymentPodsByOwnerUID(deployment.UID, replicaSets, pods)
}

func filterDeploymentPodsByOwnerUID(deploymentUID types.UID, replicaSets []*appsv1.ReplicaSet, pods []*corev1.Pod) []*corev1.Pod {
	if deploymentUID == "" {
		return []*corev1.Pod{}
	}
	ownedReplicaSets := sets.NewString()
	for _, replicaSet := range replicaSets {
		if replicaSet == nil {
			continue
		}
		controller := metav1.GetControllerOf(replicaSet)
		if controller != nil && controller.UID == deploymentUID {
			ownedReplicaSets.Insert(string(replicaSet.UID))
		}
	}

	ret := make([]*corev1.Pod, 0, len(pods))
	for _, pod := range pods {
		if pod == nil {
			continue
		}
		controller := metav1.GetControllerOf(pod)
		if controller != nil && ownedReplicaSets.Has(string(controller.UID)) {
			ret = append(ret, pod)
		}
	}
	return ret
}
