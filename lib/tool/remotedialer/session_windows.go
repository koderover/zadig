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
	"context"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func (s *Session) startPingsWhileWindows(rootCtx context.Context) {
	ctx, cancel := context.WithCancel(rootCtx)
	s.pingCancel = cancel
	s.pingWait.Add(1)

	go func() {
		defer s.pingWait.Done()

		t := time.NewTicker(PingWriteInterval)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.conn.Lock()
				if err := s.conn.conn.WriteControl(websocket.PingMessage, []byte(""), time.Now().Add(time.Second)); err != nil {
					logrus.WithError(err).Error("Error writing ping")
				}
				logrus.Debug("Wrote ping")
				s.conn.Unlock()
			}
		}
	}()
}

func (s *Session) ServeWhileWindows(ctx context.Context) (int, error) {
	if s.client {
		s.startPingsWhileWindows(ctx)
	}

	for {
		msType, reader, err := s.conn.NextReader()
		if err != nil {
			return 400, err
		}

		if msType != websocket.BinaryMessage {
			return 400, errWrongMessageType
		}

		if err := s.serveMessage(reader); err != nil {
			return 500, err
		}
	}
}
