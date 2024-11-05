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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	toolssh "github.com/koderover/zadig/v2/pkg/tool/ssh"
	"github.com/koderover/zadig/v2/pkg/tool/wsconn"
	"github.com/koderover/zadig/v2/pkg/util"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func ConnectSshPmExec(c *gin.Context, username, envName, productName, ip, hostId string, cols, rows int, log *zap.SugaredLogger) error {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorf("ws upgrade err:%s", err)
		return e.ErrLoginPm.AddErr(err)
	}

	defer ws.Close()
	resp, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		ID: hostId,
	})
	if err != nil {
		log.Errorf("PrivateKey.Find ip %s id %s error: %s", ip, hostId, err)
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, e.ErrGetPrivateKey.Error()))
		return e.ErrGetPrivateKey

	}
	if resp.Status != setting.PMHostStatusNormal {
		log.Errorf("host %s status %s, is not normal", ip, resp.Status)
		e.ErrLoginPm.AddDesc(fmt.Sprintf("host %s status %s,is not normal", ip, resp.Status))
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, e.ErrLoginPm.Error()))
		return e.ErrLoginPm
	}
	if resp.ScheduleWorkflow {
		log.Errorf("host %s is not enable login", ip)
		e.ErrLoginPm.AddDesc(fmt.Sprintf("host %s is not enable ssh", ip))
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, e.ErrLoginPm.Error()))
		return e.ErrLoginPm
	}
	if resp.Port == 0 {
		resp.Port = setting.PMHostDefaultPort
	}

	sDec, err := base64.StdEncoding.DecodeString(resp.PrivateKey)
	if err != nil {
		log.Errorf("base64 decode failed ip:%s, error:%s", ip, err)
		e.ErrLoginPm.AddDesc(fmt.Sprintf("base64 decode failed ip:%s, error:%s", ip, err))
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, e.ErrLoginPm.Error()))
		return e.ErrLoginPm
	}

	sshCli, err := toolssh.NewSshCli(sDec, resp.UserName, resp.IP, resp.Port)
	if err != nil {
		log.Errorf("NewSshCli err:%s", err)
		e.ErrLoginPm.AddErr(err)
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, e.ErrLoginPm.Error()))
		return e.ErrLoginPm
	}
	defer sshCli.Close()

	sshConn, err := wsconn.NewSshConn(cols, rows, sshCli)
	if err != nil {
		log.Errorf("NewSshConn err:%s", err)
		e.ErrLoginPm.AddErr(err)
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, e.ErrLoginPm.Error()))
		return e.ErrLoginPm
	}
	defer sshConn.Close()

	stopChan := make(chan bool, 3)
	go sshConn.ReadWsMessage(ws, stopChan)
	go sshConn.SendWsWriteMessage(ws, stopChan)
	go sshConn.SessionWait(stopChan)

	<-stopChan
	return nil
}

type VmServiceCommandType string

const (
	VmServiceCommandTypeStart   VmServiceCommandType = "start"
	VmServiceCommandTypeStop    VmServiceCommandType = "stop"
	VmServiceCommandTypeRestart VmServiceCommandType = "restart"
)

type ExecVmServiceCommandResponse struct {
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	IsTimeout bool   `json:"is_timeout"`
	IsSuccess bool   `json:"is_success"`
	Error     string `json:"error"`
}

func ExecVmServiceCommand(projectName, envName, serviceName, hostId string, commandType VmServiceCommandType, log *zap.SugaredLogger) (*ExecVmServiceCommandResponse, error) {
	host, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		ID: hostId,
	})
	if err != nil {
		err = fmt.Errorf("PrivateKey.Find host id %s error: %s", hostId, err)
		return nil, e.ErrVmExecCmd.AddErr(err)

	}
	if host.Status != setting.PMHostStatusNormal {
		err = fmt.Errorf("host %s status %s, is not normal", host.IP, host.Status)
		return nil, e.ErrVmExecCmd.AddErr(err)
	}
	if host.ScheduleWorkflow {
		err = fmt.Errorf("host %s is not enable ssh", host.IP)
		return nil, e.ErrVmExecCmd.AddErr(err)
	}
	if host.Port == 0 {
		host.Port = setting.PMHostDefaultPort
	}

	service, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{ServiceName: serviceName, ProductName: projectName})
	if err != nil {
		err = fmt.Errorf("Service.Find service %s error: %s", serviceName, err)
		return nil, e.ErrLoginPm.AddErr(err)
	}

	cmd := ""
	switch commandType {
	case VmServiceCommandTypeStart:
		cmd = service.StartCmd
	case VmServiceCommandTypeStop:
		cmd = service.StopCmd
	case VmServiceCommandTypeRestart:
		cmd = service.RestartCmd
	default:
		err = fmt.Errorf("unknown command type %s", commandType)
		log.Error(err)
		return nil, e.ErrVmExecCmd.AddErr(err)
	}
	if cmd == "" {
		err = fmt.Errorf("service %s command %s is empty", serviceName, commandType)
		log.Error(err)
		return nil, e.ErrVmExecCmd.AddErr(err)
	}

	sDec, err := base64.StdEncoding.DecodeString(host.PrivateKey)
	if err != nil {
		err = fmt.Errorf("base64 decode failed ip:%s, error:%s", host.IP, err)
		log.Error(err)
		return nil, e.ErrVmExecCmd.AddErr(err)
	}

	sshCli, err := toolssh.NewSshCli(sDec, host.UserName, host.IP, host.Port)
	if err != nil {
		err = fmt.Errorf("NewSshCli err:%s", err)
		log.Error(err)
		return nil, e.ErrVmExecCmd.AddErr(err)
	}
	defer sshCli.Close()

	session, err := sshCli.NewSession()
	if err != nil {
		err = fmt.Errorf("NewSession err:%s", err)
		log.Error(err)
		return nil, e.ErrVmExecCmd.AddErr(err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Start(cmd)
	if err != nil {
		return sshErrorHandle(err, stdout, stderr, log, commandType, cmd, host.IP, envName, serviceName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	waitChan := make(chan error, 1)
	util.Go(func() {
		waitChan <- session.Wait()
	})

	select {
	case <-ctx.Done():
		// 超时
		log.Warnf("[TIMEOUT] exec %s command `%s` on %s timed out for 3s", commandType, cmd, host.IP)
		resp := &ExecVmServiceCommandResponse{
			Stdout:    stdout.String(),
			Stderr:    stderr.String(),
			IsTimeout: true,
			IsSuccess: true,
		}
		return resp, nil
	case err = <-waitChan:
		// 命令完成
		if err != nil {
			return sshErrorHandle(err, stdout, stderr, log, commandType, cmd, host.IP, envName, serviceName)
		}
	}

	if stdout.Len() > 0 {
		log.Infof("[STDOUT] exec %s command `%s` on %s: %s", commandType, cmd, host.IP, stdout.String())
	}
	if stderr.Len() > 0 {
		log.Errorf("[STDERR] exec %s command `%s` on %s: %s", commandType, cmd, host.IP, stderr.String())
	}

	resp := &ExecVmServiceCommandResponse{
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		IsSuccess: true,
	}
	return resp, nil
}

func sshErrorHandle(err error, stdout, stderr bytes.Buffer, log *zap.SugaredLogger, commandType VmServiceCommandType, cmd, ip, envName, serviceName string) (*ExecVmServiceCommandResponse, error) {
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			if stderr.Len() > 0 {
				log.Errorf("[STDERR] exec %s command `%s` on %s: %s", commandType, cmd, ip, stderr.String())
			}
			err = fmt.Errorf("%s command `%s` exited with non-zero status %v on %s for vm service %s/%s, error: %v", commandType, cmd, exitErr.ExitStatus(), ip, envName, serviceName, exitErr)
			log.Error(err)

			resp := &ExecVmServiceCommandResponse{
				Stdout: stdout.String(),
				Stderr: stderr.String(),
				Error:  err.Error(),
			}
			return resp, nil
		} else {
			err = fmt.Errorf("failed to run %s cmd on %s for vm service %s/%s, error: %v", commandType, ip, envName, serviceName, err)
			log.Error(err)
			return nil, e.ErrVmExecCmd.AddErr(err)
		}
	}
	return nil, nil
}
