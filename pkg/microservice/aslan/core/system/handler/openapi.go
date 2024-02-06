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
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"io"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func OpenAPICreateRegistry(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPICreateRegistryReq)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("OpenAPICreateRegistry c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateRegistryNamespace json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "新增", "系统设置-Registry", fmt.Sprintf("提供商:%s,Namespace:%s", args.Provider, args.Namespace), string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	err = args.Validate()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.OpenAPICreateRegistry(ctx.UserName, args, ctx.Logger)
}

func OpenAPIListRegistry(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	registry, err := service.ListRegistries(ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}

	resp := make([]*service.OpenAPIRegistry, 0)
	for _, reg := range registry {
		resp = append(resp, &service.OpenAPIRegistry{
			ID:        reg.ID.Hex(),
			Address:   reg.RegAddr,
			Provider:  config.RegistryProvider(reg.RegProvider),
			Region:    reg.Region,
			IsDefault: reg.IsDefault,
			Namespace: reg.Namespace,
		})
	}
	ctx.Resp = resp
}

func OpenAPIGetRegistry(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	registry, _, err := commonservice.FindRegistryById(c.Param("id"), true, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}

	ret := &service.OpenAPIRegistry{
		ID:        registry.ID.Hex(),
		Address:   registry.RegAddr,
		Provider:  config.RegistryProvider(registry.RegProvider),
		Region:    registry.Region,
		IsDefault: registry.IsDefault,
		Namespace: registry.Namespace,
	}
	ctx.Resp = ret
}

func OpenAPIListCluster(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.OpenAPIListCluster(c.Query("projectName"), ctx.Logger)
}

func OpenAPIUpdateCluster(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPICluster)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		log.Errorf("Failed to bind data: %s", err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "更新", "资源配置-集群", c.Param("id"), "", ctx.Logger)

	ctx.Err = service.OpenAPIUpdateCluster(c.Param("id"), args, ctx.Logger)
}

func OpenAPIDeleteCluster(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "删除", "资源配置-集群", c.Param("id"), "", ctx.Logger)
	ctx.Err = service.OpenAPIDeleteCluster(ctx.UserName, c.Query("projectName"), ctx.Logger)
}
