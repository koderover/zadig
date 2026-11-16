package kube

import (
	"path"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
)

func IsAliyunRRSA(provider int8, enabled bool) bool {
	return enabled && provider == config.ClusterProviderAlibabaACK
}

func WorkflowServiceAccountAnnotations(provider int8, enabled bool, roleARN string) map[string]string {
	if !enabled {
		return nil
	}
	if IsAliyunRRSA(provider, enabled) {
		// ACK's role-name annotation selects a role in the cluster's Alibaba Cloud account.
		return map[string]string{"pod-identity.alibabacloud.com/role-name": path.Base(roleARN)}
	}
	return map[string]string{"eks.amazonaws.com/role-arn": roleARN}
}
