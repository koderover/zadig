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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	"github.com/koderover/zadig/v2/pkg/types"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var SonarIntegrationValidationError = errors.New("name and server must be provided")

func CreateSonarIntegration(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.SonarIntegration)

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("Create sonar integration GetRawData err : %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("Create sonar integration Unmarshal err : %s", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "系统配置-Sonar集成", fmt.Sprintf("server: %s, token: %s", args.ServerAddress, args.Token), string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to update sonar integration: %s", err)
		return
	}

	if args.ServerAddress == "" || args.Token == "" {
		ctx.RespErr = SonarIntegrationValidationError
		return
	}
	ctx.RespErr = service.CreateSonarIntegration(args, ctx.Logger)
}

func UpdateSonarIntegration(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.SonarIntegration)

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("Update sonar integration GetRawData err : %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("Update sonar integration Unmarshal err : %s", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "系统配置-Sonar集成", fmt.Sprintf("server: %s, token: %s", args.ServerAddress, args.Token), string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to update sonar integration: %s", err)
		return
	}

	if args.ServerAddress == "" || args.Token == "" {
		ctx.RespErr = SonarIntegrationValidationError
		return
	}
	ctx.RespErr = service.UpdateSonarIntegration(c.Param("id"), args, ctx.Logger)
}

func ListSonarIntegration(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// TODO: Authorization leak
	// authorization checks
	//if !ctx.Resources.IsSystemAdmin {
	//	ctx.UnAuthorized = true
	//	return
	//}

	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	aesKey, err := commonutil.GetAesKeyFromEncryptedKey(encryptedKey, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	sonarList, _, err := service.ListSonarIntegration(ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	for _, sonar := range sonarList {
		encryptedSonarToken, err := crypto.AesEncryptByKey(sonar.Token, aesKey.PlainText)
		if err != nil {
			ctx.RespErr = fmt.Errorf("failed to encrypt sonar token, err: %s", err)
			return
		}
		sonar.Token = encryptedSonarToken
	}
	ctx.Resp = sonarList
}

func GetSonarIntegration(c *gin.Context) {
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

	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	aesKey, err := commonutil.GetAesKeyFromEncryptedKey(encryptedKey, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	resp, err := service.GetSonarIntegration(c.Param("id"), ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	encryptedSonarToken, err := crypto.AesEncryptByKey(resp.Token, aesKey.PlainText)
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to encrypt sonar token, err: %s", err)
		return
	}
	resp.Token = encryptedSonarToken
	ctx.Resp = resp
}

func DeleteSonarIntegration(c *gin.Context) {
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

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "系统配置-Sonar集成", fmt.Sprintf("id:%s", c.Param("id")), "", types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = service.DeleteSonarIntegration(c.Param("id"), ctx.Logger)
}

func ValidateSonarInformation(c *gin.Context) {
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

	args := new(service.SonarIntegration)

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("Validate sonar integration GetRawData err : %s", err)
		ctx.RespErr = fmt.Errorf("validate sonar integration GetRawData err : %s", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("Validate sonar integration Unmarshal err : %s", err)
		ctx.RespErr = fmt.Errorf("validate sonar integration Unmarshal err : %s", err)
		return
	}

	ctx.RespErr = service.ValidateSonarIntegration(args, ctx.Logger)
}
