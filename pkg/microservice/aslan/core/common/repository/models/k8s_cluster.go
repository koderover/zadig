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

	"github.com/koderover/zadig/pkg/setting"
)

type K8SCluster struct {
	ID             primitive.ObjectID       `json:"id,omitempty"              bson:"_id,omitempty"`
	Name           string                   `json:"name"                      bson:"name"`
	Tags           []string                 `json:"tags"                      bson:"tags"`
	Description    string                   `json:"description"               bson:"description"`
	Namespace      string                   `json:"namespace"                 bson:"namespace"`
	Info           *K8SClusterInfo          `json:"info,omitempty"            bson:"info,omitempty"`
	AdvancedConfig *AdvancedConfig          `json:"advanced_config,omitempty" bson:"advanced_config,omitempty"`
	Status         setting.K8SClusterStatus `json:"status"                    bson:"status"`
	Error          string                   `json:"error"                     bson:"error"`
	Yaml           string                   `json:"yaml"                      bson:"yaml"`
	Production     bool                     `json:"production"                bson:"production"`
	CreatedAt      int64                    `json:"createdAt"                 bson:"createdAt"`
	CreatedBy      string                   `json:"createdBy"                 bson:"createdBy"`
	Disconnected   bool                     `json:"-"                         bson:"disconnected"`
	Token          string                   `json:"token"                     bson:"-"`
	Provider       int8                     `json:"provider"                  bson:"provider"`
	Local          bool                     `json:"local"                     bson:"local"`
}

type K8SClusterResp struct {
	ID             string          `json:"id"                          bson:"_id,omitempty"`
	Name           string          `json:"name"                        bson:"name"`
	AdvancedConfig *AdvancedConfig `json:"config,omitempty"            bson:"config,omitempty"`
}

type K8SClusterInfo struct {
	Nodes   int    `json:"nodes" bson:"nodes"`
	Version string `json:"version" bson:"version"`
	CPU     string `json:"cpu" bson:"cpu"`
	Memory  string `json:"memory" bson:"memory"`
}

type AdvancedConfig struct {
	Strategy   string                     `json:"strategy,omitempty"      bson:"strategy,omitempty"`
	NodeLabels []*NodeSelectorRequirement `json:"node_labels,omitempty"   bson:"node_labels,omitempty"`
}

type NodeSelectorRequirement struct {
	Key      string                      `json:"key"      bson:"key"`
	Value    []string                    `json:"value"    bson:"value"`
	Operator corev1.NodeSelectorOperator `json:"operator" bson:"operator"`
}

func (K8SCluster) TableName() string {
	return "k8s_cluster"
}
