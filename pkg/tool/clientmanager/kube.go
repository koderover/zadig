/*
Copyright 2024 The KodeRover Authors.

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

package clientmanager

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/log"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/pkg/errors"
	istioClient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	metricsV1Beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerRuntimeCache "sigs.k8s.io/controller-runtime/pkg/cache"
	controllerRuntimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	controllerRuntimeCluster "sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/koderover/zadig/v2/pkg/config"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	aslanClient "github.com/koderover/zadig/v2/pkg/shared/client/aslan"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	kruiseclientset "github.com/openkruise/kruise-api/client/clientset/versioned"
)

var kubeClientManagerInstance *KubeClientManager
var once sync.Once

var stopContext = ctrl.SetupSignalHandler()

// TODO: Implement a Zadig-Kubernetes client interface, forbid business code to access these clients directly

const (
	defaultInformerIdleTimeout = 60 * time.Minute
	informerReaperInterval     = 5 * time.Minute
	informerSyncTimeout        = 10 * time.Second
)

type informerEntry struct {
	clusterID string
	namespace string
	factory   informers.SharedInformerFactory
	stopCh    chan struct{}
	lastUsed  time.Time
	refCount  int
	closed    bool
}

type KubeClientManager struct {
	controllerRuntimeClusterMap sync.Map
	kubernetesClientSetMap      sync.Map
	kruiseClientMap             sync.Map
	metricsClientMap            sync.Map
	istioClientSetMap           sync.Map

	informerEntryMap sync.Map
	informerMutex    sync.Mutex
	informerReaper   sync.Once

	generalMutex sync.Mutex
}

func NewKubeClientManager() *KubeClientManager {
	once.Do(func() {
		kubeClientManagerInstance = &KubeClientManager{}
	})
	return kubeClientManagerInstance
}

func (cm *KubeClientManager) GetControllerRuntimeClient(clusterID string) (controllerRuntimeClient.Client, error) {
	clusterID = handleClusterID(clusterID)

	cls, err := cm.getControllerRuntimeCluster(clusterID)
	if err != nil {
		return nil, err
	}

	return cls.GetClient(), nil
}

func (cm *KubeClientManager) GetControllerRuntimeAPIReader(clusterID string) (controllerRuntimeClient.Reader, error) {
	clusterID = handleClusterID(clusterID)

	cls, err := cm.getControllerRuntimeCluster(clusterID)
	if err != nil {
		return nil, err
	}

	return cls.GetAPIReader(), nil
}

func (cm *KubeClientManager) GetKubernetesClientSet(clusterID string) (*kubernetes.Clientset, error) {
	clusterID = handleClusterID(clusterID)

	client, ok := cm.kubernetesClientSetMap.Load(clusterID)
	if ok {
		return client.(*kubernetes.Clientset), nil
	}

	if clusterID == setting.LocalClusterID {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		disableKeepAlive(cfg)
		cli, err := kubernetes.NewForConfig(cfg)
		if err == nil {
			cm.kubernetesClientSetMap.Store(clusterID, cli)
		}
		return cli, err
	}

	clusterInfo, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
	if err != nil {
		return nil, err
	}
	if clusterInfo == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	if clusterInfo.Status != setting.Normal {
		return nil, fmt.Errorf("unable to connect to cluster: %s, status: %s", clusterInfo.Name, clusterInfo.Status)
	}

	var cfg *rest.Config

	switch clusterInfo.Type {
	case setting.AgentClusterType, "":
		cfg = generateAgentRestConfig(clusterID, config.HubServerServiceAddress())
	case setting.KubeConfigClusterType:
		cfg, err = generateRestConfigFromKubeConfig(clusterInfo.KubeConfig)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("failed to create kubeclient: unknown cluster type: %s", clusterInfo.Type)
	}

	disableKeepAlive(cfg)
	cli, err := kubernetes.NewForConfig(cfg)
	if err == nil {
		cm.kubernetesClientSetMap.Store(clusterID, cli)
	}

	return cli, err
}

func (cm *KubeClientManager) GetKruiseClient(clusterID string) (kruiseclientset.Interface, error) {
	clusterID = handleClusterID(clusterID)

	client, ok := cm.kruiseClientMap.Load(clusterID)
	if ok {
		return client.(*kruiseclientset.Clientset), nil
	}

	if clusterID == setting.LocalClusterID {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		disableKeepAlive(cfg)
		cli, err := kruiseclientset.NewForConfig(cfg)
		if err == nil {
			cm.kruiseClientMap.Store(clusterID, cli)
		}
		return cli, err
	}

	clusterInfo, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
	if err != nil {
		return nil, err
	}
	if clusterInfo == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	if clusterInfo.Status != setting.Normal {
		return nil, fmt.Errorf("unable to connect to cluster: %s, status: %s", clusterInfo.Name, clusterInfo.Status)
	}

	var cfg *rest.Config
	switch clusterInfo.Type {
	case setting.AgentClusterType, "":
		cfg = generateAgentRestConfig(clusterID, config.HubServerServiceAddress())
	case setting.KubeConfigClusterType:
		cfg, err = generateRestConfigFromKubeConfig(clusterInfo.KubeConfig)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("failed to create kruise client: unknown cluster type: %s", clusterInfo.Type)
	}

	disableKeepAlive(cfg)
	cli, err := kruiseclientset.NewForConfig(cfg)
	if err == nil {
		cm.kruiseClientMap.Store(clusterID, cli)
	}

	return cli, err
}

func (cm *KubeClientManager) GetKubernetesMetricsClient(clusterID string) (*metricsV1Beta1.MetricsV1beta1Client, error) {
	clusterID = handleClusterID(clusterID)

	client, ok := cm.metricsClientMap.Load(clusterID)
	if ok {
		return client.(*metricsV1Beta1.MetricsV1beta1Client), nil
	}

	if clusterID == setting.LocalClusterID {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		disableKeepAlive(cfg)
		cli, err := metricsV1Beta1.NewForConfig(cfg)
		if err == nil {
			cm.metricsClientMap.Store(clusterID, cli)
		}
		return cli, err
	}

	clusterInfo, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
	if err != nil {
		return nil, err
	}
	if clusterInfo == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	if clusterInfo.Status != setting.Normal {
		return nil, fmt.Errorf("unable to connect to cluster: %s, status: %s", clusterInfo.Name, clusterInfo.Status)
	}

	var cfg *rest.Config

	switch clusterInfo.Type {
	case setting.AgentClusterType, "":
		cfg = generateAgentRestConfig(clusterID, config.HubServerServiceAddress())
	case setting.KubeConfigClusterType:
		cfg, err = generateRestConfigFromKubeConfig(clusterInfo.KubeConfig)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("failed to create kubeclient: unknown cluster type: %s", clusterInfo.Type)
	}

	disableKeepAlive(cfg)
	cli, err := metricsV1Beta1.NewForConfig(cfg)
	if err == nil {
		cm.metricsClientMap.Store(clusterID, cli)
	}

	return cli, err
}

func (cm *KubeClientManager) GetIstioClientSet(clusterID string) (*istioClient.Clientset, error) {
	clusterID = handleClusterID(clusterID)

	client, ok := cm.istioClientSetMap.Load(clusterID)
	if ok {
		return client.(*istioClient.Clientset), nil
	}

	if clusterID == setting.LocalClusterID {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		disableKeepAlive(cfg)
		cli, err := istioClient.NewForConfig(cfg)
		if err == nil {
			cm.istioClientSetMap.Store(clusterID, cli)
		}
		return cli, err
	}

	clusterInfo, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
	if err != nil {
		return nil, err
	}
	if clusterInfo == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	if clusterInfo.Status != setting.Normal {
		return nil, fmt.Errorf("unable to connect to cluster: %s, status: %s", clusterInfo.Name, clusterInfo.Status)
	}

	var cfg *rest.Config

	switch clusterInfo.Type {
	case setting.AgentClusterType, "":
		cfg = generateAgentRestConfig(clusterID, config.HubServerServiceAddress())
	case setting.KubeConfigClusterType:
		cfg, err = generateRestConfigFromKubeConfig(clusterInfo.KubeConfig)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("failed to create kubeclient: unknown cluster type: %s", clusterInfo.Type)
	}

	disableKeepAlive(cfg)
	cli, err := istioClient.NewForConfig(cfg)
	if err == nil {
		cm.istioClientSetMap.Store(clusterID, cli)
	}

	return cli, err
}

func (cm *KubeClientManager) GetInformer(clusterID, namespace string) (informers.SharedInformerFactory, error) {
	clusterID = handleClusterID(clusterID)
	key := generateInformerKey(clusterID, namespace)
	cm.startInformerReaper()

	cm.informerMutex.Lock()
	if entry, ok := cm.loadInformerEntryLocked(key); ok {
		entry.lastUsed = time.Now()
		factory := entry.factory
		cm.informerMutex.Unlock()
		return factory, nil
	}
	cm.informerMutex.Unlock()

	informerFactory, stopCh, err := cm.createInformerFactory(clusterID, namespace)
	if err != nil {
		return nil, err
	}

	cm.informerMutex.Lock()
	defer cm.informerMutex.Unlock()
	if entry, ok := cm.loadInformerEntryLocked(key); ok {
		close(stopCh)
		entry.lastUsed = time.Now()
		return entry.factory, nil
	}

	cm.informerEntryMap.Store(key, &informerEntry{
		clusterID: clusterID,
		namespace: namespace,
		factory:   informerFactory,
		stopCh:    stopCh,
		lastUsed:  time.Now(),
	})

	return informerFactory, nil
}

func (cm *KubeClientManager) AcquireInformer(clusterID, namespace string) (informers.SharedInformerFactory, func(), error) {
	clusterID = handleClusterID(clusterID)

	factory, err := cm.GetInformer(clusterID, namespace)
	if err != nil {
		return nil, nil, err
	}

	key := generateInformerKey(clusterID, namespace)
	cm.informerMutex.Lock()
	entry, ok := cm.loadInformerEntryLocked(key)
	if !ok {
		cm.informerMutex.Unlock()
		return nil, nil, fmt.Errorf("informer %s is not registered", key)
	}
	entry.refCount++
	entry.lastUsed = time.Now()
	cm.informerMutex.Unlock()

	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			cm.informerMutex.Lock()
			defer cm.informerMutex.Unlock()
			entry, ok := cm.loadInformerEntryLocked(key)
			if !ok {
				return
			}
			if entry.refCount > 0 {
				entry.refCount--
			}
			entry.lastUsed = time.Now()
		})
	}

	return factory, release, nil
}

func (cm *KubeClientManager) createInformerFactory(clusterID, namespace string) (informers.SharedInformerFactory, chan struct{}, error) {
	opts := []informers.SharedInformerOption{
		informers.WithNamespace(namespace),
		informers.WithTransform(controllerRuntimeCache.TransformStripManagedFields()),
	}
	clientset, err := cm.GetKubernetesClientSet(clusterID)
	if err != nil {
		return nil, nil, err
	}

	informerFactory := informers.NewSharedInformerFactoryWithOptions(clientset, time.Minute, opts...)
	// register the resources to be watched
	informerFactory.Apps().V1().Deployments().Lister()
	informerFactory.Apps().V1().StatefulSets().Lister()
	informerFactory.Core().V1().Services().Lister()
	informerFactory.Core().V1().Pods().Lister()
	informerFactory.Core().V1().ConfigMaps().Lister()
	informerFactory.Batch().V1().Jobs().Lister()
	versionInfo, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, nil, err
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

	stopCh := make(chan struct{})
	informerFactory.Start(stopCh)
	// wait for the cache to be synced for the first time, with a timeout to
	// prevent blocking indefinitely when the cluster is unreachable or RBAC
	// prevents list/watch operations.
	syncCtx, syncCancel := context.WithTimeout(context.Background(), informerSyncTimeout)
	defer syncCancel()
	informerFactory.WaitForCacheSync(syncCtx.Done())

	return informerFactory, stopCh, nil
}

func (cm *KubeClientManager) DeleteInformer(clusterID, namespace string) {
	clusterID = handleClusterID(clusterID)
	cm.informerMutex.Lock()
	defer cm.informerMutex.Unlock()

	cm.deleteInformerEntriesLocked(func(entry *informerEntry) bool {
		return entry.clusterID == clusterID && entry.namespace == namespace
	})
}

// GetSPDYExecutor does not return singleton since this kind of client is not commonly reused.
func (cm *KubeClientManager) GetSPDYExecutor(clusterID string, URL *url.URL) (remotecommand.Executor, error) {
	clusterID = handleClusterID(clusterID)
	var cfg *rest.Config
	var err error

	if clusterID == setting.LocalClusterID {
		cfg, err = rest.InClusterConfig()
	} else {
		clusterInfo, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
		if err != nil {
			return nil, err
		}
		if clusterInfo == nil {
			return nil, fmt.Errorf("cluster %s not found", clusterID)
		}

		if clusterInfo.Status != setting.Normal {
			return nil, fmt.Errorf("unable to connect to cluster: %s, status: %s", clusterInfo.Name, clusterInfo.Status)
		}

		switch clusterInfo.Type {
		case setting.AgentClusterType, "":
			cfg = generateAgentRestConfig(clusterID, config.HubServerServiceAddress())
		case setting.KubeConfigClusterType:
			cfg, err = generateRestConfigFromKubeConfig(clusterInfo.KubeConfig)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("failed to create kubeclient: unknown cluster type: %s", clusterInfo.Type)
		}
	}

	if err != nil {
		return nil, err
	}

	disableKeepAlive(cfg)
	return remotecommand.NewSPDYExecutor(cfg, http.MethodPost, URL)
}

// GetRestConfig should not be used by other package. TODO: DELETE THIS FUNCTION AND CHANGE THE CALLING FUNCTION
func (cm *KubeClientManager) GetRestConfig(clusterID string) (*rest.Config, error) {
	clusterID = handleClusterID(clusterID)

	if clusterID == setting.LocalClusterID {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		disableKeepAlive(cfg)
		return cfg, nil
	}

	clusterInfo, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
	if err != nil {
		return nil, err
	}
	if clusterInfo == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	var cfg *rest.Config

	switch clusterInfo.Type {
	case setting.AgentClusterType, "":
		cfg = generateAgentRestConfig(clusterID, config.HubServerServiceAddress())
	case setting.KubeConfigClusterType:
		cfg, err = generateRestConfigFromKubeConfig(clusterInfo.KubeConfig)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("failed to create kubeclient: unknown cluster type: %s", clusterInfo.Type)
	}

	disableKeepAlive(cfg)
	return cfg, err
}

func (cm *KubeClientManager) Clear(clusterID string) error {
	clusterID = handleClusterID(clusterID)

	cm.generalMutex.Lock()
	defer cm.generalMutex.Unlock()

	cm.controllerRuntimeClusterMap.Delete(clusterID)
	cm.kubernetesClientSetMap.Delete(clusterID)
	cm.kruiseClientMap.Delete(clusterID)
	cm.metricsClientMap.Delete(clusterID)
	cm.istioClientSetMap.Delete(clusterID)

	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		ClusterID: clusterID,
	})

	if err != nil {
		return fmt.Errorf("failed to list envs by clusterID %s, err %v", clusterID, err)
	}

	for _, env := range envs {
		cm.informerMutex.Lock()
		cm.deleteInformerEntriesLocked(func(entry *informerEntry) bool {
			return entry.clusterID == clusterID && entry.namespace == env.Namespace
		})
		cm.informerMutex.Unlock()
	}

	return nil
}

func (cm *KubeClientManager) getControllerRuntimeCluster(clusterID string) (controllerRuntimeCluster.Cluster, error) {
	clusterID = handleClusterID(clusterID)

	cls, ok := cm.controllerRuntimeClusterMap.Load(clusterID)
	if ok {
		return cls.(controllerRuntimeCluster.Cluster), nil
	}

	if clusterID == setting.LocalClusterID {
		cfg := ctrl.GetConfigOrDie()
		disableKeepAlive(cfg)
		controllerClient, err := createControllerRuntimeCluster(cfg)
		if err == nil {
			go func() {
				if err := controllerClient.Start(stopContext); err != nil {
					log.Errorf("failed to start controller runtime cluster, error: %s", err)
				}
			}()
			if !controllerClient.GetCache().WaitForCacheSync(context.Background()) {
				return nil, fmt.Errorf("failed to wait for controller runtime cluster to sync")
			}
			cm.controllerRuntimeClusterMap.Store(clusterID, controllerClient)
		}
		return controllerClient, err
	}

	clusterInfo, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
	if err != nil {
		return nil, err
	}
	if clusterInfo == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	if clusterInfo.Status != setting.Normal {
		return nil, fmt.Errorf("unable to connect to cluster: %s, status: %s", clusterInfo.Name, clusterInfo.Status)
	}

	var cfg *rest.Config

	switch clusterInfo.Type {
	case setting.AgentClusterType, "":
		cfg = generateAgentRestConfig(clusterID, config.HubServerServiceAddress())
	case setting.KubeConfigClusterType:
		cfg, err = generateRestConfigFromKubeConfig(clusterInfo.KubeConfig)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("failed to create kubeclient: unknown cluster type: %s", clusterInfo.Type)
	}

	disableKeepAlive(cfg)
	controllerClient, err := createControllerRuntimeCluster(cfg)
	if err == nil {
		go func() {
			if err := controllerClient.Start(stopContext); err != nil {
				log.Errorf("failed to start controller runtime cluster, error: %s", err)
			}
		}()

		if !controllerClient.GetCache().WaitForCacheSync(context.Background()) {
			return nil, fmt.Errorf("failed to wait for controller runtime cluster to sync")
		}
		cm.controllerRuntimeClusterMap.Store(clusterID, controllerClient)
	}
	return controllerClient, err
}

// handleClusterID is a data compatibility function that exchange the empty cluster id to a fixed local cluster id
func handleClusterID(clusterID string) string {
	if clusterID == "" {
		return setting.LocalClusterID
	}

	return clusterID
}

func generateAgentRestConfig(clusterID, hubServerAddr string) *rest.Config {
	return &rest.Config{
		Host: fmt.Sprintf("%s/kube/%s", hubServerAddr, clusterID),
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
}

func generateRestConfigFromKubeConfig(kubeConfig string) (*rest.Config, error) {
	tmpFile, err := os.CreateTemp("", "")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	kubeConfigByte := []byte(kubeConfig)
	err = os.WriteFile(tmpFile.Name(), kubeConfigByte, 0777)
	if err != nil {
		return nil, err
	}

	return clientcmd.BuildConfigFromFlags("", tmpFile.Name())
}

func createControllerRuntimeCluster(restConfig *rest.Config) (controllerRuntimeCluster.Cluster, error) {
	scheme := runtime.NewScheme()

	// add all known types
	// if you want to support custom types, call _ = yourCustomAPIGroup.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)
	_ = kruise.AddToScheme(scheme)

	c, err := controllerRuntimeCluster.New(restConfig, func(clusterOptions *controllerRuntimeCluster.Options) {
		clusterOptions.Scheme = scheme
		clusterOptions.Cache.DefaultTransform = controllerRuntimeCache.TransformStripManagedFields()
		clusterOptions.Client.Cache = &controllerRuntimeClient.CacheOptions{
			DisableFor: []controllerRuntimeClient.Object{
				&corev1.Secret{},
				&corev1.Event{},
				&appsv1.ReplicaSet{},
			},
		}
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to init client")
	}

	return c, nil
}

func generateInformerKey(clusterID, namespace string) string {
	return fmt.Sprintf(setting.InformerNamingConvention, clusterID, namespace)
}

func (cm *KubeClientManager) loadInformerEntryLocked(key string) (*informerEntry, bool) {
	value, ok := cm.informerEntryMap.Load(key)
	if !ok {
		return nil, false
	}
	entry := value.(*informerEntry)
	if entry.closed {
		cm.informerEntryMap.Delete(key)
		return nil, false
	}
	return entry, true
}

func (cm *KubeClientManager) startInformerReaper() {
	cm.informerReaper.Do(func() {
		go func() {
			ticker := time.NewTicker(informerReaperInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					cm.reapIdleInformers()
				case <-stopContext.Done():
					return
				}
			}
		}()
	})
}

func (cm *KubeClientManager) reapIdleInformers() {
	idleTimeout := informerIdleTimeout()
	now := time.Now()

	cm.informerMutex.Lock()
	defer cm.informerMutex.Unlock()

	cm.deleteInformerEntriesLocked(func(entry *informerEntry) bool {
		return entry.refCount == 0 && now.Sub(entry.lastUsed) > idleTimeout
	})
}

func (cm *KubeClientManager) deleteInformerEntriesLocked(shouldDelete func(*informerEntry) bool) {
	cm.informerEntryMap.Range(func(key, value interface{}) bool {
		entry := value.(*informerEntry)
		if !entry.closed && shouldDelete(entry) {
			close(entry.stopCh)
			entry.closed = true
			cm.informerEntryMap.Delete(key)
		}
		return true
	})
}

func informerIdleTimeout() time.Duration {
	minutes := config.KubeInformerIdleTimeoutMinutes()
	if minutes <= 0 {
		return defaultInformerIdleTimeout
	}
	return time.Duration(minutes) * time.Minute
}

// disableKeepAlive configures REST config to not keep connections alive
func disableKeepAlive(cfg *rest.Config) {
	if !config.DisableKubeClientKeepAlive() {
		return
	}
	// Use WrapTransport instead of directly setting Transport to avoid conflicts with TLS configuration
	cfg.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		if transport, ok := rt.(*http.Transport); ok {
			transport.DisableKeepAlives = true
		}
		return rt
	}
}
