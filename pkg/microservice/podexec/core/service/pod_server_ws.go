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
	"fmt"

	"github.com/gin-gonic/gin"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func ServeWs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	namespace := c.Param("namespace")
	podName := c.Param("podName")
	containerName := c.Param("containerName")
	clusterID := c.Query("clusterId")

	if namespace == "" || podName == "" || containerName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("namespace,podName,containerName can't be empty,please check!")
		return
	}
	log.Infof("exec containerName: %s, pod: %s, namespace: %s", containerName, podName, namespace)

	pty, err := NewTerminalSession(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorf("get pty failed: %v", err)
		ctx.Err = e.ErrInternalError.AddDesc(fmt.Sprintf("get pty failed: %v", err))
		return
	}
	defer func() {
		log.Info("close session.")
		_ = pty.Close()
	}()

	kubeCli, cfg, err := NewKubeOutClusterClient(clusterID)
	if err != nil {
		msg := fmt.Sprintf("get kubecli err :%v", err)
		log.Errorf(msg)
		_, _ = pty.Write([]byte(msg))
		pty.Done()

		ctx.Err = e.ErrInternalError.AddDesc(fmt.Sprintf("get kubecli err :%v", err))
		return
	}

	ok, err := ValidatePod(kubeCli, namespace, podName, containerName)
	if !ok {
		msg := fmt.Sprintf("Validate pod error! err: %v", err)
		log.Errorf(msg)
		_, _ = pty.Write([]byte(msg))
		pty.Done()

		ctx.Err = e.ErrInternalError.AddDesc(fmt.Sprintf("Validate pod error! err: %v", err))
		return
	}

	err = ExecPod(kubeCli, cfg, []string{"/bin/sh"}, pty, namespace, podName, containerName)
	if err != nil {
		msg := fmt.Sprintf("Exec to pod error! err: %v", err)
		log.Errorf(msg)
		_, _ = pty.Write([]byte(msg))
		pty.Done()

		ctx.Err = e.ErrInternalError.AddDesc(fmt.Sprintf("Exec to pod error! err: %v", err))
		return
	}
}
