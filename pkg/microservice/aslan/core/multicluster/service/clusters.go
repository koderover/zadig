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
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/types"
)

var namePattern = regexp.MustCompile(`^[0-9a-zA-Z_.-]{1,32}$`)

type K8SCluster struct {
	ID             string                   `json:"id,omitempty"`
	Name           string                   `json:"name"`
	Description    string                   `json:"description"`
	AdvancedConfig *AdvancedConfig          `json:"advanced_config,omitempty"`
	Status         setting.K8SClusterStatus `json:"status"`
	Production     bool                     `json:"production"`
	CreatedAt      int64                    `json:"createdAt"`
	CreatedBy      string                   `json:"createdBy"`
	Provider       int8                     `json:"provider"`
	Local          bool                     `json:"local"`
	Cache          types.Cache              `json:"cache"`
}

type AdvancedConfig struct {
	Strategy     string   `json:"strategy,omitempty"        bson:"strategy,omitempty"`
	NodeLabels   []string `json:"node_labels,omitempty"     bson:"node_labels,omitempty"`
	ProjectNames []string `json:"project_names"             bson:"project_names"`
}

func (k *K8SCluster) Clean() error {
	k.Name = strings.TrimSpace(k.Name)
	k.Description = strings.TrimSpace(k.Description)

	if !namePattern.MatchString(k.Name) {
		return fmt.Errorf("The cluster name does not meet the rules")
	}

	return nil
}

func ListClusters(ids []string, projectName string, logger *zap.SugaredLogger) ([]*K8SCluster, error) {
	idSet := sets.NewString(ids...)
	cs, err := commonrepo.NewK8SClusterColl().List(&commonrepo.ClusterListOpts{IDs: idSet.UnsortedList()})
	if err != nil {
		logger.Errorf("Failed to list clusters, err: %s", err)
		return nil, err
	}

	existClusterID := sets.NewString()
	if projectName != "" {
		projectClusterRelations, _ := commonrepo.NewProjectClusterRelationColl().List(&commonrepo.ProjectClusterRelationOption{
			ProjectName: projectName,
		})
		for _, projectClusterRelation := range projectClusterRelations {
			existClusterID.Insert(projectClusterRelation.ClusterID)
		}
	}

	var res []*K8SCluster
	for _, c := range cs {
		if projectName != "" && !existClusterID.Has(c.ID.Hex()) {
			continue
		}

		// If the advanced configuration is empty, the project information needs to be returned,
		// otherwise the front end will report an error
		if c.AdvancedConfig == nil {
			c.AdvancedConfig = &commonmodels.AdvancedConfig{}
		}

		var advancedConfig *AdvancedConfig
		if c.AdvancedConfig != nil {
			advancedConfig = &AdvancedConfig{
				Strategy:     c.AdvancedConfig.Strategy,
				NodeLabels:   convertToNodeLabels(c.AdvancedConfig.NodeLabels),
				ProjectNames: getProjectNames(c.ID.Hex(), logger),
			}
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
			Cache:          c.Cache,
		})
	}

	return res, nil
}

func GetCluster(id string, logger *zap.SugaredLogger) (*commonmodels.K8SCluster, error) {
	s, _ := kube.NewService("")

	return s.GetCluster(id, logger)
}

func getProjectNames(clusterID string, logger *zap.SugaredLogger) (projectNames []string) {
	projectClusterRelations, err := commonrepo.NewProjectClusterRelationColl().List(&commonrepo.ProjectClusterRelationOption{ClusterID: clusterID})
	if err != nil {
		logger.Errorf("Failed to list projectClusterRelation, err:%s", err)
		return []string{}
	}
	for _, projectClusterRelation := range projectClusterRelations {
		projectNames = append(projectNames, projectClusterRelation.ProjectName)
	}
	return projectNames
}

func convertToNodeLabels(nodeSelectorRequirements []*commonmodels.NodeSelectorRequirement) []string {
	var nodeLabels []string
	for _, nodeSelectorRequirement := range nodeSelectorRequirements {
		for _, labelValue := range nodeSelectorRequirement.Value {
			nodeLabels = append(nodeLabels, fmt.Sprintf("%s:%s", nodeSelectorRequirement.Key, labelValue))
		}
	}

	return nodeLabels
}

func convertToNodeSelectorRequirements(nodeLabels []string) []*commonmodels.NodeSelectorRequirement {
	var nodeSelectorRequirements []*commonmodels.NodeSelectorRequirement
	nodeLabelM := make(map[string][]string)
	for _, nodeLabel := range nodeLabels {
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
	var advancedConfig *commonmodels.AdvancedConfig
	if args.AdvancedConfig != nil {
		advancedConfig = &commonmodels.AdvancedConfig{
			Strategy:     args.AdvancedConfig.Strategy,
			NodeLabels:   convertToNodeSelectorRequirements(args.AdvancedConfig.NodeLabels),
			ProjectNames: args.AdvancedConfig.ProjectNames,
		}
	}

	// If user does not configure a cache for the cluster, object storage is used by default.
	err := setClusterCache(args)
	if err != nil {
		return nil, fmt.Errorf("failed to set cache for cluster %s: %s", args.ID, err)
	}

	cluster := &commonmodels.K8SCluster{
		Name:           args.Name,
		Description:    args.Description,
		AdvancedConfig: advancedConfig,
		Status:         args.Status,
		Production:     args.Production,
		Provider:       args.Provider,
		CreatedAt:      args.CreatedAt,
		CreatedBy:      args.CreatedBy,
		Cache:          args.Cache,
	}

	return s.CreateCluster(cluster, args.ID, logger)
}

func UpdateCluster(id string, args *K8SCluster, logger *zap.SugaredLogger) (*commonmodels.K8SCluster, error) {
	s, _ := kube.NewService("")
	advancedConfig := new(commonmodels.AdvancedConfig)
	if args.AdvancedConfig != nil {
		advancedConfig.Strategy = args.AdvancedConfig.Strategy
		advancedConfig.NodeLabels = convertToNodeSelectorRequirements(args.AdvancedConfig.NodeLabels)
		// Delete all projects associated with clusterID
		err := commonrepo.NewProjectClusterRelationColl().Delete(&commonrepo.ProjectClusterRelationOption{ClusterID: id})
		if err != nil {
			logger.Errorf("Failed to delete projectClusterRelation err:%s", err)
		}
		for _, projectName := range args.AdvancedConfig.ProjectNames {
			err = commonrepo.NewProjectClusterRelationColl().Create(&commonmodels.ProjectClusterRelation{
				ProjectName: projectName,
				ClusterID:   id,
				CreatedBy:   args.CreatedBy,
			})
			if err != nil {
				logger.Errorf("Failed to create projectClusterRelation err:%s", err)
			}
		}
	}

	// If user does not configure a cache for the cluster, object storage is used by default.
	err := setClusterCache(args)
	if err != nil {
		return nil, fmt.Errorf("failed to set cache for cluster %s: %s", id, err)
	}

	// If the user chooses to use dynamically generated storage resources, the system automatically creates the PVC.
	// TODO: If the PVC is not successfully bound to the PV, it is necessary to consider how to expose this abnormal information.
	//       Depends on product design.
	if args.Cache.MediumType == types.NFSMedium && args.Cache.NFSProperties.ProvisionType == types.DynamicProvision {
		kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), id)
		if err != nil {
			return nil, fmt.Errorf("failed to get kube client: %s", err)
		}

		var namespace string
		switch id {
		case setting.LocalClusterID:
			namespace = config.Namespace()
		default:
			namespace = setting.AttachedClusterNamespace
		}

		pvcName := fmt.Sprintf("cache-%s-%d", args.Cache.NFSProperties.StorageClass, args.Cache.NFSProperties.StorageSizeInGiB)
		pvc := &corev1.PersistentVolumeClaim{}
		err = kclient.Get(context.TODO(), client.ObjectKey{
			Name:      pvcName,
			Namespace: namespace,
		}, pvc)
		if err == nil {
			logger.Infof("PVC %s eixsts in %s", pvcName, namespace)
			args.Cache.NFSProperties.PVC = pvcName
		} else if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to find PVC %s in %s: %s", pvcName, namespace, err)
		} else {
			filesystemVolume := corev1.PersistentVolumeFilesystem
			storageQuantity, err := resource.ParseQuantity(fmt.Sprintf("%dGi", args.Cache.NFSProperties.StorageSizeInGiB))
			if err != nil {
				return nil, fmt.Errorf("failed to parse storage size: %d. err: %s", args.Cache.NFSProperties.StorageSizeInGiB, err)
			}

			pvc = &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pvcName,
					Namespace: namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &args.Cache.NFSProperties.StorageClass,
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteMany,
					},
					VolumeMode: &filesystemVolume,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: storageQuantity,
						},
					},
				},
			}

			err = kclient.Create(context.TODO(), pvc)
			if err != nil {
				return nil, fmt.Errorf("failed to create PVC %s in %s: %s", pvcName, namespace, err)
			}

			logger.Infof("Successfully create PVC %s in %s", pvcName, namespace)
			args.Cache.NFSProperties.PVC = pvcName
		}
	}

	cluster := &commonmodels.K8SCluster{
		Name:           args.Name,
		Description:    args.Description,
		AdvancedConfig: advancedConfig,
		Production:     args.Production,
		Cache:          args.Cache,
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

	if err = commonrepo.NewProjectClusterRelationColl().Delete(&commonrepo.ProjectClusterRelationOption{ClusterID: clusterID}); err != nil {
		logger.Errorf("Failed to delete projectClusterRelation err:%s", err)
	}

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

	return s.GetYaml(id, config.HubAgentImage(), config.ResourceServerImage(), configbase.SystemAddress(), hubURI, useDeployment, logger)
}

func setClusterCache(args *K8SCluster) error {
	if args.Cache.MediumType != "" {
		return nil
	}

	defaultStorage, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		return fmt.Errorf("failed to find default object storage for cluster %s: %s", args.ID, err)
	}

	// Since the attached cluster cannot access the minio in the local cluster, if the default object storage
	// is minio, the attached cluster cannot be configured to use minio as a cache.
	//
	// Note:
	// - Since the local cluster will be written to the database when Zadig is created, empty ID indicates
	//   that the cluster is attached.
	// - Currently, we do not support attaching the local cluster again.
	if (args.ID == "" || args.ID != setting.LocalClusterID) && strings.Contains(defaultStorage.Endpoint, ZadigMinioSVC) {
		return nil
	}

	args.Cache.MediumType = types.ObjectMedium
	args.Cache.ObjectProperties = types.ObjectProperties{
		ID: defaultStorage.ID.Hex(),
	}

	return nil
}
