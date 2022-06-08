/*
Copyright 2022 The KodeRover Authors.

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

package wsconn

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"

	"github.com/koderover/zadig/pkg/tool/log"
)

type wsOperationType string

const (
	wsMsgStdout wsOperationType = "stdout"
	wsMsgStdin  wsOperationType = "stdin"
	wsMsgResize wsOperationType = "resize"
)

type wsMessage struct {
	Operation wsOperationType `json:"operation"`
	Data      string          `json:"data"`
	Rows      int             `json:"rows"`
	Cols      int             `json:"cols"`
}

type wsBufferWriter struct {
	buffer bytes.Buffer
	mu     sync.Mutex
}

func (w *wsBufferWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.Write(p)
}

type SshConn struct {
	Stdin      io.WriteCloser
	WsWriter   *wsBufferWriter
	SshSession *ssh.Session
}

func NewSshConn(cols, rows int, sshClient *ssh.Client) (*SshConn, error) {
	sshSession, err := sshClient.NewSession()
	if err != nil {
		return nil, err
	}

	stdin, err := sshSession.StdinPipe()
	if err != nil {
		return nil, err
	}

	wsWriter := new(wsBufferWriter)
	sshSession.Stdout = wsWriter
	sshSession.Stderr = wsWriter

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := sshSession.RequestPty("xterm", rows, cols, modes); err != nil {
		return nil, err
	}
	if err := sshSession.Shell(); err != nil {
		return nil, err
	}
	return &SshConn{Stdin: stdin, WsWriter: wsWriter, SshSession: sshSession}, nil
}

func (ssConn *SshConn) ReadWsMessage(wsConn *websocket.Conn, stopCh chan bool) {
	defer setStop(stopCh)
	for {
		select {
		case <-stopCh:
			return
		default:
			_, wsData, err := wsConn.ReadMessage()
			if err != nil {
				log.Errorf("read ws message err:%s", err)
				return
			}
			var wsMsgObj wsMessage
			if err := json.Unmarshal(wsData, &wsMsgObj); err != nil {
				log.Error("ReceiveWsMessage Unmarshal err:", string(wsData), err)
				return
			}

			switch wsMsgObj.Operation {
			case wsMsgResize:
				if wsMsgObj.Cols > 0 && wsMsgObj.Rows > 0 {
					if err := ssConn.SshSession.WindowChange(wsMsgObj.Rows, wsMsgObj.Cols); err != nil {
						log.Error("resize windows err:", err)
					}
				}
			case wsMsgStdin:
				decodeBytes := []byte(wsMsgObj.Data)
				if _, err := ssConn.Stdin.Write(decodeBytes); err != nil {
					log.Error("ws stdin write to ssh.stdin err:", err)
					setStop(stopCh)
				}
			}
		}
	}
}

func (ssConn *SshConn) SendWsWriteMessage(wsConn *websocket.Conn, stopCh chan bool) {
	defer setStop(stopCh)
	tick := time.NewTicker(time.Millisecond * time.Duration(12))
	pingTick := time.NewTimer(10 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			if err := flushWsWriter(ssConn.WsWriter, wsConn); err != nil {
				log.Error("ssh sending output to webSocket err:", err)
				return
			}
		case <-pingTick.C:
			if err := wsConn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Error("ssh sending ping to webSocket err:", err)
				return
			}
		case <-stopCh:
			return
		}
	}
}

func (ssConn *SshConn) SessionWait(stopChan chan bool) {
	if err := ssConn.SshSession.Wait(); err != nil {
		log.Error("ssh session wait err:", err)
		setStop(stopChan)
	}
}

func (s *SshConn) Close() {
	if s.SshSession != nil {
		s.SshSession.Close()
	}
}

func setStop(ch chan bool) {
	ch <- true
}

func flushWsWriter(w *wsBufferWriter, wsConn *websocket.Conn) error {
	if w.buffer.Len() != 0 {
		msg, err := json.Marshal(wsMessage{
			Operation: wsMsgStdout,
			Data:      string(w.buffer.Bytes()),
		})
		if err != nil {
			log.Errorf("write parse message err: %s", err)
			return err
		}
		err = wsConn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			return err
		}
		w.buffer.Reset()
	}
	return nil
}
