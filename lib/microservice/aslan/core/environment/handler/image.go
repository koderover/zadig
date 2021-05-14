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
	"io/ioutil"

	"github.com/gin-gonic/gin"

	"github.com/qiniu/x/log.v7"

	"github.com/koderover/zadig/lib/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/types/permission"
)

func UpdateStatefulSetContainerImage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

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

	internalhandler.InsertOperationLog(
		c, ctx.Username, args.ProductName,
		"更新", "集成环境-服务镜像",
		fmt.Sprintf("环境名称:%s,服务名称:%s,StatefulSet:%s", args.EnvName, args.ServiceName, args.Name),
		fmt.Sprintf("%s,%s", permission.TestEnvManageUUID, permission.ProdEnvManageUUID),
		string(data), ctx.Logger)

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = service.UpdateContainerImage(args, ctx.Logger)
}

func UpdateDeploymentContainerImage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(service.UpdateContainerImageArgs)
	args.Type = setting.Deployment

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateDeploymentContainerImage c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateDeploymentContainerImage json.Unmarshal err : %v", err)
	}

	internalhandler.InsertOperationLog(
		c, ctx.Username, args.ProductName,
		"更新", "集成环境-服务镜像",
		fmt.Sprintf("环境名称:%s,服务名称:%s,Deployment:%s", args.EnvName, args.ServiceName, args.Name),
		fmt.Sprintf("%s,%s", permission.TestEnvManageUUID, permission.ProdEnvManageUUID),
		string(data), ctx.Logger)

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = service.UpdateContainerImage(args, ctx.Logger)
}

//func UpdateContainerImage(c *gin.Context) {
//	ctx := internalhandler.NewContext(c)
//	defer func() { internalhandler.JsonResponse(c, ctx) }()
//
//	args := new(service.UpdateContainerImageArgs)
//	data, err := c.GetRawData()
//	if err != nil {
//		log.Errorf("UpdateContainerImage c.GetRawData() err : %v", err)
//	}
//	if err = json.Unmarshal(data, args); err != nil {
//		log.Errorf("UpdateContainerImage json.Unmarshal err : %v", err)
//	}
//	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductName, "更新", "集成环境-服务镜像", fmt.Sprintf("环境名称:%s,服务名称:%s", args.EnvName, args.ServiceName), fmt.Sprintf("%s,%s", permission.TestEnvManageUUID, permission.ProdEnvManageUUID), string(data), ctx.Logger)
//	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))
//
//	if err := c.BindJSON(args); err != nil {
//		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
//		return
//	}
//
//	ctx.Err = service.CtrlMgr.UpdateContainerImage(args.EnvName, args.ProductName, args, ctx.Logger)
//}
