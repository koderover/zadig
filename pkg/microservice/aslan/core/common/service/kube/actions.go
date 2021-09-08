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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
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
		reg.SecretKey,
		"bot@koderover.com",
	)
	data[".dockercfg"] = []byte(dockerConfig)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      setting.DefaultImagePullSecret,
		},
		Data: data,
		Type: corev1.SecretTypeDockercfg,
	}
	return updater.UpdateOrCreateSecret(secret, kubeClient)
}

// GetDirtyResources searches for dirty active resources in the given namespace, and return their metadata.
func GetDirtyResources(ns string, kubeClient client.Client) []metav1.Object {
	var oms []metav1.Object

	empty := labels.NewSelector()
	dirty, err := labels.NewRequirement(setting.DirtyLabel, selection.Equals, []string{setting.LabelValueTrue})
	if err != nil {
		log.DPanicf("Can not create a requirement, err: %+v", err)
		return nil
	}

	active, err := labels.NewRequirement(setting.InactiveConfigLabel, selection.NotEquals, []string{setting.LabelValueTrue})
	if err != nil {
		log.DPanicf("Can not create a requirement, err: %+v", err)
		return nil
	}

	// search for dirty and active configMaps
	s := empty.Add(*dirty, *active)
	log.Debugf("Getting configMaps in namespace %s with selector %s", ns, s)
	cms, err := getter.ListConfigMaps(ns, s, kubeClient)
	if err != nil {
		log.Errorf("Failed to list ConfigMap by selector %s in namespace %s", s, ns)
		return nil
	}

	for _, cm := range cms {
		o, err := meta.Accessor(cm)
		if err != nil {
			log.Error(err)
			continue
		}
		oms = append(oms, o)
	}

	log.Debugf("Found %d matching resources in namespace %s", len(oms), ns)
	return oms
}
