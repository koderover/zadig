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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestSessionManager_CreateSession(t *testing.T) {
	sm := GetSessionManager()

	// 创建模拟的 HTTP 请求和响应
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		execCtx := &ExecContext{
			ClusterID:     "test-cluster",
			Namespace:     "default",
			PodName:       "test-pod",
			ContainerName: "test-container",
			Command:       []string{"/bin/sh"},
		}

		userInfo := &UserInfo{
			UserID:   "test-user-123",
			UserName: "testuser",
		}

		sessionID, terminal, err := sm.CreateSession(w, r, execCtx, nil, userInfo)
		if err != nil {
			t.Errorf("Failed to create session: %v", err)
			return
		}

		if sessionID == "" {
			t.Error("Session ID should not be empty")
		}

		if terminal == nil {
			t.Error("Terminal should not be nil")
		}

		// 验证会话是否被保存
		session, err := sm.GetSession(sessionID)
		if err != nil {
			t.Errorf("Failed to get session: %v", err)
		}

		if session.ExecContext.PodName != "test-pod" {
			t.Errorf("Expected pod name 'test-pod', got '%s'", session.ExecContext.PodName)
		}

		// 清理
		sm.RemoveSession(sessionID)
	}))
	defer server.Close()

	// 将 HTTP URL 转换为 WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// 创建 WebSocket 客户端连接
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()
}

func TestSessionManager_GetSession(t *testing.T) {
	sm := GetSessionManager()

	// 测试获取不存在的会话
	_, err := sm.GetSession("non-existent-session")
	if err == nil {
		t.Error("Expected error when getting non-existent session")
	}
}

func TestSessionManager_KeepAlive(t *testing.T) {
	sm := GetSessionManager()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		execCtx := &ExecContext{
			ClusterID:     "test-cluster",
			Namespace:     "default",
			PodName:       "test-pod",
			ContainerName: "test-container",
			Command:       []string{"/bin/sh"},
		}

		userInfo := &UserInfo{
			UserID:   "test-user-123",
			UserName: "testuser",
		}

		sessionID, _, err := sm.CreateSession(w, r, execCtx, nil, userInfo)
		if err != nil {
			t.Errorf("Failed to create session: %v", err)
			return
		}

		// 获取初始的最后活跃时间
		session1, _ := sm.GetSession(sessionID)
		lastActiveAt1 := session1.LastActiveAt

		// 等待一小段时间
		time.Sleep(100 * time.Millisecond)

		// 调用 KeepAlive
		sm.KeepAlive(sessionID)

		// 验证最后活跃时间已更新
		session2, _ := sm.GetSession(sessionID)
		lastActiveAt2 := session2.LastActiveAt

		if !lastActiveAt2.After(lastActiveAt1) {
			t.Error("LastActiveAt should be updated after KeepAlive")
		}

		// 清理
		sm.RemoveSession(sessionID)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()
}

func TestSessionManager_RemoveSession(t *testing.T) {
	sm := GetSessionManager()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		execCtx := &ExecContext{
			ClusterID:     "test-cluster",
			Namespace:     "default",
			PodName:       "test-pod",
			ContainerName: "test-container",
			Command:       []string{"/bin/sh"},
		}

		userInfo := &UserInfo{
			UserID:   "test-user-123",
			UserName: "testuser",
		}

		sessionID, _, err := sm.CreateSession(w, r, execCtx, nil, userInfo)
		if err != nil {
			t.Errorf("Failed to create session: %v", err)
			return
		}

		// 验证会话存在
		_, err = sm.GetSession(sessionID)
		if err != nil {
			t.Errorf("Session should exist: %v", err)
		}

		// 移除会话
		sm.RemoveSession(sessionID)

		// 验证会话已被移除
		_, err = sm.GetSession(sessionID)
		if err == nil {
			t.Error("Session should be removed")
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()
}

func TestSessionManager_GetActiveSessions(t *testing.T) {
	sm := GetSessionManager()

	initialCount := sm.GetActiveSessions()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		execCtx := &ExecContext{
			ClusterID:     "test-cluster",
			Namespace:     "default",
			PodName:       "test-pod",
			ContainerName: "test-container",
			Command:       []string{"/bin/sh"},
		}

		userInfo := &UserInfo{
			UserID:   "test-user-123",
			UserName: "testuser",
		}

		sessionID, _, err := sm.CreateSession(w, r, execCtx, nil, userInfo)
		if err != nil {
			t.Errorf("Failed to create session: %v", err)
			return
		}

		// 验证活跃会话数增加
		newCount := sm.GetActiveSessions()
		if newCount != initialCount+1 {
			t.Errorf("Expected %d active sessions, got %d", initialCount+1, newCount)
		}

		// 清理
		sm.RemoveSession(sessionID)

		// 验证活跃会话数恢复
		finalCount := sm.GetActiveSessions()
		if finalCount != initialCount {
			t.Errorf("Expected %d active sessions after removal, got %d", initialCount, finalCount)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()
}

func TestGenerateSessionID(t *testing.T) {
	// 测试生成的 Session ID 是否唯一
	id1, err := generateSessionID()
	if err != nil {
		t.Errorf("Failed to generate session ID: %v", err)
	}

	id2, err := generateSessionID()
	if err != nil {
		t.Errorf("Failed to generate session ID: %v", err)
	}

	if id1 == id2 {
		t.Error("Generated session IDs should be unique")
	}

	// 验证 ID 长度（32 个十六进制字符）
	if len(id1) != 32 {
		t.Errorf("Expected session ID length 32, got %d", len(id1))
	}
}

func TestSessionManager_UserAuthentication(t *testing.T) {
	sm := GetSessionManager()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		execCtx := &ExecContext{
			ClusterID:     "test-cluster",
			Namespace:     "default",
			PodName:       "test-pod",
			ContainerName: "test-container",
			Command:       []string{"/bin/sh"},
		}

		// 用户 A 创建会话
		userA := &UserInfo{
			UserID:   "user-a-123",
			UserName: "userA",
		}

		sessionID, _, err := sm.CreateSession(w, r, execCtx, nil, userA)
		if err != nil {
			t.Errorf("Failed to create session: %v", err)
			return
		}

		// 验证会话绑定了用户 A
		session, err := sm.GetSession(sessionID)
		if err != nil {
			t.Errorf("Failed to get session: %v", err)
			return
		}

		if session.UserID != userA.UserID {
			t.Errorf("Expected UserID %s, got %s", userA.UserID, session.UserID)
		}

		if session.UserName != userA.UserName {
			t.Errorf("Expected UserName %s, got %s", userA.UserName, session.UserName)
		}

		// 清理
		sm.RemoveSession(sessionID)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()
}

func TestSessionManager_UnauthorizedReconnect(t *testing.T) {
	sm := GetSessionManager()

	var createdSessionID string

	// 创建会话的服务器
	createServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		execCtx := &ExecContext{
			ClusterID:     "test-cluster",
			Namespace:     "default",
			PodName:       "test-pod",
			ContainerName: "test-container",
			Command:       []string{"/bin/sh"},
		}

		// 用户 A 创建会话
		userA := &UserInfo{
			UserID:   "user-a-123",
			UserName: "userA",
		}

		sessionID, _, err := sm.CreateSession(w, r, execCtx, nil, userA)
		if err != nil {
			t.Errorf("Failed to create session: %v", err)
			return
		}

		createdSessionID = sessionID
	}))
	defer createServer.Close()

	// 创建会话
	wsURL := "ws" + strings.TrimPrefix(createServer.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	ws.Close()

	// 等待会话创建完成
	time.Sleep(100 * time.Millisecond)

	// 用户 B 尝试重连到用户 A 的会话
	reconnectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userB := &UserInfo{
			UserID:   "user-b-456",
			UserName: "userB",
		}

		// 尝试重连（应该失败）
		err := sm.ReconnectSession(createdSessionID, w, r, userB)
		if err == nil {
			t.Error("Expected reconnect to fail for different user, but it succeeded")
		} else {
			t.Logf("Correctly rejected unauthorized reconnect: %v", err)
		}
	}))
	defer reconnectServer.Close()

	// 尝试重连
	wsURL2 := "ws" + strings.TrimPrefix(reconnectServer.URL, "http")
	ws2, _, err := websocket.DefaultDialer.Dial(wsURL2, nil)
	if err == nil {
		ws2.Close()
	}

	// 清理
	sm.RemoveSession(createdSessionID)
}
