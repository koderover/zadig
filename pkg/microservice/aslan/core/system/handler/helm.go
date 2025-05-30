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
	"net/url"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	helmservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/helm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func ListHelmRepos(c *gin.Context) {
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

	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.HelmRepoManagement.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = helmservice.ListHelmRepos(encryptedKey, ctx.Logger)
}

// @Summary List Helm Repos By Project
// @Description List Helm Repos By Project
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	projectName	query		string										true	"project name"
// @Success 200 		{array} 	commonmodels.HelmRepo
// @Router /api/aslan/system/helm/project [get]
func ListHelmReposByProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if len(projectKey) == 0 {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = helmservice.ListHelmReposByProject(projectKey, ctx.Logger)
}

func ListHelmReposPublic(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = helmservice.ListHelmReposPublic()
}

func CreateHelmRepo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.HelmRepo)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid helmRepo json args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.HelmRepoManagement.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	args.UpdateBy = ctx.UserName
	if _, err := url.Parse(args.URL); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid url")
		return
	}

	ctx.RespErr = service.CreateHelmRepo(args, ctx.Logger)
}

// @Summary 验证 Helm 仓库连接
// @Description
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	helmRepo		body		commonmodels.HelmRepo					true	"Helm 仓库信息"
// @Success 200
// @Router /api/aslan/system/helm/validate [post]
func ValidateHelmRepo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.HelmRepo)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid helmRepo json args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.HelmRepoManagement.Create && !ctx.Resources.SystemActions.HelmRepoManagement.Edit && !ctx.Resources.SystemActions.HelmRepoManagement.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	if _, err := url.Parse(args.URL); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid url")
		return
	}

	ctx.RespErr = service.ValidateHelmRepo(args, ctx.Logger)
}

func UpdateHelmRepo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.HelmRepo)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid helmRepo json args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.HelmRepoManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	args.UpdateBy = ctx.UserName
	ctx.RespErr = service.UpdateHelmRepo(c.Param("id"), args, ctx.Logger)
}

func DeleteHelmRepo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.HelmRepoManagement.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.DeleteHelmRepo(c.Param("id"), ctx.Logger)
}

func ListCharts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.ListCharts(c.Param("name"), ctx.Logger)
}
