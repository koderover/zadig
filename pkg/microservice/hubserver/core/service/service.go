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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/koderover/zadig/pkg/microservice/hubserver/config"
	"github.com/koderover/zadig/pkg/microservice/hubserver/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/hubserver/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/crypto"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/remotedialer"
)

var clusters sync.Map

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

	clusters.Store(cluster.ID.Hex(), input.Cluster)

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

	//_, err = mongodb.NewK8sClusterColl().Get(clientKey)
	//if err != nil {
	//	return
	//}

	clusterInfo, exists := clusters.Load(clientKey)
	if !server.HasSession(clientKey) || !exists {
		for i := 0; i < 4; i++ {
			log.Infof("stuck waiting for connection index:%d", i)
			if server.HasSession(clientKey) && exists {
				log.Infof("succeeded waiting for connection index:%d", i)
				break
			}
			time.Sleep(wait.Jitter(3*time.Second, 2))
			clusterInfo, exists = clusters.Load(clientKey)
		}
	}

	if !server.HasSession(clientKey) || !exists {
		errHandled = true
		logger.Infof("waiting for cluster %s to connect", clientKey)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	cluster, ok := clusterInfo.(*ClusterInfo)
	if !ok {
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

	httpProxy := proxy.NewUpgradeAwareHandler(endpoint, transport, true, false, er)
	httpProxy.ServeHTTP(w, r)
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
		if cluster.Status == config.Normal && !cluster.Local {
			cluster.Status = config.Abnormal
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
					log.Errorf("failed to list clusters %v", clusters)
					return
				}

				for _, cluster := range clusterInfos {
					if cluster.Type == setting.KubeConfigClusterType {
						continue
					}
					statusChanged := false
					if _, ok := clusters.Load(cluster.ID.Hex()); ok && server.HasSession(cluster.ID.Hex()) {
						if cluster.Status != config.Normal {
							log.Infof(
								"cluster %s connected changed %s => %s",
								cluster.Name, cluster.Status, config.Normal,
							)
							cluster.LastConnectionTime = time.Now().Unix()
							cluster.Status = config.Normal
							statusChanged = true
						}
					} else {
						if cluster.Status == config.Normal && !cluster.Local {
							log.Infof(
								"cluster %s disconnected changed %s => %s",
								cluster.Name, cluster.Status, config.Abnormal,
							)
							cluster.Status = config.Abnormal
							statusChanged = true
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
		if _, ok := clusters.Load(clientKey); ok {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}
