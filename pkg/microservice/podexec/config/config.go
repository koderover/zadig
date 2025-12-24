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

package config

import (
	"time"

	configbase "github.com/koderover/zadig/v2/pkg/config"
)

// WebSocket 终端配置
type TerminalConfig struct {
	// 会话 TTL，断开后会话保持时长
	SessionTTL time.Duration
	// 心跳间隔
	HeartbeatInterval time.Duration
	// 心跳超时时间
	HeartbeatTimeout time.Duration
	// Ping 超时时间
	PingTimeout time.Duration
	// 输出缓冲区大小
	OutputBufferSize int
	// 清理过期会话的间隔
	CleanupInterval time.Duration
}

// DefaultTerminalConfig 返回默认终端配置
func DefaultTerminalConfig() *TerminalConfig {
	return &TerminalConfig{
		SessionTTL:        2 * time.Minute,  // 2 分钟会话保持
		HeartbeatInterval: 10 * time.Second, // 10 秒心跳间隔
		HeartbeatTimeout:  30 * time.Second, // 30 秒心跳超时
		PingTimeout:       5 * time.Second,  // 5 秒 Ping 超时
		OutputBufferSize:  100,              // 缓冲 100 条输出
		CleanupInterval:   30 * time.Second, // 30 秒清理一次
	}
}

func HubServerAddr() string {
	return configbase.HubServerServiceAddress()
}
