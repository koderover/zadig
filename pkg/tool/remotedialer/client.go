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
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

// ConnectAuthorizer custom for authorization
type ConnectAuthorizer func(proto, address string) bool

// ClientConnect connect to WS and wait 5 seconds when error
func ClientConnect(ctx context.Context, wsURL string, headers http.Header, dialer *websocket.Dialer,
	auth ConnectAuthorizer, onConnect func(context.Context, *Session) error) error {
	if err := ConnectToProxy(ctx, wsURL, headers, auth, dialer, onConnect); err != nil {
		log.Errorf("Remotedialer proxy error: %s", err)
		time.Sleep(time.Duration(5) * time.Second)
		return err
	}
	return nil
}

// ConnectToProxy connect to websocket server
func ConnectToProxy(rootCtx context.Context, proxyURL string, headers http.Header, auth ConnectAuthorizer, dialer *websocket.Dialer, onConnect func(context.Context, *Session) error) error {
	// log.Infof("Connecting to proxy: %s", proxyURL)

	if dialer == nil {
		dialer = &websocket.Dialer{Proxy: http.ProxyFromEnvironment, HandshakeTimeout: HandshakeTimeOut}
	}
	ws, resp, err := dialer.DialContext(rootCtx, proxyURL, headers)
	if err != nil {
		if resp == nil {
			log.Errorf("Failed to connect to proxy. Empty dialer response, error: %s", err)
		} else {
			rb, err2 := ioutil.ReadAll(resp.Body)
			if err2 != nil {
				log.Errorf("Failed to connect to proxy. Response status: %d - %s. Couldn't read response body (err: %s)", resp.StatusCode, resp.Status, err2)
			} else {
				log.Errorf("Failed to connect to proxy. Response status: %d - %s. Response body: %s", resp.StatusCode, resp.Status, rb)
			}
		}
		return err
	}
	defer ws.Close()

	result := make(chan error, 2)

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	session := NewClientSession(auth, ws)
	defer session.Close()

	if onConnect != nil {
		go func() {
			if err := onConnect(ctx, session); err != nil {
				result <- err
			}
		}()
	}

	go func() {
		_, err = session.Serve(ctx)
		result <- err
	}()

	log.Infof("Connected to proxy %s", proxyURL)

	select {
	case <-ctx.Done():
		log.Infof("Proxy Done. URL: %s, err: %s", proxyURL, ctx.Err())
		return nil
	case err := <-result:
		return err
	}
}
