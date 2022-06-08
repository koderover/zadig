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

package zgctl

import (
	"errors"

	"github.com/dgraph-io/badger/v3"
	"github.com/gin-gonic/gin"

	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/types"
)

type HandlerConfig struct {
	ZadigHost  string
	ZadigToken string
	HomeDir    string
	DB         *badger.DB
}

type handlers struct {
	zgctl ZgCtler
}

type KubeconfigInfo struct {
	KubeConfig KubeConfigData `json:"kubeconfig"`
}

type KubeConfigData struct {
	Path string `json:"path"`
}

func NewHandlers(cfg *HandlerConfig) ZgCtlHandler {
	zgctl := NewZgCtl(&ZgCtlConfig{
		ZadigHost:  cfg.ZadigHost,
		ZadigToken: cfg.ZadigToken,
		HomeDir:    cfg.HomeDir,
		DB:         cfg.DB,
	})

	return &handlers{
		zgctl: zgctl,
	}
}

func (h *handlers) StartDevMode(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("envName")
	serviceName := c.Param("serviceName")
	projectName := c.Query("projectName")
	dataDir := c.Query("dataDir")

	var startInfo types.StartDevmodeInfo
	if err := c.BindJSON(&startInfo); err != nil {
		ctx.Err = errors.New("invalid request body")
		return
	}

	ctx.Err = h.zgctl.StartDevMode(c, projectName, envName, serviceName, dataDir, startInfo.DevImage)
}

func (h *handlers) StopDevMode(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("envName")
	serviceName := c.Param("serviceName")
	projectName := c.Query("projectName")

	ctx.Err = h.zgctl.StopDevMode(c, projectName, envName, serviceName)
}

func (h *handlers) DevImages(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp = h.zgctl.DevImages()
}

func (h *handlers) ConfigKubeconfig(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("envName")
	projectName := c.Query("projectName")

	var kubeconfig KubeconfigInfo
	if err := c.BindJSON(&kubeconfig); err != nil {
		ctx.Err = errors.New("invalid request body")
		return
	}

	ctx.Err = h.zgctl.ConfigKubeconfig(projectName, envName, kubeconfig.KubeConfig.Path)
}
