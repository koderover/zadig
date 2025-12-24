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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var (
	sessionManagerInstance *SessionManager
	sessionManagerOnce     sync.Once
)

// ExecContext 保存 Pod exec 的上下文信息
type ExecContext struct {
	ClusterID     string
	Namespace     string
	PodName       string
	ContainerName string
	Command       []string
}

// UserInfo 用户身份信息
type UserInfo struct {
	UserID   string // 用户 ID
	UserName string // 用户名
}

// ManagedSession 管理的会话对象
type ManagedSession struct {
	ID            string
	Terminal      *TerminalSession
	ExecContext   *ExecContext
	SessionOption *TerminalSessionOption
	LastActiveAt  time.Time
	CreatedAt     time.Time
	mutex         sync.RWMutex
	execStarted   bool

	// 安全增强：用户身份信息
	UserID   string // 用户 ID
	UserName string // 用户名
	ClientIP string // 客户端 IP（用于审计）
}

// SessionManager 会话管理器，管理所有活动会话
type SessionManager struct {
	sessions      sync.Map // sessionID -> *ManagedSession
	sessionTTL    time.Duration
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// GetSessionManager 获取全局会话管理器单例
func GetSessionManager() *SessionManager {
	sessionManagerOnce.Do(func() {
		sessionManagerInstance = &SessionManager{
			sessionTTL:    2 * time.Minute, // 默认 2 分钟
			cleanupTicker: time.NewTicker(30 * time.Second),
			stopCleanup:   make(chan struct{}),
		}
		// 启动后台清理 goroutine
		go sessionManagerInstance.cleanup()
		log.Info("Session manager initialized")
	})
	return sessionManagerInstance
}

// generateSessionID 生成唯一的会话 ID
func generateSessionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateSession 创建新会话并返回会话 ID 和 TerminalSession
func (sm *SessionManager) CreateSession(w http.ResponseWriter, r *http.Request, execCtx *ExecContext, opt *TerminalSessionOption, userInfo *UserInfo) (string, *TerminalSession, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate session ID: %v", err)
	}

	// 创建 TerminalSession
	terminal, err := NewTerminalSessionWithID(w, r, nil, sessionID, opt)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create terminal session: %v", err)
	}

	// 创建 ManagedSession
	managedSession := &ManagedSession{
		ID:            sessionID,
		Terminal:      terminal,
		ExecContext:   execCtx,
		SessionOption: opt,
		LastActiveAt:  time.Now(),
		CreatedAt:     time.Now(),
		execStarted:   false,

		// 安全增强：保存用户身份信息
		UserID:   userInfo.UserID,
		UserName: userInfo.UserName,
		ClientIP: r.RemoteAddr,
	}

	// 保存到 sessions map
	sm.sessions.Store(sessionID, managedSession)

	log.Infof("session %s created for user %s (%s) from %s, pod=%s/%s/%s",
		sessionID, userInfo.UserName, userInfo.UserID, r.RemoteAddr,
		execCtx.Namespace, execCtx.PodName, execCtx.ContainerName)

	return sessionID, terminal, nil
}

// GetSession 获取指定会话
func (sm *SessionManager) GetSession(sessionID string) (*ManagedSession, error) {
	value, ok := sm.sessions.Load(sessionID)
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	session := value.(*ManagedSession)
	session.mutex.RLock()
	defer session.mutex.RUnlock()

	// 检查会话是否过期
	if time.Since(session.LastActiveAt) > sm.sessionTTL {
		return nil, fmt.Errorf("session %s expired", sessionID)
	}

	return session, nil
}

// ReconnectSession 重新连接到现有会话
func (sm *SessionManager) ReconnectSession(sessionID string, w http.ResponseWriter, r *http.Request, userInfo *UserInfo) error {
	session, err := sm.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.mutex.Lock()
	defer session.mutex.Unlock()

	// 🔒 安全验证：检查用户身份
	if session.UserID != userInfo.UserID {
		log.Warnf("unauthorized reconnect attempt: user %s (%s) tried to reconnect to session %s owned by user %s (%s) from %s",
			userInfo.UserName, userInfo.UserID, sessionID, session.UserName, session.UserID, r.RemoteAddr)
		return fmt.Errorf("unauthorized: session belongs to another user")
	}

	// 审计日志：IP 变化
	if session.ClientIP != r.RemoteAddr {
		log.Infof("IP changed for session %s user %s: %s -> %s",
			sessionID, userInfo.UserName, session.ClientIP, r.RemoteAddr)
		// 更新 IP（允许 IP 变化，但记录日志）
		session.ClientIP = r.RemoteAddr
	}

	// 升级为 WebSocket 连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return fmt.Errorf("failed to upgrade connection: %v", err)
	}

	// 切换客户端连接
	if err := session.Terminal.SwitchClient(conn); err != nil {
		conn.Close()
		return fmt.Errorf("failed to switch client: %v", err)
	}

	// 更新最后活跃时间
	session.LastActiveAt = time.Now()

	log.Infof("session %s successfully reconnected for user %s (%s) from %s",
		sessionID, userInfo.UserName, userInfo.UserID, r.RemoteAddr)

	return nil
}

// KeepAlive 更新会话的最后活跃时间
func (sm *SessionManager) KeepAlive(sessionID string) {
	value, ok := sm.sessions.Load(sessionID)
	if !ok {
		return
	}

	session := value.(*ManagedSession)
	session.mutex.Lock()
	defer session.mutex.Unlock()

	session.LastActiveAt = time.Now()
}

// RemoveSession 移除指定会话
func (sm *SessionManager) RemoveSession(sessionID string) {
	value, ok := sm.sessions.Load(sessionID)
	if !ok {
		return
	}

	session := value.(*ManagedSession)
	session.mutex.Lock()
	defer session.mutex.Unlock()

	// 🆕 在关闭前发送退出消息，让前端知道这是正常退出
	if session.Terminal != nil {
		// 发送退出通知
		_ = session.Terminal.SendExitMessage("Session ended")
		// 关闭 terminal
		session.Terminal.Close()
	}

	sm.sessions.Delete(sessionID)
	log.Infof("session %s removed", sessionID)
}

// MarkExecStarted 标记会话的 exec 已启动
func (sm *SessionManager) MarkExecStarted(sessionID string) {
	value, ok := sm.sessions.Load(sessionID)
	if !ok {
		return
	}

	session := value.(*ManagedSession)
	session.mutex.Lock()
	defer session.mutex.Unlock()

	session.execStarted = true
}

// cleanup 后台清理过期会话的 goroutine
func (sm *SessionManager) cleanup() {
	for {
		select {
		case <-sm.cleanupTicker.C:
			sm.cleanupExpiredSessions()
		case <-sm.stopCleanup:
			sm.cleanupTicker.Stop()
			return
		}
	}
}

// cleanupExpiredSessions 清理所有过期的会话
func (sm *SessionManager) cleanupExpiredSessions() {
	now := time.Now()
	expiredSessions := []string{}

	sm.sessions.Range(func(key, value interface{}) bool {
		sessionID := key.(string)
		session := value.(*ManagedSession)

		session.mutex.RLock()
		lastActiveAt := session.LastActiveAt
		createdAt := session.CreatedAt
		session.mutex.RUnlock()

		// 检查是否过期
		if now.Sub(lastActiveAt) > sm.sessionTTL {
			expiredSessions = append(expiredSessions, sessionID)
			age := now.Sub(createdAt)
			log.Infof("session %s expired and will be cleaned up, age=%v, inactive=%v",
				sessionID, age, now.Sub(lastActiveAt))
		}

		return true
	})

	// 移除过期会话
	for _, sessionID := range expiredSessions {
		sm.RemoveSession(sessionID)
	}

	if len(expiredSessions) > 0 {
		log.Infof("cleaned up %d expired sessions", len(expiredSessions))
	}
}

// GetActiveSessions 获取当前活动会话数量
func (sm *SessionManager) GetActiveSessions() int {
	count := 0
	sm.sessions.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// Stop 停止会话管理器
func (sm *SessionManager) Stop() {
	close(sm.stopCleanup)

	// 关闭所有会话
	sm.sessions.Range(func(key, value interface{}) bool {
		sessionID := key.(string)
		sm.RemoveSession(sessionID)
		return true
	})

	log.Info("Session manager stopped")
}
