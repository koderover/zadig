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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/buraksezer/consistent"
	"github.com/cespare/xxhash"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/util/proxy"

	"github.com/koderover/zadig/v2/pkg/config"
	hubconfig "github.com/koderover/zadig/v2/pkg/microservice/hubserver/config"
	"github.com/koderover/zadig/v2/pkg/microservice/hubserver/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/hubserver/core/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/remotedialer"
)

const (
	clustersKey = "hubserver_clusters"
)

type Member string

func (m Member) String() string {
	return string(m)
}

type hasher struct{}

func (h hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

var (
	consistentHash *consistent.Consistent
	hashMutex      sync.RWMutex
	redisCache     *cache.RedisCache
)

func init() {
	cfg := consistent.Config{
		PartitionCount:    271,
		ReplicationFactor: 20,
		Load:              1.25,
		Hasher:            hasher{},
	}
	consistentHash = consistent.New(nil, cfg)
	redisCache = cache.NewRedisCache(config.RedisCommonCacheTokenDB())
}

func Authorize(req *http.Request) (clientKey string, authed bool, err error) {
	log := log.SugaredLogger()
	token := req.Header.Get(setting.Token)

	clusterID, err := crypto.AesDecrypt(token)
	if err != nil {
		err = fmt.Errorf("token is illegal %s: %v", token, err)
		return
	}

	var cluster *models.K8SCluster
	if cluster, err = mongodb.NewK8sClusterColl().Get(clusterID); err != nil {
		err = fmt.Errorf("unknown cluster, cluster id:%s, err:%v", token, err)
		return
	}

	if cluster.Disconnected {
		err = fmt.Errorf("cluster %s is marked as disconnected", cluster.Name)
		log.Info(err.Error())
		return
	}

	params := req.Header.Get(setting.Params)
	var input input
	bytes, err := base64.StdEncoding.DecodeString(params)
	if err != nil {
		return
	}

	if err = json.Unmarshal(bytes, &input); err != nil {
		return
	}

	if input.Cluster == nil {
		err = fmt.Errorf("no cluster info found")
		return
	}

	input.Cluster.ClusterID = cluster.ID.Hex()
	input.Cluster.Joined = time.Now()
	input.Cluster.PodIP = os.Getenv("POD_IP")

	bytes, err = json.Marshal(input.Cluster)
	if err != nil {
		log.Errorf("Failed to marshal cluster info: %v", err)
		return
	}

	err = redisCache.HWrite(clustersKey, cluster.ID.Hex(), string(bytes), 0)
	if err != nil {
		log.Errorf("Failed to write cluster info to Redis: %v", err)
		return
	}

	cluster.Status = "normal"
	cluster.LastConnectionTime = time.Now().Unix()
	err = mongodb.NewK8sClusterColl().UpdateStatus(cluster)
	if err != nil {
		log.Errorf("failed to update clusters status %s %v", cluster.Name, err)
		return
	}

	log.Infof("cluster %s connected", cluster.Name)
	return cluster.ID.Hex(), true, nil
}

func Disconnect(server *remotedialer.Server, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientKey := vars["id"]
	var err error

	defer func() {
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
		}
	}()

	if err = mongodb.NewK8sClusterColl().UpdateConnectState(clientKey, true); err != nil {
		return
	}

	server.Disconnect(clientKey)
	w.WriteHeader(http.StatusOK)
}

func Restore(w http.ResponseWriter, r *http.Request) {
	log := log.SugaredLogger()
	vars := mux.Vars(r)
	clientKey := vars["id"]
	var err error

	defer func() {
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
		}
	}()

	if err = mongodb.NewK8sClusterColl().UpdateConnectState(clientKey, false); err != nil {
		return
	}

	log.Infof("cluster %s is restored", clientKey)
	w.WriteHeader(http.StatusOK)
}

var (
	er = &errorResponder{}
)

type errorResponder struct {
}

func (e *errorResponder) Error(w http.ResponseWriter, req *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(err.Error()))
}

func Forward(server *remotedialer.Server, w http.ResponseWriter, r *http.Request) {
	logger := log.SugaredLogger()

	vars := mux.Vars(r)
	clientKey := vars["id"]
	path := vars["path"]

	logger.Debugf("got forward request %s %s", clientKey, path)

	var (
		err        error
		errHandled bool
	)

	defer func() {
		if err != nil && !errHandled {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf("%#v", err)))
		}
	}()

	clusterInfoStr, err := redisCache.HGetString(clustersKey, clientKey)
	if err != nil {
		if err == redis.Nil {
			// key不存在，跳过当前循环
			errHandled = true
			logger.Infof("waiting for cluster %s to connect", clientKey)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Errorf("Failed to get cluster info: %v", err)
		return
	}

	var cluster ClusterInfo
	err = json.Unmarshal([]byte(clusterInfoStr), &cluster)
	if err != nil {
		errHandled = true
		logger.Infof("waiting for cluster %s to connect", clientKey)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var endpoint *url.URL

	endpoint, err = url.Parse(cluster.Address)
	if err != nil {
		return
	}

	endpoint.Path = path
	endpoint.RawQuery = r.URL.RawQuery
	r.URL.Host = r.Host
	r.Header.Set("authorization", "Bearer "+cluster.Token)

	transport, err := server.GetTransport(cluster.CACert, clientKey)
	if err != nil {
		log.Errorf(fmt.Sprintf("failed to get transport, err %s", err))
		return
	}

	proxy := proxy.NewUpgradeAwareHandler(endpoint, transport, false, false, er)
	proxy.ServeHTTP(w, r)
}

func Reset() {
	log := log.SugaredLogger()

	clusters, err := mongodb.NewK8sClusterColl().FindConnectedClusters()
	if err != nil {
		log.Errorf("failed to list clusters %v", clusters)
		return
	}

	for _, cluster := range clusters {
		if cluster.Type == setting.KubeConfigClusterType {
			continue
		}
		if cluster.Status == hubconfig.Normal && !cluster.Local {
			cluster.Status = hubconfig.Abnormal
			err := mongodb.NewK8sClusterColl().UpdateStatus(cluster)
			if err != nil {
				log.Errorf("failed to update clusters status %s %v", cluster.Name, err)
			}
		}
	}
}

func Sync(server *remotedialer.Server, stopCh <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	log := log.SugaredLogger()

	for {
		select {
		case <-ticker.C:
			func() {
				clusterInfos, err := mongodb.NewK8sClusterColl().FindConnectedClusters()
				if err != nil {
					log.Errorf("failed to list clusters, err %v", err)
					return
				}

				for _, cluster := range clusterInfos {
					if cluster.Type == setting.KubeConfigClusterType {
						continue
					}
					statusChanged := false

					// 获取集群信息
					clusterInfoStr, err := redisCache.HGetString(clustersKey, cluster.ID.Hex())
					if err != nil {
						if err == redis.Nil {
							continue
						}
						log.Errorf("Failed to get cluster info: %v", err)
						continue
					}

					var clusterInfo ClusterInfo
					err = json.Unmarshal([]byte(clusterInfoStr), &clusterInfo)
					if err != nil {
						log.Errorf("Failed to unmarshal cluster info: %v", err)
						continue
					}

					// 检查 Pod IP 是否与当前 IP 匹配
					if clusterInfo.PodIP == os.Getenv("POD_IP") {
						exists, err := redisCache.HEXISTS(clustersKey, cluster.ID.Hex())
						if err != nil {
							log.Errorf("Failed to check cluster existence: %v", err)
							continue
						}
						if exists && server.HasSession(cluster.ID.Hex()) {
							if cluster.Status != hubconfig.Normal {
								log.Infof(
									"cluster %s connected changed %s => %s",
									cluster.Name, cluster.Status, hubconfig.Normal,
								)
								cluster.LastConnectionTime = time.Now().Unix()
								cluster.Status = hubconfig.Normal
								statusChanged = true
							}
						} else {
							if cluster.Status == hubconfig.Normal && !cluster.Local {
								log.Infof(
									"cluster %s disconnected changed %s => %s",
									cluster.Name, cluster.Status, hubconfig.Abnormal,
								)
								cluster.Status = hubconfig.Abnormal
								statusChanged = true
							}
						}
					}

					if statusChanged {
						err := mongodb.NewK8sClusterColl().UpdateStatus(cluster)
						if err != nil {
							log.Errorf("failed to update clusters status %s %v", cluster.Name, err)
						}
					}
				}
			}()
		case <-stopCh:
			return
		}
	}
}

func HasSession(handler *remotedialer.Server, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientKey := vars["id"]

	if handler.HasSession(clientKey) {
		exists, err := redisCache.HEXISTS(clustersKey, clientKey)
		if err != nil {
			log.Errorf("Failed to check cluster existence: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if exists {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

func CheckPodStatus(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// lookup hub-server dns
			ips, err := net.LookupIP(setting.Services[setting.HubServer].Name)
			if err != nil {
				log.Errorf("failed to lookup hub-server dns: %v", err)
				return fmt.Errorf("failed to lookup hub-server dns: %v", err)
			}

			// 获取当前一致性哈希中的节点
			hashMutex.RLock()
			currentMembers := make(map[string]struct{})
			for _, member := range consistentHash.GetMembers() {
				currentMembers[member.String()] = struct{}{}
			}

			// 检查是否有变化
			hasChange := false
			newIPs := make(map[string]struct{})
			for _, ip := range ips {
				newIPs[ip.String()] = struct{}{}
				if _, exists := currentMembers[ip.String()]; !exists {
					hasChange = true
				}
			}

			// 检查是否有节点被删除
			for member := range currentMembers {
				if _, exists := newIPs[member]; !exists {
					hasChange = true
				}
			}
			hashMutex.RUnlock()

			// 只在有变化时更新一致性哈希
			if hasChange {
				hashMutex.Lock()
				// 清空现有节点
				for _, member := range consistentHash.GetMembers() {
					consistentHash.Remove(member.String())
				}
				// 添加新节点
				for _, ip := range ips {
					consistentHash.Add(Member(ip.String()))
				}
				hashMutex.Unlock()
				log.Infof("Updated consistent hash ring with new IPs: %v", ips)

				time.Sleep(5 * time.Second)
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func GetNodeByKey(key string) string {
	hashMutex.RLock()
	defer hashMutex.RUnlock()
	member := consistentHash.LocateKey([]byte(key))
	if member == nil {
		return ""
	}
	return member.String()
}
