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

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/lib/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/types/permission"
)

type ListServicePodsArgs struct {
	serviceName string `json:"service_name"`
	ProductName string `json:"product_name"`
	EnvName     string `json:"env_name"`
}

func ListKubeEvents(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Query("productName")
	name := c.Query("name")
	rtype := c.Query("type")

	if !(rtype == setting.Deployment || rtype == setting.StatefulSet) {
		ctx.Resp = make([]interface{}, 0)
		return
	}

	ctx.Resp, ctx.Err = service.ListKubeEvents(envName, productName, name, rtype, ctx.Logger)
}

func ListAvailableNamespaces(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListAvailableNamespaces(c.Query("clusterId"), ctx.Logger)
}

func ListServicePods(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(ListServicePodsArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = service.ListServicePods(
		args.ProductName,
		args.EnvName,
		args.serviceName,
		ctx.Logger,
	)
}

func DeletePod(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	podName := c.Param("podName")
	envName := c.Query("envName")
	productName := c.Query("productName")

	internalhandler.InsertOperationLog(c, ctx.Username, c.Query("productName"), "重启", "集成环境-服务实例", fmt.Sprintf("环境名称:%s,pod名称:%s", c.Query("envName"), c.Param("podName")), fmt.Sprintf("%s,%s", permission.TestEnvManageUUID, permission.ProdEnvManageUUID), "", ctx.Logger)
	ctx.Err = service.DeletePod(envName, productName, podName, ctx.Logger)
}

func ListPodEvents(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Query("productName")
	podName := c.Param("podName")

	ctx.Resp, ctx.Err = service.ListPodEvents(envName, productName, podName, ctx.Logger)
}
