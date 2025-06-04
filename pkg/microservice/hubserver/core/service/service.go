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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/buraksezer/consistent"
	"github.com/cespare/xxhash"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/proxy"

	"github.com/koderover/zadig/v2/pkg/config"
	hubconfig "github.com/koderover/zadig/v2/pkg/microservice/hubserver/config"
	"github.com/koderover/zadig/v2/pkg/microservice/hubserver/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/hubserver/core/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
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

	cfg = consistent.Config{
		PartitionCount:    7,
		ReplicationFactor: 20,
		Load:              1.25,
		Hasher:            hasher{},
	}
)

func init() {
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
	input.Cluster.PodIP = config.PodIP()

	cluster, err = SetClusterInfo(input.Cluster, cluster)
	if err != nil {
		return "", false, err
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
		log.Errorf("failed to update connect state %s %v", clientKey, err)
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
	er = &ErrorResponder{}
)

type ErrorResponder struct {
}

func (e *ErrorResponder) Error(w http.ResponseWriter, req *http.Request, err error) {
	log.Errorf("respond error: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(err.Error()))
}

func Forward(server *remotedialer.Server, w http.ResponseWriter, r *http.Request) {
	logger := log.SugaredLogger()

	vars := mux.Vars(r)
	clientKey := vars["id"]
	path := vars["path"]

	// logger.Debugf("got forward request %s %s", clientKey, path)

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

	cluster, found, err := GetClusterInfo(clientKey)
	if err != nil {
		log.Errorf("failed to get cluster info: %v", err)
		return
	}

	if !found {
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
					if clusterInfo.PodIP == config.PodIP() {
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

func CheckReplicas(ctx context.Context, handler *remotedialer.Server) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			informer, err := clientmanager.NewKubeClientManager().GetInformer(setting.LocalClusterID, config.Namespace())
			if err != nil {
				return fmt.Errorf("failed to get informer: %v", err)
			}

			selector := labels.SelectorFromSet(labels.Set{
				"app.kubernetes.io/component": "hub-server",
				"app.kubernetes.io/name":      "zadig",
			})
			pods, err := getter.ListPodsWithCache(selector, informer)
			if err != nil {
				return fmt.Errorf("failed to list pods: %v", err)
			}

			ips := make([]string, 0)
			for _, pod := range pods {
				if wrapper.Pod(pod).Ready() {
					ips = append(ips, wrapper.Pod(pod).Status.PodIP)
				}
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
				newIPs[ip] = struct{}{}
				if _, exists := currentMembers[ip]; !exists {
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

				old := &consistent.Consistent{}
				if len(consistentHash.GetMembers()) > 0 {
					old = consistent.New(consistentHash.GetMembers(), cfg)
				}

				// 清空现有节点
				for _, member := range consistentHash.GetMembers() {
					consistentHash.Remove(member.String())
				}
				// 添加新节点
				for _, ip := range ips {
					consistentHash.Add(Member(ip))
				}

				log.Infof("Updated consistent hash ring with new IPs: %v", ips)

				disconnectClusters := handler.CleanSessions(old, consistentHash)
				for _, cluster := range disconnectClusters {
					DeleteClusterInfo(cluster)
				}

				hashMutex.Unlock()
			}

			time.Sleep(1 * time.Second)
		}
	}
}

func CheckConnectionStatus(ctx context.Context, handler *remotedialer.Server) {
	for {
		time.Sleep(10 * time.Second)
		select {
		case <-ctx.Done():
			return
		default:
			checkConnectionStatus(handler)
		}
	}
}

type responseRecorder struct {
	StatusCode int
	Body       io.ReadCloser
	Headers    http.Header
}

func (r *responseRecorder) Header() http.Header {
	if r.Headers == nil {
		r.Headers = make(http.Header)
	}
	return r.Headers
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.Body == nil {
		buf := &bytes.Buffer{}
        r.Body = io.NopCloser(buf)
	}
	if buf, ok := r.Body.(io.ReadWriteCloser); ok {
        return buf.Write(data)
    }
	return 0, fmt.Errorf("body is not writable")
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.StatusCode = statusCode
}

func (r *responseRecorder) Flush() {
}

func (r *responseRecorder) CloseNotify() <-chan bool {
	return make(<-chan bool)
}

func checkConnectionStatus(server *remotedialer.Server) {
	logger := log.SugaredLogger()
	for clusterID := range allClusterMap {
		cluster, found, err := GetClusterInfo(clusterID)
		if err != nil {
			log.Errorf("failed to get cluster info in connection health check, error: %v", err)
		}

		if !found {
			logger.Debugf("cluster %s not found in clusterInfo but found in map registry, removing map entry", clusterID)
			delete(allClusterMap, clusterID)
		}

		var endpoint *url.URL

		endpoint, err = url.Parse(cluster.Address)
		if err != nil {
			return
		}

		endpoint.Path = "/api/v1/namespaces/default"
		req, err := http.NewRequest("GET", endpoint.String(), nil)
		if err != nil {
			log.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("authorization", "Bearer "+cluster.Token)

		transport, err := server.GetTransport(cluster.CACert, clusterID)
		if err != nil {
			log.Errorf(fmt.Sprintf("failed to get transport, err %s", err))
			return
		}

		recorder := &responseRecorder{}

		proxy := proxy.NewUpgradeAwareHandler(endpoint, transport, false, false, er)
		proxy.ServeHTTP(recorder, req)

		if recorder.StatusCode >= 400 {
			// TODO: unavailable status, remove the connection from hubserver
			fmt.Printf("Connection check failed, status code: %d\n", recorder.StatusCode)
		} else {
			fmt.Printf("Connection successful, status code: %d\n", recorder.StatusCode)
		}
		body, _ := io.ReadAll(recorder.Body)
		fmt.Printf("Response body: %s\n", string(body))
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
