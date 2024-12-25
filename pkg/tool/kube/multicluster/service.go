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
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

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

func (s *Agent) ClusterConnected(clusterID string) bool {
	if err := s.hubClient.HasSession(clusterID); err != nil {
		return false
	}

	return true
}

func (s *Agent) ProxyAgent(writer gin.ResponseWriter, request *http.Request) {
	s.hubClient.AgentProxy(writer, request)
}
