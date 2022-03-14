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

	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/koderover/zadig/pkg/setting"
)

var InformersMap sync.Map
var StopChanMap sync.Map

// NewInformer initialize and start an informer for specific namespace in given cluster.
// Currently the informer will NOT stop unless the service is down
// If you want to watch a new resource, remember to register here
// Current list:
// - Deployment
// - StatefulSet
// - Service
// - Pod
// - Ingress (extentions/v1beta1) <- as of version 1.9.0, this is the resource we watch
func NewInformer(clusterID, namespace string, cls *kubernetes.Clientset) (informers.SharedInformerFactory, error) {
	// this is a stupid compatibility code
	if clusterID == "" {
		clusterID = setting.LocalClusterID
	}
	key := generateInformerKey(clusterID, namespace)
	if informer, ok := InformersMap.Load(key); ok {
		return informer.(informers.SharedInformerFactory), nil
	}
	opts := informers.WithNamespace(namespace)
	informerFactory := informers.NewSharedInformerFactoryWithOptions(cls, time.Minute, opts)
	// register the resources to be watched
	informerFactory.Apps().V1().Deployments().Lister()
	informerFactory.Apps().V1().StatefulSets().Lister()
	informerFactory.Core().V1().Services().Lister()
	informerFactory.Core().V1().Pods().Lister()
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

	// stop channel will be stored for future stop
	stopchan := make(chan struct{})
	informerFactory.Start(stopchan)
	// wait for the cache to be synced for the first time
	informerFactory.WaitForCacheSync(make(chan struct{}))
	// in case there is a concurrent situation, we find if there is one informer in the map
	if _, ok := InformersMap.Load(key); ok {
		// if we found that stop channel
		if stopchan, ok := StopChanMap.Load(key); ok {
			close(stopchan.(chan struct{}))
			StopChanMap.Delete(key)
		}
	}
	InformersMap.Store(key, informerFactory)
	StopChanMap.Store(key, stopchan)
	return informerFactory, nil
}

func DeleteInformer(clusterID, namespace string) {
	key := generateInformerKey(clusterID, namespace)
	// if informer exists
	if _, ok := InformersMap.Load(key); ok {
		// if we found that stop channel
		if stopchan, ok := StopChanMap.Load(key); ok {
			close(stopchan.(chan struct{}))
			StopChanMap.Delete(key)
		}
	}
	InformersMap.Delete(key)
}

func generateInformerKey(clusterID, namespace string) string {
	return fmt.Sprintf(setting.InformerNamingConvention, clusterID, namespace)
}
