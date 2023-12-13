/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/dingtalk"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func GetDingTalkDepartment(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	deptID := c.Param("department_id")
	if deptID == "root" {
		deptID = "1"
	}
	ctx.Resp, ctx.Err = dingtalk.GetDingTalkDepartment(c.Param("id"), deptID)
}

func GetDingTalkUserID(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	userID, err := dingtalk.GetDingTalkUserIDByMobile(c.Param("id"), c.Query("mobile"))
	ctx.Resp, ctx.Err = map[string]string{"user_id": userID}, err
}

func DingTalkEventHandler(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	log.Infof("DingTalkEventHandler: New request url %s", c.Request.RequestURI)
	body, err := c.GetRawData()
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = dingtalk.EventHandler(c.Param("ak"), body,
		c.Query("signature"), c.Query("timestamp"), c.Query("nonce"))
}
