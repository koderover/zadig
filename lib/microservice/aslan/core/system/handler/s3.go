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
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/lib/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/types/permission"
)

func ListS3Storage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListS3Storage(ctx.Logger)
	return
}

func CreateS3Storage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.S3Storage)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateS3Storage c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateS3Storage json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, "", "新增", "系统设置-对象存储", fmt.Sprintf("地址:%s", c.GetString("s3StorageEndpoint")), permission.SuperUserUUID, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	storage := &s3.S3{
		S3Storage: args,
	}
	if err := storage.Validate(); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.CreateS3Storage(ctx.Username, args, ctx.Logger)
}

func GetS3Storage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetS3Storage(c.Param("id"), ctx.Logger)
}

func UpdateS3Storage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.S3Storage)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateS3Storage c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateS3Storage json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, "", "更新", "系统设置-对象存储", fmt.Sprintf("地址:%s", c.GetString("s3StorageEndpoint")), permission.SuperUserUUID, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	storage := &s3.S3{
		S3Storage: args,
	}
	if err := storage.Validate(); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	id := c.Param("id")
	ctx.Resp, ctx.Err = service.UpdateS3Storage(ctx.Username, id, args, ctx.Logger)
}

func DeleteS3Storage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.Username, "", "删除", "系统设置-对象存储", fmt.Sprintf("s3Storage ID:", c.Param("id")), permission.SuperUserUUID, "", ctx.Logger)

	ctx.Err = service.DeleteS3Storage(ctx.Username, c.Param("id"), ctx.Logger)
}
