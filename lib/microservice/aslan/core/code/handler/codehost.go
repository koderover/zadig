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

	"github.com/koderover/zadig/lib/microservice/aslan/core/code/service"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
)

func GetCodeHostList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	orgIDStr := c.Query("orgId")
	orgID, err := strconv.Atoi(orgIDStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("orgId can't be empty!")
		return
	}

	codeHostSlice := make([]*codehost.CodeHost, 0)
	codeHosts, err := codehost.GetCodehostList(orgID)
	ctx.Err = err
	for _, codeHost := range codeHosts {
		codeHost.AccessToken = setting.MaskValue

		codeHostSlice = append(codeHostSlice, codeHost)
	}
	ctx.Resp = codeHostSlice
}

func CodeHostGetNamespaceList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	codehostId := c.Param("codehostId")
	keyword := c.Query("key")

	if codehostId == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}
	chID, _ := strconv.Atoi(codehostId)
	ctx.Resp, ctx.Err = service.CodehostListNamespaces(chID, keyword, ctx.Logger)
	return
}

func CodeHostGetProjectsList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	namespaceType := c.DefaultQuery("type", "group")
	codehostId := c.Param("codehostId")
	namespace := c.Param("namespace")
	keyword := c.Query("key")

	if codehostId == "" {
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
	if namespaceType != service.UESRKIND && namespaceType != service.GROUPKIND && namespaceType != service.ORGKIND {
		ctx.Err = e.ErrInvalidParam.AddDesc("namespaceType must be user/group/org")
		return
	}

	chID, _ := strconv.Atoi(codehostId)
	ctx.Resp, ctx.Err = service.CodehostListProjects(
		chID,
		strings.Replace(namespace, "%2F", "/", -1),
		namespaceType,
		keyword,
		ctx.Logger)
	return
}

func CodeHostGetBranchList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	codehostId := c.Param("codehostId")
	namespace := c.Param("namespace")
	projectName := c.Param("projectName") // pro Name, id/name -> gitlab = id

	if codehostId == "" {
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

	chID, _ := strconv.Atoi(codehostId)
	ctx.Resp, ctx.Err = service.CodehostListBranches(
		chID,
		projectName,
		strings.Replace(namespace, "%2F", "/", -1),
		ctx.Logger)
	return
}

func CodeHostGetTagList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	codehostId := c.Param("codehostId")
	namespace := c.Param("namespace")
	projectName := c.Param("projectName") // pro Name, id/name -> gitlab = id

	if codehostId == "" {
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

	chID, _ := strconv.Atoi(codehostId)
	ctx.Resp, ctx.Err = service.CodehostListTags(chID, projectName, strings.Replace(namespace, "%2F", "/", -1), ctx.Logger)
	return
}

func CodeHostGetPRList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	codehostId := c.Param("codehostId")
	namespace := c.Param("namespace")
	projectName := c.Param("projectName") // pro Name, id/name -> gitlab = id

	if codehostId == "" {
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

	chID, _ := strconv.Atoi(codehostId)
	ctx.Resp, ctx.Err = service.CodehostListPRs(chID, projectName, strings.Replace(namespace, "%2F", "/", -1), targetBr, ctx.Logger)
	return
}

func ListRepoInfos(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(service.RepoInfoList)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid repo args")
		return
	}
	param := c.Query("param")
	ctx.Resp, ctx.Err = service.ListRepoInfos(args.Infos, param, ctx.Logger)
	return
}
