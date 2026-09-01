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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
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

const EndOfTransmission = "\u0004"

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

type TerminalSessionType string

const (
	Environment TerminalSessionType = "env"
	Workflow    TerminalSessionType = "workflow"
)

// TerminalSession implements PtyHandler
type TerminalSession struct {
	wsConn     *websocket.Conn
	sizeChan   chan remotecommand.TerminalSize
	doneChan   chan struct{}
	closeOnce  sync.Once
	writeMu    sync.Mutex
	closeErr   error
	sessionID  string
	recorder   terminalio.Recorder
	SecretEnvs []string
	Type       TerminalSessionType
	// outputSanitizer preserves workflow debug's existing display masking.
	outputSanitizer terminalio.Sanitizer
}

type TerminalSessionOption struct {
	SecretEnvs []string
	Type       TerminalSessionType
}

func NewTerminalSession(w http.ResponseWriter, r *http.Request, responseHeader http.Header, opt ...*TerminalSessionOption) (*TerminalSession, error) {
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}
	session := &TerminalSession{
		wsConn:   conn,
		sizeChan: make(chan remotecommand.TerminalSize),
		doneChan: make(chan struct{}),
		recorder: terminalio.NopRecorder{},
		Type:     Environment,
	}
	if len(opt) > 0 && opt[0] != nil {
		session.SecretEnvs = opt[0].SecretEnvs
		session.Type = opt[0].Type
	}
	return session, nil
}

func (t *TerminalSession) attachAudit(sessionID string, recorder terminalio.Recorder) {
	t.sessionID = sessionID
	t.recorder = recorder
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
		log.Errorf("read message err: sessionID=%s err=%v", t.sessionID, err)
		_ = t.Close()
		if isExpectedTerminalClose(err) {
			return 0, io.EOF
		}
		return copy(p, EndOfTransmission), err
	}
	var msg TerminalMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Errorf("read parse message err: sessionID=%s err=%v", t.sessionID, err)
		return copy(p, EndOfTransmission), err
	}
	switch msg.Operation {
	case "stdin":
		t.recorder.RecordInput(msg.Data)
		return copy(p, msg.Data), nil
	case "resize":
		t.recorder.RecordResize(msg.Cols, msg.Rows)
		select {
		case t.sizeChan <- remotecommand.TerminalSize{Width: msg.Cols, Height: msg.Rows}:
			return 0, nil
		case <-t.doneChan:
			return 0, io.EOF
		}
	default:
		log.Errorf("unknown message type '%s', sessionID=%s", msg.Operation, t.sessionID)
		return copy(p, EndOfTransmission), fmt.Errorf("unknown message type '%s'", msg.Operation)
	}
}

// Write called from remotecommand whenever there is any output
func (t *TerminalSession) Write(p []byte) (int, error) {
	output := string(p)
	t.recorder.RecordOutput(output)
	if t.outputSanitizer != nil {
		output = t.outputSanitizer.Mask(output)
	}
	if err := t.writeOutput(output); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (t *TerminalSession) writeOutput(output string) error {
	if output == "" {
		return nil
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	msg, err := json.Marshal(TerminalMessage{
		Operation: "stdout",
		Data:      output,
	})
	if err != nil {
		log.Errorf("write parse message err: %v", err)
		return err
	}
	if t.outputSanitizer == nil && t.Type == Workflow {
		for _, secretEnv := range t.SecretEnvs {
			msg = bytes.ReplaceAll(msg, []byte(secretEnv), []byte("********"))
		}
	}
	if err := t.wsConn.WriteMessage(websocket.TextMessage, msg); err != nil {
		log.Errorf("write message err: sessionID=%s err=%v", t.sessionID, err)
		return err
	}
	return nil
}

// Close close session
func (t *TerminalSession) Close() error {
	t.closeOnce.Do(func() {
		close(t.doneChan)
		if t.outputSanitizer != nil {
			_ = t.writeOutput(t.outputSanitizer.Flush())
		}
		t.writeMu.Lock()
		t.closeErr = t.wsConn.Close()
		t.writeMu.Unlock()
		log.Infof("close terminal session, sessionID=%s err=%v", t.sessionID, t.closeErr)
	})
	return t.closeErr
}

// 验证是否存在
func ValidatePod(kubeClient *kubernetes.Clientset, namespace, podName, containerName string) (bool, error) {
	_, err := getValidatedPod(kubeClient, namespace, podName, containerName)
	return err == nil, err
}

func getValidatedPod(kubeClient *kubernetes.Clientset, namespace, podName, containerName string) (*corev1.Pod, error) {
	pod, err := kubeClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return nil, fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", pod.Status.Phase)
	}

	for _, c := range pod.Spec.Containers {
		if containerName == c.Name {
			return pod, nil
		}
	}

	if wrapper.CheckEphemeralContainerFieldExist(&pod.Spec) {
		for _, c := range pod.Spec.EphemeralContainers {
			if containerName == c.Name {
				return pod, nil
			}
		}
	}

	return nil, fmt.Errorf("pod has no container '%s'", containerName)
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
