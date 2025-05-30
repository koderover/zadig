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

package registries

import (
	"context"
	"encoding/base64"
	"fmt"

	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"

	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func PrepareDinD(client *kubernetes.Clientset, namespace string, regList []*RegistryInfoForDinDUpdate) error {
	insecureRegistryList := make([]string, 0)

	mountFlag := false
	insecureFlag := false
	sourceList := make([]corev1.VolumeProjection, 0)
	insecureMap := sets.NewString()

	for _, reg := range regList {
		// compatibility changes before 1.11
		if reg.AdvancedSetting != nil {
			// remove the http:// https:// prefix
			addr := strings.Split(reg.RegAddr, "//")
			// if a registry is marked as insecure, we add a record to insecure-registries
			if !reg.AdvancedSetting.TLSEnabled {
				insecureFlag = true
				if !insecureMap.Has(addr[1]) {
					insecureRegistryList = append(insecureRegistryList, fmt.Sprintf("--insecure-registry=%s", addr[1]))
					insecureMap.Insert(addr[1])
				}
			}
			// if a registry is marked as secure and a TLS cert is given, we mount this certificate to dind daemon
			if reg.AdvancedSetting.TLSEnabled && reg.AdvancedSetting.TLSCert != "" {
				if !insecureMap.Has(addr[1]) {
					mountName := fmt.Sprintf("%s-cert", reg.ID.Hex())
					err := ensureCertificateSecret(client, mountName, namespace, reg.AdvancedSetting.TLSCert)
					if err != nil {
						log.Errorf("failed to ensure secret: %s, the error is: %s", mountName, err)
						return err
					}

					// create projected volumes sources list
					sourceList = append(sourceList, corev1.VolumeProjection{
						Secret: &corev1.SecretProjection{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: mountName,
							},
							Items: []corev1.KeyToPath{
								{
									Key:  "ca.crt",
									Path: fmt.Sprintf("%s/%s", addr[1], "ca.crt"),
								},
							},
						},
					})

					// set mountFlag to add mounting volume to dind
					mountFlag = true
					insecureMap.Insert(addr[1])
				}
			}
		}
	}

	dindSts, err := client.AppsV1().StatefulSets(namespace).Get(context.TODO(), "dind", metav1.GetOptions{})
	if err != nil {
		log.Errorf("failed to get dind statefulset, the error is: %s", err)
		return err
	}

	if len(dindSts.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("failed to extract container from dind sts")
	}

	if mountFlag {
		volumeMount := corev1.VolumeMount{
			MountPath: "/etc/docker/certs.d",
			Name:      "cert-mount",
		}

		// update spec.template.spec.containers[0].volumeMounts
		dindSts.Spec.Template.Spec.Containers[0].VolumeMounts = append(dindSts.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMount)

		volume := corev1.Volume{
			Name: "cert-mount",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: sourceList,
				},
			},
		}

		// update spec.template.spec.volumes
		dindSts.Spec.Template.Spec.Volumes = append(dindSts.Spec.Template.Spec.Volumes, volume)
	}

	if insecureFlag {
		// update spec.template.spec.containers[0].args
		dindSts.Spec.Template.Spec.Containers[0].Args = insecureRegistryList
	}

	_, updateErr := client.AppsV1().StatefulSets(namespace).Update(context.TODO(), dindSts, metav1.UpdateOptions{})
	if updateErr != nil {
		log.Errorf("failed to update dind, the error is: %s", updateErr)
	}
	return updateErr
}

func ensureCertificateSecret(client *kubernetes.Clientset, secretName, namespace, cert string) error {
	certify := base64.StdEncoding.EncodeToString([]byte(cert))
	datamap := map[string][]byte{
		"ca.crt": []byte(certify),
	}

	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	// if there is an error, either because of not found or anything else, we try to create a secret with the given information
	if err != nil {
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
			},
			Data: datamap,
			Type: "Opaque",
		}

		_, err := client.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			log.Errorf("failed to create secret: %s, the error is: %s", secretName, err)
		}
		return err
	} else {
		secret.Data = datamap
		_, err := client.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			log.Errorf("failed to update secret: %s, the error is: %s", secretName, err)
		}
		return err
	}
}
