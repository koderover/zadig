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
	"text/template"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var restartPatchTemplate = template.Must(template.New("restart-patch-template").Parse(`{
  "spec": {
    "template": {
      "metadata": {
        "annotations": {
          "restart-by-koderover": "{{.Time}}"
        }
      }
    }
  }
}`))

func PatchDeployment(ns, name string, patchBytes []byte, cl client.Client) error {
	return patchObject(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}, patchBytes, cl)
}

// TODO: LOU: it is not the right way to restart a deployment, since it is a hack and it
// will generate a new revision which will pollute the revision history.
func RestartDeployment(ns, name string, cl client.Client) error {
	now := time.Now().Format(time.RFC3339Nano)
	payload := bytes.NewBufferString("")
	_ = restartPatchTemplate.Execute(payload, struct {
		Time string
	}{now})

	if err := PatchDeployment(ns, name, payload.Bytes(), cl); err != nil {
		return fmt.Errorf("failed to restart %s/deploy/%s: %v", ns, name, err)
	}

	return nil
}

func DeleteDeployments(ns string, selector labels.Selector, cl client.Client) error {
	return deleteObjectsWithDefaultOptions(ns, selector, &appsv1.Deployment{}, cl)
}

func UpdateDeploymentImage(ns, name, container, image string, cl client.Client) error {
	patchBytes := []byte(fmt.Sprintf(`{"spec":{"template":{"spec":{"containers":[{"name":"%s","image":"%s"}]}}}}`, container, image))

	return PatchDeployment(ns, name, patchBytes, cl)
}

func ScaleDeployment(ns, name string, replicas int, cl client.Client) error {
	patchBytes := []byte(fmt.Sprintf(`{"spec":{"replicas": %d}}`, replicas))
	return PatchDeployment(ns, name, patchBytes, cl)
}

func CreateOrPatchDeployment(d *appsv1.Deployment, cl client.Client) error {
	return createOrPatchObject(d, cl)
}
