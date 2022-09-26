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
	"fmt"
	"io"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func ListConfigMaps(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.ListConfigMapArgs{
		EnvName:     c.Param("envName"),
		ProductName: c.Query("projectName"),
		ServiceName: c.Query("serviceName"),
	}

	ctx.Resp, ctx.Err = service.ListConfigMaps(args, ctx.Logger)
}

func RollBackConfigMap(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.RollBackConfigMapArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("RollBackConfigMap c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("RollBackConfigMap json.Unmarshal err : %v", err)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, args.ProductName, setting.OperationSceneEnv, "回滚", "环境-服务-configMap", fmt.Sprintf("环境名称:%s,服务名称:%s", args.EnvName, args.ServiceName), string(data), ctx.Logger, args.EnvName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if args.SrcConfigName == args.DestinConfigName {
		ctx.Err = e.ErrRollBackConfigMap.AddDesc("same source and destination configmap name.")
		return
	}

	ctx.Err = service.RollBackConfigMap(args.EnvName, args, ctx.UserName, ctx.UserID, ctx.Logger)
}

func MigrateHistoryConfigMaps(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Query("projectName")

	ctx.Resp, ctx.Err = service.MigrateHistoryConfigMaps(envName, productName, ctx.Logger)
}
