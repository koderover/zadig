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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"strings"

	"github.com/koderover/zadig/pkg/tool/log"
)

func PrepareDinD(dynamicClient dynamic.Interface, namespace string, regList []*RegistryInfoForDinDUpdate) error {
	// set statefulset GVR
	stsResource := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}

	volumeMountList := make([]interface{}, 0)
	volumeList := make([]interface{}, 0)
	insecureRegistryList := make([]interface{}, 0)

	mountFlag := false
	insecureFlag := false
	sourceList := make([]interface{}, 0)

	for _, reg := range regList {
		// compatibility changes before 1.11
		if reg.AdvancedSetting != nil {
			// remove the http:// https:// prefix
			addr := strings.Split(reg.RegAddr, "//")
			// if a registry is marked as insecure, we add a record to insecure-registries
			if !reg.AdvancedSetting.TLSEnabled {
				insecureFlag = true
				insecureRegistryList = append(insecureRegistryList, fmt.Sprintf("--insecure-registry=%s", addr[1]))
			}
			// if a registry is marked as secure and a TLS cert is given, we mount this certificate to dind daemon
			if reg.AdvancedSetting.TLSEnabled && reg.AdvancedSetting.TLSCert != "" {
				mountName := fmt.Sprintf("%s-cert", reg.ID.Hex())
				err := ensureCertificateSecret(dynamicClient, mountName, namespace, reg.AdvancedSetting.TLSCert)
				if err != nil {
					log.Errorf("failed to ensure secret: %s, the error is: %s", mountName, err)
					return err
				}
				// create secret mount info
				secretMount := map[string]interface{}{
					"name": mountName,
					"items": []interface{}{
						map[string]interface{}{
							"key":  "ca.crt",
							"path": fmt.Sprintf("%s/%s", addr[1], "ca.crt"),
						},
					},
				}

				// create projected volumes sources list
				sourceList = append(sourceList, map[string]interface{}{
					"secret": secretMount,
				})

				// set mountFlag to add mounting volume to dind
				mountFlag = true
			}
		}
	}

	if mountFlag {
		volumeMountMap := map[string]interface{}{
			"mountPath": "/etc/docker/certs.d",
			"name":      "cert-mount",
		}
		volumeMountList = append(volumeMountList, volumeMountMap)
		volumeList = append(volumeList, map[string]interface{}{
			"name": "cert-mount",
			"projected": map[string]interface{}{
				"sources": sourceList,
			},
		})
	}

	result, getErr := dynamicClient.Resource(stsResource).Namespace(namespace).Get(context.TODO(), "dind", metav1.GetOptions{})
	if getErr != nil {
		log.Errorf("failed to get dind statefulset, the error is: %s", getErr)
		return getErr
	}

	// extract spec containers
	containers, found, err := unstructured.NestedSlice(result.Object, "spec", "template", "spec", "containers")
	if err != nil || !found || containers == nil {
		return err
	}

	if mountFlag {
		// update spec.template.spec.containers[0].volumeMounts
		if err := unstructured.SetNestedField(containers[0].(map[string]interface{}), volumeMountList, "volumeMounts"); err != nil {
			return err
		}
		// update spec.template.spec.volumes
		if err := unstructured.SetNestedField(result.Object, volumeList, "spec", "template", "spec", "volumes"); err != nil {
			return err
		}
	}

	if insecureFlag {
		// update spec.template.spec.containers[0].args
		if err := unstructured.SetNestedField(containers[0].(map[string]interface{}), insecureRegistryList, "args"); err != nil {
			return err
		}
	}

	if err := unstructured.SetNestedField(result.Object, containers, "spec", "template", "spec", "containers"); err != nil {
		return err
	}

	_, updateErr := dynamicClient.Resource(stsResource).Namespace(namespace).Update(context.TODO(), result, metav1.UpdateOptions{})
	if updateErr != nil {
		log.Errorf("failed to update dind, the error is: %s", updateErr)
	}
	return updateErr
}

func ensureCertificateSecret(dynamicClient dynamic.Interface, secretName, namespace, cert string) error {
	certificateString := base64.StdEncoding.EncodeToString([]byte(cert))
	datamap := map[string]interface{}{
		"ca.crt": certificateString,
	}

	// setup secret GVR for future use
	secretsGVR := schema.GroupVersionResource{
		Version:  "v1",
		Resource: "secrets",
	}

	secret, err := dynamicClient.Resource(secretsGVR).Namespace(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	// if there is an error, either because of not found or anything else, we try to create a secret with the given information
	if err != nil {
		secret := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]interface{}{
					"name": secretName,
				},
				"type": "Opaque",
				"data": datamap,
			},
		}

		_, err := dynamicClient.Resource(secretsGVR).Namespace(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			log.Errorf("failed to create secret: %s, the error is: %s", secretName, err)
		}
		return err
	} else {
		if err := unstructured.SetNestedField(secret.Object, datamap, "data"); err != nil {
			log.Errorf("failed to set data in secret object, the error is: %s", err)
			return err
		}
		_, err := dynamicClient.Resource(secretsGVR).Namespace(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			log.Errorf("failed to update secret: %s, the error is: %s", secretName, err)
		}
		return err
	}
}
