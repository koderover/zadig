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
	"reflect"

	corev1 "k8s.io/api/core/v1"
)

func CheckEphemeralContainerFieldExist(podSpec *corev1.PodSpec) bool {
	metaValue := reflect.ValueOf(podSpec).Elem()
	return metaValue.FieldByName("EphemeralContainers") != (reflect.Value{})
}

func CheckEphemeralContainerStatusFieldExist(podStatus *corev1.PodStatus) bool {
	metaValue := reflect.ValueOf(podStatus).Elem()
	return metaValue.FieldByName("EphemeralContainerStatuses") != (reflect.Value{})
}
