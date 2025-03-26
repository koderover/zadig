/*
Copyright 2023 The KodeRover Authors.

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
	"github.com/koderover/zadig/v2/pkg/types"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func CreateOrUpdateSecuritySettings(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.SecurityAndPrivacySettings)

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("upsert security settings GetRawData err : %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("upsert security settings Unmarshal err : %s", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "安全与隐私", fmt.Sprintf("token expiration: %d \n improvement plan: %v", args.TokenExpirationTime, args.ImprovementPlan), string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to update sonar integration: %s", err)
		return
	}

	if args.TokenExpirationTime > 8640 {
		ctx.RespErr = errors.New("token expiration time cannot be greater than 8640 hour")
		return
	}
	ctx.RespErr = service.CreateOrUpdateSecuritySettings(args, ctx.Logger)
}

func GetSecuritySettings(c *gin.Context) {
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

	resp, err := service.GetSecuritySettings(ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp = resp
}
