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
	"fmt"
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
	// terminalWatchWriteWait bounds a single frame write so a stuck spectator
	// connection cannot leak a goroutine forever.
	terminalWatchWriteWait = 10 * time.Second
	// terminalWatchPingPeriod keeps the spectator connection alive through
	// proxies during quiet periods. Must be shorter than the pong wait.
	terminalWatchPingPeriod = 30 * time.Second
	terminalWatchPongWait   = 60 * time.Second
)

// WatchTerminalSession streams the live asciicast of an in-progress terminal
// session to a read-only spectator over WebSocket. It is a system-admin-only
// audit capability: it lets an administrator watch, in real time, the commands
// and output of an active SSH / pod exec / workflow debug session.
//
// The spectator connection is strictly read-only. Any inbound frames are
// discarded and never forwarded to the real pty, so watching cannot interfere
// with or inject into the session being observed.
//
// Authorization is enforced BEFORE the WebSocket upgrade, because once upgraded
// the response is hijacked and the standard JSON error path is unavailable.
func WatchTerminalSession(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		internalhandler.JSONResponse(c, ctx)
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		internalhandler.JSONResponse(c, ctx)
		return
	}

	sessionID := c.Param("sessionID")

	// Subscribe before the upgrade so a "not live" session can still be reported
	// through the normal JSON error path.
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

	// Read pump: a spectator is read-only, so we only read to detect a closed
	// connection (and to service control frames like pong/close). All payloads
	// are discarded and never reach the real session.
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
				// Session ended: tell the spectator so it can switch to replay.
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
