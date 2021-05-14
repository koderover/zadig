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
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	krkubeclient "github.com/koderover/zadig/lib/tool/kube/client"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func GetKubeClient(hubServerAddr, clusterId string) (client.Client, error) {
	if clusterId == "" {
		return krkubeclient.Client(), nil
	}

	clusterService, err := NewAgent(hubServerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get clusterService: %v", err)
	}

	return clusterService.GetKubeClient(clusterId)
}

func GetKubeAPIReader(hubServerAddr, clusterId string) (client.Reader, error) {
	if clusterId == "" {
		return krkubeclient.APIReader(), nil
	}

	clusterService, err := NewAgent(hubServerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get clusterService: %v", err)
	}

	return clusterService.GetKubeClient(clusterId)
}

func GetRESTConfig(hubServerAddr, clusterId string) (*rest.Config, error) {
	if clusterId == "" {
		return krkubeclient.RESTConfig(), nil
	}

	return krkubeclient.RESTConfigFromAPIConfig(generateAPIConfig(clusterId, hubServerAddr))
}

// GetClientset returns a client to interact with APIServer which implements kubernetes.Interface
func GetClientset(hubServerAddr, clusterId string) (kubernetes.Interface, error) {
	if clusterId == "" {
		return krkubeclient.Clientset(), nil
	}

	clusterService, err := NewAgent(hubServerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get clusterService: %v", err)
	}

	return clusterService.GetClientset(clusterId)
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

func (s *Agent) DisconnectCluster(username string, clusterId string, logger *xlog.Logger) error {
	err := s.hubClient.Disconnect(clusterId)
	if err != nil {
		logger.Errorf("failed to disconnect %s by %s: %v", clusterId, username, err)
		return err
	}

	logger.Infof("cluster is %s disconnected successfully by %s", clusterId, username)
	return nil
}

func (s *Agent) ReconnectCluster(username string, clusterId string, logger *xlog.Logger) error {
	err := s.hubClient.Restore(clusterId)
	if err != nil {
		logger.Errorf("failed to restore %s by %s: %v", clusterId, username, err)
		return err
	}

	logger.Infof("cluster is %s restored successfully by %s", clusterId, username)
	return nil
}

func (s *Agent) GetKubeClient(clusterId string) (client.Client, error) {
	if err := s.hubClient.HasSession(clusterId); err != nil {
		return nil, errors.Wrapf(err, "cluster is not connected %s", clusterId)
	}

	return krkubeclient.NewClientFromAPIConfig(generateAPIConfig(clusterId, s.hubServerAddr))
}

func (s *Agent) ClusterConnected(clusterId string) bool {
	if err := s.hubClient.HasSession(clusterId); err != nil {
		return false
	}

	return true
}

func (s *Agent) GetClientset(clusterId string) (kubernetes.Interface, error) {
	if err := s.hubClient.HasSession(clusterId); err != nil {
		return nil, errors.Wrapf(err, "cluster is not connected %s", clusterId)
	}

	return krkubeclient.NewClientsetFromAPIConfig(generateAPIConfig(clusterId, s.hubServerAddr))
}

func (s *Agent) ProxyAgent(writer gin.ResponseWriter, request *http.Request) {
	s.hubClient.AgentProxy(writer, request)
}

func generateAPIConfig(clusterId, hubServerAddr string) *api.Config {
	return &api.Config{
		Kind:       "Config",
		APIVersion: "v1",
		Clusters: map[string]*api.Cluster{
			"hubserver": {
				InsecureSkipTLSVerify: true,
				Server:                fmt.Sprintf("%s/kube/%s", hubServerAddr, clusterId),
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
