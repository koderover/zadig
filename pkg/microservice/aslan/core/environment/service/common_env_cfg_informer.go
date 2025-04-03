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

package service

import (
	"context"
	"sync"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var ClusterInformersMap sync.Map
var ClusterStopChanMap sync.Map

func NewClusterInformerFactory(clusterID string, cls *kubernetes.Clientset) (informers.SharedInformerFactory, error) {
	if clusterID == "" {
		clusterID = setting.LocalClusterID
	}
	if informer, ok := ClusterInformersMap.Load(clusterID); ok {
		return informer.(informers.SharedInformerFactory), nil
	}

	informerFactory := informers.NewSharedInformerFactoryWithOptions(cls, time.Minute)
	informerFactory.Apps().V1().Deployments().Lister()
	informerFactory.Apps().V1().Deployments().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			go onDeploymentAddAndUpdate(obj)
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			newDeploy := newObj.(*appsv1.Deployment)
			oldDeploy := oldObj.(*appsv1.Deployment)
			if newDeploy.ResourceVersion == oldDeploy.ResourceVersion {
				return
			}
			go onDeploymentAddAndUpdate(newObj)
		},
		DeleteFunc: onDeploymentDelete,
	})

	informerFactory.Apps().V1().StatefulSets().Lister()
	informerFactory.Apps().V1().StatefulSets().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			go onStatefulSetAddAndUpdate(obj)
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			newDeploy := newObj.(*appsv1.StatefulSet)
			oldDeploy := oldObj.(*appsv1.StatefulSet)
			if newDeploy.ResourceVersion == oldDeploy.ResourceVersion {
				return
			}
			go onStatefulSetAddAndUpdate(newObj)
		},
		DeleteFunc: onStatefulSetDelete,
	})

	stopchan := make(chan struct{})
	informerFactory.Start(stopchan)
	informerFactory.WaitForCacheSync(make(chan struct{}))
	if _, ok := ClusterInformersMap.Load(clusterID); ok {
		if stopchan, ok := ClusterStopChanMap.Load(clusterID); ok {
			close(stopchan.(chan struct{}))
			ClusterStopChanMap.Delete(clusterID)
		}
	}
	ClusterInformersMap.Store(clusterID, informerFactory)
	ClusterStopChanMap.Store(clusterID, stopchan)
	return informerFactory, nil
}

func DeleteClusterInformer(clusterID string) {
	if _, ok := ClusterInformersMap.Load(clusterID); ok {
		if stopchan, ok := ClusterStopChanMap.Load(clusterID); ok {
			close(stopchan.(chan struct{}))
			ClusterStopChanMap.Delete(clusterID)
		}
	}
	ClusterInformersMap.Delete(clusterID)
}

func onDeploymentAddAndUpdate(obj interface{}) {
	deploy := obj.(*appsv1.Deployment)
	labels := deploy.GetLabels()
	serviceName := labels[setting.ServiceLabel]
	product, continueFlag := GetProductAndFilterNs(deploy.Namespace, deploy.Name, serviceName)
	if !continueFlag {
		return
	}

	configMaps, secrets, pvcs := VisitDeployment(deploy)
	envSvcDepend := &models.EnvSvcDepend{
		ProductName:   product.ProductName,
		EnvName:       product.EnvName,
		Namespace:     deploy.Namespace,
		ServiceName:   serviceName,
		ServiceModule: deploy.Name,
		WorkloadType:  setting.Deployment,
		ConfigMaps:    configMaps.List(),
		Pvcs:          pvcs.List(),
		Secrets:       secrets.List(),
	}
	if err := commonrepo.NewEnvSvcDependColl().CreateOrUpdate(envSvcDepend); err != nil {
		log.Error("onDeploymentAdd EnvSvcDepend CreateOrUpdate error:", err)
	}
}

func onDeploymentDelete(obj interface{}) {
	deploy := obj.(*appsv1.Deployment)
	labels := deploy.GetLabels()
	serviceName := labels[setting.ServiceLabel]
	product, continueFlag := GetProductAndFilterNs(deploy.Namespace, deploy.Name, serviceName)
	if !continueFlag {
		return
	}
	opts := &commonrepo.DeleteEnvSvcDependOption{
		ProductName:   product.ProductName,
		EnvName:       product.EnvName,
		ServiceName:   serviceName,
		ServiceModule: deploy.Name,
	}
	if err := commonrepo.NewEnvSvcDependColl().Delete(opts); err != nil {
		log.Error("onDeploymentDelete EnvSvcDepend Delete error:", err)
	}
}

func VisitDeployment(deployment *appsv1.Deployment) (sets.String, sets.String, sets.String) {
	cfgSets := sets.NewString()
	secretSets := sets.NewString()
	pvcSets := sets.NewString()
	for _, initCon := range deployment.Spec.Template.Spec.InitContainers {
		cfg, sec := visitContainerConfigmapAndSecretNames(initCon)
		cfgSets = cfgSets.Union(cfg)
		secretSets = secretSets.Union(sec)
	}
	for _, con := range deployment.Spec.Template.Spec.Containers {
		cfg, sec := visitContainerConfigmapAndSecretNames(con)
		cfgSets = cfgSets.Union(cfg)
		secretSets = secretSets.Union(sec)
	}
	for _, ephemeralCon := range deployment.Spec.Template.Spec.EphemeralContainers {
		cfg, sec := visitContainerConfigmapAndSecretNames(corev1.Container(ephemeralCon.DeepCopy().EphemeralContainerCommon))
		cfgSets = cfgSets.Union(cfg)
		secretSets = secretSets.Union(sec)
	}
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		cfg, sec, pvc := visitVolumeConfigmapAndSecretAndPvcNames(volume)
		cfgSets = cfgSets.Union(cfg)
		secretSets = secretSets.Union(sec)
		pvcSets = pvcSets.Union(pvc)
	}
	for _, sec := range deployment.Spec.Template.Spec.ImagePullSecrets {
		secretSets.Insert(sec.Name)
	}
	return cfgSets, secretSets, pvcSets
}

func onStatefulSetAddAndUpdate(obj interface{}) {
	sts := obj.(*appsv1.StatefulSet)
	labels := sts.GetLabels()
	serviceName := labels[setting.ServiceLabel]
	product, continueFlag := GetProductAndFilterNs(sts.Namespace, sts.Name, serviceName)
	if !continueFlag {
		return
	}

	configMaps, secrets, pvcs := VisitStatefulSet(sts)
	envSvcDepend := &models.EnvSvcDepend{
		ProductName:   product.ProductName,
		EnvName:       product.EnvName,
		Namespace:     sts.Namespace,
		ServiceName:   serviceName,
		ServiceModule: sts.Name,
		WorkloadType:  setting.StatefulSet,
		ConfigMaps:    configMaps.List(),
		Pvcs:          pvcs.List(),
		Secrets:       secrets.List(),
	}

	if err := commonrepo.NewEnvSvcDependColl().CreateOrUpdate(envSvcDepend); err != nil {
		log.Error("onStatefulSetAdd EnvSvcDepend CreateOrUpdate error:", err)
	}
}

func onStatefulSetDelete(obj interface{}) {
	sts := obj.(*appsv1.StatefulSet)
	labels := sts.GetLabels()
	serviceName := labels[setting.ServiceLabel]
	product, continueFlag := GetProductAndFilterNs(sts.Namespace, sts.Name, serviceName)
	if !continueFlag {
		return
	}
	opts := &commonrepo.DeleteEnvSvcDependOption{
		ProductName:   product.ProductName,
		EnvName:       product.EnvName,
		ServiceName:   serviceName,
		ServiceModule: sts.Name,
	}
	if err := commonrepo.NewEnvSvcDependColl().Delete(opts); err != nil {
		log.Error("onStatefulSetDelete EnvSvcDepend Delete error:", err)
	}
}

func VisitStatefulSet(sts *appsv1.StatefulSet) (sets.String, sets.String, sets.String) {
	cfgSets := sets.NewString()
	secretSets := sets.NewString()
	pvcSets := sets.NewString()
	for _, initCon := range sts.Spec.Template.Spec.InitContainers {
		cfg, sec := visitContainerConfigmapAndSecretNames(initCon)
		cfgSets = cfgSets.Union(cfg)
		secretSets = secretSets.Union(sec)
	}
	for _, con := range sts.Spec.Template.Spec.Containers {
		cfg, sec := visitContainerConfigmapAndSecretNames(con)
		cfgSets = cfgSets.Union(cfg)
		secretSets = secretSets.Union(sec)
	}
	for _, ephemeralCon := range sts.Spec.Template.Spec.EphemeralContainers {
		cfg, sec := visitContainerConfigmapAndSecretNames(corev1.Container(ephemeralCon.DeepCopy().EphemeralContainerCommon))
		cfgSets = cfgSets.Union(cfg)
		secretSets = secretSets.Union(sec)
	}
	for _, volume := range sts.Spec.Template.Spec.Volumes {
		cfg, sec, pvc := visitVolumeConfigmapAndSecretAndPvcNames(volume)
		cfgSets = cfgSets.Union(cfg)
		secretSets = secretSets.Union(sec)
		pvcSets = pvcSets.Union(pvc)
	}
	for _, sec := range sts.Spec.Template.Spec.ImagePullSecrets {
		secretSets.Insert(sec.Name)
	}
	return cfgSets, secretSets, pvcSets
}

func GetProductAndFilterNs(namespace, workloadName, svcName string) (*models.Product, bool) {
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Namespace: namespace})
	if err != nil {
		if !commonrepo.IsErrNoDocuments(err) {
			log.Error("GetProductAndFilterNs err:", err)
		}
		return nil, false
	}

	for _, product := range products {
		if !product.IsExisted {
			clientset, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(product.ClusterID)
			if err != nil {
				log.Errorf("failed to create kubernetes clientset for clusterID: %s, the error is: %s", product.ClusterID, err)
				return nil, false
			}
			ns, err := clientset.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
			if err != nil {
				return nil, false
			}
			labels := ns.GetLabels()
			if val, ok := labels[setting.EnvCreatedBy]; !ok || val != setting.EnvCreator {
				return nil, false
			}
		}
		for _, psvc := range product.GetServiceMap() {
			if psvc.Type == setting.K8SDeployType && (psvc.ServiceName == workloadName || psvc.ServiceName == svcName) {
				return product, true
			}
		}
	}
	return nil, false
}

func visitContainerConfigmapAndSecretNames(container corev1.Container) (sets.String, sets.String) {
	cfgSets := sets.NewString()
	secretSets := sets.NewString()
	for _, env := range container.EnvFrom {
		if env.ConfigMapRef != nil {
			cfgSets.Insert(env.ConfigMapRef.Name)
		}
		if env.SecretRef != nil {
			secretSets.Insert(env.SecretRef.Name)
		}
	}
	for _, envVar := range container.Env {
		if envVar.ValueFrom != nil && envVar.ValueFrom.ConfigMapKeyRef != nil {
			cfgSets.Insert(envVar.ValueFrom.ConfigMapKeyRef.Name)
		}
		if envVar.ValueFrom != nil && envVar.ValueFrom.SecretKeyRef != nil {
			secretSets.Insert(envVar.ValueFrom.SecretKeyRef.Name)
		}
	}
	return cfgSets, secretSets
}

func visitVolumeConfigmapAndSecretAndPvcNames(volume corev1.Volume) (sets.String, sets.String, sets.String) {
	cfgSets := sets.NewString()
	secretSets := sets.NewString()
	pvcSets := sets.NewString()
	source := &volume.VolumeSource
	switch {
	case source.Projected != nil:
		for j := range source.Projected.Sources {
			if source.Projected.Sources[j].ConfigMap != nil {
				cfgSets.Insert(source.Projected.Sources[j].ConfigMap.Name)
			}
			if source.Projected.Sources[j].Secret != nil {
				secretSets.Insert(source.Projected.Sources[j].Secret.Name)
			}
		}
	case source.ConfigMap != nil:
		cfgSets.Insert(source.ConfigMap.Name)
	case source.Secret != nil:
		secretSets.Insert(source.Secret.SecretName)
	case source.PersistentVolumeClaim != nil:
		pvcSets.Insert(source.PersistentVolumeClaim.ClaimName)
	}
	return cfgSets, secretSets, pvcSets
}

func StartClusterInformer() {
	for {
		k8sClusters, err := mongodb.NewK8SClusterColl().List(nil)
		if err != nil && !mongodb.IsErrNoDocuments(err) {
			log.Error("StartClusterInformer list cluster error:%s", err)
			return
		}

		for _, cluster := range k8sClusters {
			if cluster.Status != setting.Normal {
				DeleteClusterInformer(cluster.ID.Hex())
				continue
			}
			clientset, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(cluster.ID.Hex())
			if err != nil {
				log.Errorf("failed to create kubernetes clientset for clusterID: %s, the error is: %s", cluster.ID.Hex(), err)
				continue
			}

			_, err = NewClusterInformerFactory(cluster.ID.Hex(), clientset)
			if err != nil {
				log.Errorf("failed to NewClusterInformerFactory clusterID: %s, the error is: %s", cluster.ID.Hex(), err)
				continue
			}
		}

		deleteClustInf := func(key, value interface{}) bool {
			for _, cluster := range k8sClusters {
				if key.(string) == cluster.ID.Hex() {
					return true
				}
			}
			DeleteClusterInformer(key.(string))
			return true
		}
		ClusterInformersMap.Range(deleteClustInf)
	}
}
