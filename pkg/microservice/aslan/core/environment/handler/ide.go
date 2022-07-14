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

package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/types"
)

func PatchWorkload(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	serviceName := c.Param("serviceName")
	projectName := c.Query("projectName")

	var staretInfo types.StartDevmodeInfo
	if err := c.BindJSON(&staretInfo); err != nil {
		ctx.Err = errors.New("invalid request body")
		return
	}

	ctx.Resp, ctx.Err = service.PatchWorkload(c, projectName, envName, serviceName, staretInfo.DevImage)
}

func RecoverWorkload(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	serviceName := c.Param("serviceName")
	projectName := c.Query("projectName")

	ctx.Err = service.RecoverWorkload(c, projectName, envName, serviceName)
}
