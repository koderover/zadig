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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func DeleteCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("envName")
	productName := c.Query("projectName")
	commonEnvCfgType := c.Query("commonEnvCfgType")
	objectName := c.Param("objectName")
	if envName == "" || productName == "" || objectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("param envName or projectName or objectName is invalid")
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, productName, setting.OperationSceneEnv, "删除", "环境配置", fmt.Sprintf("%s:%s:%s", envName, commonEnvCfgType, objectName), "", ctx.Logger, envName)

	ctx.Err = service.DeleteCommonEnvCfg(envName, productName, objectName, config.CommonEnvCfgType(commonEnvCfgType), ctx.Logger)
}

func CreateCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(models.CreateUpdateCommonEnvCfgArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateCommonEnvCfg c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateCommonEnvCfg json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv, "新建", "环境配置", fmt.Sprintf("%s:%s", args.EnvName, args.CommonEnvCfgType), string(data), ctx.Logger, c.Param("envName"))
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if args.YamlData == "" {
		ctx.Err = e.ErrInvalidParam
		return
	}
	args.EnvName = c.Param("envName")
	args.ProductName = c.Query("projectName")
	ctx.Err = service.CreateCommonEnvCfg(args, ctx.UserName, ctx.Logger)
}

func UpdateCommonEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(models.CreateUpdateCommonEnvCfgArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateCommonEnvCfg c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateCommonEnvCfg json.Unmarshal err : %v", err)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv, "更新", "环境配置", fmt.Sprintf("%s:%s:%s", args.EnvName, args.CommonEnvCfgType, args.Name), string(data), ctx.Logger, c.Param("envName"))
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if len(args.YamlData) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("yaml info can't be nil")
		return
	}
	args.EnvName = c.Param("envName")
	args.ProductName = c.Query("projectName")
	isRollBack := false
	if len(c.Query("rollback")) > 0 {
		isRollBack, err = strconv.ParseBool(c.Query("rollback"))
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddErr(err)
			return
		}
	}

	ctx.Err = service.UpdateCommonEnvCfg(args, ctx.UserName, isRollBack, ctx.Logger)
}

func ListCommonEnvCfgHistory(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.ListCommonEnvCfgHistoryArgs)
	args.EnvName = c.Param("envName")
	args.ProjectName = c.Query("projectName")
	args.CommonEnvCfgType = config.CommonEnvCfgType(c.Query("commonEnvCfgType"))
	args.Name = c.Param("objectName")

	ctx.Resp, ctx.Err = service.ListEnvResourceHistory(args, ctx.Logger)
}

func ListLatestEnvCfg(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.ListCommonEnvCfgHistoryArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.Err = service.ListLatestEnvResources(args, ctx.Logger)
}

func SyncEnvResource(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.SyncEnvResourceArg{
		EnvName:     c.Param("envName"),
		ProductName: c.Query("projectName"),
		Name:        c.Param("objectName"),
		Type:        c.Param("type"),
	}
	ctx.Err = service.SyncEnvResource(args, ctx.Logger)
}
