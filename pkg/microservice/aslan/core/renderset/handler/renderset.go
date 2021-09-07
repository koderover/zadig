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
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/renderset/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types/permission"
)

func GetServiceRenderset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("ListServiceRenderset c.GetRawData() err : %v", err)
	}

	args := new(service.RendersetValuesArgs)
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("ListServiceRenderset json.Unmarshal err : %v", err)
	}

	args.EnvName = c.Query("envName")

	if args.EnvName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can not be null!")
		return
	}

	//retList, err := service.ListChartValues(c.Param("productName"), c.Param("serviceName"),  args, ctx.Logger)
	ctx.Resp, ctx.Err = service.ListChartValues(c.Param("productName"), c.Param("serviceName"),  args, ctx.Logger)

	//if err != nil {
	//	ctx.Resp, ctx.Err = nil, err
	//	return
	//}
	//if len(retList) == 1 {
	//	ctx.Resp, ctx.Err = retList[0], nil
	//} else {
	//	ctx.Resp, ctx.Err = nil, nil
	//}
}

func CreateOrUpdateRenderset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateOrUpdateRenderset c.GetRawData() err : %v", err)
	}

	args := new(service.RendersetValuesArgs)
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateOrUpdateRenderset json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
	}
	internalhandler.InsertOperationLog(c, ctx.Username, c.Param("productName"), "新增", "环境变量", args.EnvName, fmt.Sprintf("%s,%s", permission.TestEnvManageUUID, permission.ProdEnvManageUUID), string(data), ctx.Logger)
	args.EnvName = c.Query("envName")

	if args.EnvName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can not be null!")
		return
	}
	args.UpdateBy = ctx.Username
	args.RequestID = ctx.RequestID

	ctx.Err = service.CreateOrUpdateChartValues(c.Param("productName"), c.Param("serviceName"),  args, ctx.Logger)
}
