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
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/lib/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/types/permission"
)

func GetService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	envName := c.Query("envName")
	productName := c.Param("productName")
	serviceName := c.Param("serviceName")

	ctx.Resp, ctx.Err = service.GetService(envName, productName, serviceName, ctx.Logger)
}

func RestartService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := &service.SvcOptArgs{
		EnvName:     c.Query("envName"),
		ProductName: c.Param("productName"),
		ServiceName: c.Param("serviceName"),
	}

	internalhandler.InsertOperationLog(c, ctx.Username, c.Param("productName"), "重启", "集成环境-服务", fmt.Sprintf("环境名称:%s,服务名称:%s", c.Query("envName"), c.Param("serviceName")), fmt.Sprintf("%s,%s", permission.TestEnvManageUUID, permission.ProdEnvManageUUID), "", ctx.Logger)
	ctx.Err = service.RestartService(args.EnvName, args, ctx.Logger)
}

func UpdateService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.Username, c.Param("productName"), "更新", "集成环境-单服务", fmt.Sprintf("环境名称:%s,服务名称:%s", c.Query("envName"), c.Param("serviceName")), fmt.Sprintf("%s,%s", permission.TestEnvManageUUID, permission.ProdEnvManageUUID), "", ctx.Logger)

	svcRev := new(service.SvcRevision)
	if err := c.BindJSON(svcRev); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	args := &service.SvcOptArgs{
		EnvName:     c.Query("envName"),
		ProductName: c.Param("productName"),
		ServiceName: c.Param("serviceName"),
		ServiceType: c.Param("serviceType"),
		ServiceRev:  svcRev,
		UpdateBy:    ctx.Username,
	}

	ctx.Err = service.UpdateService(args.EnvName, args, ctx.Logger)
}

func RestartNewService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := &service.RestartScaleArgs{
		EnvName:     c.Query("envName"),
		ProductName: c.Param("productName"),
		ServiceName: c.Param("serviceName"),
		Type:        c.Query("type"),
		Name:        c.Query("name"),
	}

	internalhandler.InsertOperationLog(
		c, ctx.Username,
		c.Param("productName"),
		"重启",
		"集成环境-服务",
		fmt.Sprintf(
			"环境名称:%s,服务名称:%s,%s:%s", args.EnvName, args.ServiceName, args.Type, args.Name,
		),
		fmt.Sprintf("%s,%s", permission.TestEnvManageUUID, permission.ProdEnvManageUUID),
		"", ctx.Logger,
	)

	ctx.Err = service.RestartScale(args, ctx.Logger)
	return
}

func ScaleNewService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(service.ScaleArgs)
	args.Type = setting.Deployment

	productName := c.Param("productName")
	serviceName := c.Param("serviceName")
	envName := c.Query("envName")
	resourceType := c.Query("type")
	name := c.Query("name")

	internalhandler.InsertOperationLog(
		c, ctx.Username,
		c.Param("productName"),
		"伸缩",
		"集成环境-服务",
		fmt.Sprintf("环境名称:%s,%s:%s", args.EnvName, args.Type, args.Name),
		fmt.Sprintf("%s,%s", permission.TestEnvManageUUID, permission.ProdEnvManageUUID), "", ctx.Logger)

	number, err := strconv.Atoi(c.Param("number"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid number format")
		return
	}

	ctx.Err = service.Scale(&service.ScaleArgs{
		Type:        resourceType,
		ProductName: productName,
		EnvName:     envName,
		ServiceName: serviceName,
		Name:        name,
		Number:      number,
	}, ctx.Logger)
}

func ScaleService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.Username, c.Param("productName"), "伸缩", "集成环境-服务", fmt.Sprintf("环境名称:%s,服务名称:%s", c.Query("envName"), c.Param("serviceName")), fmt.Sprintf("%s,%s", permission.TestEnvManageUUID, permission.ProdEnvManageUUID), "", ctx.Logger)

	number, err := strconv.Atoi(c.Param("number"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid number format")
		return
	}

	productName := c.Param("productName")
	serviceName := c.Param("serviceName")
	envName := c.Query("envName")

	ctx.Err = service.ScaleService(
		envName,
		productName,
		serviceName,
		number,
		ctx.Logger,
	)
}

func GetServiceContainer(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	envName := c.Query("envName")
	//namespace := c.Param("namespace")
	productName := c.Param("productName")
	serviceName := c.Param("serviceName")
	container := c.Param("container")

	ctx.Err = service.GetServiceContainer(envName, productName, serviceName, container, ctx.Logger)
}
