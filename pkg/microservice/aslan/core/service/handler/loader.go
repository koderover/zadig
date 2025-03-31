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

func PreloadServiceTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostIDStr := c.Param("codehostId")

	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cannot convert codehost id to int")
		return
	}

	repoName := c.Query("repoName")
	repoUUID := c.Query("repoUUID")
	if repoName == "" && repoUUID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("repoName and repoUUID cannot be empty at the same time")
		return
	}

	branchName := c.Query("branchName")

	path := c.Query("path")
	isDir := c.Query("isDir") == "true"
	remoteName := c.Query("remoteName")
	repoOwner := c.Query("repoOwner")

	ctx.Resp, ctx.RespErr = svcservice.PreloadServiceFromCodeHost(codehostID, repoOwner, repoName, repoUUID, branchName, remoteName, path, isDir, ctx.Logger)
}

func LoadServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	codehostIDStr := c.Param("codehostId")

	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cannot convert codehost id to string")
		return
	}

	repoName := c.Query("repoName")
	repoUUID := c.Query("repoUUID")
	if repoName == "" && repoUUID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("repoName and repoUUID cannot be empty at the same time")
		return
	}

	branchName := c.Query("branchName")

	args := new(svcservice.LoadServiceReq)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid LoadServiceReq json args")
		return
	}

	remoteName := c.Query("remoteName")
	repoOwner := c.Query("repoOwner")
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = repoOwner
	}

	production := c.Query("production") == "true"
	detail := "项目管理-服务"
	if production {
		detail = "项目管理-生产服务"
	}

	// Note we can't get the service name from handler layer since it parsed from files on git repo
	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", detail, "", string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[args.ProductName].ProductionService.Create {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[args.ProductName].Service.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	ctx.RespErr = svcservice.LoadServiceFromCodeHost(ctx.UserName, codehostID, repoOwner, namespace, repoName, repoUUID, branchName, remoteName, args, false, production, ctx.Logger)
}

func SyncServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	codehostIDStr := c.Param("codehostId")

	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cannot convert codehost id to string")
		return
	}

	repoName := c.Query("repoName")
	repoUUID := c.Query("repoUUID")
	if repoName == "" && repoUUID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("repoName and repoUUID cannot be empty at the same time")
		return
	}

	branchName := c.Query("branchName")

	args := new(svcservice.LoadServiceReq)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid LoadServiceReq json args")
		return
	}

	remoteName := c.Query("remoteName")
	repoOwner := c.Query("repoOwner")
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = repoOwner
	}

	production := c.Query("production") == "true"
	detail := "项目管理-服务"
	if production {
		detail = "项目管理-生产服务"
	}

	// Note we can't get the service name from handler layer since it parsed from files on git repo
	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", detail, "", string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[args.ProductName].ProductionService.Edit {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[args.ProductName].Service.Edit {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	ctx.RespErr = svcservice.LoadServiceFromCodeHost(ctx.UserName, codehostID, repoOwner, namespace, repoName, repoUUID, branchName, remoteName, args, true, production, ctx.Logger)
}

// ValidateServiceUpdate seems to require no privilege
func ValidateServiceUpdate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostIDStr := c.Param("codehostId")

	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cannot convert codehost id to string")
		return
	}

	repoName := c.Query("repoName")
	repoUUID := c.Query("repoUUID")
	if repoName == "" && repoUUID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("repoName and repoUUID cannot be empty at the same time")
		return
	}

	branchName := c.Query("branchName")

	path := c.Query("path")
	isDir := c.Query("isDir") == "true"
	remoteName := c.Query("remoteName")
	repoOwner := c.Query("repoOwner")
	serviceName := c.Query("serviceName")

	ctx.RespErr = svcservice.ValidateServiceUpdate(codehostID, serviceName, repoOwner, repoName, repoUUID, branchName, remoteName, path, isDir, ctx.Logger)
}
