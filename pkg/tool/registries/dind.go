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
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

		newVolumeMounts := make([]corev1.VolumeMount, 0)
		found := false
		for _, volMount := range dindSts.Spec.Template.Spec.Containers[0].VolumeMounts {
			if volMount.Name == "cert-mount" {
				if found {
					// remove duplicate
					continue
				} else {
					newVolumeMounts = append(newVolumeMounts, volMount)
					found = true
				}
			} else {
				newVolumeMounts = append(newVolumeMounts, volMount)
			}
		}
		if !found {
			newVolumeMounts = append(newVolumeMounts, volumeMount)
		}

		// update spec.template.spec.containers[0].volumeMounts
		dindSts.Spec.Template.Spec.Containers[0].VolumeMounts = newVolumeMounts

		volume := corev1.Volume{
			Name: "cert-mount",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: sourceList,
				},
			},
		}

		newVolumes := make([]corev1.Volume, 0)
		found = false
		for _, vol := range dindSts.Spec.Template.Spec.Volumes {
			if vol.Name == "cert-mount" {
				if found {
					// remove duplicate
					continue
				} else {
					newVolumes = append(newVolumes, vol)
					found = true
				}
			} else {
				newVolumes = append(newVolumes, vol)
			}
		}
		if !found {
			newVolumes = append(newVolumes, volume)
		}

		// update spec.template.spec.volumes
		dindSts.Spec.Template.Spec.Volumes = newVolumes
	}

	currentArgs := dindSts.Spec.Template.Spec.Containers[0].Args
	finalArgs := make([]string, 0)

	for _, arg := range currentArgs {
		if strings.HasPrefix(arg, "--insecure-registry=") {
			continue
		}

		finalArgs = append(finalArgs, arg)
	}

	finalArgs = append(finalArgs, insecureRegistryList...)

	needsUpdate := false
	if len(finalArgs) != len(currentArgs) {
		needsUpdate = true
	} else {
		for i, arg := range finalArgs {
			if currentArgs[i] != arg {
				needsUpdate = true
				break
			}
		}
	}

	if needsUpdate || insecureFlag {
		dindSts.Spec.Template.Spec.Containers[0].Args = finalArgs
	}

	// Check if StatefulSet is stuck (e.g., due to wrong storage driver) and handle it
	isStuck := isStatefulSetStuckInUpdate(dindSts)
	if isStuck {
		log.Warnf("StatefulSet %s/dind is stuck, attempting to fix by deleting stuck pods before update", namespace)
		if fixErr := handleStuckStatefulSet(dindSts, client); fixErr != nil {
			log.Warnf("Failed to clean up stuck pods for StatefulSet %s/dind: %v", namespace, fixErr)
			// Continue with update even if cleanup fails
		}
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

// isStatefulSetStuckInUpdate checks if a StatefulSet is stuck in an update state
func isStatefulSetStuckInUpdate(sts *appsv1.StatefulSet) bool {
	if sts == nil {
		return false
	}

	status := sts.Status
	spec := sts.Spec

	if spec.Replicas == nil {
		return false
	}

	desiredReplicas := *spec.Replicas

	// Check if StatefulSet is in the middle of an update
	// The update is in progress when currentRevision != updateRevision
	if status.CurrentRevision != "" && status.UpdateRevision != "" &&
		status.CurrentRevision != status.UpdateRevision {

		// StatefulSet is stuck if:
		// 1. Not all replicas are updated (updatedReplicas < desiredReplicas)
		// 2. OR not all replicas are ready (readyReplicas < desiredReplicas)
		// 3. OR the update hasn't fully rolled out (currentRevision != updateRevision means rollout incomplete)
		isStuck := status.UpdatedReplicas < desiredReplicas || status.ReadyReplicas < desiredReplicas

		if isStuck {
			log.Warnf("StatefulSet %s/%s appears to be stuck in update: currentRevision=%s, updateRevision=%s, replicas=%d, updatedReplicas=%d, readyReplicas=%d, currentReplicas=%d",
				sts.Namespace, sts.Name, status.CurrentRevision, status.UpdateRevision,
				desiredReplicas, status.UpdatedReplicas, status.ReadyReplicas, status.CurrentReplicas)
			return true
		}
	}

	return false
}

// handleStuckStatefulSet attempts to fix a stuck StatefulSet by deleting non-ready pods
func handleStuckStatefulSet(sts *appsv1.StatefulSet, clientSet *kubernetes.Clientset) error {
	log.Warnf("Attempting to fix stuck StatefulSet %s/%s by deleting non-ready pods", sts.Namespace, sts.Name)

	selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
	if err != nil {
		return errors.Wrapf(err, "failed to convert label selector for StatefulSet %s/%s", sts.Namespace, sts.Name)
	}

	pods, err := clientSet.CoreV1().Pods(sts.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return errors.Wrapf(err, "failed to list pods for StatefulSet %s/%s", sts.Namespace, sts.Name)
	}

	// Delete non-ready pods to unblock the update
	deletedCount := 0
	for _, pod := range pods.Items {
		// Check if pod is not ready
		isReady := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				isReady = true
				break
			}
		}

		// Also check if pod is in a failed state
		isStuck := !isReady || pod.Status.Phase == corev1.PodPending ||
			pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodUnknown

		if isStuck {
			log.Warnf("Deleting stuck pod %s/%s (phase=%s, ready=%v) to unblock StatefulSet update",
				pod.Namespace, pod.Name, pod.Status.Phase, isReady)

			// Use zero grace period to force delete if necessary
			gracePeriodSeconds := int64(0)
			err := clientSet.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &gracePeriodSeconds,
			})
			if err != nil && !apierrors.IsNotFound(err) {
				log.Errorf("Failed to delete stuck pod %s/%s: %v", pod.Namespace, pod.Name, err)
				// Continue trying to delete other pods
				continue
			}
			deletedCount++
		}
	}

	if deletedCount > 0 {
		log.Infof("Deleted %d stuck pod(s) for StatefulSet %s/%s, waiting briefly for controller to react", deletedCount, sts.Namespace, sts.Name)
		// Brief pause to allow Kubernetes controller to process the deletions
		time.Sleep(1 * time.Second)
	} else {
		log.Infof("No stuck pods found for StatefulSet %s/%s", sts.Namespace, sts.Name)
	}

	return nil
}
