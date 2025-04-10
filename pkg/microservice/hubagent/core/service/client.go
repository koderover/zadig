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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/hubagent/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/remotedialer"
)

const (
	defaultTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultCaPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

type clientConfig struct {
	Token       string
	Server      string
	TokenPath   string
	CaPath      string
	ServiceHost string
	ServicePort string
}

type Client struct {
	clientConfig
	logger *zap.SugaredLogger
}

func newClient(config clientConfig) *Client {
	return &Client{
		clientConfig: config,
		logger:       log.SugaredLogger(),
	}
}

type input struct {
	Cluster *ClusterInfo `json:"cluster"`
}

type ClusterInfo struct {
	ClusterID string    `json:"_"`
	Joined    time.Time `json:"_"`

	Address string `json:"address"`
	Token   string `json:"token"`
	CACert  string `json:"caCert"`
}

func (c *Client) getParams() (*input, error) {
	caData, err := ioutil.ReadFile(c.CaPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", c.CaPath)
	}

	token, err := ioutil.ReadFile(c.TokenPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", c.TokenPath)
	}

	return &input{
		Cluster: &ClusterInfo{
			Address: fmt.Sprintf("https://%s:%s", c.ServiceHost, c.ServicePort),
			Token:   strings.TrimSpace(string(token)),
			CACert:  base64.StdEncoding.EncodeToString(caData),
		},
	}, nil
}

func Init(ctx context.Context) error {
	token := config.HubAgentToken()
	if token == "" {
		return fmt.Errorf("token must be configured")
	}

	server := config.HubServerBaseAddr()
	if server == "" {
		return fmt.Errorf("server must be configured")
	}

	serviceHost := config.KubernetesServiceHost()
	if serviceHost == "" {
		return fmt.Errorf("kube service host must be configured")
	}

	servicePort := config.KubernetesServicePort()
	if servicePort == "" {
		return fmt.Errorf("kube service port must be configured")
	}

	app := newClient(
		clientConfig{
			Server:      server,
			Token:       token,
			TokenPath:   defaultTokenPath,
			CaPath:      defaultCaPath,
			ServiceHost: serviceHost,
			ServicePort: servicePort,
		},
	)

	return app.Start(ctx)
}

func (c *Client) Start(ctx context.Context) error {
	params, err := c.getParams()
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(params)
	if err != nil {
		return err
	}

	headers := map[string][]string{
		setting.Token:  {c.Token},
		setting.Params: {base64.StdEncoding.EncodeToString(bytes)},
	}

	connectURL := fmt.Sprintf("%s/connect", c.Server)
	c.logger.Infof("Connect to %s with token %s", connectURL, c.Token)

	bo := backoff.NewExponentialBackOff()

	bo.InitialInterval = 1 * time.Second
	// never stops
	bo.MaxElapsedTime = 0
	retries := 0
	timeout := 10 * time.Second

	errChan := make(chan error, 1)
	go func() {
		errChan <- backoff.Retry(func() error {
			if retries > 0 {
				c.logger.Infof("Retrying to connect to %s", connectURL)
			}

			tm := time.Now()

			remotedialer.ClientConnect(
				context.Background(),
				connectURL,
				headers,
				&websocket.Dialer{
					HandshakeTimeout: timeout,
					Proxy:            http.ProxyFromEnvironment,
				},
				func(proto, address string) bool {
					switch proto {
					case "tcp":
						return true
					case "unix":
						return address == "/var/run/docker.sock"
					}
					return false
				}, nil)

			// 如果连接时间超过2倍的超时时间, 则认为已经成功连接，需要立即重连
			if time.Since(tm) > 2*timeout {
				c.logger.Errorf("Connection timeout")
				bo.Reset()
			}

			retries++
			return errors.New("retry")
		}, bo)
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errChan:
		return err
	}
}
