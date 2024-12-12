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
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/pkg/errors"
	istioClient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	metricsV1Beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerRuntimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	controllerRuntimeCluster "sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	aslanClient "github.com/koderover/zadig/v2/pkg/shared/client/aslan"
)

var kubeClientManagerInstance *KubeClientManager
var once sync.Once

// TODO: Implement a Zadig-Kubernetes client interface, forbid business code to access these clients directly

type KubeClientManager struct {
	controllerRuntimeClusterMap sync.Map
	kubernetesClientSetMap      sync.Map
	metricsClientMap            sync.Map
	istioClientSetMap           sync.Map

	clientReaderMap     sync.Map
	informerStopChanMap sync.Map
	informerFactoryMap  sync.Map

	mutex sync.Mutex
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

	cli, err := kubernetes.NewForConfig(cfg)
	if err == nil {
		cm.kubernetesClientSetMap.Store(clusterID, cli)
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

	cli, err := istioClient.NewForConfig(cfg)
	if err == nil {
		cm.istioClientSetMap.Store(clusterID, cli)
	}

	return cli, err
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

	return remotecommand.NewSPDYExecutor(cfg, http.MethodPost, URL)
}

// GetRestConfig should not be used by other package. TODO: DELETE THIS FUNCTION AND CHANGE THE CALLING FUNCTION
func (cm *KubeClientManager) GetRestConfig(clusterID string) (*rest.Config, error) {
	clusterID = handleClusterID(clusterID)

	if clusterID == setting.LocalClusterID {
		return rest.InClusterConfig()
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

	return cfg, err
}

func (cm *KubeClientManager) getControllerRuntimeCluster(clusterID string) (controllerRuntimeCluster.Cluster, error) {
	clusterID = handleClusterID(clusterID)

	cls, ok := cm.controllerRuntimeClusterMap.Load(clusterID)
	if ok {
		return cls.(controllerRuntimeCluster.Cluster), nil
	}

	if clusterID == setting.LocalClusterID {
		cls, err := createControllerRuntimeCluster(ctrl.GetConfigOrDie())
		if err == nil {
			cm.controllerRuntimeClusterMap.Store(clusterID, cls)
		}
		return cls, err
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

	controllerClient, err := createControllerRuntimeCluster(cfg)
	if err == nil {
		cm.controllerRuntimeClusterMap.Store(clusterID, cls)
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

	c, err := controllerRuntimeCluster.New(restConfig, func(clusterOptions *controllerRuntimeCluster.Options) {
		clusterOptions.Scheme = scheme
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to init client")
	}

	return c, nil
}
