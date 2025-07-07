/*
Copyright 2023 The KodeRover Authors.

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
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/testing/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func ListTestingWithStat(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.TestCenter.View {
			ctx.UnAuthorized = true
			return
		}
	}

	pageSizeStr := c.Query("pageSize")
	pageNumStr := c.Query("pageNum")

	var pageSize, pageNum int

	if pageSizeStr == "" {
		pageSize = 50
	} else {
		pageSize, err = strconv.Atoi(pageSizeStr)
		if err != nil {
			ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("pageSize args err :%s", err))
			return
		}
	}

	if pageNumStr == "" {
		pageNum = 1
	} else {
		pageNum, err = strconv.Atoi(pageNumStr)
		if err != nil {
			ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("page args err :%s", err))
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.ListTesting(ctx.UserID, pageNum, pageSize, c.Query("search"), ctx.Logger)
}
