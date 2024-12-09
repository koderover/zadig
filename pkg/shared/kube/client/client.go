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

package client

import (
	"fmt"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	aslanClient "github.com/koderover/zadig/v2/pkg/shared/client/aslan"
	"github.com/koderover/zadig/v2/pkg/tool/kube/multicluster"
)

func GetKubeAPIReader(hubServerAddr, clusterID string) (client.Reader, error) {
	if clusterID == setting.LocalClusterID || clusterID == "" {
		if clusterID == setting.LocalClusterID {
			clusterID = ""
		}
		return multicluster.GetKubeAPIReader(hubServerAddr, clusterID)
	}
	cluster, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
	if err != nil {
		return nil, err
	}

	switch cluster.Type {
	case setting.AgentClusterType, "":
		return multicluster.GetKubeAPIReader(hubServerAddr, clusterID)
	case setting.KubeConfigClusterType:
		return multicluster.GetKubeClientFromKubeConfig(clusterID, cluster.KubeConfig)
	default:
		return nil, fmt.Errorf("failed to create kubeclient: unknown cluster type: %s", cluster.Type)
	}
}

func GetRESTConfig(hubServerAddr, clusterID string) (*rest.Config, error) {
	if clusterID == setting.LocalClusterID || clusterID == "" {
		if clusterID == setting.LocalClusterID {
			clusterID = ""
		}
		return multicluster.GetRESTConfig(hubServerAddr, clusterID)
	}
	cluster, err := aslanClient.New(config.AslanServiceAddress()).GetClusterInfo(clusterID)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	switch cluster.Type {
	case setting.AgentClusterType, "":
		return multicluster.GetRESTConfig(hubServerAddr, clusterID)
	case setting.KubeConfigClusterType:
		return multicluster.GetRestConfigFromKubeConfig(clusterID, cluster.KubeConfig)
	default:
		return nil, fmt.Errorf("failed to create kubeclient: unknown cluster type: %s", cluster.Type)
	}
}
