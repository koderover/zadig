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
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ConnectSshPmExec(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	ip := c.Query("ip")
	hostId := c.Query("hostId")
	name := c.Param("name")
	if projectName == "" || ip == "" || name == "" || hostId == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("param projectName or ip or name or hostId is empty")
	}
	colsStr := c.DefaultQuery("cols", "135")
	cols, err := strconv.Atoi(colsStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
	}
	rowsStr := c.DefaultQuery("rows", "40")
	rows, err := strconv.Atoi(rowsStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
	}

	ctx.Err = service.ConnectSshPmExec(c, ctx.UserName, name, projectName, ip, hostId, cols, rows, ctx.Logger)
}
