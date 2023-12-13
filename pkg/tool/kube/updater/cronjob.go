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
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/tool/kube/util"
)

func DeleteCronJobs(namespace string, selector labels.Selector, clientset *kubernetes.Clientset) error {
	deletePolicy := metav1.DeletePropagationForeground
	err := clientset.BatchV1().CronJobs(namespace).DeleteCollection(
		context.TODO(),
		metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		},
		metav1.ListOptions{
			LabelSelector: selector.String(),
		},
	)

	return util.IgnoreNotFoundError(err)
}

func CreateOrPatchCronJob(cj client.Object, cl client.Client) error {
	return createOrPatchObject(cj, cl)
}

func PatchCronJob(ns, name string, patchBytes []byte, cl client.Client, versionLessThan121 bool) error {
	if versionLessThan121 {
		return patchObject(&batchv1beta1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		}, patchBytes, cl)
	}
	return patchObject(&batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}, patchBytes, cl)
}

func UpdateCronJobImage(ns, name, container, image string, cl client.Client, versionLessThan121 bool) error {
	patchBytes := []byte(fmt.Sprintf(`{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"containers":[{"name":"%s","image":"%s"}]}}}}}}`, container, image))
	return PatchCronJob(ns, name, patchBytes, cl, versionLessThan121)
}

func SuspendCronJob(ns, name string, cl client.Client, versionLessThan121 bool) error {
	patchBytes := []byte(`{"spec":{"suspend":true}}`)
	return PatchCronJob(ns, name, patchBytes, cl, versionLessThan121)
}

func ResumeCronJob(ns, name string, cl client.Client, versionLessThan121 bool) error {
	patchBytes := []byte(`{"spec":{"suspend":false}}`)
	return PatchCronJob(ns, name, patchBytes, cl, versionLessThan121)
}
