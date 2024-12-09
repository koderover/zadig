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
	"sync"

	istioV1Alpha3 "istio.io/client-go/pkg/clientset/versioned/typed/networking/v1alpha3"
	"k8s.io/client-go/kubernetes"
	metricsV1Beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	controllerRuntimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	aslanClient "github.com/koderover/zadig/v2/pkg/shared/client/aslan"
	"github.com/koderover/zadig/v2/pkg/tool/kube/multicluster"
)

var instance *KubeClientManager
var once sync.Once

// TODO: Implement a Zadig-Kubernetes client interface, forbid business code to access these clients directly

type KubeClientManager struct {
	controllerRuntimeClientMap sync.Map
	kubernetesClientSetMap     sync.Map
	metricsClientMap           sync.Map
	istioClientMap             sync.Map

	clientReaderMap     sync.Map
	informerStopChanMap sync.Map
	informerFactoryMap  sync.Map

	mutex sync.Mutex
}

func NewKubeClientManager() *KubeClientManager {
	once.Do(func() {
		instance = &KubeClientManager{}
	})
	return instance
}

func (cm *KubeClientManager) GetControllerRuntimeClient(clusterID string) (controllerRuntimeClient.Client, error) {
	clusterID = handleClusterID(clusterID)

	client, ok := cm.controllerRuntimeClientMap.Load(clusterID)
	if ok {
		return client.(controllerRuntimeClient.Client), nil
	}

	if clusterID == setting.LocalClusterID {
		cli, err := multicluster.GetKubeClient(config.HubServerServiceAddress(), clusterID)
		if err == nil {
			cm.controllerRuntimeClientMap.Store(clusterID, cli)
		}
		return cli, err
	}

	cluster, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	if cluster.Status != setting.Normal {
		return nil, fmt.Errorf("unable to connect to cluster: %s, status: %s", cluster.Name, cluster.Status)
	}

	switch cluster.Type {
	case setting.AgentClusterType, "":
		cli, err := multicluster.GetKubeClient(config.HubServerServiceAddress(), clusterID)
		if err == nil {
			cm.controllerRuntimeClientMap.Store(clusterID, cli)
		}
		return cli, err
	case setting.KubeConfigClusterType:
		cli, err := multicluster.GetKubeClientFromKubeConfig(clusterID, cluster.KubeConfig)
		if err == nil {
			cm.controllerRuntimeClientMap.Store(cluster, cli)
		}
		return cli, err
	default:
		return nil, fmt.Errorf("failed to create kubeclient: unknown cluster type: %s", cluster.Type)
	}
}

func (cm *KubeClientManager) GetKubernetesClientSet(clusterID string) (*kubernetes.Clientset, error) {
	clusterID = handleClusterID(clusterID)

	client, ok := cm.kubernetesClientSetMap.Load(clusterID)
	if ok {
		return client.(*kubernetes.Clientset), nil
	}

	cluster, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	if cluster.Status != setting.Normal {
		return nil, fmt.Errorf("unable to connect to cluster: %s, status: %s", cluster.Name, cluster.Status)
	}

	switch cluster.Type {
	case setting.AgentClusterType, "":
		cli, err := multicluster.GetKubeClientSet(config.HubServerServiceAddress(), clusterID)
		if err == nil {
			cm.kubernetesClientSetMap.Store(clusterID, cli)
		}
		return cli, err
	case setting.KubeConfigClusterType:
		cli, err := multicluster.GetKubeClientSetFromKubeConfig(clusterID, cluster.KubeConfig)
		if err == nil {
			cm.kubernetesClientSetMap.Store(clusterID, cli)
		}
		return cli, err
	default:
		return nil, fmt.Errorf("failed to create kubeclient: unknown cluster type: %s", cluster.Type)
	}
}

func (cm *KubeClientManager) GetKubernetesMetricsClient(clusterID string) (*metricsV1Beta1.MetricsV1beta1Client, error) {
	clusterID = handleClusterID(clusterID)

	client, ok := cm.metricsClientMap.Load(clusterID)
	if ok {
		return client.(*metricsV1Beta1.MetricsV1beta1Client), nil
	}

	cluster, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	if cluster.Status != setting.Normal {
		return nil, fmt.Errorf("unable to connect to cluster: %s, status: %s", cluster.Name, cluster.Status)
	}

	switch cluster.Type {
	case setting.AgentClusterType, "":
		cli, err := multicluster.GetKubeMetricsClient(config.HubServerServiceAddress(), clusterID)
		if err == nil {
			cm.metricsClientMap.Store(clusterID, cli)
		}
		return cli, err
	case setting.KubeConfigClusterType:
		cli, err := multicluster.GetKubeMetricsClientFromKubeConfig(clusterID, cluster.KubeConfig)
		if err == nil {
			cm.metricsClientMap.Store(clusterID, cli)
		}
		return cli, err
	default:
		return nil, fmt.Errorf("failed to create kubeclient: unknown cluster type: %s", cluster.Type)
	}
}

func (cm *KubeClientManager) GetIstioV1Alpha3Client(clusterID string) (*istioV1Alpha3.NetworkingV1alpha3Client, error) {
	clusterID = handleClusterID(clusterID)

	client, ok := cm.istioClientMap.Load(clusterID)
	if ok {
		return client.(*istioV1Alpha3.NetworkingV1alpha3Client), nil
	}

	cluster, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	if cluster.Status != setting.Normal {
		return nil, fmt.Errorf("unable to connect to cluster: %s, status: %s", cluster.Name, cluster.Status)
	}

	switch cluster.Type {
	case setting.AgentClusterType, "":
		cli, err := multicluster.GetIstioV1Alpha3Client(config.HubServerServiceAddress(), clusterID)
		if err == nil {
			cm.istioClientMap.Store(clusterID, cli)
		}
		return cli, err
	case setting.KubeConfigClusterType:
		cli, err := multicluster.GetIstioV1Alpha3ClientFromKubeConfig(clusterID, cluster.KubeConfig)
		if err == nil {
			cm.istioClientMap.Store(clusterID, cli)
		}
		return cli, err
	default:
		return nil, fmt.Errorf("failed to create kubeclient: unknown cluster type: %s", cluster.Type)
	}
}

// handleClusterID is a data compatibility function that exchange the empty cluster id to a fixed local cluster id
func handleClusterID(clusterID string) string {
	if clusterID == "" {
		return setting.LocalClusterID
	}

	return clusterID
}
