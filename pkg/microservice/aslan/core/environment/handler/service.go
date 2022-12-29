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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func GetService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	envName := c.Param("name")
	projectName := c.Query("projectName")
	serviceName := c.Param("serviceName")
	workLoadType := c.Query("workLoadType")

	ctx.Resp, ctx.Err = service.GetService(envName, projectName, serviceName, workLoadType, ctx.Logger)
}

func RestartService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.SvcOptArgs{
		EnvName:     c.Param("name"),
		ProductName: c.Query("projectName"),
		ServiceName: c.Param("serviceName"),
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv,
		"重启", "环境-服务", fmt.Sprintf("环境名称:%s,服务名称:%s", c.Param("name"), c.Param("serviceName")),
		"", ctx.Logger, args.EnvName)
	ctx.Err = service.RestartService(args.EnvName, args, ctx.Logger)
}

func UpdateService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	projectName := c.Query("projectName")
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv,
		"更新", "环境-单服务", fmt.Sprintf("环境名称:%s,服务名称:%s", envName, c.Param("serviceName")),
		"", ctx.Logger, envName)

	svcRev := new(service.SvcRevision)
	if err := c.BindJSON(svcRev); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if c.Param("serviceName") != svcRev.ServiceName {
		ctx.Err = e.ErrInvalidParam.AddDesc("serviceName not match")
		return
	}

	args := &service.SvcOptArgs{
		EnvName:           envName,
		ProductName:       projectName,
		ServiceName:       c.Param("serviceName"),
		ServiceType:       svcRev.Type,
		ServiceRev:        svcRev,
		UpdateBy:          ctx.UserName,
		UpdateServiceTmpl: svcRev.UpdateServiceTmpl,
	}

	ctx.Err = service.UpdateService(args, ctx.Logger)
}

func RestartWorkload(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.RestartScaleArgs{
		EnvName:     c.Param("name"),
		ProductName: c.Query("projectName"),
		ServiceName: c.Param("serviceName"),
		Type:        c.Query("type"),
		Name:        c.Query("name"),
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName,
		c.Query("projectName"),
		setting.OperationSceneEnv,
		"重启",
		"环境-服务",
		fmt.Sprintf(
			"环境名称:%s,服务名称:%s,%s:%s", args.EnvName, args.ServiceName, args.Type, args.Name,
		),
		"", ctx.Logger, args.EnvName,
	)

	ctx.Err = service.RestartScale(args, ctx.Logger)
}

func ScaleNewService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.ScaleArgs)
	args.Type = setting.Deployment

	projectName := c.Query("projectName")
	serviceName := c.Param("serviceName")
	envName := c.Param("name")
	resourceType := c.Query("type")
	name := c.Query("name")

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName,
		projectName, setting.OperationSceneEnv,
		"伸缩",
		"环境-服务",
		fmt.Sprintf("环境名称:%s,%s:%s", envName, resourceType, name),
		"", ctx.Logger, envName)

	number, err := strconv.Atoi(c.Query("number"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid number format")
		return
	}

	ctx.Err = service.Scale(&service.ScaleArgs{
		Type:        resourceType,
		ProductName: projectName,
		EnvName:     envName,
		ServiceName: serviceName,
		Name:        name,
		Number:      number,
	}, ctx.Logger)
}

func GetServiceContainer(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	projectName := c.Query("projectName")
	serviceName := c.Param("serviceName")
	container := c.Param("container")

	ctx.Err = service.GetServiceContainer(envName, projectName, serviceName, container, ctx.Logger)
}
