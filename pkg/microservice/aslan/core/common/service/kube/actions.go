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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
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

const (
	registrySecretSuffix                  = "-registry-secret"
	defaultRegistrySecretDataKey          = ".dockercfg"
	zadigCreatedRegistryAddressAnnotation = "koderover.io/zadig-created-registry-address"
	defaultRegistrySecretEmail            = "bot@koderover.com"
)

type dockerConfigCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

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

	if secretName == setting.DefaultImagePullSecret {
		return upsertDefaultRegistryCredential(namespace, clusterID, reg)
	}

	credential, err := json.Marshal(dockerConfigCredential{
		Username: reg.AccessKey,
		Password: reg.SecretKey,
		Email:    defaultRegistrySecretEmail,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal registry credential for %s: %w", reg.RegAddr, err)
	}
	dockerConfig, err := json.Marshal(map[string]json.RawMessage{reg.RegAddr: credential})
	if err != nil {
		return fmt.Errorf("failed to marshal docker config for %s: %w", reg.RegAddr, err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      secretName,
		},
		Data: map[string][]byte{
			defaultRegistrySecretDataKey: dockerConfig,
		},
		Type: corev1.SecretTypeDockercfg,
	}
	return updater.CreateOrUpdateSecretV2(context.TODO(), clusterID, secret)
}

func upsertDefaultRegistryCredential(namespace, clusterID string, reg *commonmodels.RegistryNamespace) error {
	c, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	return upsertDefaultRegistryCredentialWithClient(context.TODO(), c.CoreV1().Secrets(namespace), namespace, reg)
}

func upsertDefaultRegistryCredentialWithClient(ctx context.Context, secrets corev1client.SecretInterface, namespace string, reg *commonmodels.RegistryNamespace) error {
	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		return apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err)
	}, func() error {
		secret, err := secrets.Get(ctx, setting.DefaultImagePullSecret, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      setting.DefaultImagePullSecret,
				},
				Data: map[string][]byte{
					defaultRegistrySecretDataKey: []byte("{}"),
				},
				Type: corev1.SecretTypeDockercfg,
			}
			if err := mergeDefaultRegistryCredential(secret, reg); err != nil {
				return err
			}
			_, err = secrets.Create(ctx, secret, metav1.CreateOptions{})
			return err
		}
		if err != nil {
			return err
		}

		if err := mergeDefaultRegistryCredential(secret, reg); err != nil {
			return err
		}
		_, err = secrets.Update(ctx, secret, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to upsert registry credential in secret %s/%s: %w", namespace, setting.DefaultImagePullSecret, err)
	}
	return nil
}

func mergeDefaultRegistryCredential(secret *corev1.Secret, reg *commonmodels.RegistryNamespace) error {
	if secret.Type != corev1.SecretTypeDockercfg {
		return fmt.Errorf("secret %s/%s has unsupported type %q", secret.Namespace, secret.Name, secret.Type)
	}

	dockerConfigData, ok := secret.Data[defaultRegistrySecretDataKey]
	if !ok {
		return fmt.Errorf("secret %s/%s is missing %s", secret.Namespace, secret.Name, defaultRegistrySecretDataKey)
	}

	dockerConfig := make(map[string]json.RawMessage)
	if err := json.Unmarshal(dockerConfigData, &dockerConfig); err != nil {
		return fmt.Errorf("failed to parse %s in secret %s/%s: %w", defaultRegistrySecretDataKey, secret.Namespace, secret.Name, err)
	}
	if dockerConfig == nil {
		return fmt.Errorf("%s in secret %s/%s must be a JSON object", defaultRegistrySecretDataKey, secret.Namespace, secret.Name)
	}

	_, targetExisted := dockerConfig[reg.RegAddr]
	managedAddress := secret.Annotations[zadigCreatedRegistryAddressAnnotation]
	if managedAddress != "" && managedAddress != reg.RegAddr {
		delete(dockerConfig, managedAddress)
		delete(secret.Annotations, zadigCreatedRegistryAddressAnnotation)
	}

	credential, err := json.Marshal(dockerConfigCredential{
		Username: reg.AccessKey,
		Password: reg.SecretKey,
		Email:    defaultRegistrySecretEmail,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal registry credential for %s: %w", reg.RegAddr, err)
	}
	dockerConfig[reg.RegAddr] = credential

	switch {
	case managedAddress == reg.RegAddr:
	case !targetExisted:
		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}
		secret.Annotations[zadigCreatedRegistryAddressAnnotation] = reg.RegAddr
	default:
		delete(secret.Annotations, zadigCreatedRegistryAddressAnnotation)
	}

	secret.Data[defaultRegistrySecretDataKey], err = json.Marshal(dockerConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal %s for secret %s/%s: %w", defaultRegistrySecretDataKey, secret.Namespace, secret.Name, err)
	}
	return nil
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
