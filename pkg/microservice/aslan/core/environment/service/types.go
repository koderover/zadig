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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalresource "github.com/koderover/zadig/v2/pkg/shared/kube/resource"
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
	ServiceName       string                          `json:"service_name"`
	Type              string                          `json:"type"`
	CurrentRevision   int64                           `json:"current_revision"`
	NextRevision      int64                           `json:"next_revision"`
	Updatable         bool                            `json:"updatable"`
	DeployStrategy    string                          `json:"deploy_strategy"`
	Error             string                          `json:"error"`
	Deleted           bool                            `json:"deleted"`
	New               bool                            `json:"new"`
	Containers        []*commonmodels.Container       `json:"containers,omitempty"`
	UpdateServiceTmpl bool                            `json:"update_service_tmpl"`
	VariableYaml      string                          `json:"variable_yaml"`
	VariableKVs       []*commontypes.RenderVariableKV `json:"variable_kvs"`
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

type PreviewServiceArgs struct {
	ProductName           string                          `json:"product_name"`
	EnvName               string                          `json:"env_name"`
	ServiceName           string                          `json:"service_name"`
	UpdateServiceRevision bool                            `json:"update_service_revision"`
	ServiceModules        []*commonmodels.Container       `json:"service_modules"`
	VariableKVs           []*commontypes.RenderVariableKV `json:"variable_kvs"`
}

type RestartScaleArgs struct {
	Type        string `json:"type"`
	ProductName string `json:"product_name"`
	EnvName     string `json:"env_name"`
	Name        string `json:"name"`
	// deprecated, since it is not used
	ServiceName string `json:"service_name"`
}

type ScaleArgs struct {
	Type        string `json:"type"`
	ProductName string `json:"product_name"`
	EnvName     string `json:"env_name"`
	ServiceName string `json:"service_name"`
	Name        string `json:"name"`
	Number      int    `json:"number"`
	Production  bool   `json:"production"`
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

type IstioGrayscaleReady struct {
	IsReady bool                 `json:"is_ready"`
	Checks  IstioGrayscaleChecks `json:"checks"`
}

type IstioGrayscaleChecks struct {
	NamespaceHasIstioLabel  bool `json:"namespace_has_istio_label"`
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

// Note: `WorkloadsHaveK8sService` is an optional condition.
func (s *IstioGrayscaleReady) CheckAndSetReady(state ShareEnvOp) {
	if !s.Checks.WorkloadsReady {
		s.IsReady = false
		return
	}

	switch state {
	case ShareEnvEnable:
		if !s.Checks.NamespaceHasIstioLabel || !s.Checks.PodsHaveIstioProxy {
			s.IsReady = false
		} else {
			s.IsReady = true
		}
	default:
		if !s.Checks.NamespaceHasIstioLabel && !s.Checks.PodsHaveIstioProxy {
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

type OpenAPIScaleServiceReq struct {
	ProjectKey     string `json:"project_key"`
	EnvName        string `json:"env_key"`
	WorkloadName   string `json:"workload_name"`
	WorkloadType   string `json:"workload_type"`
	TargetReplicas int    `json:"target_replicas"`
}

func (req *OpenAPIScaleServiceReq) Validate() error {
	if req.ProjectKey == "" {
		return fmt.Errorf("project_key is required")
	}
	if req.EnvName == "" {
		return fmt.Errorf("env_key is required")
	}
	if req.WorkloadName == "" {
		return fmt.Errorf("workload_name is required")
	}
	if req.WorkloadType == "" {
		return fmt.Errorf("workload_type is required")
	}

	switch req.WorkloadType {
	case setting.Deployment, setting.StatefulSet:
	default:
		return fmt.Errorf("unsupported workload type: %s", req.WorkloadType)
	}

	if req.TargetReplicas < 0 {
		return fmt.Errorf("target_replicas must be greater than or equal to 0")
	}

	return nil
}

type OpenAPIApplyYamlServiceReq struct {
	EnvName     string               `json:"env_key"`
	ServiceList []*YamlServiceWithKV `json:"service_list"`
}

type YamlServiceWithKV struct {
	ServiceName string                          `json:"service_name"`
	VariableKvs []*commontypes.RenderVariableKV `json:"variable_kvs"`
}

func (req *OpenAPIApplyYamlServiceReq) Validate() error {
	if req.EnvName == "" {
		return fmt.Errorf("env_key is required")
	}

	for _, serviceDef := range req.ServiceList {
		if serviceDef.ServiceName == "" {
			return fmt.Errorf("service_name is required for all services")
		}
	}
	return nil
}

type OpenAPIDeleteYamlServiceFromEnvReq struct {
	EnvName           string   `json:"env_key"`
	ServiceNames      []string `json:"service_names"`
	NotDeleteResource bool     `json:"not_delete_resource"`
}

func (req *OpenAPIDeleteYamlServiceFromEnvReq) Validate() error {
	if req.EnvName == "" {
		return fmt.Errorf("env_key is required")
	}

	return nil
}

type OpenAPIEnvCfgBrief struct {
	Name             string                  `json:"name"`
	EnvName          string                  `json:"env_name"`
	ProjectName      string                  `json:"project_name"`
	CommonEnvCfgType config.CommonEnvCfgType `json:"common_env_cfg_type"`
	UpdateBy         string                  `json:"update_by,omitempty"`
	UpdateTime       int64                   `json:"update_time,omitempty"`
}

type OpenAPIEnvCfgArgs struct {
	Name                 string                       `json:"name"`
	CommonEnvCfgType     config.CommonEnvCfgType      `json:"common_env_cfg_type"`
	CreatedBy            string                       `json:"created_by,omitempty"`
	CreatedTime          int64                        `json:"created_time,omitempty"`
	UpdateBy             string                       `json:"update_by,omitempty"`
	UpdateTime           int64                        `json:"update_time,omitempty"`
	EnvName              string                       `json:"env_key"`
	ProductName          string                       `json:"product_name"`
	ServiceName          string                       `json:"service_name,omitempty"`
	Services             []string                     `json:"services,omitempty"`
	YamlData             string                       `json:"yaml_data,omitempty"`
	SourceDetail         *commonmodels.CreateFromRepo `json:"source_detail"`
	RestartAssociatedSvc bool                         `json:"restart_associated_svc,omitempty"`
	AutoSync             bool                         `json:"auto_sync"`
}

type OpenAPIEnvCfgDetail struct {
	IngressDetail   *OpenAPIEnvCfgIngressDetail   `json:"ingress_detail,omitempty"`
	SecretDetail    *OpenAPIEnvCfgSecretDetail    `json:"secret_detail,omitempty"`
	PvcDetail       *OpenAPIEnvCfgPvcDetail       `json:"pvc_detail,omitempty"`
	ConfigMapDetail *OpenAPIEnvCfgConfigMapDetail `json:"configMap_detail,omitempty"`
}

type OpenAPIEnvCfgIngressDetail struct {
	Name             string                  `json:"name"`
	ProjectName      string                  `json:"project_key"`
	EnvName          string                  `json:"env_key"`
	CommonEnvCfgType config.CommonEnvCfgType `json:"common_env_cfg_type"`
	CreatedBy        string                  `json:"created_by,omitempty"`
	CreatedTime      int64                   `json:"created_time,omitempty"`
	UpdateBy         string                  `json:"update_by,omitempty"`
	UpdateTime       int64                   `json:"update_time,omitempty"`
	YamlData         string                  `json:"yaml_data,omitempty"`
	Services         []string                `json:"services,omitempty"`
	SourceDetail     *CfgRepoInfo            `json:"source_detail"`
	HostInfo         string                  `json:"host_info,omitempty"`
	Address          string                  `json:"address,omitempty"`
	Ports            string                  `json:"ports,omitempty"`
	ErrorReason      string                  `json:"error_reason,omitempty"`
}

type CfgRepoInfo struct {
	GitRepoConfig *GitRepoConfig `bson:"git_repo_config,omitempty"      json:"git_repo_config,omitempty"`
	LoadPath      string         `bson:"load_path,omitempty"            json:"load_path,omitempty"`
}

type GitRepoConfig struct {
	CodehostKey string   `json:"codehost_key"`
	Owner       string   `json:"owner"`
	Repo        string   `json:"repo"`
	Branch      string   `json:"branch"`
	ValuesPaths []string `json:"values_paths,omitempty"`
}

type OpenAPIEnvCfgSecretDetail struct {
	Name             string                  `json:"name"`
	ProjectName      string                  `json:"project_key"`
	EnvName          string                  `json:"env_key"`
	CommonEnvCfgType config.CommonEnvCfgType `json:"common_env_cfg_type"`
	CreatedBy        string                  `json:"created_by,omitempty"`
	CreatedTime      int64                   `json:"created_time,omitempty"`
	UpdateBy         string                  `json:"update_by,omitempty"`
	UpdateTime       int64                   `json:"update_time,omitempty"`
	YamlData         string                  `json:"yaml_data,omitempty"`
	Services         []string                `json:"services,omitempty"`
	SourceDetail     *CfgRepoInfo            `json:"source_detail"`
	SecretType       string                  `json:"secret_type,omitempty"`
}

type OpenAPIEnvCfgPvcDetail struct {
	Name             string                  `json:"name"`
	ProjectName      string                  `json:"project_key"`
	EnvName          string                  `json:"env_key"`
	CommonEnvCfgType config.CommonEnvCfgType `json:"common_env_cfg_type"`
	CreatedBy        string                  `json:"created_by,omitempty"`
	CreatedTime      int64                   `json:"created_time,omitempty"`
	UpdateBy         string                  `json:"update_by,omitempty"`
	UpdateTime       int64                   `json:"update_time,omitempty"`
	YamlData         string                  `json:"yaml_data,omitempty"`
	Services         []string                `json:"services,omitempty"`
	SourceDetail     *CfgRepoInfo            `json:"source_detail"`
	Status           string                  `json:"status,omitempty"`
	Volume           string                  `json:"volume,omitempty"`
	AccessModes      string                  `json:"access_modes,omitempty"`
	StorageClass     string                  `json:"storage_class,omitempty"`
	Capacity         string                  `json:"capacity,omitempty"`
}

type OpenAPIEnvCfgConfigMapDetail struct {
	Name             string                  `json:"name"`
	ProjectName      string                  `json:"project_key"`
	EnvName          string                  `json:"env_key"`
	CommonEnvCfgType config.CommonEnvCfgType `json:"common_env_cfg_type"`
	CreatedBy        string                  `json:"created_by,omitempty"`
	CreatedTime      int64                   `json:"created_time,omitempty"`
	UpdateBy         string                  `json:"update_by,omitempty"`
	UpdateTime       int64                   `json:"update_time,omitempty"`
	YamlData         string                  `json:"yaml_data,omitempty"`
	Services         []string                `json:"services,omitempty"`
	SourceDetail     *CfgRepoInfo            `json:"source_detail"`
	Immutable        bool                    `json:"immutable,omitempty"`
	CmData           map[string]string       `json:"cm_data,omitempty"`
}

func (req *OpenAPIEnvCfgArgs) Validate() error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.EnvName == "" {
		return fmt.Errorf("env_key is required")
	}
	if req.ProductName == "" {
		return fmt.Errorf("projectKey is required")
	}
	if req.CommonEnvCfgType == "" {
		return fmt.Errorf("common_env_cfg_type is required")
	}
	if req.YamlData == "" {
		return fmt.Errorf("yaml_data is required")
	}
	return nil
}

type OpenAPICreateEnvArgs struct {
	Production      bool                              `json:"production"`
	ProjectName     string                            `json:"project_key"`
	EnvName         string                            `json:"env_key"`
	ClusterID       string                            `json:"cluster_id"`
	Namespace       string                            `json:"namespace"`
	RegistryID      string                            `json:"registry_id"`
	Alias           string                            `json:"env_name"`
	GlobalVariables []*commontypes.GlobalVariableKV   `json:"global_variables"`
	ChartValues     []*ProductHelmServiceCreationInfo `json:"chart_values"`
	Services        []*OpenAPICreateServiceArgs       `json:"services"`
	EnvConfigs      []*EnvCfgArgs                     `json:"env_configs"`
}

type EnvCfgArgs struct {
	Name             string                  `json:"name"`
	AutoSync         bool                    `json:"auto_sync"`
	CommonEnvCfgType string                  `json:"common_env_cfg_type"`
	YamlData         string                  `json:"yaml_data"`
	GitRepoConfig    *template.GitRepoConfig `json:"git_repo_config"`
}

type OpenAPICreateServiceArgs struct {
	ServiceName    string                          `json:"service_name"`
	DeployStrategy string                          `json:"deploy_strategy"`
	Containers     []*commonmodels.Container       `json:"containers"`
	VariableKVs    []*commontypes.RenderVariableKV `json:"variable_kvs"`
	Status         string                          `json:"status"`
	Type           string                          `json:"type"`
}

type OpenAPIServiceDetail struct {
	ServiceName string                           `json:"service_name"`
	Containers  []*commonmodels.Container        `json:"containers"`
	VariableKVs []*commontypes.ServiceVariableKV `json:"variable_kvs"`
	Status      string                           `json:"status"`
	Type        string                           `json:"type"`
}

func (env *OpenAPICreateEnvArgs) Validate() error {
	if env.ProjectName == "" {
		return fmt.Errorf("project key is required")
	}
	if env.EnvName == "" {
		return fmt.Errorf("env key is required")
	}
	if env.ClusterID == "" {
		return fmt.Errorf("cluster_id is required")
	}
	if env.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if env.RegistryID == "" {
		return fmt.Errorf("registry_id is required")
	}
	return nil
}

type OpenAPIEnvDetail struct {
	ProjectName     string                            `json:"project_key"`
	EnvName         string                            `json:"env_key"`
	UpdateBy        string                            `json:"update_by"`
	UpdateTime      int64                             `json:"update_time"`
	ClusterID       string                            `json:"cluster_id"`
	Namespace       string                            `json:"namespace"`
	RegistryID      string                            `json:"registry_id"`
	Alias           string                            `json:"env_name,omitempty"`
	GlobalVariables []*commontypes.GlobalVariableKV   `json:"global_variables"`
	ChartValues     []*ProductHelmServiceCreationInfo `json:"chart_values,omitempty"`
	Services        []*OpenAPIServiceDetail           `json:"services"`
	Status          string                            `json:"status"`
}

type EnvBasicInfoArgs struct {
	RegistryID string `json:"registry_id"`
	Alias      string `json:"env_name"`
}

type OpenAPIListEnvBrief struct {
	Alias      string `json:"envName,omitempty"`
	EnvName    string `json:"env_key"`
	ClusterID  string `json:"cluster_id"`
	Namespace  string `json:"namespace"`
	Production bool   `json:"production"`
	RegistryID string `json:"registry_id"`
	Status     string `json:"status"`
	UpdateBy   string `json:"update_by"`
	UpdateTime int64  `json:"update_time"`
}

type GlobalVariables struct {
	Variables           []*commontypes.ServiceVariableKV `json:"variables"`
	ProductionVariables []*commontypes.ServiceVariableKV `json:"production_variables"`
}

type OpenAPIServiceVariablesReq struct {
	ServiceList []*ServiceVariable `json:"service_list"`
}

type ServiceVariable struct {
	ServiceName string                          `json:"service_name"`
	Variables   []*commontypes.RenderVariableKV `json:"variable_kvs"`
}

type OpenAPIEnvGlobalVariables struct {
	GlobalVariables []*commontypes.GlobalVariableKV `json:"global_variables"`
}

type OpenAPIEnvServiceDetail struct {
	ServiceName    string                           `json:"service_name"`
	Variables      []*commontypes.ServiceVariableKV `json:"variables"`
	Images         []string                         `json:"images"`
	Status         string                           `json:"status"`
	Type           string                           `json:"type"`
	Error          string                           `json:"error"`
	DeployStrategy string                           `json:"deploy_strategy"`
}
