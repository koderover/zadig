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
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	terminalaudit "github.com/koderover/zadig/v2/pkg/shared/terminalaudit"
	"github.com/koderover/zadig/v2/pkg/shared/terminalio"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	HandshakeTimeout: 5 * time.Second,
	CheckOrigin: func(r *http.Request) bool { //允许跨域
		return true
	},
}

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
	wsConn    *websocket.Conn
	sizeChan  chan remotecommand.TerminalSize
	doneChan  chan struct{}
	closeOnce sync.Once
	SessionID string
	Recorder  terminalio.Recorder
	Sanitizer terminalio.Sanitizer
}

func NewTerminalSession(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*TerminalSession, error) {
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}
	session := &TerminalSession{
		wsConn:    conn,
		sizeChan:  make(chan remotecommand.TerminalSize),
		doneChan:  make(chan struct{}),
		Sanitizer: terminalaudit.NewSanitizer(nil, nil),
	}
	return session, nil
}

func (t *TerminalSession) SetupAudit(audit *terminalaudit.AuditSession) {
	if audit == nil {
		return
	}
	t.SessionID = audit.SessionID
	t.Sanitizer = audit.Sanitizer
	t.Recorder = audit.Recorder
	log.Infof("terminal session audit attached, sessionID=%s", t.SessionID)
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
		log.Errorf("read message err: sessionID=%s err=%v", t.SessionID, err)
		_ = t.Close()
		return 0, io.EOF
	}
	var msg TerminalMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Errorf("read parse message err: sessionID=%s err=%v", t.SessionID, err)
		_ = t.Close()
		return 0, err
	}
	switch msg.Operation {
	case "stdin":
		if t.Recorder != nil {
			t.Recorder.RecordInput(msg.Data)
		}
		return copy(p, msg.Data), nil
	case "resize":
		if t.Recorder != nil {
			t.Recorder.RecordResize(msg.Cols, msg.Rows)
		}
		t.sizeChan <- remotecommand.TerminalSize{Width: msg.Cols, Height: msg.Rows}
		return 0, nil
	default:
		log.Errorf("unknown message type '%s', sessionID=%s", msg.Operation, t.SessionID)
		_ = t.Close()
		return 0, fmt.Errorf("unknown message type '%s'", msg.Operation)
	}
}

// Write called from remotecommand whenever there is any output
func (t *TerminalSession) Write(p []byte) (int, error) {
	output := terminalio.ProcessOutput(string(p), t.Recorder, t.Sanitizer)
	msg, err := json.Marshal(TerminalMessage{
		Operation: "stdout",
		Data:      output,
	})
	if err != nil {
		log.Errorf("write parse message err: %v", err)
		return 0, err
	}
	if err := t.wsConn.WriteMessage(websocket.TextMessage, msg); err != nil {
		log.Errorf("write message err: sessionID=%s err=%v", t.SessionID, err)
		_ = t.Close()
		return 0, err
	}
	return len(p), nil
}

// Close close session
func (t *TerminalSession) Close() error {
	log.Infof("terminal session close start, sessionID=%s", t.SessionID)
	t.closeOnce.Do(func() {
		log.Infof("terminal session close doneChan, sessionID=%s", t.SessionID)
		close(t.doneChan)
	})
	err := t.wsConn.Close()
	log.Infof("terminal session close finish, sessionID=%s err=%v", t.SessionID, err)
	return err
}

// 验证是否存在
func ValidatePod(kubeClient *kubernetes.Clientset, namespace, podName, containerName string) (bool, error) {
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
func ExecPod(clusterID string, cmd []string, ptyHandler PtyHandler, namespace, podName, containerName string) error {
	kubeClient, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return err
	}

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

	executor, err := clientmanager.NewKubeClientManager().GetSPDYExecutor(clusterID, req.URL())
	if err != nil {
		log.Errorf("NewSPDYExecutor err: %v", err)
		return err
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-ptyHandler.Done():
			log.Infof("pod exec stream context canceled by terminal close, namespace=%s pod=%s container=%s", namespace, podName, containerName)
			cancel()
		case <-streamCtx.Done():
		}
	}()

	err = executor.StreamWithContext(streamCtx, remotecommand.StreamOptions{
		Stdin:             ptyHandler,
		Stdout:            ptyHandler,
		Stderr:            ptyHandler,
		TerminalSizeQueue: ptyHandler,
		Tty:               true,
	})
	if errors.Is(err, context.Canceled) {
		log.Infof("pod exec stream canceled by terminal close, namespace=%s pod=%s container=%s", namespace, podName, containerName)
		return nil
	}
	log.Infof("pod exec stream completed, namespace=%s pod=%s container=%s err=%v", namespace, podName, containerName, err)
	if err != nil {
		log.Errorf("Stream err: %v", err)
		return err
	}
	return nil
}
