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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/shared/poetry"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func GetCodeHostList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codeHostSlice := make([]*poetry.CodeHost, 0)
	codeHosts, err := codehost.GetCodeHostList()
	ctx.Err = err
	for _, codeHost := range codeHosts {
		codeHost.AccessToken = setting.MaskValue
		codeHost.AccessKey = setting.MaskValue
		codeHost.SecretKey = setting.MaskValue
		codeHost.Password = setting.MaskValue

		codeHostSlice = append(codeHostSlice, codeHost)
	}
	ctx.Resp = codeHostSlice
}

type CodeHostGetNamespaceListArgs struct {
	Keywork string `json:"key"          form:"key"`
	PerPage int    `json:"perPage"      form:"perPage,default:100"`
	Page    int    `json:"page"         form:"page,default:1"`
}

func CodeHostGetNamespaceList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostID := c.Param("codehostId")
	if codehostID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}
	args := &CodeHostGetNamespaceListArgs{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}
	chID, _ := strconv.Atoi(codehostID)
	ctx.Resp, ctx.Err = service.CodeHostListNamespaces(chID, args.Keywork, args.Page, args.PerPage, ctx.Logger)
}

func CodeHostGetProjectsList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	namespaceType := c.DefaultQuery("type", "group")
	codehostID := c.Param("codehostId")
	namespace := c.Param("namespace")
	keyword := c.Query("key")

	if codehostID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}
	if namespace == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty namespace")
		return
	}
	if namespaceType == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty namespaceType")
		return
	}
	if namespaceType != service.UserKind && namespaceType != service.GroupKind && namespaceType != service.OrgKind {
		ctx.Err = e.ErrInvalidParam.AddDesc("namespaceType must be user/group/org")
		return
	}

	chID, _ := strconv.Atoi(codehostID)
	ctx.Resp, ctx.Err = service.CodeHostListProjects(
		chID,
		strings.Replace(namespace, "%2F", "/", -1),
		namespaceType,
		keyword,
		ctx.Logger)
}

type CodeHostGetBranchListArgs struct {
	PerPage int `json:"perPage"      form:"perPage,default:20"`
	Page    int `json:"page"         form:"page,default:1"`
}

func CodeHostGetBranchList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostID := c.Param("codehostId")
	namespace := c.Param("namespace")
	projectName := c.Param("projectName") // pro Name, id/name -> gitlab = id

	args := new(CodeHostGetBranchListArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if codehostID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}
	if namespace == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty namespace")
		return
	}
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty projectName")
		return
	}

	chID, _ := strconv.Atoi(codehostID)
	ctx.Resp, ctx.Err = service.CodeHostListBranches(
		chID,
		projectName,
		strings.Replace(namespace, "%2F", "/", -1),
		args.Page,
		args.PerPage,
		ctx.Logger)
}

func CodeHostGetTagList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostID := c.Param("codehostId")
	namespace := c.Param("namespace")
	projectName := c.Param("projectName") // pro Name, id/name -> gitlab = id

	if codehostID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}
	if namespace == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty namespace")
		return
	}
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty projectName")
		return
	}

	chID, _ := strconv.Atoi(codehostID)
	ctx.Resp, ctx.Err = service.CodeHostListTags(chID, projectName, strings.Replace(namespace, "%2F", "/", -1), ctx.Logger)
}

func CodeHostGetPRList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostID := c.Param("codehostId")
	namespace := c.Param("namespace")
	projectName := c.Param("projectName") // pro Name, id/name -> gitlab = id

	if codehostID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}
	if namespace == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty namespace")
		return
	}
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty projectName")
		return
	}

	targetBr := c.Query("targetBranch")

	chID, _ := strconv.Atoi(codehostID)
	ctx.Resp, ctx.Err = service.CodeHostListPRs(chID, projectName, strings.Replace(namespace, "%2F", "/", -1), targetBr, ctx.Logger)
}

func ListRepoInfos(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.RepoInfoList)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid repo args")
		return
	}
	param := c.Query("param")
	ctx.Resp, ctx.Err = service.ListRepoInfos(args.Infos, param, ctx.Logger)
}
