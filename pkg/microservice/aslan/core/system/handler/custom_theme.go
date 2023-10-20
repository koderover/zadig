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
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/pkg/shared/client/plutusvendor"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetThemeInfos(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	if err != nil {
		ctx.Err = fmt.Errorf("failed to validate zadig license status, error: %s", err)
		return
	}
	if !(licenseStatus.Type == plutusvendor.ZadigSystemTypeProfessional && licenseStatus.Status == plutusvendor.ZadigXLicenseStatusNormal) {
		ctx.Err = e.ErrLicenseInvalid
		return
	}

	ctx.Resp, ctx.Err = service.GetThemeInfos(ctx.Logger)
}

func UpdateThemeInfo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateThemeInfo GetRawData err :%v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	theme := &models.Theme{}
	if err := json.Unmarshal(args, theme); err != nil {
		log.Errorf("UpdateThemeInfo Unmarshal err :%v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	if theme == nil {
		return
	}

	licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	if err != nil {
		ctx.Err = fmt.Errorf("failed to validate zadig license status, error: %s", err)
		return
	}
	if !(licenseStatus.Type == plutusvendor.ZadigSystemTypeProfessional && licenseStatus.Status == plutusvendor.ZadigXLicenseStatusNormal) {
		ctx.Err = e.ErrLicenseInvalid
		return
	}

	ctx.Err = service.UpdateThemeInfo(theme, ctx.Logger)
}
