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
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core"
	"github.com/koderover/zadig/pkg/microservice/policy/core/service"
	"github.com/koderover/zadig/pkg/microservice/policy/core/yamlconfig"
	"github.com/koderover/zadig/pkg/microservice/policy/server/rest"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	toolClient "github.com/koderover/zadig/pkg/tool/kube/client"
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
		if err := toolClient.Start(ctx); err != nil {
			panic(err)
		}
	}()

	if err := migratePolicyMeta(); err != nil {
		log.Errorf("fail to migrate policyMeta, err:%s", err)
		return err
	}

	if err := migrateRole(); err != nil {
		log.Errorf("fail to migrate role , err:%s", err)
		return err
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Errorf("Failed to start http server, error: %s", err)
		return err
	}

	<-stopChan

	return nil
}

type PresetRoleConfigYaml struct {
	PresetRoles []*service.Role `json:"preset_roles"`
	Description string          `json:"description"`
}

func migrateRole() error {

	bs := yamlconfig.PresetRolesBytes()
	config := PresetRoleConfigYaml{}

	if err := yaml.Unmarshal(bs, &config); err != nil {
		log.Errorf("yaml Unmarshal err:%s", err)
		return err
	}

	for _, role := range config.PresetRoles {
		if role.Name == "admin" {
			role.Namespace = "*"
		}
		for _, rule := range role.Rules {
			rule.Kind = "resource"
		}
		role.Type = "system"
	}

	for _, role := range config.PresetRoles {
		if err := service.UpdateOrCreateRole(role.Namespace, role, nil); err != nil {
			log.Errorf("UpdateOrCreateRole err:%s", err)
			return err
		}
	}
	return nil

}

// migratePolicyMeta migrate the policy meta db date
func migratePolicyMeta() error {
	client, err := kubeclient.GetKubeClient(commonconfig.HubServerServiceAddress(), setting.LocalClusterID)
	if err != nil {
		log.DPanic(err)
	}
	clientset, err := kubeclient.GetKubeClientSet(commonconfig.HubServerServiceAddress(), setting.LocalClusterID)
	if err != nil {
		log.DPanic(err)
	}

	namespace := commonconfig.Namespace()
	policyMetasConfig := yamlconfig.DefaultPolicyMetasConfig()
	policyMetaConfigBytes, err := yaml.Marshal(policyMetasConfig)
	if err != nil {
		return err
	}
	urls := yamlconfig.GetDefaultEmbedUrlConfig()
	urlsBytes, err := yaml.Marshal(urls)
	if err != nil {
		return err
	}

	metaConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      setting.PolicyMetaConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"meta.yaml": string(policyMetaConfigBytes),
		},
	}

	urlConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      setting.PolicyURLConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"urls.yaml": string(urlsBytes),
		},
	}
	roleConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      setting.PolicyRoleConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"roles.yaml": string(yamlconfig.PresetRolesBytes()),
		},
	}

	if err := initConfigMap(metaConfigMap, client, clientset); err != nil {
		log.Errorf("init config map meta err:%s", err)
	}
	if err := initConfigMap(urlConfigMap, client, clientset); err != nil {
		log.Errorf("init config map url err:%s", err)
	}

	if err := initConfigMap(roleConfigMap, client, clientset); err != nil {
		log.Errorf("init config map roles err:%s", err)
	}

	for _, v := range yamlconfig.DefaultPolicyMetas() {
		if err := service.CreateOrUpdatePolicyRegistration(v, nil); err != nil {
			log.DPanic(err)
		}
	}
	_, err = NewClusterInformerFactory("", clientset)
	return err
}

func initConfigMap(cm *corev1.ConfigMap, client client.Client, clientset *kubernetes.Clientset) error {
	_, found, err := getter.GetConfigMap(cm.Namespace, cm.Name, client)
	if err != nil {
		log.Errorf("get config map err:%s", err)
	}
	if !found {
		err := updater.CreateConfigMap(cm, client)
		if err != nil {
			log.Infof("create config map err:%s", err)
			return err
		}
	} else {
		if err := updater.UpdateConfigMap(cm.Namespace, cm, clientset); err != nil {
			log.Infof("update config map err:%s", err)
			return err
		}
	}
	return nil
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
			if configMap.Name == setting.PolicyMetaConfigMapName {
				if b, ok := configMap.Data["meta.yaml"]; ok {
					log.Infof("start to update the meta configMap")
					for _, v := range yamlconfig.PolicyMetasFromBytes([]byte(b)) {
						if err := service.CreateOrUpdatePolicyRegistration(v, nil); err != nil {
							log.Errorf("fail to CreateOrUpdatePolicyRegistration,err:%s", err)
							continue
						}
					}
				}
			}
			if configMap.Name == setting.PolicyURLConfigMapName {
				if b, ok := configMap.Data["urls.yaml"]; ok {
					log.Infof("start to refresh url configMap data")
					if err := yamlconfig.RefreshConfigMapByte([]byte(b)); err != nil {
						log.Errorf("refresh urls err:%s", err)
					}
				}
			}
			if configMap.Name == setting.PolicyRoleConfigMapName {
				if b, ok := configMap.Data["roles.yaml"]; ok {
					log.Infof("start to refresh role configmap data")
					yamlconfig.RefreshRoles([]byte(b))
					if err := migrateRole(); err != nil {
						log.Errorf("refresh role err:%s", err)
					}
				}
			}

		},
	})
	stop := make(chan struct{})
	informerFactory.Start(stop)
	return informerFactory, nil
}
