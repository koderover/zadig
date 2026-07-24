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

package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	terminalaudit "github.com/koderover/zadig/v2/pkg/shared/terminalaudit"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var terminalWatchUpgrader = websocket.Upgrader{
	ReadBufferSize:   1024,
	WriteBufferSize:  4096,
	HandshakeTimeout: 5 * time.Second,
	Subprotocols:     []string{"v2.asciicast"},
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const (
	terminalWatchWriteWait  = 10 * time.Second
	terminalWatchPingPeriod = 30 * time.Second
	terminalWatchPongWait   = 60 * time.Second
)

// WatchTerminalSession streams an active session to a read-only administrator.
func WatchTerminalSession(c *gin.Context) {
	ctx, authorized := newTerminalAuditAdminContext(c)
	if !authorized {
		internalhandler.JSONResponse(c, ctx)
		return
	}

	sessionID := c.Param("sessionID")

	// Subscribe before upgrading so lookup errors can still use the HTTP response.
	frames, unsubscribe, err := terminalaudit.WatchSession(sessionID)
	if err != nil {
		ctx.RespErr = err
		internalhandler.JSONResponse(c, ctx)
		return
	}
	defer unsubscribe()

	conn, err := terminalWatchUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorf("terminal watch upgrade failed, sessionID=%s err=%v", sessionID, err)
		return
	}
	defer conn.Close()
	log.Infof("terminal watch attached, sessionID=%s user=%s", sessionID, ctx.UserName)

	// Drain inbound frames only to handle control messages and disconnection.
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		conn.SetReadLimit(512)
		_ = conn.SetReadDeadline(time.Now().Add(terminalWatchPongWait))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(terminalWatchPongWait))
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ping := time.NewTicker(terminalWatchPingPeriod)
	defer ping.Stop()

	for {
		select {
		case <-closed:
			log.Infof("terminal watch spectator disconnected, sessionID=%s", sessionID)
			return
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(terminalWatchWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case line, ok := <-frames:
			if !ok {
				_ = conn.SetWriteDeadline(time.Now().Add(terminalWatchWriteWait))
				_ = conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session ended"))
				log.Infof("terminal watch stream ended, sessionID=%s", sessionID)
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(terminalWatchWriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
				log.Errorf("terminal watch write failed, sessionID=%s err=%v", sessionID, err)
				return
			}
		}
	}
}
