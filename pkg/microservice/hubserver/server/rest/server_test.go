/*
Copyright 2026 The KodeRover Authors.

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

package rest

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func TestWebSocketProxyPingWriteTimeoutConfig(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"", 15 * time.Second},
		{"25s", 25 * time.Second},
		{"invalid", 15 * time.Second},
		{"0s", 15 * time.Second},
		{"-1s", 15 * time.Second},
	} {
		t.Setenv(setting.ENVHubServerWebSocketPingWriteTimeout, tc.value)
		if got := config.HubServerWebSocketPingWriteTimeout(); got != tc.want {
			t.Errorf("timeout for %q = %s, want %s", tc.value, got, tc.want)
		}
	}
}

func TestWebSocketProxyHeartbeatAndCancellation(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	var backendPing atomic.Bool
	backendDone := make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(backendDone)
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		pong := conn.PingHandler()
		conn.SetPingHandler(func(data string) error {
			backendPing.Store(true)
			return pong(data)
		})
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil || conn.WriteMessage(messageType, data) != nil {
				return
			}
		}
	}))
	defer backend.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proxyDone := make(chan struct{})
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(proxyDone)
		handleWebSocketProxy(ctx, w, r, backend.Listener.Addr().String())
	}))
	defer proxy.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+proxy.Listener.Addr().String()+"/connect", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	deadline := time.Now().Add(15 * time.Second)
	conn.SetReadDeadline(deadline)
	conn.SetWriteDeadline(deadline)

	clientPing := false
	pong := conn.PingHandler()
	conn.SetPingHandler(func(data string) error {
		clientPing = true
		return pong(data)
	})
	payload := bytes.Repeat([]byte("test payload"), 1024)
	for !clientPing || !backendPing.Load() {
		if err := conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
			t.Fatal(err)
		}
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if messageType != websocket.BinaryMessage || !bytes.Equal(data, payload) {
			t.Fatalf("unexpected echo: type %d, length %d", messageType, len(data))
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	select {
	case <-proxyDone:
	case <-shutdown.Done():
		t.Fatal("proxy did not exit after cancellation")
	}
	select {
	case <-backendDone:
	case <-shutdown.Done():
		t.Fatal("backend connection remained open after cancellation")
	}
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := conn.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
		t.Fatalf("expected client connection to close after cancellation, got %v", err)
	}
}
