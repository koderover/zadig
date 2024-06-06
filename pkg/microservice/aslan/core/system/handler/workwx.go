/*
 * Copyright 2024 The KodeRover Authors.
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
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/workwx"
)

type getWorkWXDepartmentReq struct {
	DepartmentID int `json:"department_id" form:"department_id"`
}

func GetWorkWxDepartment(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(getWorkWXDepartmentReq)
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	appID := c.Param("id")

	ctx.Resp, ctx.Err = service.GetWorkWxDepartment(appID, req.DepartmentID, ctx.Logger)
}

type getWorkWXUsersReq struct {
	DepartmentID int `json:"department_id" form:"department_id"`
}

func GetWorkWxUsers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(getWorkWXUsersReq)
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	appID := c.Param("id")

	ctx.Resp, ctx.Err = service.GetWorkWxUsers(appID, req.DepartmentID, ctx.Logger)
}

type validateWorkWXCallbackReq struct {
	EchoString   string `json:"echostr"       form:"echostr"`
	MsgSignature string `json:"msg_signature" form:"msg_signature"`
	Nonce        string `json:"nonce"         form:"nonce"`
	Timestamp    string `json:"timestamp"     form:"timestamp"`
}

func ValidateWorkWXCallback(c *gin.Context) {
	query := new(validateWorkWXCallbackReq)

	err := c.ShouldBindQuery(query)
	if err != nil {
		c.Set(setting.ResponseError, err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	signatureOK := workwx.CallbackValidate(query.MsgSignature, "g2azdvjwblQKTneS0R7", query.Timestamp, query.Nonce, query.EchoString)
	if !signatureOK {
		c.Set(setting.ResponseError, fmt.Errorf("invalid signarture vs content"))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	plaintext, receiveID, err := workwx.DecodeEncryptedMessage("W9shnP9pST6FEOT6u7q8jEAKprKV5glEWF4qPgND5Aa", query.EchoString)
	if err != nil {
		fmt.Println("nooooo, err:", err)
		c.Set(setting.ResponseError, err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	fmt.Println("plain text:", string(plaintext))
	fmt.Println("receive id:", string(receiveID))
	c.XML(http.StatusOK, plaintext)
}
