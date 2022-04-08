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

package cache

import (
	"fmt"

	"github.com/gin-gonic/gin"

	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

type handlers struct {
	cache Cacher
}

func NewHandlers() *handlers {
	return &handlers{
		cache: NewCache(),
	}
}

func (h *handlers) Get(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	data, _ := h.cache.Get(c.Param("key"))
	if data == nil {
		data = ""
	}

	c.Header(HeaderXData, fmt.Sprint(data))
	ctx.Resp, ctx.Err = data, nil
}

func (h *handlers) Set(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	h.cache.Set(c.Param("key"), c.Param("value"))

	ctx.Resp, ctx.Err = MsgSuccess, nil
}

func (h *handlers) Delete(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	h.cache.Delete(c.Param("key"))

	ctx.Resp, ctx.Err = MsgSuccess, nil
}

func (h *handlers) List(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	cacheData := h.cache.List()

	ctx.Resp, ctx.Err = cacheData, nil
}

func (h *handlers) Purge(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	h.cache.Purge()

	ctx.Resp, ctx.Err = MsgSuccess, nil
}
