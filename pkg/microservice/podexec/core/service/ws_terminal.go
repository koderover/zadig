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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	conf "github.com/koderover/zadig/pkg/microservice/podexec/config"
	"github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/tool/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	HandshakeTimeout: 5 * time.Second,
	CheckOrigin: func(r *http.Request) bool { //允许跨域
		return true
	},
}

const (
	EndOfTransmission = "\u0004"
)

// TerminalMessage is the messaging protocol between ShellController and TerminalSession.
type TerminalMessage struct {
	Operation string `json:"operation"`
	Data      string `json:"data"`
	Rows      uint16 `json:"rows"`
	Cols      uint16 `json:"cols"`
}

type PtyHandler interface {
	io.Reader
	io.Writer
	remotecommand.TerminalSizeQueue
	Done() chan struct{}
}

// TerminalSession implements PtyHandler
type TerminalSession struct {
	wsConn   *websocket.Conn
	sizeChan chan remotecommand.TerminalSize
	doneChan chan struct{}
}

func NewTerminalSession(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*TerminalSession, error) {
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}
	session := &TerminalSession{
		wsConn:   conn,
		sizeChan: make(chan remotecommand.TerminalSize),
		doneChan: make(chan struct{}),
	}
	return session, nil
}

// Done done
func (t *TerminalSession) Done() chan struct{} {
	return t.doneChan
}

// Next called in a loop from remotecommand as long as the process is running
func (t *TerminalSession) Next() *remotecommand.TerminalSize {
	select {
	case size := <-t.sizeChan:
		return &size
	case <-t.doneChan:
		return nil
	}
}

// Read called in a loop from remotecommand as long as the process is running
func (t *TerminalSession) Read(p []byte) (int, error) {
	_, message, err := t.wsConn.ReadMessage()
	if err != nil {
		log.Errorf("read message err: %v", err)
		return copy(p, EndOfTransmission), err
	}
	var msg TerminalMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Errorf("read parse message err: %v", err)
		return copy(p, EndOfTransmission), err
	}
	switch msg.Operation {
	case "stdin":
		return copy(p, msg.Data), nil
	case "resize":
		t.sizeChan <- remotecommand.TerminalSize{Width: msg.Cols, Height: msg.Rows}
		return 0, nil
	default:
		log.Errorf("unknown message type '%s'", msg.Operation)
		return copy(p, EndOfTransmission), fmt.Errorf("unknown message type '%s'", msg.Operation)
	}
}

// Write called from remotecommand whenever there is any output
func (t *TerminalSession) Write(p []byte) (int, error) {
	msg, err := json.Marshal(TerminalMessage{
		Operation: "stdout",
		Data:      string(p),
	})
	if err != nil {
		log.Errorf("write parse message err: %v", err)
		return 0, err
	}
	if err := t.wsConn.WriteMessage(websocket.TextMessage, msg); err != nil {
		log.Errorf("write message err: %v", err)
		return 0, err
	}
	return len(p), nil
}

// Close close session
func (t *TerminalSession) Close() error {
	return t.wsConn.Close()
}

// 验证是否存在
func ValidatePod(kubeClient kubernetes.Interface, namespace, podName, containerName string) (bool, error) {
	pod, err := kubeClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return false, fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", pod.Status.Phase)
	}

	for _, c := range pod.Spec.Containers {
		if containerName == c.Name {
			return true, nil
		}
	}

	if wrapper.CheckEphemeralContainerFieldExist(&pod.Spec) {
		for _, c := range pod.Spec.EphemeralContainers {
			if containerName == c.Name {
				return true, nil
			}
		}
	}

	return false, fmt.Errorf("pod has no container '%s'", containerName)
}

// ExecPod do pod exec
func ExecPod(kubeClient kubernetes.Interface, cfg *rest.Config, cmd []string, ptyHandler PtyHandler, namespace, podName, containerName string) error {
	req := kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		log.Errorf("NewSPDYExecutor err: %v", err)
		return err
	}

	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:             ptyHandler,
		Stdout:            ptyHandler,
		Stderr:            ptyHandler,
		TerminalSizeQueue: ptyHandler,
		Tty:               true,
	})
	if err != nil {
		log.Errorf("Stream err: %v", err)
		return err
	}
	return nil
}

// NewKubeOutClusterClient returns kubeClient and config
func NewKubeOutClusterClient(clusterID string) (kubernetes.Interface, *rest.Config, error) {
	clientset, err := client.GetClientset(conf.HubServerAddr(), clusterID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to init clientset for cluster: %s, err:%v", clusterID, err)
	}

	config, err := client.GetRESTConfig(conf.HubServerAddr(), clusterID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get restconfig for cluster: %s, err:%v", clusterID, err)
	}

	config.QPS = 20
	config.Burst = 40
	return clientset, config, nil
}
