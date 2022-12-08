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

package multicluster

import (
	"fmt"
	"io/ioutil"
	"istio.io/client-go/pkg/clientset/versioned/typed/networking/v1alpha3"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
)

func GetKubeClient(hubServerAddr, clusterID string) (client.Client, error) {
	if clusterID == "" {
		return krkubeclient.Client(), nil
	}

	clusterService, err := NewAgent(hubServerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get clusterService: %v", err)
	}

	return clusterService.GetKubeClient(clusterID)
}

func GetKubeClientFromKubeConfig(clusterID, kubeConfig string) (client.Client, error) {
	cfg, err := GetRestConfigFromKubeConfig(clusterID, kubeConfig)
	if err != nil {
		log.Errorf("failed to get kubeconfig from file, error: %s", err)
		return nil, err
	}

	return krkubeclient.GetKubeClientFromRestConfig(cfg)
}

func GetKubeClientSet(hubServerAddr, clusterID string) (*kubernetes.Clientset, error) {
	if clusterID == "" {
		return krkubeclient.NewClientSet()
	}

	clusterService, err := NewAgent(hubServerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get clusterService: %v", err)
	}

	return clusterService.GetClientGoKubeClient(clusterID)
}

func GetKubeClientSetFromKubeConfig(clusterID, kubeConfig string) (*kubernetes.Clientset, error) {
	cfg, err := GetRestConfigFromKubeConfig(clusterID, kubeConfig)
	if err != nil {
		log.Errorf("failed to get kubeconfig from file, error: %s", err)
		return nil, err
	}

	return kubernetes.NewForConfig(cfg)
}

func GetDynamicKubeclient(hubServerAddr, clusterID string) (dynamic.Interface, error) {
	if clusterID == "" {
		return krkubeclient.NewDynamicClient()
	}

	clusterService, err := NewAgent(hubServerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get clusterService: %v", err)
	}

	return clusterService.GetDynamicKubeClient(clusterID)
}

func GetDynamicKubeclientFromKubeConfig(clusterID, kubeConfig string) (dynamic.Interface, error) {
	cfg, err := GetRestConfigFromKubeConfig(clusterID, kubeConfig)
	if err != nil {
		log.Errorf("failed to get kubeconfig from file, error: %s", err)
		return nil, err
	}

	return dynamic.NewForConfig(cfg)
}

func GetIstioV1Alpha3Client(hubServerAddr, clusterID string) (*v1alpha3.NetworkingV1alpha3Client, error) {
	if clusterID == "" {
		return krkubeclient.NewIstioV1Alpha3Client()
	}

	clusterService, err := NewAgent(hubServerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get clusterService: %v", err)
	}

	return clusterService.GetIstioV1Alpha3Client(clusterID)
}

func GetIstioV1Alpha3ClientFromKubeConfig(clusterID, kubeConfig string) (*v1alpha3.NetworkingV1alpha3Client, error) {
	cfg, err := GetRestConfigFromKubeConfig(clusterID, kubeConfig)
	if err != nil {
		log.Errorf("failed to get kubeconfig from file, error: %s", err)
		return nil, err
	}

	return v1alpha3.NewForConfig(cfg)
}

func GetDiscoveryClient(hubServerAddr, clusterID string) (*discovery.DiscoveryClient, error) {
	if clusterID == "" {
		return krkubeclient.NewDiscoveryClient()
	}

	clusterService, err := NewAgent(hubServerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get clusterService: %v", err)
	}

	return clusterService.GetDisCoveryClient(clusterID)
}

func GetDiscoveryClientFromKubeConfig(clusterID, kubeConfig string) (*discovery.DiscoveryClient, error) {
	cfg, err := GetRestConfigFromKubeConfig(clusterID, kubeConfig)
	if err != nil {
		log.Errorf("failed to get kubeconfig from file, error: %s", err)
		return nil, err
	}

	return discovery.NewDiscoveryClientForConfig(cfg)
}

func GetKubeAPIReader(hubServerAddr, clusterID string) (client.Reader, error) {
	if clusterID == "" {
		return krkubeclient.APIReader(), nil
	}

	clusterService, err := NewAgent(hubServerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get clusterService: %v", err)
	}

	return clusterService.GetKubeClient(clusterID)
}

func GetRESTConfig(hubServerAddr, clusterID string) (*rest.Config, error) {
	if clusterID == "" {
		return krkubeclient.RESTConfig(), nil
	}

	return krkubeclient.RESTConfigFromAPIConfig(generateAPIConfig(clusterID, hubServerAddr))
}

func GetRestConfigFromKubeConfig(clusterID, kubeConfig string) (*rest.Config, error) {
	// since we cannot build a config directly from a text kubeconfig, we write it into a temporary file
	file, err := ioutil.TempFile("/", clusterID)
	if err != nil {
		log.Errorf("failed to create temporary file for kubeconfig, error: %s", err)
		return nil, err
	}
	defer os.Remove(file.Name())

	kubeConfigByte := []byte(kubeConfig)
	err = os.WriteFile(file.Name(), kubeConfigByte, 0777)
	if err != nil {
		log.Errorf("failed to write temporary file for kubeconfig, error: %s", err)
		return nil, err
	}

	return clientcmd.BuildConfigFromFlags("", file.Name())
}

// GetClientset returns a client to interact with APIServer which implements kubernetes.Interface
func GetClientset(hubServerAddr, clusterID string) (kubernetes.Interface, error) {
	if clusterID == "" {
		return krkubeclient.Clientset(), nil
	}

	clusterService, err := NewAgent(hubServerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get clusterService: %v", err)
	}

	return clusterService.GetClientset(clusterID)
}

func GetClientSetFromKubeConfig(clusterID, kubeConfig string) (kubernetes.Interface, error) {
	cfg, err := GetRestConfigFromKubeConfig(clusterID, kubeConfig)
	if err != nil {
		log.Errorf("failed to get kubeconfig from file, error: %s", err)
		return nil, err
	}

	cl, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return cl, nil
}

type Agent struct {
	hubClient     *HubClient
	hubServerAddr string
}

func NewAgent(hubServerAddr string) (*Agent, error) {
	cli, err := NewHubClient(hubServerAddr)
	if err != nil {
		return nil, err
	}

	return &Agent{
		hubClient:     cli,
		hubServerAddr: hubServerAddr,
	}, nil
}

func (s *Agent) DisconnectCluster(username string, clusterID string, logger *zap.SugaredLogger) error {
	err := s.hubClient.Disconnect(clusterID)
	if err != nil {
		logger.Errorf("failed to disconnect %s by %s: %v", clusterID, username, err)
		return err
	}

	logger.Infof("cluster is %s disconnected successfully by %s", clusterID, username)
	return nil
}

func (s *Agent) ReconnectCluster(username string, clusterID string, logger *zap.SugaredLogger) error {
	err := s.hubClient.Restore(clusterID)
	if err != nil {
		logger.Errorf("failed to restore %s by %s: %v", clusterID, username, err)
		return err
	}

	logger.Infof("cluster is %s restored successfully by %s", clusterID, username)
	return nil
}

func (s *Agent) GetKubeClient(clusterID string) (client.Client, error) {
	if err := s.hubClient.HasSession(clusterID); err != nil {
		return nil, errors.Wrapf(err, "cluster is not connected %s", clusterID)
	}

	return krkubeclient.NewClientFromAPIConfig(generateAPIConfig(clusterID, s.hubServerAddr))
}

func (s *Agent) GetClientGoKubeClient(clusterID string) (*kubernetes.Clientset, error) {
	config := generateRestConfig(clusterID, s.hubServerAddr)
	return kubernetes.NewForConfig(config)
}

func (s *Agent) GetDynamicKubeClient(clusterID string) (dynamic.Interface, error) {
	config := generateRestConfig(clusterID, s.hubServerAddr)
	return dynamic.NewForConfig(config)
}

func (s *Agent) GetIstioV1Alpha3Client(clusterID string) (*v1alpha3.NetworkingV1alpha3Client, error) {
	config := generateRestConfig(clusterID, s.hubServerAddr)
	return v1alpha3.NewForConfig(config)
}

func (s *Agent) GetDisCoveryClient(clusterID string) (*discovery.DiscoveryClient, error) {
	config := generateRestConfig(clusterID, s.hubServerAddr)
	return discovery.NewDiscoveryClientForConfig(config)
}

func (s *Agent) ClusterConnected(clusterID string) bool {
	if err := s.hubClient.HasSession(clusterID); err != nil {
		return false
	}

	return true
}

func (s *Agent) GetClientset(clusterID string) (kubernetes.Interface, error) {
	if err := s.hubClient.HasSession(clusterID); err != nil {
		return nil, errors.Wrapf(err, "cluster is not connected %s", clusterID)
	}

	return krkubeclient.NewClientsetFromAPIConfig(generateAPIConfig(clusterID, s.hubServerAddr))
}

func (s *Agent) ProxyAgent(writer gin.ResponseWriter, request *http.Request) {
	s.hubClient.AgentProxy(writer, request)
}

func generateAPIConfig(clusterID, hubServerAddr string) *api.Config {
	return &api.Config{
		Kind:       "Config",
		APIVersion: "v1",
		Clusters: map[string]*api.Cluster{
			"hubserver": {
				InsecureSkipTLSVerify: true,
				Server:                fmt.Sprintf("%s/kube/%s", hubServerAddr, clusterID),
			},
		},
		Contexts: map[string]*api.Context{
			"hubserver": {
				Cluster: "hubserver",
			},
		},
		CurrentContext: "hubserver",
	}
}

func generateRestConfig(clusterID, hubServerAddr string) *rest.Config {
	return &rest.Config{
		Host: fmt.Sprintf("%s/kube/%s", hubServerAddr, clusterID),
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
}
