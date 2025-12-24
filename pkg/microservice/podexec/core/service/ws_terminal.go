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
	"container/ring"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
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

const (
	EndOfTransmission = "\u0004"

	// 心跳配置
	HeartbeatInterval = 10 * time.Second // 心跳间隔
	HeartbeatTimeout  = 30 * time.Second // 心跳超时
	PingTimeout       = 5 * time.Second  // Ping 超时

	// 输出缓冲配置
	OutputBufferSize = 100 // 环形缓冲区大小
)

// 消息操作类型常量
const (
	OpStdin     = "stdin"
	OpStdout    = "stdout"
	OpResize    = "resize"
	OpHeartbeat = "heartbeat"
	OpPong      = "pong"
	OpSessionID = "session_id"
)

// TerminalMessage is the messaging protocol between ShellController and TerminalSession.
type TerminalMessage struct {
	Operation string `json:"operation"`
	Data      string `json:"data"`
	Rows      uint16 `json:"rows"`
	Cols      uint16 `json:"cols"`
}

// OutputBufferItem 输出缓冲项
type OutputBufferItem struct {
	Data      []byte
	Timestamp time.Time
}

type PtyHandler interface {
	io.Reader
	io.Writer
	remotecommand.TerminalSizeQueue
	Done() chan struct{}
}

type TerminalSessionType string

const (
	// Environment is the debug terminal session type for environment
	Environment TerminalSessionType = "env"
	// Workflow is the debug terminal session type for workflow, which need musk secret envs
	Workflow TerminalSessionType = "workflow"
)

// TerminalSession implements PtyHandler
type TerminalSession struct {
	wsConn   *websocket.Conn
	sizeChan chan remotecommand.TerminalSize
	doneChan chan struct{}
	// SecretEnvs is a list of environment variables that should be hidden from the client.
	SecretEnvs []string
	Type       TerminalSessionType

	// 生产级功能新增字段
	sessionID     string
	wsMutex       sync.Mutex    // 保护 wsConn 的并发写
	lastPongAt    time.Time     // 最后收到 pong 的时间
	stopHeartbeat chan struct{} // 停止心跳信号
	outputBuffer  *ring.Ring    // 环形缓冲区保存最近输出
	bufferMutex   sync.RWMutex  // 保护输出缓冲区
}

type TerminalSessionOption struct {
	SecretEnvs []string
	Type       TerminalSessionType
}

func NewTerminalSession(w http.ResponseWriter, r *http.Request, responseHeader http.Header, opt ...*TerminalSessionOption) (*TerminalSession, error) {
	return NewTerminalSessionWithID(w, r, responseHeader, "", opt...)
}

// NewTerminalSessionWithID 创建带 Session ID 的终端会话
func NewTerminalSessionWithID(w http.ResponseWriter, r *http.Request, responseHeader http.Header, sessionID string, opt ...*TerminalSessionOption) (*TerminalSession, error) {
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}

	session := &TerminalSession{
		wsConn:        conn,
		sizeChan:      make(chan remotecommand.TerminalSize),
		doneChan:      make(chan struct{}),
		Type:          Environment,
		sessionID:     sessionID,
		lastPongAt:    time.Now(),
		stopHeartbeat: make(chan struct{}),
		outputBuffer:  ring.New(OutputBufferSize),
	}

	if len(opt) > 0 && opt[0] != nil {
		session.SecretEnvs = opt[0].SecretEnvs
		session.Type = opt[0].Type
	}

	// 设置心跳机制
	session.setupHeartbeat(conn)

	// 如果有 sessionID，发送给客户端
	if sessionID != "" {
		session.sendSessionID(sessionID)
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
	t.wsMutex.Lock()
	conn := t.wsConn
	t.wsMutex.Unlock()

	if conn == nil {
		return copy(p, EndOfTransmission), fmt.Errorf("connection closed")
	}

	_, message, err := conn.ReadMessage()
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
	case OpStdin:
		return copy(p, msg.Data), nil
	case OpResize:
		t.sizeChan <- remotecommand.TerminalSize{Width: msg.Cols, Height: msg.Rows}
		return 0, nil
	case OpPong:
		// 收到应用层 pong，更新时间
		t.lastPongAt = time.Now()
		return 0, nil
	default:
		log.Errorf("unknown message type '%s'", msg.Operation)
		return copy(p, EndOfTransmission), fmt.Errorf("unknown message type '%s'", msg.Operation)
	}
}

// Write called from remotecommand whenever there is any output
func (t *TerminalSession) Write(p []byte) (int, error) {
	// 保存到输出缓冲区
	t.bufferOutput(p)

	msg, err := json.Marshal(TerminalMessage{
		Operation: OpStdout,
		Data:      string(p),
	})
	if err != nil {
		log.Errorf("write parse message err: %v", err)
		return 0, err
	}

	// 过滤敏感信息
	if t.Type == Workflow {
		for _, secretEnv := range t.SecretEnvs {
			msg = bytes.ReplaceAll(msg, []byte(secretEnv), []byte("********"))
		}
	}

	// 使用互斥锁保护 WebSocket 写操作
	t.wsMutex.Lock()
	defer t.wsMutex.Unlock()

	if t.wsConn != nil {
		if err := t.wsConn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Errorf("write message err: %v", err)
			return 0, err
		}
	}

	return len(p), nil
}

// Close close session
func (t *TerminalSession) Close() error {
	// 停止心跳
	select {
	case <-t.stopHeartbeat:
		// 已经关闭
	default:
		close(t.stopHeartbeat)
	}

	t.wsMutex.Lock()
	defer t.wsMutex.Unlock()

	if t.wsConn != nil {
		err := t.wsConn.Close()
		t.wsConn = nil
		return err
	}
	return nil
}

// SwitchClient 切换客户端连接（用于重连场景）
func (t *TerminalSession) SwitchClient(newConn *websocket.Conn) error {
	t.wsMutex.Lock()
	oldConn := t.wsConn
	t.wsConn = newConn
	t.wsMutex.Unlock()

	// 关闭旧连接
	if oldConn != nil {
		oldConn.Close()
	}

	// 重置心跳时间
	t.lastPongAt = time.Now()

	// 为新连接设置心跳
	t.setupHeartbeat(newConn)

	// 重放缓冲区内容
	if err := t.replayRecentOutput(newConn); err != nil {
		log.Warnf("failed to replay output buffer: %v", err)
	}

	return nil
}

// setupHeartbeat 设置双重心跳机制
func (t *TerminalSession) setupHeartbeat(conn *websocket.Conn) {
	// 设置 WebSocket 原生 Pong 处理器
	conn.SetPongHandler(func(appData string) error {
		t.lastPongAt = time.Now()
		return nil
	})

	// 启动应用层心跳 goroutine
	go func() {
		ticker := time.NewTicker(HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				t.wsMutex.Lock()
				currentConn := t.wsConn
				t.wsMutex.Unlock()

				if currentConn == nil {
					return
				}

				// 发送 WebSocket 原生 Ping
				if err := currentConn.WriteControl(websocket.PingMessage, []byte{},
					time.Now().Add(PingTimeout)); err != nil {
					log.Warnf("failed to send ping: %v", err)
					return
				}

				// 发送应用层心跳
				if err := t.sendHeartbeat(); err != nil {
					log.Warnf("failed to send heartbeat: %v", err)
					return
				}

				// 检查心跳超时
				if time.Since(t.lastPongAt) > HeartbeatTimeout {
					log.Warnf("session %s heartbeat timeout, last_pong=%v", t.sessionID, t.lastPongAt)
					currentConn.Close()
					return
				}

			case <-t.stopHeartbeat:
				return

			case <-t.doneChan:
				return
			}
		}
	}()
}

// sendHeartbeat 发送应用层心跳消息
func (t *TerminalSession) sendHeartbeat() error {
	msg, err := json.Marshal(TerminalMessage{
		Operation: OpHeartbeat,
		Data:      time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	t.wsMutex.Lock()
	defer t.wsMutex.Unlock()

	if t.wsConn != nil {
		return t.wsConn.WriteMessage(websocket.TextMessage, msg)
	}
	return nil
}

// sendSessionID 发送 Session ID 给客户端
func (t *TerminalSession) sendSessionID(sessionID string) error {
	msg, err := json.Marshal(TerminalMessage{
		Operation: OpSessionID,
		Data:      sessionID,
	})
	if err != nil {
		return err
	}

	t.wsMutex.Lock()
	defer t.wsMutex.Unlock()

	if t.wsConn != nil {
		return t.wsConn.WriteMessage(websocket.TextMessage, msg)
	}
	return nil
}

// bufferOutput 将输出保存到环形缓冲区
func (t *TerminalSession) bufferOutput(data []byte) {
	t.bufferMutex.Lock()
	defer t.bufferMutex.Unlock()

	// 复制数据以避免引用问题
	bufferedData := make([]byte, len(data))
	copy(bufferedData, data)

	// 保存到环形缓冲区
	t.outputBuffer.Value = &OutputBufferItem{
		Data:      bufferedData,
		Timestamp: time.Now(),
	}
	t.outputBuffer = t.outputBuffer.Next()
}

// replayRecentOutput 重放最近的输出给新连接
func (t *TerminalSession) replayRecentOutput(conn *websocket.Conn) error {
	t.bufferMutex.RLock()
	defer t.bufferMutex.RUnlock()

	var items []*OutputBufferItem

	// 收集所有缓冲的输出
	t.outputBuffer.Do(func(v interface{}) {
		if v != nil {
			if item, ok := v.(*OutputBufferItem); ok {
				items = append(items, item)
			}
		}
	})

	// 按时间顺序重放
	for _, item := range items {
		msg, err := json.Marshal(TerminalMessage{
			Operation: OpStdout,
			Data:      string(item.Data),
		})
		if err != nil {
			continue
		}

		// 过滤敏感信息
		if t.Type == Workflow {
			for _, secretEnv := range t.SecretEnvs {
				msg = bytes.ReplaceAll(msg, []byte(secretEnv), []byte("********"))
			}
		}

		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return err
		}
	}

	log.Infof("replayed %d buffered output items for session %s", len(items), t.sessionID)
	return nil
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
