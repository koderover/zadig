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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/releaseutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
	"github.com/koderover/zadig/pkg/util/ginzap"
)

var namePattern = regexp.MustCompile(`^[0-9a-zA-Z_.-]{1,32}$`)

type K8SCluster struct {
	ID                     string                   `json:"id,omitempty"`
	Name                   string                   `json:"name"`
	Description            string                   `json:"description"`
	AdvancedConfig         *AdvancedConfig          `json:"advanced_config,omitempty"`
	Status                 setting.K8SClusterStatus `json:"status"`
	Production             bool                     `json:"production"`
	CreatedAt              int64                    `json:"createdAt"`
	CreatedBy              string                   `json:"createdBy"`
	Provider               int8                     `json:"provider"`
	Local                  bool                     `json:"local"`
	Cache                  types.Cache              `json:"cache"`
	LastConnectionTime     int64                    `json:"last_connection_time"`
	UpdateHubagentErrorMsg string                   `json:"update_hubagent_error_msg"`
	DindCfg                *commonmodels.DindCfg    `json:"dind_cfg"`

	// new field in 1.14, intended to enable kubeconfig for cluster management
	Type       string `json:"type"` // either agent or kubeconfig supported
	KubeConfig string `json:"config"`
}

type AdvancedConfig struct {
	Strategy     string   `json:"strategy,omitempty"        bson:"strategy,omitempty"`
	NodeLabels   []string `json:"node_labels,omitempty"     bson:"node_labels,omitempty"`
	ProjectNames []string `json:"project_names"             bson:"project_names"`
	Tolerations  string   `json:"tolerations"               bson:"tolerations"`
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
				Tolerations:  c.AdvancedConfig.Tolerations,
			}
		}

		if c.DindCfg == nil {
			c.DindCfg = &commonmodels.DindCfg{
				Replicas: kube.DefaultDindReplicas,
				Resources: &commonmodels.Resources{
					Limits: &commonmodels.Limits{
						CPU:    kube.DefaultDindLimitsCPU,
						Memory: kube.DefaultDindLimitsMemory,
					},
				},
				Storage: &commonmodels.DindStorage{
					Type: kube.DefaultDindStorageType,
				},
			}
		}

		clusterItem := &K8SCluster{
			ID:                     c.ID.Hex(),
			Name:                   c.Name,
			Description:            c.Description,
			Status:                 c.Status,
			Production:             c.Production,
			CreatedBy:              c.CreatedBy,
			CreatedAt:              c.CreatedAt,
			Provider:               c.Provider,
			Local:                  c.Local,
			AdvancedConfig:         advancedConfig,
			Cache:                  c.Cache,
			LastConnectionTime:     c.LastConnectionTime,
			UpdateHubagentErrorMsg: c.UpdateHubagentErrorMsg,
			DindCfg:                c.DindCfg,
			KubeConfig:             c.KubeConfig,
			Type:                   c.Type,
		}

		// compatibility for the data before 1.14, since type is a new field since 1.14
		if clusterItem.Type == "" {
			clusterItem.Type = setting.AgentClusterType
		}

		res = append(res, clusterItem)
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
	s, err := kube.NewService("")
	if err != nil {
		return nil, fmt.Errorf("failed to new kube service: %s", err)
	}

	var advancedConfig *commonmodels.AdvancedConfig
	if args.AdvancedConfig != nil {
		advancedConfig = &commonmodels.AdvancedConfig{
			Strategy:     args.AdvancedConfig.Strategy,
			NodeLabels:   convertToNodeSelectorRequirements(args.AdvancedConfig.NodeLabels),
			ProjectNames: args.AdvancedConfig.ProjectNames,
			Tolerations:  args.AdvancedConfig.Tolerations,
		}
	}

	err = buildConfigs(args)
	if err != nil {
		return nil, err
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
		DindCfg:        args.DindCfg,
		Type:           args.Type,
		KubeConfig:     args.KubeConfig,
	}

	return s.CreateCluster(cluster, args.ID, logger)
}

func UpdateCluster(id string, args *K8SCluster, logger *zap.SugaredLogger) (*commonmodels.K8SCluster, error) {
	s, err := kube.NewService("")
	if err != nil {
		return nil, fmt.Errorf("failed to new kube service: %s", err)
	}

	advancedConfig := new(commonmodels.AdvancedConfig)
	if args.AdvancedConfig != nil {
		advancedConfig.Strategy = args.AdvancedConfig.Strategy
		advancedConfig.NodeLabels = convertToNodeSelectorRequirements(args.AdvancedConfig.NodeLabels)
		advancedConfig.Tolerations = args.AdvancedConfig.Tolerations
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

	err = buildConfigs(args)
	if err != nil {
		return nil, err
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
			args.DindCfg = nil
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
		DindCfg:        args.DindCfg,
		Type:           args.Type,
		KubeConfig:     args.KubeConfig,
	}

	cluster, err = s.UpdateCluster(id, cluster, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to update cluster %q: %s", id, err)
	}

	return cluster, UpgradeAgent(id, logger)
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

func UpgradeAgent(id string, logger *zap.SugaredLogger) error {
	s, err := kube.NewService("")
	if err != nil {
		return fmt.Errorf("failed to new kube service: %s", err)
	}

	clusterInfo, err := s.GetCluster(id, logger)
	if err != nil {
		return err
	}
	if clusterInfo.Status != setting.Normal {
		return fmt.Errorf("cluster %s status %s not support Upgrade Agent", clusterInfo.Name, clusterInfo.Status)
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), id)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %s cluster: %s", err, clusterInfo.Name)
	}

	// Upgrade local cluster.
	if id == setting.LocalClusterID {
		return UpgradeDind(kubeClient, clusterInfo, config.Namespace())
	}

	// Upgrade attached cluster.
	yamls, err := s.GetYaml(id, config.HubAgentImage(), config.ResourceServerImage(), configbase.SystemAddress(), "/api/hub", true, logger)
	if err != nil {
		return err
	}

	errList := new(multierror.Error)
	manifests := releaseutil.SplitManifests(string(yamls))
	resources := make([]*unstructured.Unstructured, 0, len(manifests))
	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			log.Errorf("[UpgradeAgent] Failed to convert yaml to Unstructured, manifest is\n%s\n, error: %v", item, err)
			errList = multierror.Append(errList, err)
			continue
		}
		resources = append(resources, u)
	}

	for _, u := range resources {
		if u.GetKind() == "StatefulSet" && u.GetName() == types.DindStatefulSetName {
			err = UpgradeDind(kubeClient, clusterInfo, setting.AttachedClusterNamespace)
		} else {
			err = updater.CreateOrPatchUnstructured(u, kubeClient)
		}

		if err != nil {
			log.Errorf("[UpgradeAgent] Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err)
			errList = multierror.Append(errList, err)
			continue
		}
	}
	updateHubagentErrorMsg := ""
	if len(errList.Errors) > 0 {
		updateHubagentErrorMsg = errList.Error()
	}

	return s.UpdateUpgradeAgentInfo(id, updateHubagentErrorMsg)
}

func buildConfigs(args *K8SCluster) error {
	// If user does not configure a cache for the cluster, object storage is used by default.
	err := setClusterCache(args)
	if err != nil {
		return fmt.Errorf("failed to set cache for cluster %s: %s", args.ID, err)
	}

	// If user does not set a dind config for the cluster, set the default values.
	err = setClusterDind(args)
	if err != nil {
		return fmt.Errorf("failed to set dind args for cluster %s: %s", args.ID, err)
	}

	// validate tolerations config
	err = validateTolerations(args)
	if err != nil {
		return fmt.Errorf("failed to validate toleration config for cluster %s: %s", args.ID, err)
	}
	return nil
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
	if (args.ID != setting.LocalClusterID) && strings.Contains(defaultStorage.Endpoint, ZadigMinioSVC) {
		return nil
	}

	args.Cache.MediumType = types.ObjectMedium
	args.Cache.ObjectProperties = types.ObjectProperties{
		ID: defaultStorage.ID.Hex(),
	}

	return nil
}

func setClusterDind(cluster *K8SCluster) error {
	if cluster.DindCfg == nil {
		cluster.DindCfg = &commonmodels.DindCfg{
			Replicas: kube.DefaultDindReplicas,
			Resources: &commonmodels.Resources{
				Limits: &commonmodels.Limits{
					CPU:    kube.DefaultDindLimitsCPU,
					Memory: kube.DefaultDindLimitsMemory,
				},
			},
			Storage: &commonmodels.DindStorage{
				Type: kube.DefaultDindStorageType,
			},
		}

		return nil
	}

	if cluster.DindCfg.Replicas <= 0 {
		cluster.DindCfg.Replicas = kube.DefaultDindReplicas
	}
	if cluster.DindCfg.Resources == nil {
		cluster.DindCfg.Resources = &commonmodels.Resources{
			Limits: &commonmodels.Limits{
				CPU:    kube.DefaultDindLimitsCPU,
				Memory: kube.DefaultDindLimitsMemory,
			},
		}
	}
	if cluster.DindCfg.Storage == nil {
		cluster.DindCfg.Storage = &commonmodels.DindStorage{
			Type: kube.DefaultDindStorageType,
		}
	}
	if cluster.DindCfg.Storage.Type == commonmodels.DindStorageRootfs {
		cluster.DindCfg.Storage.StorageClass = ""
		cluster.DindCfg.Storage.StorageSizeInGiB = 0
	}

	return nil
}

func validateTolerations(cluster *K8SCluster) error {
	if cluster.AdvancedConfig == nil || len(cluster.AdvancedConfig.Tolerations) == 0 {
		return nil
	}
	ts := make([]corev1.Toleration, 0)
	return yaml.Unmarshal([]byte(cluster.AdvancedConfig.Tolerations), &ts)
}

func ClusterApplyUpgrade() {
	for i := 0; i < 3; i++ {
		time.Sleep(10 * time.Second)
		k8sClusters, err := commonrepo.NewK8SClusterColl().List(nil)
		if err != nil && !commonrepo.IsErrNoDocuments(err) {
			log.Errorf("[ClusterApplyUpgrade] list cluster error:%s", err)
			return
		}

		for _, cluster := range k8sClusters {
			if cluster.Status != setting.Normal {
				log.Warnf("[ClusterApplyUpgrade] not support apply. cluster %s status %s ", cluster.Name, cluster.Status)
				continue
			}
			if err := UpgradeAgent(cluster.ID.Hex(), ginzap.WithContext(&gin.Context{}).Sugar()); err != nil {
				log.Errorf("[ClusterApplyUpgrade] UpgradeAgent cluster %s error:%s", cluster.Name, err)
			} else {
				log.Infof("[ClusterApplyUpgrade] UpgradeAgent cluster %s successed", cluster.Name)
			}
		}
		time.Sleep(20 * time.Second)
	}
}

func CheckEphemeralContainers(ctx context.Context, clusterID string) (bool, error) {
	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return false, fmt.Errorf("failed to get rest config: %s", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return false, fmt.Errorf("failed to new discovery client: %s", err)
	}

	resource, err := discoveryClient.ServerResourcesForGroupVersion("v1")
	if err != nil {
		return false, fmt.Errorf("failed to get server resource: %s", err)
	}

	for _, apiResource := range resource.APIResources {
		if apiResource.Name == "pods/ephemeralcontainers" {
			return true, nil
		}
	}

	return false, nil
}

func UpgradeDind(kclient client.Client, cluster *commonmodels.K8SCluster, ns string) error {
	if cluster.DindCfg == nil {
		return nil
	}

	ctx := context.TODO()
	dindSts := &appsv1.StatefulSet{}
	err := kclient.Get(ctx, client.ObjectKey{
		Name:      types.DindStatefulSetName,
		Namespace: ns,
	}, dindSts)
	if err != nil {
		return fmt.Errorf("failed to get dind StatefulSet in local cluster: %s", err)
	}

	dindSts.Spec.Replicas = util.GetInt32Pointer(int32(cluster.DindCfg.Replicas))

	if cluster.DindCfg.Resources != nil && cluster.DindCfg.Resources.Limits != nil {
		cpuSize := fmt.Sprintf("%dm", cluster.DindCfg.Resources.Limits.CPU)
		cpuQuantity, err := resource.ParseQuantity(cpuSize)
		if err != nil {
			return fmt.Errorf("failed to parse CPU resource %q: %s", cpuSize, err)
		}

		memSize := fmt.Sprintf("%dMi", cluster.DindCfg.Resources.Limits.Memory)
		memQuantity, err := resource.ParseQuantity(memSize)
		if err != nil {
			return fmt.Errorf("failed to parse Memory resource %q: %s", memSize, err)
		}

		for i, container := range dindSts.Spec.Template.Spec.Containers {
			if container.Name != types.DindContainerName {
				continue
			}

			container.Resources.Limits = corev1.ResourceList{
				corev1.ResourceCPU:    cpuQuantity,
				corev1.ResourceMemory: memQuantity,
			}
			dindSts.Spec.Template.Spec.Containers[i] = container
		}
	}

	if cluster.DindCfg.Storage != nil {
		switch cluster.DindCfg.Storage.Type {
		case commonmodels.DindStorageRootfs:
			for i, container := range dindSts.Spec.Template.Spec.Containers {
				if container.Name != types.DindContainerName {
					continue
				}

				volumeMounts := []corev1.VolumeMount{}
				for _, volumeMount := range container.VolumeMounts {
					if volumeMount.MountPath == types.DindMountPath {
						continue
					}

					volumeMounts = append(volumeMounts, volumeMount)
				}
				container.VolumeMounts = volumeMounts
				dindSts.Spec.Template.Spec.Containers[i] = container
			}

			volumeClaimTemplates := []corev1.PersistentVolumeClaim{}
			for _, volumeClaimTemplate := range dindSts.Spec.VolumeClaimTemplates {
				if volumeClaimTemplate.Name == types.DindMountName {
					continue
				}

				volumeClaimTemplates = append(volumeClaimTemplates, volumeClaimTemplate)
			}
			dindSts.Spec.VolumeClaimTemplates = volumeClaimTemplates
		default:
			for i, container := range dindSts.Spec.Template.Spec.Containers {
				if container.Name != types.DindContainerName {
					continue
				}

				var exists bool
				for _, volumeMount := range container.VolumeMounts {
					if volumeMount.MountPath == types.DindMountPath {
						exists = true
						break
					}
				}

				if exists {
					break
				}

				container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
					Name:      types.DindMountName,
					MountPath: types.DindMountPath,
				})
				dindSts.Spec.Template.Spec.Containers[i] = container
			}

			var dockerStorageExists bool
			for _, volumeClaimTemplate := range dindSts.Spec.VolumeClaimTemplates {
				if volumeClaimTemplate.Name == types.DindMountName {
					dockerStorageExists = true
					break
				}
			}

			if !dockerStorageExists {
				storageSize := fmt.Sprintf("%dGi", cluster.DindCfg.Storage.StorageSizeInGiB)
				storageQuantity, err := resource.ParseQuantity(storageSize)
				if err != nil {
					return fmt.Errorf("failed to parse quantity %q: %s", storageSize, err)
				}

				dindSts.Spec.VolumeClaimTemplates = append(dindSts.Spec.VolumeClaimTemplates, corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: types.DindMountName,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageClassName: util.GetStrPointer(cluster.DindCfg.Storage.StorageClass),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageQuantity,
							},
						},
					},
				})
			}
		}
	}

	err = kclient.Update(ctx, dindSts)
	if apierrors.IsInvalid(err) {
		deleteOption := metav1.DeletePropagationForeground
		err = kclient.Delete(ctx, dindSts, &client.DeleteOptions{
			PropagationPolicy: &deleteOption,
		})
		if err != nil {
			return fmt.Errorf("failed to delete StatefulSet `dind`: %s", err)
		}

		newDindSts := &appsv1.StatefulSet{}
		newDindSts.Name = dindSts.Name
		newDindSts.Namespace = dindSts.Namespace
		newDindSts.Labels = dindSts.Labels
		newDindSts.Annotations = dindSts.Annotations
		newDindSts.Spec = dindSts.Spec

		return wait.PollImmediate(3*time.Second, 30*time.Second, func() (bool, error) {
			err := kclient.Create(ctx, newDindSts)
			if err == nil {
				return true, nil
			}

			log.Warnf("Failed to create StatefulSet `dind`: %s", err)
			return false, nil
		})
	}

	return err
}
