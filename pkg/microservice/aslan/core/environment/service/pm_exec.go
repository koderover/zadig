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

package service

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	toolssh "github.com/koderover/zadig/pkg/tool/ssh"
	"github.com/koderover/zadig/pkg/tool/wsconn"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func ConnectSshPmExec(c *gin.Context, username, envName, productName, ip, hostId string, cols, rows int, log *zap.SugaredLogger) error {
	resp, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		ID: hostId,
	})
	if err != nil {
		log.Errorf("PrivateKey.Find ip %s id %s error: %s", ip, hostId, err)
		return e.ErrGetPrivateKey
	}
	if resp.Status != setting.PMHostStatusNormal {
		return e.ErrLoginPm.AddDesc(fmt.Sprintf("host %s status %s,is not normal", ip, resp.Status))
	}
	if resp.Port == 0 {
		resp.Port = setting.PMHostDefaultPort
	}

	sDec, err := base64.StdEncoding.DecodeString(resp.PrivateKey)
	if err != nil {
		log.Errorf("base64 decode failed ip:%s, error:%s", ip, err)
		return e.ErrLoginPm.AddDesc(fmt.Sprintf("base64 decode failed ip:%s, error:%s", ip, err))
	}

	sshCli, err := toolssh.NewSshCli(sDec, resp.UserName, resp.IP, resp.Port)
	if err != nil {
		log.Errorf("NewSshCli err:%s", err)
		return e.ErrLoginPm.AddErr(err)
	}

	sshConn, err := wsconn.NewSshConn(cols, rows, sshCli)
	if err != nil {
		log.Errorf("NewSshConn err:%s", err)
		return e.ErrLoginPm.AddErr(err)
	}
	defer sshConn.Close()

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorf("ws upgrade err:%s", err)
		return e.ErrLoginPm.AddErr(err)
	}
	defer ws.Close()

	stopChan := make(chan bool, 3)
	go sshConn.ReadWsMessage(ws, stopChan)
	go sshConn.SendWsWriteMessage(ws, stopChan)
	go sshConn.SessionWait(stopChan)

	<-stopChan
	return nil
}
