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

package wrapper

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/koderover/zadig/pkg/tool/kube/getter"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CheckEphemeralContainerFieldExist(podSpec *corev1.PodSpec) bool {
	metaValue := reflect.ValueOf(podSpec).Elem()
	return metaValue.FieldByName("EphemeralContainers") != (reflect.Value{})
}

func CheckEphemeralContainerStatusFieldExist(podStatus *corev1.PodStatus) bool {
	metaValue := reflect.ValueOf(podStatus).Elem()
	return metaValue.FieldByName("EphemeralContainerStatuses") != (reflect.Value{})
}

func CheckWorkLoadsDeployStat(kubeClient client.Client, namespace string, labelMaps []map[string]string, ownerUID string) error {
	for _, label := range labelMaps {
		selector := labels.Set(label).AsSelector()
		pods, err := getter.ListPods(namespace, selector, kubeClient)
		if err != nil {
			return err
		}
		sort.Slice(pods, func(i, j int) bool {
			return pods[i].CreationTimestamp.After(pods[j].CreationTimestamp.Time)
		})
		if len(pods) <= 0 {
			return nil
		}
		pod := pods[0]
		if !Pod(pod).IsOwnerMatched(ownerUID) {
			continue
		}
		allContainerStatuses := make([]corev1.ContainerStatus, 0)
		allContainerStatuses = append(allContainerStatuses, pod.Status.InitContainerStatuses...)
		allContainerStatuses = append(allContainerStatuses, pod.Status.ContainerStatuses...)
		for _, cs := range allContainerStatuses {
			if cs.State.Waiting != nil {
				switch cs.State.Waiting.Reason {
				case "ImagePullBackOff", "ErrImagePull", "CrashLoopBackOff", "ErrImageNeverPull":
					return fmt.Errorf("pod: %s, %s: %s", pod.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message)
				}
			}
		}
	}
	return nil
}
