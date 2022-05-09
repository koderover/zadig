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
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"

	commonconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core"
	"github.com/koderover/zadig/pkg/microservice/policy/core/meta"
	"github.com/koderover/zadig/pkg/microservice/policy/core/service"
	"github.com/koderover/zadig/pkg/microservice/policy/server/rest"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
)

func Serve(ctx context.Context) error {
	core.Start(ctx)
	defer core.Stop(ctx)

	log.Info("Start policy service")

	engine := rest.NewEngine()
	server := &http.Server{Addr: ":80", Handler: engine}

	stopChan := make(chan struct{})
	go func() {
		defer close(stopChan)

		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("Failed to stop server, error: %s", err)
		}
	}()
	go func() {
		if err := client.Start(ctx); err != nil {
			panic(err)
		}
	}()

	if err := migratePolicyMeta(); err != nil {
		log.Errorf("fatil to migrage policyMeta")
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Errorf("Failed to start http server, error: %s", err)
		return err
	}

	<-stopChan

	return nil
}

// migratePolicyMeta migrate the policy meta db date
func migratePolicyMeta() error {
	namespace := commonconfig.Namespace()
	policyMetas := meta.DefaultPolicyMetas()
	log.Info(len(policyMetas))
	metasBytes, _ := yaml.Marshal(policyMetas)
	metaConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      setting.PolicyMetaConfigName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"meta.yaml": string(metasBytes),
		},
	}
	client, err := kubeclient.GetKubeClient(commonconfig.HubServerServiceAddress(), setting.LocalClusterID)
	if err != nil {
		log.DPanic(err)
	}
	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), setting.LocalClusterID)
	if err != nil {
		log.DPanic(err)
	}

	_, found, err := getter.GetConfigMap(namespace, "policymeta", client)
	if err != nil {
		log.Errorf("get config map err:%s", err)
	}
	if !found {
		err := updater.CreateConfigMap(metaConfigMap, client)
		if err != nil {
			log.Infof("create config map err:%s", err)
		}
	} else {
		if err := updater.UpdateConfigMap(namespace, metaConfigMap, clientset); err != nil {
			log.Infof("update config map err:%s", err)
		}
	}

	for _, v := range meta.DefaultPolicyMetas() {
		if err := service.CreateOrUpdatePolicyRegistration(v, nil); err != nil {
			log.DPanic(err)
		}
	}
	_, err = NewClusterInformerFactory("", clientset)
	return err
}

func NewClusterInformerFactory(clusterId string, cls *kubernetes.Clientset) (informers.SharedInformerFactory, error) {
	if clusterId == "" {
		clusterId = setting.LocalClusterID
	}
	informerFactory := informers.NewSharedInformerFactoryWithOptions(cls, time.Hour*24, informers.WithNamespace(commonconfig.Namespace()))
	configMapsInformer := informerFactory.Core().V1().ConfigMaps().Informer()
	configMapsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			configMap, ok := newObj.(*corev1.ConfigMap)
			if !ok {
				return
			}
			if configMap.Name == setting.PolicyMetaConfigName {
				if b, ok := configMap.Data["meta.yaml"]; ok {
					log.Infof("****update start")
					for _, v := range meta.PolicyMetasFromBytes([]byte(b)) {
						log.Infof("***name %v", v.Resource)
						if err := service.CreateOrUpdatePolicyRegistration(v, nil); err != nil {
							log.DPanic(err)
						}
					}
				}
			}
		},
	})
	stop := make(chan struct{})
	informerFactory.Start(stop)
	return informerFactory, nil
}
