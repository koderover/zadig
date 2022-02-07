package informer

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/koderover/zadig/pkg/setting"
)

var InformersMap sync.Map

// NewInformer initialize and start an informer for specific namespace in given cluster.
// Currently the informer will NOT stop unless the service is down
// If you want to watch a new resource, remember to register here
// Current list:
// - Deployment
// - StatefulSet
// - Service
// - Pod
// - Ingress (extentions/v1beta1) <- as of version 1.9.0, this is the resource we watch
func NewInformer(clusterID, namespace string, cls *kubernetes.Clientset) informers.SharedInformerFactory {
	// this is a stupid compatibility code
	if clusterID == "" {
		clusterID = setting.LocalClusterID
	}
	key := fmt.Sprintf("%s-%s", clusterID, namespace)
	if informer, ok := InformersMap.Load(key); ok {
		return informer.(informers.SharedInformerFactory)
	}
	opts := informers.WithNamespace(namespace)
	informerFactory := informers.NewSharedInformerFactoryWithOptions(cls, time.Minute, opts)
	// register the resources to be watched
	informerFactory.Apps().V1().Deployments().Lister()
	informerFactory.Apps().V1().StatefulSets().Lister()
	informerFactory.Core().V1().Services().Lister()
	informerFactory.Core().V1().Pods().Lister()
	informerFactory.Extensions().V1beta1().Ingresses().Lister()
	// Never stops unless the pod is killed
	informerFactory.Start(make(<-chan struct{}))
	// wait for the cache to be synced for the first time
	informerFactory.WaitForCacheSync(make(<-chan struct{}))
	InformersMap.Store(key, informerFactory)
	return informerFactory
}
