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
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

var namePattern = regexp.MustCompile(`^[0-9a-zA-Z_.-]{1,32}$`)

type K8SCluster struct {
	ID             string                       `json:"id,omitempty"`
	Name           string                       `json:"name"`
	Tags           []string                     `json:"tags"`
	Description    string                       `json:"description"`
	Namespace      string                       `json:"namespace"`
	Info           *commonmodels.K8SClusterInfo `json:"info,omitempty"`
	AdvancedConfig *AdvancedConfig              `json:"config,omitempty"`
	Status         setting.K8SClusterStatus     `json:"status"`
	Error          string                       `json:"error"`
	Yaml           string                       `json:"yaml"`
	Production     bool                         `json:"production"`
	CreatedAt      int64                        `json:"createdAt"`
	CreatedBy      string                       `json:"createdBy"`
	Disconnected   bool                         `json:"-"`
	Token          string                       `json:"token"`
	Provider       int8                         `json:"provider"`
	Local          bool                         `json:"local"`
}

type AdvancedConfig struct {
	Strategy   string   `json:"strategy,omitempty"      bson:"strategy,omitempty"`
	NodeLabels []string `json:"node_labels,omitempty"   bson:"node_labels,omitempty"`
}

func (k *K8SCluster) Clean() error {
	k.Name = strings.TrimSpace(k.Name)
	tags := k.Tags
	k.Tags = []string{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			k.Tags = append(k.Tags, tag)
		}
	}

	k.Description = strings.TrimSpace(k.Description)

	if !namePattern.MatchString(k.Name) {
		return fmt.Errorf("集群名称不符合规则")
	}

	return nil
}

func ListClusters(ids []string, logger *zap.SugaredLogger) ([]*K8SCluster, error) {
	idSet := sets.NewString(ids...)
	cs, err := commonrepo.NewK8SClusterColl().List(&commonrepo.ClusterListOpts{IDs: idSet.UnsortedList()})
	if err != nil {
		logger.Errorf("Failed to list clusters, err: %s", err)
		return nil, err
	}

	var res []*K8SCluster
	for _, c := range cs {
		advancedConfig := new(AdvancedConfig)
		if c.AdvancedConfig != nil {
			advancedConfig.Strategy = c.AdvancedConfig.Strategy
			var nodeLabels []string
			for _, nodeSelectorRequirement := range c.AdvancedConfig.NodeLabels {
				for _, labelValue := range nodeSelectorRequirement.Value {
					nodeLabels = append(nodeLabels, fmt.Sprintf("%s:%s", nodeSelectorRequirement.Key, labelValue))
				}
			}
			advancedConfig.NodeLabels = nodeLabels
		}
		res = append(res, &K8SCluster{
			ID:             c.ID.Hex(),
			Name:           c.Name,
			Description:    c.Description,
			Status:         c.Status,
			Production:     c.Production,
			CreatedBy:      c.CreatedBy,
			CreatedAt:      c.CreatedAt,
			Provider:       c.Provider,
			Local:          c.Local,
			AdvancedConfig: advancedConfig,
		})
	}

	return res, nil
}

func GetCluster(id string, logger *zap.SugaredLogger) (*commonmodels.K8SCluster, error) {
	s, _ := kube.NewService("")

	return s.GetCluster(id, logger)
}

func convertToNodeSelectorRequirements(args *K8SCluster) []*commonmodels.NodeSelectorRequirement {
	var nodeSelectorRequirements []*commonmodels.NodeSelectorRequirement
	nodeLabelM := make(map[string][]string)
	for _, nodeLabel := range args.AdvancedConfig.NodeLabels {
		if !strings.Contains(nodeLabel, ":") || len(strings.Split(nodeLabel, ":")) != 2 {
			continue
		}
		key := strings.Split(nodeLabel, ":")[0]
		value := strings.Split(nodeLabel, ":")[1]
		nodeLabelM[key] = append(nodeLabelM[key], value)
	}
	for key, value := range nodeLabelM {
		nodeSelectorRequirements = append(nodeSelectorRequirements, &commonmodels.NodeSelectorRequirement{
			Key:      key,
			Value:    value,
			Operator: corev1.NodeSelectorOpIn,
		})
	}
	return nodeSelectorRequirements
}

func CreateCluster(args *K8SCluster, logger *zap.SugaredLogger) (*commonmodels.K8SCluster, error) {
	s, _ := kube.NewService("")
	advancedConfig := new(commonmodels.AdvancedConfig)
	if args.AdvancedConfig != nil {
		advancedConfig.Strategy = args.AdvancedConfig.Strategy
		advancedConfig.NodeLabels = convertToNodeSelectorRequirements(args)
	}
	cluster := &commonmodels.K8SCluster{
		Name:           args.Name,
		Tags:           args.Tags,
		Description:    args.Description,
		Namespace:      args.Namespace,
		Info:           args.Info,
		AdvancedConfig: advancedConfig,
		Status:         args.Status,
		Error:          args.Error,
		Yaml:           args.Yaml,
		Production:     args.Production,
		Disconnected:   args.Disconnected,
		Token:          args.Token,
		Provider:       args.Provider,
		Local:          args.Local,
		CreatedAt:      args.CreatedAt,
		CreatedBy:      args.CreatedBy,
	}
	if args.ID != "" {
		cluster.ID, _ = primitive.ObjectIDFromHex(args.ID)
	}
	return s.CreateCluster(cluster, logger)
}

func UpdateCluster(id string, args *K8SCluster, logger *zap.SugaredLogger) (*commonmodels.K8SCluster, error) {
	s, _ := kube.NewService("")
	advancedConfig := new(commonmodels.AdvancedConfig)
	if args.AdvancedConfig != nil {
		advancedConfig.Strategy = args.AdvancedConfig.Strategy
		advancedConfig.NodeLabels = convertToNodeSelectorRequirements(args)
	}
	cluster := &commonmodels.K8SCluster{
		Name:           args.Name,
		Tags:           args.Tags,
		Description:    args.Description,
		Namespace:      args.Namespace,
		AdvancedConfig: advancedConfig,
		Production:     args.Production,
	}
	return s.UpdateCluster(id, cluster, logger)
}

func DeleteCluster(username, clusterID string, logger *zap.SugaredLogger) error {
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		ClusterID: clusterID,
	})

	if err != nil {
		return e.ErrDeleteCluster.AddErr(err)
	}

	if len(products) > 0 {
		return e.ErrDeleteCluster.AddDesc("请删除在该集群创建的环境后，再尝试删除该集群")
	}

	s, _ := kube.NewService("")

	return s.DeleteCluster(username, clusterID, logger)
}

func DisconnectCluster(username string, clusterID string, logger *zap.SugaredLogger) error {
	s, _ := kube.NewService(config.HubServerAddress())

	return s.DisconnectCluster(username, clusterID, logger)
}

func ReconnectCluster(username string, clusterID string, logger *zap.SugaredLogger) error {
	s, _ := kube.NewService(config.HubServerAddress())

	return s.ReconnectCluster(username, clusterID, logger)
}

func ProxyAgent(writer gin.ResponseWriter, request *http.Request) {
	s, _ := kube.NewService(config.HubServerAddress())

	s.ProxyAgent(writer, request)
}

func GetYaml(id, hubURI string, useDeployment bool, logger *zap.SugaredLogger) ([]byte, error) {
	s, _ := kube.NewService("")

	return s.GetYaml(id, config.HubAgentImage(), configbase.SystemAddress(), hubURI, useDeployment, logger)
}
