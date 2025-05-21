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
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func CreateSystemCodeHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	req := new(models.CodeHost)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		if req.Type == setting.SourceFromGiteeEE {
			ctx.RespErr = err
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.CreateSystemCodeHost(req, ctx.Logger)
}

func ListSystemCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.RespErr = e.ErrInvalidParam
		return
	}
	ctx.Resp, ctx.RespErr = service.SystemList(encryptedKey, c.Query("address"), c.Query("owner"), c.Query("source"), ctx.Logger)
}

func ListCodeHostInternal(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.RespErr = service.ListInternal(c.Query("address"), c.Query("owner"), c.Query("source"), ctx.Logger)
}

func DeleteSystemCodeHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.RespErr = err
		return
	}
	ctx.RespErr = service.DeleteCodeHost(id, ctx.Logger)
}

func GetSystemCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.RespErr = err
		return
	}

	ignoreDelete := false
	if len(c.Query("ignoreDelete")) > 0 {
		ignoreDelete, err = strconv.ParseBool(c.Query("ignoreDelete"))
		if err != nil {
			ctx.RespErr = fmt.Errorf("failed to parse param ignoreDelete, err: %s", err)
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.GetCodeHost(id, ignoreDelete, ctx.Logger)
}

func AuthCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	idInt, err := strconv.Atoi(id)
	if err != nil {
		ctx.RespErr = err
		return
	}

	url, err := service.AuthCodeHost(c.Query("redirect_url"), idInt, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		ctx.Logger.Errorf("auth err,id:%d,err: %s", idInt, err)
		return
	}
	c.Redirect(http.StatusFound, url)
}

func Callback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	state := c.Query("state")
	redirectURL, err := service.HandleCallback(state, c.Request, ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("Callback err:%s", err)
		ctx.RespErr = err
		return
	}
	c.Redirect(http.StatusFound, redirectURL)
}

func UpdateSystemCodeHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.RespErr = err
		return
	}
	req := &models.CodeHost{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}
	req.ID = id

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		if req.Type == setting.SourceFromGiteeEE {
			ctx.RespErr = e.ErrLicenseInvalid.AddDesc("")
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.UpdateCodeHost(req, ctx.Logger)
}

// @Summary 验证代码源连接
// @Description
// @Tags 	codehost
// @Accept 	json
// @Produce json
// @Param 	codehost		body		models.CodeHost					true	"代码库信息"
// @Success 200
// @Router /api/v1/codehosts/validate [post]
func ValidateCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(models.CodeHost)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.ValidateCodeHost(ctx, req)
}
