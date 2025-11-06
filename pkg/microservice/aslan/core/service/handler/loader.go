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

type PreloadServiceTemplateReq struct {
	RepoName   string                          `json:"repo_name"`
	RepoUUID   string                          `json:"repo_uuid"`
	BranchName string                          `json:"branch_name"`
	RemoteName string                          `json:"remote_name"`
	RepoOwner  string                          `json:"repo_owner"`
	Paths      []svcservice.PreLoadServicePath `json:"paths"`
}

// @summary 从代码库预加载服务
// @description
// @tags 	service
// @accept 	json
// @produce json
// @Param 	codehostId 		path 		int 							  true 	"codehostId"
// @Param 	body 			body 		PreloadServiceTemplateReq         true 	"body"
// @success 200             {array}     svcservice.LoadServicePath
// @router /api/aslan/service/loader/preload/:codehostId [post]
func PreloadServiceTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostIDStr := c.Param("codehostId")
	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cannot convert codehost id to int")
		return
	}

	var req PreloadServiceTemplateReq
	if err := c.BindJSON(&req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid PreloadServiceTemplateReq json args")
		return
	}

	if req.RepoName == "" && req.RepoUUID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("repoName and repoUUID cannot be empty at the same time")
		return
	}

	ctx.Resp, ctx.RespErr = svcservice.PreloadServiceFromCodeHost(codehostID, req.RepoOwner, req.RepoName, req.RepoUUID, req.BranchName, req.RemoteName, req.Paths, ctx.Logger)
}

// @summary 从代码库创建服务
// @description
// @tags 	service
// @accept 	json
// @produce json
// @Param 	codehostId 		path 		int 							  true 	"codehostId"
// @Param   repoName 		query 		string 							  true 	"repoName"
// @Param   repoUUID 		query 		string 							  true 	"repoUUID"
// @Param   branchName 		query 		string 							  true 	"branchName"
// @Param   remoteName 		query 		string 							  true 	"remoteName"
// @Param   repoOwner 		query 		string 							  true 	"repoOwner"
// @Param   namespace 		query 		string 							  true 	"namespace"
// @Param   production 		query 		bool 							  true 	"production"
// @Param 	body 			body 		svcservice.LoadServiceReq         true 	"body"
// @success 200
// @router /api/aslan/service/loader/load/:codehostId [post]
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
	function := "项目管理-服务"
	if production {
		function = "项目管理-生产服务"
	}

	// Note we can't get the service name from handler layer since it parsed from files on git repo
	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", function, "", "", string(bs), types.RequestBodyTypeJSON, ctx.Logger)

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

	ctx.RespErr = svcservice.LoadServiceFromCodeHost(ctx.UserName, codehostID, repoOwner, namespace, repoName, repoUUID, branchName, remoteName, args, false, false, production, ctx.Logger)
}

// @summary 从代码库更新服务
// @description
// @tags 	service
// @accept 	json
// @produce json
// @Param 	codehostId 		path 		int 							  true 	"codehostId"
// @Param   repoName 		query 		string 							  true 	"repoName"
// @Param   repoUUID 		query 		string 							  true 	"repoUUID"
// @Param   branchName 		query 		string 							  true 	"branchName"
// @Param   remoteName 		query 		string 							  true 	"remoteName"
// @Param   repoOwner 		query 		string 							  true 	"repoOwner"
// @Param   namespace 		query 		string 							  true 	"namespace"
// @Param   production 		query 		bool 							  true 	"production"
// @Param 	body 			body 		svcservice.LoadServiceReq         true 	"body"
// @success 200
// @router /api/aslan/service/loader/:codehostId [put]
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

	if len(args.ServicePaths) != 1 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("servicePaths must contain only one path")
		return
	}

	remoteName := c.Query("remoteName")
	repoOwner := c.Query("repoOwner")
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = repoOwner
	}

	production := c.Query("production") == "true"
	function := "项目管理-服务"
	if production {
		function = "项目管理-生产服务"
	}

	// Note we can't get the service name from handler layer since it parsed from files on git repo
	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", function, "", "", string(bs), types.RequestBodyTypeJSON, ctx.Logger)

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

	ctx.RespErr = svcservice.LoadServiceFromCodeHost(ctx.UserName, codehostID, repoOwner, namespace, repoName, repoUUID, branchName, remoteName, args, true, true, production, ctx.Logger)
}
