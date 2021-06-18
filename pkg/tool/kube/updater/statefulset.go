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
	"bytes"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PatchStatefulSet(ns, name string, patchBytes []byte, cl client.Client) error {
	return patchObject(&appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}, patchBytes, cl)
}

// TODO: LOU: it is not the right way to restart a statefulSet, since it is a hack and it
// will generate a new revision which will pollute the revision history.
func RestartStatefulSet(ns, name string, cl client.Client) error {
	now := time.Now().Format(time.RFC3339Nano)
	payload := bytes.NewBufferString("")
	_ = restartPatchTemplate.Execute(payload, struct {
		Time string
	}{now})

	if err := PatchStatefulSet(ns, name, payload.Bytes(), cl); err != nil {
		return fmt.Errorf("failed to restart %s/deploy/%s: %v", ns, name, err)
	}

	return nil
}

func DeleteStatefulSets(ns string, selector labels.Selector, cl client.Client) error {
	return deleteObjectsWithDefaultOptions(ns, selector, &appsv1.StatefulSet{}, cl)
}

func UpdateStatefulSetImage(ns, name, container, image string, cl client.Client) error {
	patchBytes := []byte(fmt.Sprintf(`{"spec":{"template":{"spec":{"containers":[{"name":"%s","image":"%s"}]}}}}`, container, image))

	return PatchStatefulSet(ns, name, patchBytes, cl)
}

func ScaleStatefulSet(ns, name string, replicas int, cl client.Client) error {
	patchBytes := []byte(fmt.Sprintf(`{"spec":{"replicas": %d}}`, replicas))
	return PatchStatefulSet(ns, name, patchBytes, cl)
}

func CreateOrPatchStatefulSet(sts *appsv1.StatefulSet, cl client.Client) error {
	return createOrPatchObject(sts, cl)
}
