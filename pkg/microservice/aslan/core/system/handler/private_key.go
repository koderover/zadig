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
	"io"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/koderover/zadig/v2/pkg/types"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func ListPrivateKeysInternal(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.ListPrivateKeysInternal(ctx.Logger)
}

func ListPrivateKeys(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	// TODO: Authorization leak
	// comment: since currently there are multiple functionalities that wish to used this API without authorization,
	// we temporarily disabled the permission checks for this API.

	// authorization checks
	//if !ctx.Resources.IsSystemAdmin {
	//	ctx.UnAuthorized = true
	//	return
	//}

	ctx.Resp, ctx.RespErr = service.ListPrivateKeys(encryptedKey, "", c.Query("keyword"), true, ctx.Logger)
}

func GetPrivateKey(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.GetPrivateKey(c.Param("id"), ctx.Logger)
}

func CreatePrivateKey(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.PrivateKey)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreatePrivateKey c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreatePrivateKey json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "资源管理-主机管理", fmt.Sprintf("hostName:%s ip:%s", args.Name, args.IP), string(data), types.RequestBodyTypeJSON, ctx.Logger)

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.VMManagement.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid PrivateKey args")
		return
	}
	args.UpdateBy = ctx.UserName

	err = args.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.CreatePrivateKey(args, ctx.Logger)
}

func UpdatePrivateKey(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.PrivateKey)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdatePrivateKey c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdatePrivateKey json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "资源管理-主机管理", fmt.Sprintf("hostName:%s ip:%s", args.Name, args.IP), string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.VMManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid PrivateKey args")
		return
	}
	args.UpdateBy = ctx.UserName

	err = args.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.UpdatePrivateKey(c.Param("id"), args, ctx.Logger)
}

func DeletePrivateKey(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "资源管理-主机管理", fmt.Sprintf("id:%s", c.Param("id")), "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.VMManagement.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.DeletePrivateKey(c.Param("id"), ctx.UserName, ctx.Logger)
}

func ListLabels(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.ListLabels()
}

type privateKeyArgs struct {
	Option string                     `json:"option"`
	Data   []*commonmodels.PrivateKey `json:"data"`
}

func BatchCreatePrivateKey(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(privateKeyArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("batchCreatePrivateKey c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("batchCreatePrivateKey json.Unmarshal err : %v", err)
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "批量新增", "资源管理-主机管理", "", string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.VMManagement.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid PrivateKey args")
		return
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.BatchCreatePrivateKey(args.Data, args.Option, ctx.UserName, ctx.Logger)
}
