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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/kube/updater"
)

func CreateNamespace(namespace string, kubeClient client.Client) error {
	err := updater.CreateNamespaceByName(namespace, map[string]string{setting.EnvCreatedBy: setting.EnvCreator}, kubeClient)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func CreateOrUpdateRegistrySecret(namespace string, reg *commonmodels.RegistryNamespace, kubeClient client.Client) error {
	data := make(map[string][]byte)
	dockerConfig := fmt.Sprintf(
		`{"%s":{"username":"%s","password":"%s","email":"%s"}}`,
		reg.RegAddr,
		reg.AccessKey,
		reg.SecretyKey,
		"bot@koderover.com",
	)
	data[".dockercfg"] = []byte(dockerConfig)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      setting.DefaultCandidateImagePullSecret,
		},
		Data: data,
		Type: corev1.SecretTypeDockercfg,
	}
	return updater.UpdateOrCreateSecret(secret, kubeClient)
}
