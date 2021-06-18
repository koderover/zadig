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
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type ConnectAuthorizer func(proto, address string) bool

func ClientConnect(wsURL string, headers http.Header, dialer *websocket.Dialer, auth ConnectAuthorizer, onConnect func(context.Context) error) {
	if err := connectToProxy(wsURL, headers, auth, dialer, onConnect); err != nil {
		logrus.WithError(err).Error("Failed to connect to proxy")
		time.Sleep(time.Duration(5) * time.Second)
	}
}

func connectToProxy(proxyURL string, headers http.Header, auth ConnectAuthorizer, dialer *websocket.Dialer, onConnect func(context.Context) error) error {
	logrus.WithField("url", proxyURL).Info("Connecting to proxy")

	if dialer == nil {
		dialer = &websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	}
	ws, _, err := dialer.Dial(proxyURL, headers)
	if err != nil {
		logrus.WithError(err).Error("Failed to connect to proxy")
		return err
	}
	defer ws.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if onConnect != nil {
		if err := onConnect(ctx); err != nil {
			return err
		}
	}

	session := NewClientSession(auth, ws)
	_, err = session.Serve()
	session.Close()
	return err
}
