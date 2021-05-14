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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/qiniu/x/log.v7"
)

func ServeWs(w http.ResponseWriter, r *http.Request) {
	// 获取路径中的参数
	pathParams := mux.Vars(r)
	namespace := pathParams["namespace"]
	podName := pathParams["podName"]
	containerName := pathParams["containerName"]
	// 获取query中的参数
	queryList := r.URL.Query()
	clusterId := queryList.Get("clusterId")

	if namespace == "" || podName == "" || containerName == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(&EndpointResponse{ResultCode: http.StatusBadRequest, ErrorMsg: "namespace,podName,containerName can't be empty,please check!"})
		return
	}
	log.Infof("exec containerName: %s, pod: %s, namespace: %s", containerName, podName, namespace)

	pty, err := NewTerminalSession(w, r, nil)
	if err != nil {
		log.Errorf("get pty failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(&EndpointResponse{ResultCode: http.StatusInternalServerError, ErrorMsg: fmt.Sprintf("get pty failed: %v", err)})
		return
	}
	defer func() {
		log.Info("close session.")
		_ = pty.Close()
	}()

	kubeCli, cfg, err := NewKubeOutClusterClient(clusterId)
	if err != nil {
		msg := fmt.Sprintf("get kubecli err :%v", err)
		log.Errorf(msg)
		_, _ = pty.Write([]byte(msg))
		pty.Done()

		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(&EndpointResponse{ResultCode: http.StatusInternalServerError, ErrorMsg: fmt.Sprintf("get kubecli err :%v", err)})
		return
	}

	ok, err := ValidatePod(kubeCli, namespace, podName, containerName)
	if !ok {
		msg := fmt.Sprintf("Validate pod error! err: %v", err)
		log.Errorf(msg)
		_, _ = pty.Write([]byte(msg))
		pty.Done()

		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(&EndpointResponse{ResultCode: http.StatusBadRequest, ErrorMsg: fmt.Sprintf("Validate pod error! err: %v", err)})
		return
	}

	err = ExecPod(kubeCli, cfg, []string{"/bin/sh"}, pty, namespace, podName, containerName)
	if err != nil {
		msg := fmt.Sprintf("Exec to pod error! err: %v", err)
		log.Errorf(msg)
		_, _ = pty.Write([]byte(msg))
		pty.Done()

		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(&EndpointResponse{ResultCode: http.StatusInternalServerError, ErrorMsg: fmt.Sprintf("Exec to pod error! err: %v", err)})
	}
}
