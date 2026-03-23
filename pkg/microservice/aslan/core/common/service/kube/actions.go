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
	"context"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	zadigtypes "github.com/koderover/zadig/v2/pkg/types"
)

var registrySecretSuffix = "-registry-secret"

func CreateNamespace(namespace, clusterID string, customLabels map[string]string, enableIstioInjection bool) error {
	nsLabels := map[string]string{
		setting.EnvCreatedBy: setting.EnvCreator,
	}
	if enableIstioInjection {
		nsLabels[zadigtypes.IstioLabelKeyInjection] = zadigtypes.IstioLabelValueInjection
	}

	if customLabels == nil {
		customLabels = map[string]string{}
	}
	mergedLabels := labels.Merge(customLabels, nsLabels)
	createErr := updater.CreateNamespaceByNameV2(context.TODO(), clusterID, namespace, mergedLabels)
	if createErr != nil && !apierrors.IsAlreadyExists(createErr) {
		return createErr
	}

	if enableIstioInjection && createErr != nil && apierrors.IsAlreadyExists(createErr) {
		err := updater.UpdateNamespaceV2(context.TODO(), clusterID, namespace, func(ns *corev1.Namespace) error {
			if ns.Labels == nil {
				ns.Labels = make(map[string]string)
			}
			ns.Labels[zadigtypes.IstioLabelKeyInjection] = zadigtypes.IstioLabelValueInjection
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to add istio-injection label and update namespace %s: %s", namespace, err)
		}
	}

	if createErr != nil && apierrors.IsAlreadyExists(createErr) {
		c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
		if err != nil {
			return fmt.Errorf("failed to get kube client: %w", err)
		}
		nsObj, err := c.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get namespace %s: %w", namespace, err)
		}
		if nsObj.Status.Phase == corev1.NamespaceTerminating {
			return fmt.Errorf("namespace `%s` is in terminating state, please wait for a while and try again", namespace)
		}
	}

	return nil
}

func EnsureNamespaceLabels(namespace, clusterID string, customLabels map[string]string) error {
	return updater.UpdateNamespaceV2(context.TODO(), clusterID, namespace, func(ns *corev1.Namespace) error {
		if labels.SelectorFromValidatedSet(customLabels).Matches(labels.Set(ns.Labels)) {
			return nil
		}
		ns.Labels = labels.Merge(ns.Labels, customLabels)
		return nil
	})
}

func CreateOrUpdateRSASecret(publicKey, privateKey []byte, clusterID string) error {
	data := make(map[string][]byte)

	data["publicKey"] = publicKey
	data["privateKey"] = privateKey

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.Namespace(),
			Name:      setting.RSASecretName,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}
	return updater.CreateOrUpdateSecretV2(context.TODO(), clusterID, secret)
}

func CreateOrUpdateDefaultRegistrySecret(namespace, clusterID string, reg *commonmodels.RegistryNamespace) error {
	return CreateOrUpdateRegistrySecret(namespace, clusterID, reg, true)
}

func CreateOrUpdateRegistrySecret(namespace, clusterID string, reg *commonmodels.RegistryNamespace, isDefault bool) error {
	var secretName string
	var err error
	if !isDefault {
		secretName, err = GenRegistrySecretName(reg)
		if err != nil {
			return fmt.Errorf("failed to generate registry secret name: %s", err)
		}
	} else {
		secretName = setting.DefaultImagePullSecret
	}

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
			Name:      secretName,
		},
		Data: data,
		Type: corev1.SecretTypeDockercfg,
	}
	return updater.CreateOrUpdateSecretV2(context.TODO(), clusterID, secret)
}

func GenRegistrySecretName(reg *commonmodels.RegistryNamespace) (string, error) {
	if reg.IsDefault {
		return setting.DefaultImagePullSecret, nil
	}

	arr := strings.Split(reg.Namespace, "/")
	namespaceInRegistry := arr[len(arr)-1]

	// for AWS ECR, there are no namespace, thus we need to find the NS from the URI
	if namespaceInRegistry == "" {
		uriDecipher := strings.Split(reg.RegAddr, ".")
		namespaceInRegistry = uriDecipher[0]
	}

	filteredName, err := formatRegistryName(namespaceInRegistry)
	if err != nil {
		return "", err
	}

	secretName := filteredName + registrySecretSuffix
	if reg.RegType != "" {
		secretName = filteredName + "-" + reg.RegType + registrySecretSuffix
	}

	return secretName, nil
}

// Note: The name of a Secret object must be a valid DNS subdomain name:
//
//	https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
func formatRegistryName(namespaceInRegistry string) (string, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9\\.-]+")
	if err != nil {
		return "", err
	}
	processedName := reg.ReplaceAllString(namespaceInRegistry, "")
	processedName = strings.ToLower(processedName)
	if len(processedName) > 237 {
		processedName = processedName[:237]
	}
	return processedName, nil
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
