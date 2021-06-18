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

package remotedialer

import (
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type wsConn struct {
	sync.Mutex
	conn *websocket.Conn
}

func newWSConn(conn *websocket.Conn) *wsConn {
	w := &wsConn{
		conn: conn,
	}
	w.setupDeadline()
	return w
}

func (w *wsConn) WriteMessage(messageType int, data []byte) error {
	w.Lock()
	defer w.Unlock()
	w.conn.SetWriteDeadline(time.Now().Add(PingWaitDuration))
	return w.conn.WriteMessage(messageType, data)
}

func (w *wsConn) NextReader() (int, io.Reader, error) {
	return w.conn.NextReader()
}

func (w *wsConn) setupDeadline() {
	w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration))
	w.conn.SetPingHandler(func(string) error {
		w.Lock()
		w.conn.WriteControl(websocket.PongMessage, []byte(""), time.Now().Add(time.Second))
		w.Unlock()
		return w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration))
	})
	w.conn.SetPongHandler(func(string) error {
		return w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration))
	})

}
