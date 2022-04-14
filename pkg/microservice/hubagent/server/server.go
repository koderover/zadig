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

package server

import (
	"context"
	"encoding/base64"
	"fmt"
	config2 "github.com/koderover/zadig/pkg/microservice/hubagent/config"
	"github.com/koderover/zadig/pkg/shared/client/aslan"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"net/http"
	"strings"
	"time"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/hubagent/core/service"
	"github.com/koderover/zadig/pkg/microservice/hubagent/server/rest"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	log.Init(&log.Config{
		Level:       config.LogLevel(),
		Filename:    config.LogFile(),
		SendToFile:  config.SendLogToFile(),
		Development: config.Mode() != setting.ReleaseMode,
	})
}

func Serve(ctx context.Context) error {
	log.Info("Start Hub-Agent service.")

	engine := rest.NewEngine()
	server := &http.Server{Addr: ":80", Handler: engine}

	initDinD()

	go func() {
		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("Failed to stop server, error: %s", err)
		}
	}()

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Errorf("Failed to start http server, error: %s", err)
			return
		}
	}()

	if err := service.Init(); err != nil {
		return err
	}

	return nil
}

func initDinD() {
	client := aslan.New(config2.HubServerBaseAddr())

	ls, err := client.ListRegistries()
	if err != nil {
		log.Fatalf("failed to get information from zadig server to set DinD, err: %s", err)
	}

	stsResource := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}

	dynamicClient, err := kubeclient.GetDynamicKubeClient(config2.HubServerBaseAddr(), setting.LocalClusterID)
	if err != nil {
		log.Fatalf("failed to create dynamic kubernetes clientset for clusterID: %s, the error is: %s", setting.LocalClusterID, err)
	}

	volumeMountList := make([]interface{}, 0)
	volumeList := make([]interface{}, 0)

	insecureRegistryList := make([]string, 0)
	for _, reg := range ls {
		// compatibility changes before 1.11
		if reg.AdvancedSetting != nil {
			// if a registry is marked as insecure, we add a record to insecure-registries
			if !reg.AdvancedSetting.TLSEnabled {
				insecureRegistryList = append(insecureRegistryList, reg.RegAddr)
			}
			// if a registry is marked as secure and a TLS cert is given, we mount this certificate to dind daemon
			if reg.AdvancedSetting.TLSEnabled && reg.AdvancedSetting.TLSCert != "" {
				mountName := fmt.Sprintf("%s-cert", reg.ID.Hex())
				err := ensureCertificateSecret(mountName, "koderover-agent", reg.AdvancedSetting.TLSCert, log.SugaredLogger())
				if err != nil {
					log.Fatalf("failed to ensure secret: %s, the error is: %s", mountName, err)
				}

				addr := strings.Split(reg.RegAddr, "//")
				// create volumeMount info
				volumeMountMap := map[string]interface{}{
					"mountPath": fmt.Sprintf("/etc/docker/certs.d/%s", addr[1]),
					"name":      mountName,
				}
				volumeMountList = append(volumeMountList, volumeMountMap)
				// create volume info
				secretItemList := make([]interface{}, 0)
				secretItemList = append(secretItemList, map[string]interface{}{
					"key":  "cert.crt",
					"path": "cert.crt",
				})
				secretInfo := map[string]interface{}{
					"items":      secretItemList,
					"secretName": mountName,
				}
				volumeMap := map[string]interface{}{
					"name":   mountName,
					"secret": secretInfo,
				}
				volumeList = append(volumeList, volumeMap)
			}
		}
	}

	// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
	result, getErr := dynamicClient.Resource(stsResource).Namespace("koderover-agent").Get(context.TODO(), "dind", metav1.GetOptions{})
	if getErr != nil {
		log.Fatalf("failed to get dind statefulset, the error is: %s", getErr)
	}

	// extract spec containers
	containers, found, err := unstructured.NestedSlice(result.Object, "spec", "template", "spec", "containers")
	if err != nil || !found || containers == nil {
		log.Fatal(err)
	}

	if err := unstructured.SetNestedField(containers[0].(map[string]interface{}), volumeMountList, "volumeMounts"); err != nil {
		log.Fatal(err)
	}
	if err := unstructured.SetNestedField(result.Object, containers, "spec", "template", "spec", "containers"); err != nil {
		log.Fatal(err)
	}
	if err := unstructured.SetNestedField(result.Object, volumeList, "spec", "template", "spec", "volumes"); err != nil {
		log.Fatal(err)
	}
	_, updateErr := dynamicClient.Resource(stsResource).Namespace(config.Namespace()).Update(context.TODO(), result, metav1.UpdateOptions{})
	if updateErr != nil {
		log.Errorf("failed to update dind, the error is: %s", updateErr)
	}
	log.Fatal(updateErr)
}

func ensureCertificateSecret(secretName, namespace, cert string, log *zap.SugaredLogger) error {
	certificateString := base64.StdEncoding.EncodeToString([]byte(cert))
	datamap := map[string]interface{}{
		"cert.crt": certificateString,
	}

	secretsGVR := schema.GroupVersionResource{
		Version:  "v1",
		Resource: "secrets",
	}

	dynamicClient, err := kubeclient.GetDynamicKubeClient(config2.HubServerBaseAddr(), setting.LocalClusterID)
	if err != nil {
		log.Errorf("failed to create dynamic kubernetes clientset for clusterID: %s, the error is: %s", setting.LocalClusterID, err)
		return err
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
