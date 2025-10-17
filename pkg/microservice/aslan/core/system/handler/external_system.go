/*
Copyright 2021 The KodeRover Authors.

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
	"errors"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func CreateExternalSystem(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.ExternalSystemDetail)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateExternalSystem GetRawData err : %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateExternalSystem Unmarshal err : %s", err)
	}

	detail := fmt.Sprintf("name:%s server:%s", args.Name, args.Server)
	detailEn := fmt.Sprintf("Name: %s, Server: %s", args.Name, args.Server)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "系统配置-外部系统", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if args.Name == "" || args.Server == "" {
		ctx.RespErr = errors.New("name and server must be provided")
		return
	}

	ctx.RespErr = service.CreateExternalSystem(args, ctx.Logger)
}

type listQuery struct {
	PageSize int64 `json:"page_size" form:"page_size,default=100"`
	PageNum  int64 `json:"page_num"  form:"page_num,default=1"`
}

type listExternalResp struct {
	SystemList []*service.ExternalSystemDetail `json:"external_system"`
	Total      int64                           `json:"total"`
}

func ListExternalSystem(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// Query Verification
	args := &listQuery{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = err
		return
	}

	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.RespErr = e.ErrInvalidParam
		return
	}
	systemList, length, err := service.ListExternalSystem(encryptedKey, args.PageNum, args.PageSize, ctx.Logger)
	if err == nil {
		ctx.Resp = &listExternalResp{
			SystemList: systemList,
			Total:      length,
		}
		return
	}
	ctx.RespErr = err
}

func GetExternalSystemDetail(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.GetExternalSystemDetail(c.Param("id"), ctx.Logger)
}

func UpdateExternalSystem(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(service.ExternalSystemDetail)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateExternalSystem GetRawData err : %s", err)
	}
	if err = json.Unmarshal(data, req); err != nil {
		log.Errorf("UpdateExternalSystem Unmarshal err : %s", err)
	}

	detail := fmt.Sprintf("name:%s server:%s", req.Name, req.Server)
	detailEn := fmt.Sprintf("Name: %s, Server: %s", req.Name, req.Server)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "系统配置-外部系统", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if req.Name == "" || req.Server == "" {
		ctx.RespErr = errors.New("name and server must be provided")
		return
	}

	ctx.RespErr = service.UpdateExternalSystem(c.Param("id"), req, ctx.Logger)
}

func DeleteExternalSystem(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	detail := fmt.Sprintf("id:%s", c.Param("id"))
	detailEn := fmt.Sprintf("ID: %s", c.Param("id"))
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "系统配置-外部系统", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.RespErr = service.DeleteExternalSystem(c.Param("id"), ctx.Logger)
}
