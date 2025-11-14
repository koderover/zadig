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

package aslan

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type cluster struct {
	ID     string                   `json:"id,omitempty"`
	Name   string                   `json:"name"`
	Status setting.K8SClusterStatus `json:"status"`
	Local  bool                     `json:"local"`
}

func (c *Client) AddLocalCluster() error {
	url := "/cluster/clusters"
	req := cluster{
		ID:   setting.LocalClusterID,
		Name: fmt.Sprintf("%s-%s", "local", time.Now().Format("20060102150405")),
	}

	_, err := c.Post(url, httpclient.SetBody(req))
	if err != nil {
		return fmt.Errorf("Failed to add multi cluster, error: %s", err)
	}

	return nil
}

type clusterResp struct {
	Name  string `json:"name"`
	Local bool   `json:"local"`
}

type ErrorMessage struct {
	Type string `json:"type"`
	Code int    `json:"code"`
}

func (c *Client) GetLocalCluster() (*clusterResp, error) {
	url := fmt.Sprintf("/cluster/clusters/%s", setting.LocalClusterID)

	clusterResp := &clusterResp{}
	resp, err := c.Get(url, httpclient.SetResult(clusterResp))
	if err != nil {
		log.Errorf("failed to get local cluster, err: %v", err)
		errorMessage := new(ErrorMessage)
		err := json.Unmarshal(resp.Body(), errorMessage)
		if err != nil {
			return nil, fmt.Errorf("Failed to get cluster, error: %s", err)
		}
		if errorMessage.Code == 6643 {
			return nil, nil
		}
		return nil, fmt.Errorf("Failed to get cluster, error: %s", err)
	}

	return clusterResp, nil
}

type ClusterDetail struct {
	ID           primitive.ObjectID       `json:"id,omitempty"              bson:"_id,omitempty"`
	Name         string                   `json:"name"                      bson:"name"`
	Tags         []string                 `json:"tags"                      bson:"tags"`
	Description  string                   `json:"description"               bson:"description"`
	Status       setting.K8SClusterStatus `json:"status"                    bson:"status"`
	Error        string                   `json:"error"                     bson:"error"`
	Yaml         string                   `json:"yaml"                      bson:"yaml"`
	Production   bool                     `json:"production"                bson:"production"`
	CreatedAt    int64                    `json:"createdAt"                 bson:"createdAt"`
	CreatedBy    string                   `json:"createdBy"                 bson:"createdBy"`
	Disconnected bool                     `json:"-"                         bson:"disconnected"`
	Token        string                   `json:"token"                     bson:"-"`
	Provider     int8                     `json:"provider"                  bson:"provider"`
	Local        bool                     `json:"local"                     bson:"local"`
	Cache        types.Cache              `json:"cache"                     bson:"cache"`
	DindCfg      *DindCfg                 `json:"dind_cfg"                  bson:"dind_cfg"`

	// new field in 1.14, intended to enable kubeconfig for cluster management
	Type       string `json:"type"           bson:"type"` // either agent or kubeconfig supported
	KubeConfig string `json:"kube_config"    bson:"kube_config"`

	// Deprecated field, it should be deleted in version 1.15 since no more namespace settings is used
	Namespace      string          `json:"namespace"                 bson:"namespace"`
	AdvancedConfig *AdvancedConfig `json:"advanced_config"           bson:"advanced_config"`
}

type AdvancedConfig struct {
	Strategy          string   `json:"strategy,omitempty"       bson:"strategy,omitempty"`
	ProjectNames      []string `json:"-"                        bson:"-"`
	Tolerations       string   `json:"tolerations"              bson:"tolerations"`
	ClusterAccessYaml string   `json:"cluster_access_yaml"      bson:"cluster_access_yaml"`
	ScheduleWorkflow  bool     `json:"schedule_workflow"        bson:"schedule_workflow"`
	EnableIRSA        bool     `json:"enable_irsa"              bson:"enable_irsa"`
	IRSARoleARM       string   `json:"irsa_role_arn"            bson:"irsa_role_arn"`
}

type DindCfg struct {
	Replicas      int          `json:"replicas"       bson:"replicas"`
	Resources     *Resources   `json:"resources"      bson:"resources"`
	Storage       *DindStorage `json:"storage"        bson:"storage"`
	StrategyID    string       `json:"strategy_id"    bson:"strategy_id"`
	StorageDriver string       `json:"storage_driver" bson:"storage_driver"`
}

type Resources struct {
	Limits *Limits `json:"limits" bson:"limits"`
}

type Limits struct {
	CPU    int `json:"cpu"    bson:"cpu"`
	Memory int `json:"memory" bson:"memory"`
}

type DindStorage struct {
	Type             string `json:"type"                bson:"type"`
	StorageClass     string `json:"storage_class"       bson:"storage_class"`
	StorageSizeInGiB int64  `json:"storage_size_in_gib" bson:"storage_size_in_gib"`
}

func (c *Client) GetClusterInfo(clusterID string) (*ClusterDetail, error) {
	url := fmt.Sprintf("/cluster/clusters/%s", clusterID)

	clusterResp := &ClusterDetail{}
	resp, err := c.Get(url, httpclient.SetResult(clusterResp))
	if err != nil {
		errorMessage := new(ErrorMessage)
		err := json.Unmarshal(resp.Body(), errorMessage)
		if err != nil {
			return nil, fmt.Errorf("Failed to get cluster, error: %s", err)
		}
		if errorMessage.Code == 6643 {
			return nil, nil
		}
		return nil, fmt.Errorf("Failed to get cluster, error: %s", err)
	}
	return clusterResp, nil
}
