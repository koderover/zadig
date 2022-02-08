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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
)

func GetSelectedPodsInfo(selector labels.Selector, informer informers.SharedInformerFactory, log *zap.SugaredLogger) (string, string, []string) {
	pods, err := getter.ListPodsWithCache(selector, informer)
	if err != nil {
		return setting.PodError, setting.PodNotReady, nil
	}

	if len(pods) == 0 {
		return setting.PodNonStarted, setting.PodNotReady, nil
	}

	imageSet := sets.String{}
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
