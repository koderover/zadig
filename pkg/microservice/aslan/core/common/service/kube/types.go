/*
Copyright 2023 The KodeRover Authors.

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

package kube

type EnvoyClusterConfigLoadAssignment struct {
	ClusterName string             `json:"cluster_name"`
	Endpoints   []EnvoyLBEndpoints `json:"endpoints"`
}

type EnvoyLBEndpoints struct {
	LBEndpoints []EnvoyEndpoints `json:"lb_endpoints"`
}

type EnvoyEndpoints struct {
	Endpoint EnvoyEndpoint `json:"endpoint"`
}

type EnvoyEndpoint struct {
	Address EnvoyAddress `json:"address"`
}

type EnvoyAddress struct {
	SocketAddress EnvoySocketAddress `json:"socket_address"`
}

type EnvoySocketAddress struct {
	Protocol  string `json:"protocol"`
	Address   string `json:"address"`
	PortValue int    `json:"port_value"`
}

type ShareEnvOp string

const (
	ShareEnvEnable  ShareEnvOp = "enable"
	ShareEnvDisable ShareEnvOp = "disable"
)

type MatchedEnv struct {
	EnvName   string
	Namespace string
}

type ShareEnvReady struct {
	IsReady bool                `json:"is_ready"`
	Checks  ShareEnvReadyChecks `json:"checks"`
}

type ShareEnvReadyChecks struct {
	NamespaceHasIstioLabel  bool `json:"namespace_has_istio_label"`
	VirtualServicesDeployed bool `json:"virtualservice_deployed"`
	PodsHaveIstioProxy      bool `json:"pods_have_istio_proxy"`
	WorkloadsReady          bool `json:"workloads_ready"`
	WorkloadsHaveK8sService bool `json:"workloads_have_k8s_service"`
}

// Note: `WorkloadsHaveK8sService` is an optional condition.
func (s *ShareEnvReady) CheckAndSetReady(state ShareEnvOp) {
	if !s.Checks.WorkloadsReady {
		s.IsReady = false
		return
	}

	switch state {
	case ShareEnvEnable:
		if !s.Checks.NamespaceHasIstioLabel || !s.Checks.VirtualServicesDeployed || !s.Checks.PodsHaveIstioProxy {
			s.IsReady = false
		} else {
			s.IsReady = true
		}
	default:
		if !s.Checks.NamespaceHasIstioLabel && !s.Checks.VirtualServicesDeployed && !s.Checks.PodsHaveIstioProxy {
			s.IsReady = true
		} else {
			s.IsReady = false
		}
	}
}
