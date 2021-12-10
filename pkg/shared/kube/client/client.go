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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/kube/multicluster"
)

func GetKubeClient(hubServerAddr, clusterID string) (client.Client, error) {
	if clusterID == setting.LocalClusterID {
		clusterID = ""
	}

	return multicluster.GetKubeClient(hubServerAddr, clusterID)
}

func GetKubeAPIReader(hubServerAddr, clusterID string) (client.Reader, error) {
	if clusterID == setting.LocalClusterID {
		clusterID = ""
	}
	return multicluster.GetKubeAPIReader(hubServerAddr, clusterID)
}

func GetRESTConfig(hubServerAddr, clusterID string) (*rest.Config, error) {
	if clusterID == setting.LocalClusterID {
		clusterID = ""
	}
	return multicluster.GetRESTConfig(hubServerAddr, clusterID)
}

func GetClientset(hubServerAddr, clusterID string) (kubernetes.Interface, error) {
	if clusterID == setting.LocalClusterID {
		clusterID = ""
	}

	return multicluster.GetClientset(hubServerAddr, clusterID)
}
