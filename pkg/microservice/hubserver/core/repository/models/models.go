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
	"github.com/koderover/zadig/v2/pkg/microservice/hubserver/config"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type K8SClusterInfo struct {
	Nodes   int    `json:"nodes"    bson:"nodes"`
	Version string `json:"version"  bson:"version"`
	CPU     string `json:"cpu"      bson:"cpu"`
	Memory  string `json:"memory"   bson:"memory"`
}

type K8SCluster struct {
	ID                 primitive.ObjectID      `json:"id,omitempty"              bson:"_id,omitempty"`
	Name               string                  `json:"name"                      bson:"name"`
	Tags               []string                `json:"tags"                      bson:"tags"`
	Description        string                  `json:"description"               bson:"description"`
	Info               *K8SClusterInfo         `json:"info,omitempty"            bson:"info,omitempty"`
	Status             config.K8SClusterStatus `json:"status"                    bson:"status"`
	Error              string                  `json:"error"                     bson:"error"`
	Yaml               string                  `json:"yaml"                      bson:"yaml"`
	Production         bool                    `json:"production"                bson:"production"`
	CreatedAt          int64                   `json:"createdAt"                 bson:"createdAt"`
	CreatedBy          string                  `json:"createdBy"                 bson:"createdBy"`
	Disconnected       bool                    `json:"-"                         bson:"disconnected"`
	Token              string                  `json:"token"                     bson:"-"`
	Local              bool                    `json:"local"                     bson:"local"`
	LastConnectionTime int64                   `json:"last_connection_time"      bson:"last_connection_time"`
	AdvancedConfig     *AdvancedConfig         `json:"advanced_config"           bson:"advanced_config"`

	// new field in 1.14, intended to enable kubeconfig for cluster management
	Type       string `json:"type"           bson:"type"` // either agent or kubeconfig supported
	KubeConfig string `json:"kube_config"    bson:"kube_config"`

	// Deprecated field, it should be deleted in version 1.15 since no more namespace settings is used
	Namespace string `json:"namespace"                 bson:"namespace"`
}

type AdvancedConfig struct {
	Strategy          string   `json:"strategy,omitempty"       bson:"strategy,omitempty"`
	ProjectNames      []string `json:"-"                        bson:"-"`
	Tolerations       string   `json:"tolerations"              bson:"tolerations"`
	ClusterAccessYaml string   `json:"cluster_access_yaml"      bson:"cluster_access_yaml"`
	ScheduleWorkflow  bool     `json:"schedule_workflow"        bson:"schedule_workflow"`
	EnableIRSA        bool     `json:"enable_irsa"              bson:"enable_irsa"`
	IRSARoleARM       string   `json:"irsa_role_arn"            bson:"irsa_role_arn"`
	// TLS configuration for Kubernetes API connections
	InsecureSkipTLSVerify bool `json:"insecure_skip_tls_verify" bson:"insecure_skip_tls_verify"`
}

func (K8SCluster) TableName() string {
	return "k8s_cluster"
}
