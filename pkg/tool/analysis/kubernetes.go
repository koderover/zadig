/*
Copyright 2023 The K8sGPT Authors.
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

// Some parts of this file have been modified to make it functional in Zadig

package analysis

import (
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

type Client struct {
	Client        kubernetes.Interface
	ServerVersion *version.Info
}

func (c *Client) GetClient() kubernetes.Interface {
	return c.Client
}

func NewClient(clusterID string) (*Client, error) {
	clientSet, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return nil, err
	}

	serverVersion, err := clientSet.ServerVersion()
	if err != nil {
		return nil, err
	}

	return &Client{
		Client:        clientSet,
		ServerVersion: serverVersion,
	}, nil
}
