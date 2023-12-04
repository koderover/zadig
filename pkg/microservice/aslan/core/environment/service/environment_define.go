/*
Copyright 2022 The KodeRover Authors.

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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
)

const (
	Timeout = 60
)

const (
	usageScenarioCreateEnv       = "createEnv"
	usageScenarioUpdateEnv       = "updateEnv"
	usageScenarioUpdateRenderSet = "updateRenderSet"
)

type EnvStatus struct {
	EnvName    string `json:"env_name,omitempty"`
	Status     string `json:"status"`
	ErrMessage string `json:"err_message"`
}

type EnvResp struct {
	ProjectName string   `json:"projectName"`
	Status      string   `json:"status"`
	Error       string   `json:"error"`
	Name        string   `json:"name"`
	UpdateBy    string   `json:"updateBy"`
	UpdateTime  int64    `json:"updateTime"`
	IsPublic    bool     `json:"isPublic"`
	ClusterName string   `json:"clusterName"`
	ClusterID   string   `json:"cluster_id"`
	Namespace   string   `json:"namespace"`
	Alias       string   `json:"alias"`
	Production  bool     `json:"production"`
	Source      string   `json:"source"`
	RegistryID  string   `json:"registry_id"`
	BaseRefs    []string `json:"base_refs"`
	BaseName    string   `json:"base_name"`
	IsExisted   bool     `json:"is_existed"`
	IsFavorite  bool     `json:"is_favorite"`
	SharedNS    bool     `json:"shared_ns"`

	// New Since v1.11.0
	ShareEnvEnable  bool   `json:"share_env_enable"`
	ShareEnvIsBase  bool   `json:"share_env_is_base"`
	ShareEnvBaseEnv string `json:"share_env_base_env"`

	// New Since v2.1.0
	IstioGrayscaleEnable  bool   `json:"istio_grayscale_enable"`
	IstioGrayscaleIsBase  bool   `json:"istio_grayscale_is_base"`
	IstioGrayscaleBaseEnv string `json:"istio_grayscale_base_env"`
}

type SharedNSEnvs struct {
	ProjectName string `json:"project_name"`
	EnvName     string `json:"env_name"`
	Production  bool   `json:"production"`
}

type ProductResp struct {
	ID          string                           `json:"id"`
	ProductName string                           `json:"product_name"`
	Namespace   string                           `json:"namespace"`
	Status      string                           `json:"status"`
	Error       string                           `json:"error"`
	EnvName     string                           `json:"env_name"`
	UpdateBy    string                           `json:"update_by"`
	UpdateTime  int64                            `json:"update_time"`
	Services    [][]*commonmodels.ProductService `json:"services"`
	Render      *commonmodels.RenderInfo         `json:"render"`
	Vars        []*templatemodels.RenderKV       `json:"vars"`
	IsPublic    bool                             `json:"isPublic"`
	ClusterID   string                           `json:"cluster_id,omitempty"`
	ClusterName string                           `json:"cluster_name,omitempty"`
	RecycleDay  int                              `json:"recycle_day"`
	IsProd      bool                             `json:"is_prod"`
	IsLocal     bool                             `json:"is_local"`
	IsExisted   bool                             `json:"is_existed"`
	Source      string                           `json:"source"`
	RegisterID  string                           `json:"registry_id"`

	// New Since v1.11.0
	ShareEnvEnable  bool   `json:"share_env_enable"`
	ShareEnvIsBase  bool   `json:"share_env_is_base"`
	ShareEnvBaseEnv string `json:"share_env_base_env"`

	// New Since v2.1.0
	RelatedEnvs           []*SharedNSEnvs `json:"related_envs"`
	IstioGrayscaleEnable  bool            `json:"istio_grayscale_enable"`
	IstioGrayscaleIsBase  bool            `json:"istio_grayscale_is_base"`
	IstioGrayscaleBaseEnv string          `json:"istio_grayscale_base_env"`
}

type ProductParams struct {
	IsPublic        bool     `json:"isPublic"`
	EnvName         string   `json:"envName"`
	RoleID          int      `json:"roleId"`
	PermissionUUIDs []string `json:"permissionUUIDs"`
}

type EstimateValuesArg struct {
	ChartRepo      string                  `json:"chartRepo,omitempty"`
	ChartName      string                  `json:"chartName,omitempty"`
	ChartVersion   string                  `json:"chartVersion,omitempty"`
	DefaultValues  string                  `json:"defaultValues"`
	OverrideYaml   string                  `json:"overrideYaml"`
	OverrideValues []*commonservice.KVPair `json:"overrideValues,omitempty"`
	Production     bool                    `json:"-"`
}

type EnvRenderChartArg struct {
	ChartValues []*commonservice.HelmSvcRenderArg `json:"chartValues"`
}

type EnvRendersetArg struct {
	DeployType        string                            `json:"deployType"`
	DefaultValues     string                            `json:"defaultValues"`
	ValuesData        *commonservice.ValuesDataArgs     `json:"valuesData"`
	ChartValues       []*commonservice.HelmSvcRenderArg `json:"chartValues"`
	UpdateServiceTmpl bool                              `json:"updateServiceTmpl"`
}

type K8sRendersetArg struct {
	VariableYaml string `json:"variable_yaml"`
}

type ProductK8sServiceCreationInfo struct {
	*commonmodels.ProductService
	DeployStrategy string `json:"deploy_strategy"`
}

type ProductHelmServiceCreationInfo struct {
	*commonservice.HelmSvcRenderArg
	DeployStrategy string `json:"deploy_strategy"`
}

type CreateSingleProductArg struct {
	ProductName string `json:"product_name"`
	EnvName     string `json:"env_name"`
	Namespace   string `json:"namespace"`
	ClusterID   string `json:"cluster_id"`
	RegistryID  string `json:"registry_id"`
	Production  bool   `json:"production"`
	Alias       string `json:"alias"`
	BaseEnvName string `json:"base_env_name"`
	BaseName    string `json:"base_name,omitempty"` // for collaboration mode

	DefaultValues string `json:"default_values"`
	// TODO fix me
	HelmDefaultValues string `json:"defaultValues"`
	// for helm products
	ValuesData  *commonservice.ValuesDataArgs     `json:"valuesData"`
	ChartValues []*ProductHelmServiceCreationInfo `json:"chartValues"`

	// for k8s products
	GlobalVariables []*commontypes.GlobalVariableKV    `json:"global_variables"`
	Services        [][]*ProductK8sServiceCreationInfo `json:"services"`

	IsExisted bool `json:"is_existed"`

	// New Since v1.12.0
	ShareEnv commonmodels.ProductShareEnv `json:"share_env"`
	// New Since v1.13.0
	EnvConfigs []*commonmodels.CreateUpdateCommonEnvCfgArgs `json:"env_configs"`
	// New Since v2.1.0
	IstioGrayscale commonmodels.IstioGrayscale `json:"istio_grayscale"`
}

type UpdateMultiHelmProductArg struct {
	ProductName     string                            `json:"productName"`
	EnvNames        []string                          `json:"envNames"`
	ChartValues     []*commonservice.HelmSvcRenderArg `json:"chartValues"`
	DeletedServices []string                          `json:"deletedServices"`
	ReplacePolicy   string                            `json:"replacePolicy"` // TODO logic not implemented
}

type RawYamlResp struct {
	YamlContent string `json:"yamlContent"`
}

type CreateEnvRequest struct {
	Scene       string `form:"scene"`
	Type        string `form:"type"`
	ProjectName string `form:"projectName"`
	Auto        bool   `form:"auto"`
	EnvType     string `form:"envType"`
}

type UpdateEnvRequest struct {
	Type        string `form:"type"`
	ProjectName string `form:"projectName"`
	Force       bool   `form:"force"`
}

// ------------ used for api of getting deploy status of k8s resource/helm release

type K8sDeployStatusCheckRequest struct {
	EnvName   string                           `json:"env_name"`
	Services  []*commonservice.K8sSvcRenderArg `json:"services"`
	ClusterID string                           `json:"cluster_id"`
	Namespace string                           `json:"namespace"`
}

type HelmDeployStatusCheckRequest struct {
	EnvName   string   `json:"env_name"`
	Services  []string `json:"services"`
	ClusterID string   `json:"cluster_id"`
	Namespace string   `json:"namespace"`
}

type DeployStatus string

const (
	StatusDeployed   DeployStatus = "deployed"
	StatusUnDeployed DeployStatus = "undeployed"
)

type ResourceDeployStatus struct {
	Type         string                  `json:"type"`
	Name         string                  `json:"name"`
	Status       DeployStatus            `json:"status"`
	GVK          schema.GroupVersionKind `json:"-"`
	OverrideYaml string                  `json:"override_yaml"`
	OverrideKvs  []*commonservice.KVPair `json:"override_kvs"`
}

type ServiceDeployStatus struct {
	ServiceName string                  `json:"service_name"`
	Resources   []*ResourceDeployStatus `json:"resources"`
}

// type intervalExecutorHandler func(data *commonmodels.Service, productSvc *commonmodels.ProductService, releaseName string, isRetry bool, log *zap.SugaredLogger) error
type intervalExecutorHandler func(data *kube.ReleaseInstallParam, isRetry bool, log *zap.SugaredLogger) error
type svcUpgradeFilter func(svc *commonmodels.ProductService) bool
