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

package informer

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
)

var (
	informerMapInstance = NewNewInformerMap()
)

type InformerMap struct {
	stopchanMap        map[string]chan struct{}
	informerFactoryMap map[string]informers.SharedInformerFactory
	mutex              sync.Mutex
}

func NewNewInformerMap() *InformerMap {
	return &InformerMap{
		stopchanMap:        make(map[string]chan struct{}),
		informerFactoryMap: make(map[string]informers.SharedInformerFactory),
	}
}

func (im *InformerMap) Get(clusterID, namespace string) (informers.SharedInformerFactory, bool) {
	key := generateInformerKey(clusterID, namespace)
	im.mutex.Lock()
	defer im.mutex.Unlock()

	informer, ok := im.informerFactoryMap[key]
	if !ok {
		return nil, false
	}
	return informer, true
}

func (im *InformerMap) Set(clusterID, namespace string, informer informers.SharedInformerFactory) {
	key := generateInformerKey(clusterID, namespace)
	im.mutex.Lock()
	defer im.mutex.Unlock()

	// stop channel will be stored for future stop
	stopchan := make(chan struct{})
	informer.Start(stopchan)
	// wait for the cache to be synced for the first time
	informer.WaitForCacheSync(make(chan struct{}))

	// in case there is a concurrent situation, we find if there is one informer in the map
	if _, ok := im.informerFactoryMap[key]; ok {
		// if we found that stop channel
		if stopchan, ok := im.stopchanMap[key]; ok {
			close(stopchan)
			delete(im.stopchanMap, key)
		}
	}

	im.informerFactoryMap[key] = informer
	im.stopchanMap[key] = stopchan
}

func (im *InformerMap) Delete(clusterID, namespace string) {
	key := generateInformerKey(clusterID, namespace)
	im.mutex.Lock()
	defer im.mutex.Unlock()

	// if informer exists
	if _, ok := im.informerFactoryMap[key]; ok {
		// if we found that stop channel
		if stopchan, ok := im.stopchanMap[key]; ok {
			close(stopchan)
			delete(im.stopchanMap, key)
		}
		delete(im.informerFactoryMap, key)
	}
}

// NewInformer initialize and start an informer for specific namespace in given cluster.
// Currently the informer will NOT stop unless the service is down
// If you want to watch a new resource, remember to register here
// Current list:
// - Deployment
// - StatefulSet
// - Service
// - Pod
// - Ingress (extentions/v1beta1) <- as of version 1.9.0, this is the resource we watch
// - ConfigMap
// - Job
// - CronJob (batch/v1beta1)
func NewInformer(clusterID, namespace string, cls *kubernetes.Clientset) (informers.SharedInformerFactory, error) {
	// this is a stupid compatibility code
	if clusterID == "" {
		clusterID = setting.LocalClusterID
	}

	if informer, ok := informerMapInstance.Get(clusterID, namespace); ok {
		return informer, nil
	}

	opts := informers.WithNamespace(namespace)
	informerFactory := informers.NewSharedInformerFactoryWithOptions(cls, time.Minute, opts)
	// register the resources to be watched
	informerFactory.Apps().V1().Deployments().Lister()
	informerFactory.Apps().V1().StatefulSets().Lister()
	informerFactory.Core().V1().Services().Lister()
	informerFactory.Core().V1().Pods().Lister()
	informerFactory.Core().V1().ConfigMaps().Lister()
	informerFactory.Batch().V1().Jobs().Lister()
	versionInfo, err := cls.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}
	// if less than v1.22.0, then we look for the extensions/v1beta1 ingress
	if kubeclient.VersionLessThan122(versionInfo) {
		informerFactory.Extensions().V1beta1().Ingresses().Lister()
	} else {
		// otherwise above resource is deprecated, we watch for the k8s.networking.io/v1 ingress
		informerFactory.Networking().V1().Ingresses().Lister()
	}

	if kubeclient.VersionLessThan121(versionInfo) {
		informerFactory.Batch().V1beta1().CronJobs().Lister()
	} else {
		informerFactory.Batch().V1().CronJobs().Lister()
	}

	informerMapInstance.Set(clusterID, namespace, informerFactory)

	return informerFactory, nil
}

func DeleteInformer(clusterID, namespace string) {
	informerMapInstance.Delete(clusterID, namespace)
}

func generateInformerKey(clusterID, namespace string) string {
	return fmt.Sprintf(setting.InformerNamingConvention, clusterID, namespace)
}
