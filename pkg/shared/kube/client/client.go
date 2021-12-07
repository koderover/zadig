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
