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

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	corev1 "k8s.io/api/core/v1"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

type K8SCluster struct {
	ID                     primitive.ObjectID       `json:"id,omitempty"              bson:"_id,omitempty"`
	Name                   string                   `json:"name"                      bson:"name"`
	Tags                   []string                 `json:"tags"                      bson:"tags"`
	Description            string                   `json:"description"               bson:"description"`
	Info                   *K8SClusterInfo          `json:"info,omitempty"            bson:"info,omitempty"`
	AdvancedConfig         *AdvancedConfig          `json:"advanced_config,omitempty" bson:"advanced_config,omitempty"`
	Status                 setting.K8SClusterStatus `json:"status"                    bson:"status"`
	Error                  string                   `json:"error"                     bson:"error"`
	Yaml                   string                   `json:"yaml"                      bson:"yaml"`
	Production             bool                     `json:"production"                bson:"production"`
	CreatedAt              int64                    `json:"createdAt"                 bson:"createdAt"`
	CreatedBy              string                   `json:"createdBy"                 bson:"createdBy"`
	Disconnected           bool                     `json:"-"                         bson:"disconnected"`
	Token                  string                   `json:"token"                     bson:"-"`
	Provider               int8                     `json:"provider"                  bson:"provider"`
	Local                  bool                     `json:"local"                     bson:"local"`
	Cache                  types.Cache              `json:"cache"                     bson:"cache"`
	ShareStorage           types.ShareStorage       `json:"share_storage"             bson:"share_storage"`
	LastConnectionTime     int64                    `json:"last_connection_time"      bson:"last_connection_time"`
	UpdateHubagentErrorMsg string                   `json:"update_hubagent_error_msg" bson:"update_hubagent_error_msg"`
	DindCfg                *DindCfg                 `json:"dind_cfg"                  bson:"dind_cfg"`

	// new field in 1.14, intended to enable kubeconfig for cluster management
	Type       string `json:"type"           bson:"type"` // either agent or kubeconfig supported
	KubeConfig string `json:"kube_config"    bson:"kube_config"`

	// Deprecated field, it should be deleted in version 1.15 since no more namespace settings is used
	Namespace string `json:"namespace"                 bson:"namespace"`
}

type K8SClusterResp struct {
	ID             string          `json:"id"                          bson:"id,omitempty"`
	Name           string          `json:"name"                        bson:"name"`
	AdvancedConfig *AdvancedConfig `json:"advanced_config,omitempty"   bson:"advanced_config,omitempty"`
}

type K8SClusterInfo struct {
	Nodes   int    `json:"nodes" bson:"nodes"`
	Version string `json:"version" bson:"version"`
	CPU     string `json:"cpu" bson:"cpu"`
	Memory  string `json:"memory" bson:"memory"`
}

type AdvancedConfig struct {
	Strategy          string                     `json:"strategy,omitempty"       bson:"strategy,omitempty"`
	NodeLabels        []*NodeSelectorRequirement `json:"node_labels,omitempty"    bson:"node_labels,omitempty"`
	ProjectNames      []string                   `json:"-"                        bson:"-"`
	ClusterAccessYaml string                     `json:"cluster_access_yaml"      bson:"cluster_access_yaml"`
	ScheduleWorkflow  bool                       `json:"schedule_workflow"        bson:"schedule_workflow"`
	ScheduleStrategy  []*ScheduleStrategy        `json:"schedule_strategy"        bson:"schedule_strategy"`
	EnableIRSA        bool                       `json:"enable_irsa"              bson:"enable_irsa"`
	IRSARoleARM       string                     `json:"irsa_role_arn"            bson:"irsa_role_arn"`

	AgentNodeSelector string `json:"agent_node_selector"            bson:"agent_node_selector"`
	AgentToleration   string `json:"agent_toleration"               bson:"agent_toleration"`
	AgentAffinity     string `json:"agent_affinity"                 bson:"agent_affinity"`
}

type ScheduleStrategy struct {
	StrategyID   string                     `json:"strategy_id"   bson:"strategy_id"`
	StrategyName string                     `json:"strategy_name" bson:"strategy_name"`
	Strategy     string                     `json:"strategy"      bson:"strategy"`
	NodeLabels   []*NodeSelectorRequirement `json:"node_labels"   bson:"node_labels"`
	Tolerations  string                     `json:"tolerations"   bson:"tolerations"`
	Default      bool                       `json:"default"       bson:"default"`
}

type NodeSelectorRequirement struct {
	Key      string                      `json:"key"      bson:"key"`
	Value    []string                    `json:"value"    bson:"value"`
	Operator corev1.NodeSelectorOperator `json:"operator" bson:"operator"`
}

type Limits struct {
	CPU    int `json:"cpu"    bson:"cpu"`
	Memory int `json:"memory" bson:"memory"`
}

type Resources struct {
	Limits *Limits `json:"limits" bson:"limits"`
}

type DindCfg struct {
	Replicas      int          `json:"replicas"    bson:"replicas"`
	Resources     *Resources   `json:"resources"      bson:"resources"`
	Storage       *DindStorage `json:"storage"        bson:"storage"`
	StrategyID    string       `json:"strategy_id"    bson:"strategy_id"`
	StorageDriver string       `json:"storage_driver" bson:"storage_driver"`
}

type DindStorageType string

const (
	DindStorageRootfs  DindStorageType = "rootfs"
	DindStorageDynamic DindStorageType = "dynamic"
)

type DindStorage struct {
	Type             DindStorageType `json:"type"                bson:"type"`
	StorageClass     string          `json:"storage_class"       bson:"storage_class"`
	StorageSizeInGiB int64           `json:"storage_size_in_gib" bson:"storage_size_in_gib"`
}

func (K8SCluster) TableName() string {
	return "k8s_cluster"
}
