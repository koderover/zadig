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

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/types/permission"
)

func ListRegistries(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListRegistries(ctx.Logger)
}

func ListRegistryNamespaces(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = commonservice.ListRegistryNamespaces(ctx.Logger)
}

func CreateRegistryNamespace(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.RegistryNamespace)
	data, err := c.GetRawData()
	if err != nil {
		log.Error("CreateRegistryNamespace c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Error("CreateRegistryNamespace json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, "", "新增", "系统设置-Registry", fmt.Sprintf("提供商:%s,Namespace:%s", args.RegProvider, args.Namespace), permission.SuperUserUUID, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if err := args.Validate(); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	//args.ID = bson.NewObjectId()

	ctx.Err = service.CreateRegistryNamespace(ctx.Username, args, ctx.Logger)
}

func UpdateRegistryNamespace(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.RegistryNamespace)
	data, err := c.GetRawData()
	if err != nil {
		log.Error("UpdateRegistryNamespace c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Error("UpdateRegistryNamespace json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, "", "更新", "系统设置-Registry", fmt.Sprintf("提供商:%s,Namespace:%s", args.RegProvider, args.Namespace), permission.SuperUserUUID, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	if err := args.Validate(); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Err = service.UpdateRegistryNamespace(ctx.Username, c.Param("id"), args, ctx.Logger)
}

func DeleteRegistryNamespace(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.Username, "", "删除", "系统设置-Registry", fmt.Sprintf("registry ID:", c.Param("id")), permission.SuperUserUUID, "", ctx.Logger)

	ctx.Err = service.DeleteRegistryNamespace(c.Param("id"), ctx.Logger)
}

func ListAllRepos(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListAllRepos(ctx.Logger)
}

type ListImagesOption struct {
	Names []string `json:"names"`
}

func ListImages(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	//判断当前registryId是否为空
	regOps := new(commonrepo.FindRegOps)
	registryID := c.Query("registryId")
	if registryID != "" {
		regOps.ID = registryID
	} else {
		regOps.IsDefault = true
	}
	registryInfo, err := service.GetRegistryNamespace(regOps, ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("can't find candidate registry err :%v", err)
		return
	}

	args := new(ListImagesOption)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	names := args.Names
	images, err := service.ListReposTags(registryInfo, names, ctx.Logger)
	ctx.Resp, ctx.Err = images, err
}

func ListRepoImages(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	regOps := new(commonrepo.FindRegOps)
	regOps.IsDefault = true
	registryInfo, err := service.GetRegistryNamespace(regOps, ctx.Logger)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	name := c.Param("name")

	resp, err := service.GetRepoTags(registryInfo, name, ctx.Logger)
	ctx.Resp, ctx.Err = resp, err
}
