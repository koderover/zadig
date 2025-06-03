package service

import (
	"encoding/json"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/hubserver/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/hubserver/core/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/redis/go-redis/v9"
)

var allClusterMap map[string]*models.K8SCluster

func GetClustersByPodIP(podIP string) ([]*models.K8SCluster, error) {
	clusters, err := mongodb.NewK8sClusterColl().FindConnectedClusters()
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %v", err)
	}

	var result []*models.K8SCluster
	for _, cluster := range clusters {
		clusterInfo, found, err := GetClusterInfo(cluster.ID.Hex())
		if err != nil {
			continue
		}
		if found && clusterInfo.PodIP == podIP {
			result = append(result, cluster)
		}
	}

	return result, nil
}

func DeleteClusterInfo(clientKey string) error {
	log.Debugf("Deleting cluster info for clientKey: %s", clientKey)
	err := redisCache.HDelete(clustersKey, clientKey)
	if err != nil {
		return fmt.Errorf("failed to delete cluster info: %v", err)
	}
	delete(allClusterMap, clientKey)
	return nil
}

func SetClusterInfo(clusterInfo *ClusterInfo, cluster *models.K8SCluster) (*models.K8SCluster, error) {
	bytes, err := json.Marshal(clusterInfo)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal cluster info: %v", err)
	}

	err = redisCache.HWrite(clustersKey, cluster.ID.Hex(), string(bytes), 0)
	if err != nil {
		return nil, fmt.Errorf("Failed to write cluster info to Redis: %v", err)
	}
	allClusterMap[cluster.ID.Hex()] = cluster
	return cluster, nil
}

func GetClusterInfo(clientKey string) (ClusterInfo, bool, error) {
	clusterInfoStr, err := redisCache.HGetString(clustersKey, clientKey)
	if err != nil {
		if err == redis.Nil {
			return ClusterInfo{}, false, nil
		}
		return ClusterInfo{}, false, fmt.Errorf("failed to get cluster info: %v", err)
	}

	var cluster ClusterInfo
	err = json.Unmarshal([]byte(clusterInfoStr), &cluster)
	if err != nil {
		return ClusterInfo{}, false, fmt.Errorf("failed to unmarshal cluster info: %v", err)
	}
	return cluster, true, nil
}
