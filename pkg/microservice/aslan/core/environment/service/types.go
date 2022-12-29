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

package service

import (
	"fmt"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	internalresource "github.com/koderover/zadig/pkg/shared/kube/resource"
)

type ProductRevision struct {
	ID          string `json:"id,omitempty"`
	EnvName     string `json:"env_name"`
	ProductName string `json:"product_name"`
	// 表示该产品更新前版本
	CurrentRevision int64 `json:"current_revision"`
	// 表示该产品更新后版本
	NextRevision int64 `json:"next_revision"`
	// true: 表示该产品的服务发生变化, 需要更新
	// false: 表示该产品的服务未发生变化, 无需更新
	Updatable bool `json:"updatable"`
	// 可以自动更新产品, 展示用户更新前和更新后的服务组以及服务详细对比
	ServiceRevisions []*SvcRevision `json:"services"`
	IsPublic         bool           `json:"isPublic"`
}

type SvcRevision struct {
	ServiceName       string                    `json:"service_name"`
	Type              string                    `json:"type"`
	CurrentRevision   int64                     `json:"current_revision"`
	NextRevision      int64                     `json:"next_revision"`
	Updatable         bool                      `json:"updatable"`
	DeployStrategy    string                    `json:"deploy_strategy"`
	Error             string                    `json:"error"`
	Deleted           bool                      `json:"deleted"`
	New               bool                      `json:"new"`
	Containers        []*commonmodels.Container `json:"containers,omitempty"`
	UpdateServiceTmpl bool                      `json:"update_service_tmpl"`
	VariableYaml      string                    `json:"variable_yaml"`
}

type ProductIngressInfo struct {
	IngressInfos []*commonservice.IngressInfo `json:"ingress_infos"`
	EnvName      string                       `json:"env_name"`
}

type SvcOptArgs struct {
	EnvName           string
	ProductName       string
	ServiceName       string
	ServiceType       string
	ServiceRev        *SvcRevision
	UpdateBy          string
	UpdateServiceTmpl bool
}

type RestartScaleArgs struct {
	Type        string `json:"type"`
	ProductName string `json:"product_name"`
	EnvName     string `json:"env_name"`
	ServiceName string `json:"service_name"`
	Name        string `json:"name"`
}

type ScaleArgs struct {
	Type        string `json:"type"`
	ProductName string `json:"product_name"`
	EnvName     string `json:"env_name"`
	ServiceName string `json:"service_name"`
	Name        string `json:"name"`
	Number      int    `json:"number"`
}

// SvcResp struct 产品-服务详情页面Response
type SvcResp struct {
	ServiceName string                       `json:"service_name"`
	Scales      []*internalresource.Workload `json:"scales"`
	Ingress     []*internalresource.Ingress  `json:"ingress"`
	Services    []*internalresource.Service  `json:"service_endpoints"`
	EnvName     string                       `json:"env_name"`
	ProductName string                       `json:"product_name"`
	GroupName   string                       `json:"group_name"`
}

func (pr *ProductRevision) GroupsUpdated() bool {
	if pr.ServiceRevisions == nil || len(pr.ServiceRevisions) == 0 {
		return false
	}
	for _, serviceRev := range pr.ServiceRevisions {
		if serviceRev.Updatable {
			return true
		}
	}
	return pr.Updatable
}

type ContainerNotFound struct {
	ServiceName string
	Container   string
	EnvName     string
	ProductName string
}

func (c *ContainerNotFound) Error() string {
	return fmt.Sprintf("serviceName:%s,container:%s", c.ServiceName, c.Container)
}

type NodeResp struct {
	Nodes  []*internalresource.Node `json:"data"`
	Labels []string                 `json:"labels"`
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
