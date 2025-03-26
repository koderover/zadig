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
	"github.com/koderover/zadig/v2/pkg/types"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/v2/pkg/shared/client/plutusvendor"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func ListS3Storage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.S3StorageManagement.View {
			ctx.UnAuthorized = true
			return
		}
	}

	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.RespErr = e.ErrInvalidParam
		return
	}
	ctx.Resp, ctx.RespErr = service.ListS3Storage(encryptedKey, ctx.Logger)
}

// @Summary List S3 Storage By Project
// @Description List S3 Storage By Project
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	projectName	query		string										true	"project name"
// @Success 200 		{array} 	commonmodels.S3Storage
// @Router /api/aslan/system/s3storage/project [get]
func ListS3StorageByProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if len(projectName) == 0 {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.ListS3StorageByProject(projectName, ctx.Logger)
}

func CreateS3Storage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.S3Storage)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateS3Storage c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateS3Storage json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "系统设置-对象存储", fmt.Sprintf("地址:%s", c.GetString("s3StorageEndpoint")), string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.S3StorageManagement.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to validate zadig license status, error: %s", err)
		return
	}
	if args.Provider == config.S3StorageProviderAmazonS3 {
		if !((licenseStatus.Type == plutusvendor.ZadigSystemTypeProfessional ||
			licenseStatus.Type == plutusvendor.ZadigSystemTypeEnterprise) &&
			licenseStatus.Status == plutusvendor.ZadigXLicenseStatusNormal) {
			ctx.RespErr = e.ErrLicenseInvalid.AddDesc("")
			return
		}
	}

	storage := &s3.S3{
		S3Storage: args,
	}
	if err := storage.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.RespErr = service.CreateS3Storage(ctx.UserName, args, ctx.Logger)
}

func GetS3Storage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.S3StorageManagement.View {
			ctx.UnAuthorized = true
		}
		return
	}

	ctx.Resp, ctx.RespErr = service.GetS3Storage(c.Param("id"), ctx.Logger)
}

func UpdateS3Storage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.S3Storage)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateS3Storage c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateS3Storage json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "系统设置-对象存储", fmt.Sprintf("地址:%s", c.GetString("s3StorageEndpoint")), string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.S3StorageManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to validate zadig license status, error: %s", err)
		return
	}
	if args.Provider == config.S3StorageProviderAmazonS3 {
		if !((licenseStatus.Type == plutusvendor.ZadigSystemTypeProfessional ||
			licenseStatus.Type == plutusvendor.ZadigSystemTypeEnterprise) &&
			licenseStatus.Status == plutusvendor.ZadigXLicenseStatusNormal) {
			ctx.RespErr = e.ErrLicenseInvalid.AddDesc("")
			return
		}
	}

	storage := &s3.S3{
		S3Storage: args,
	}
	if err := storage.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	id := c.Param("id")
	ctx.RespErr = service.UpdateS3Storage(ctx.UserName, id, args, ctx.Logger)
}

func DeleteS3Storage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "系统设置-对象存储", fmt.Sprintf("s3Storage ID:%s", c.Param("id")), "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.S3StorageManagement.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.DeleteS3Storage(ctx.UserName, c.Param("id"), ctx.Logger)
}

type ListTarsOption struct {
	Names []string `json:"names"`
}

func ListTars(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(ListTarsOption)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.ListTars(c.Param("id"), c.Query("kind"), args.Names, ctx.Logger)
}
