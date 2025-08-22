/*
Copyright 2025 The KodeRover Authors.

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
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func GetSystemNavigation(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.RespErr = service.GetSystemNavigation(ctx.Logger)
}

func UpdateSystemNavigation(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	// admin only
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}
	var body struct {
		Items []*commonmodels.NavigationItem `json:"items"`
	}
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateSystemNavigation c.GetRawData() err : %s", err)
	}
	if err = json.Unmarshal(data, &body); err != nil {
		log.Errorf("UpdateSystemNavigation json.Unmarshal err : %s", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "系统配置-自定义导航", "", string(data), types.RequestBodyTypeJSON, ctx.Logger)

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	if err := c.ShouldBindJSON(&body); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid navigation args")
		return
	}
	ctx.RespErr = service.UpdateSystemNavigation(ctx.UserName, body.Items, ctx.Logger)
}
