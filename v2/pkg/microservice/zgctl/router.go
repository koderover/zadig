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
	"github.com/dgraph-io/badger/v3"
	"github.com/gin-gonic/gin"
)

type RouterConfig struct {
	ZadigHost  string
	ZadigToken string
	HomeDir    string
	DB         *badger.DB
}

type Router struct {
	handlers ZgCtlHandler
}

func NewRouter(cfg *RouterConfig) *Router {
	handlers := NewHandlers(&HandlerConfig{
		ZadigHost:  cfg.ZadigHost,
		ZadigToken: cfg.ZadigToken,
		HomeDir:    cfg.HomeDir,
		DB:         cfg.DB,
	})

	return &Router{
		handlers: handlers,
	}
}

func (r *Router) Inject(router *gin.RouterGroup) {
	router.POST("/devmode/:envName/:serviceName", r.handlers.StartDevMode)
	router.DELETE("/devmode/:envName/:serviceName", r.handlers.StopDevMode)
	router.POST("/devmode/:envName/kubeconfig", r.handlers.ConfigKubeconfig)

	router.GET("/devmode/images", r.handlers.DevImages)
}
