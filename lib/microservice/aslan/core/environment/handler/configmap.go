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
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/types/permission"
)

func ListConfigMaps(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := &service.ListConfigMapArgs{
		EnvName:     c.Query("envName"),
		ProductName: c.Query("productName"),
		ServiceName: c.Query("serviceName"),
	}

	ctx.Resp, ctx.Err = service.ListConfigMaps(args, ctx.Logger)
	return
}

func UpdateConfigMap(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(service.UpdateConfigMapArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateConfigMap c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateConfigMap json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductName, "更新", "集成环境-服务-configMap", fmt.Sprintf("环境名称:%s,服务名称:%s", args.EnvName, args.ServiceName), fmt.Sprintf("%s,%s", permission.TestEnvManageUUID, permission.ProdEnvManageUUID), string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if len(args.Data) == 0 {
		ctx.Err = e.ErrInvalidParam
		return
	}

	// TODO: REVERT AUTH is disabled
	//authed := service.CtrlMgr.IsProductAuthed(ctx.UserIDStr, args.ProductOwner, args.ProductName, collection.ProductWritePermission, ctx.Logger)
	//if !authed {
	//	ctx.Err = e.ErrUpdateConfigMap.AddDesc(e.ProductAccessDeniedErrMsg)
	//	return
	//}

	ctx.Err = service.UpdateConfigMap(args.EnvName, args, ctx.User.Name, ctx.User.ID, ctx.Logger)
	return
}

func RollBackConfigMap(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(service.RollBackConfigMapArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("RollBackConfigMap c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("RollBackConfigMap json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductName, "回滚", "集成环境-服务-configMap", fmt.Sprintf("环境名称:%s,服务名称:%s", args.EnvName, args.ServiceName), fmt.Sprintf("%s,%s", permission.TestEnvManageUUID, permission.ProdEnvManageUUID), string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if args.SrcConfigName == args.DestinConfigName {
		ctx.Err = e.ErrRollBackConfigMap.AddDesc("same source and destination configmap name.")
		return
	}

	ctx.Err = service.RollBackConfigMap(args.EnvName, args, ctx.User.Name, ctx.User.ID, ctx.Logger)
	return
}
