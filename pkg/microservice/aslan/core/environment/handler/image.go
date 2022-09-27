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

func UpdateStatefulSetContainerImage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.UpdateContainerImageArgs)
	args.Type = setting.StatefulSet

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateStatefulSetContainerImage c.GetRawData() err : %v", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateStatefulSetContainerImage json.Unmarshal err : %v", err)
		return
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName, args.ProductName, setting.OperationSceneEnv,
		"更新", "环境-服务镜像",
		fmt.Sprintf("环境名称:%s,服务名称:%s,StatefulSet:%s", args.EnvName, args.ServiceName, args.Name),
		string(data), ctx.Logger, args.EnvName)

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = service.UpdateContainerImage(ctx.RequestID, args, ctx.Logger)
}

func UpdateDeploymentContainerImage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.UpdateContainerImageArgs)
	args.Type = setting.Deployment

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateDeploymentContainerImage c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateDeploymentContainerImage json.Unmarshal err : %v", err)
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName, args.ProductName, setting.OperationSceneEnv,
		"更新", "环境-服务镜像",
		fmt.Sprintf("环境名称:%s,服务名称:%s,Deployment:%s", args.EnvName, args.ServiceName, args.Name),
		string(data), ctx.Logger, args.EnvName)

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = service.UpdateContainerImage(ctx.RequestID, args, ctx.Logger)
}
