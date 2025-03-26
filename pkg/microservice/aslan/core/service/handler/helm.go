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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	svcservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func ListHelmServices(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("productName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.View {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	ctx.Resp, ctx.RespErr = svcservice.ListHelmServices(projectKey, production, ctx.Logger)
}

func GetHelmServiceModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("productName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.View {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.RespErr = svcservice.GetHelmServiceModule(c.Param("serviceName"), projectKey, revision, production, ctx.Logger)
}

func GetFilePath(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("productName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.View {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	revision := int64(0)
	if len(c.Query("revision")) > 0 {
		revision, err = strconv.ParseInt(c.Query("revision"), 10, 64)
	}
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.RespErr = svcservice.GetFilePath(c.Param("serviceName"), projectKey, revision, c.Query("dir"), production, ctx.Logger)
}

func GetFileContent(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("productName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.View {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	param := new(svcservice.GetFileContentParam)
	err = c.ShouldBindQuery(param)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = svcservice.GetFileContent(c.Param("serviceName"), projectKey, param, production, ctx.Logger)
}

func UpdateFileContent(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Create &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Edit {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.Create &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.Edit {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	param := new(svcservice.HelmChartEditInfo)
	err = c.ShouldBind(param)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	param.Production = production
	ctx.RespErr = svcservice.EditFileContent(c.Param("serviceName"), projectKey, ctx.UserName, ctx.RequestID, param, ctx.Logger)
}

func CreateOrUpdateHelmService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be nil")
		return
	}

	args := new(svcservice.HelmServiceCreationArgs)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid HelmService json args")
		return
	}
	args.CreatedBy, args.RequestID = ctx.UserName, ctx.RequestID

	production := c.Query("production") == "true"
	detail := "项目管理-服务"
	if production {
		detail = "项目管理-生产服务"
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "新增", detail, fmt.Sprintf("服务名称:%s", args.Name), string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Create {
				ctx.UnAuthorized = true
				return
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	args.Production = production
	ctx.Resp, ctx.RespErr = svcservice.CreateOrUpdateHelmService(projectKey, args, false, ctx.Logger)
}

func UpdateHelmService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be nil")
		return
	}

	args := new(svcservice.HelmServiceCreationArgs)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid HelmService json args")
		return
	}
	args.CreatedBy, args.RequestID = ctx.UserName, ctx.RequestID

	production := c.Query("production") == "true"
	detail := "项目管理-服务"
	if production {
		detail = "项目管理-生产服务"
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "更新", detail, fmt.Sprintf("服务名称:%s", args.Name), string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Create &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Edit {
				ctx.UnAuthorized = true
				return
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.Create &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.Edit {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	args.Production = production
	ctx.Resp, ctx.RespErr = svcservice.CreateOrUpdateHelmService(projectKey, args, true, ctx.Logger)
}

func CreateOrUpdateBulkHelmServices(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be nil")
		return
	}

	args := new(svcservice.BulkHelmServiceCreationArgs)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid HelmService json args")
		return
	}
	args.CreatedBy, args.RequestID = ctx.UserName, ctx.RequestID

	production := c.Query("production") == "true"
	detail := "项目管理-服务"
	if production {
		detail = "项目管理-生产服务"
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "新增", detail, "", string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Create {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	ctx.Resp, ctx.RespErr = svcservice.CreateOrUpdateBulkHelmService(projectKey, args, false, ctx.Logger)
}
