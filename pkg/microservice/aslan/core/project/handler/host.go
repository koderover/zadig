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

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	systemservice "github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

type batchCreatePMArgs struct {
	Option string                     `json:"option"`
	Data   []*commonmodels.PrivateKey `json:"data"`
}

func ListPMHosts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.Err = e.ErrInvalidParam
		return
	}
	ctx.Resp, ctx.Err = systemservice.ListPrivateKeys(encryptedKey, c.Query("projectName"), c.Query("keyword"), false, ctx.Logger)
}

func GetPMHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetPrivateKey(c.Param("id"), ctx.Logger)
}

func ListLabels(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListLabels()
}

func CreatePMHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	args := new(commonmodels.PrivateKey)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreatePMHost c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreatePMHost json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "项目资源-主机管理", fmt.Sprintf("hostName:%s ip:%s", args.Name, args.IP), string(data), ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid physical machine args")
		return
	}
	args.UpdateBy = ctx.UserName
	args.ProjectName = projectKey
	ctx.Err = systemservice.CreatePrivateKey(args, ctx.Logger)
}

func BatchCreatePMHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	args := new(batchCreatePMArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("BatchCreatePMHost c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("BatchCreatePMHost json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "批量新增", "项目资源-主机管理", "", string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid physical machine args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	for _, pmArg := range args.Data {
		pmArg.ProjectName = projectKey
	}
	ctx.Err = systemservice.BatchCreatePrivateKey(args.Data, args.Option, ctx.UserName, ctx.Logger)
}

func UpdatePMHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.PrivateKey)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdatePhysicalHost c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdatePhysicalHost json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "项目资源-主机管理", fmt.Sprintf("hostName:%s ip:%s", args.Name, args.IP), string(data), ctx.Logger)

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid physical machine args")
		return
	}
	args.UpdateBy = ctx.UserName

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Err = systemservice.UpdatePrivateKey(c.Param("id"), args, ctx.Logger)
}

// TODO: add authorization to this
func DeletePMHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "项目资源-主机管理", fmt.Sprintf("id:%s", c.Param("id")), "", ctx.Logger)
	ctx.Err = systemservice.DeletePrivateKey(c.Param("id"), ctx.UserName, ctx.Logger)
}
