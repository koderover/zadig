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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type OperationLog struct {
	Count int                    `json:"count"`
	Logs  []*models.OperationLog `json:"logs"`
}

func GetOperationLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		ctx.Err = e.ErrFindOperationLog.AddErr(err)
		return
	}

	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if err != nil {
		ctx.Err = e.ErrFindOperationLog.AddErr(err)
		return
	}

	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))
	if err != nil {
		ctx.Err = e.ErrFindOperationLog.AddErr(err)
		return
	}

	projectName := c.Query("projectName")
	if len(projectName) == 0 {
		ctx.Err = e.ErrFindOperationLog.AddDesc("projectName can't be nil")
		return
	}

	args := &service.OperationLogArgs{
		ExactProduct: projectName,
		Username:     c.Query("username"),
		Function:     c.Query("function"),
		Scene:        setting.OperationSceneEnv,
		TargetID:     c.Query("envName"),
		Status:       status,
		PerPage:      pageSize,
		Page:         page,
		Detail:       c.Query("detail"),
	}

	logs, count, err := service.FindOperation(args, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp = &OperationLog{
		Count: count,
		Logs:  logs,
	}
}
