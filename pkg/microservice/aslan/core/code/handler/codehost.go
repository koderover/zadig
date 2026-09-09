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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	codehostrepo "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func GetCodeHostList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codeHostSlice := make([]*systemconfig.CodeHost, 0)
	codeHosts, err := systemconfig.New().ListCodeHostsInternal()
	ctx.RespErr = err
	for _, codeHost := range codeHosts {
		codeHost.AccessToken = setting.MaskValue
		codeHost.AccessKey = setting.MaskValue
		codeHost.SecretKey = setting.MaskValue
		codeHost.Password = setting.MaskValue

		codeHostSlice = append(codeHostSlice, codeHost)
	}
	ctx.Resp = codeHostSlice
}

func CodeHostGetNamespaceList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostID := c.Param("codehostId")
	keyword := c.Query("key")

	if codehostID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}
	chID, _ := strconv.Atoi(codehostID)
	namespaces, err := service.CodeHostListNamespaces(chID, keyword, ctx.Logger)
	if err != nil {
		ctx.RespErr = e.NewWithDesc(e.ErrCodehostListNamespaces, util.FormatCodeHostErrorWithDefault("Failed to connect to the code host or the configuration is invalid. Please check your code host settings", err))
		return
	}
	ctx.Resp = namespaces
}

type CodeHostListProjectsArgs struct {
	PerPage int    `json:"per_page"     form:"per_page,default=30"`
	Page    int    `json:"page"         form:"page,default=1"`
	Key     string `json:"key"          form:"key"`
}

func CodeHostGetProjectsList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	namespaceType := c.DefaultQuery("type", "group")
	codehostID := c.Param("codehostId")
	repoOwner := c.Query("repoOwner")

	if codehostID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}
	if repoOwner == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty repoOwner")
		return
	}
	if namespaceType == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty namespaceType")
		return
	}
	if namespaceType != service.UserKind && namespaceType != service.GroupKind && namespaceType != service.OrgKind && namespaceType != service.EnterpriseKind {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("namespaceType must be user/group/org")
		return
	}

	args := &CodeHostListProjectsArgs{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = err
		return
	}

	chID, _ := strconv.Atoi(codehostID)
	projects, err := service.CodeHostListProjects(
		chID,
		strings.Replace(repoOwner, "%2F", "/", -1),
		namespaceType,
		args.Page,
		args.PerPage,
		args.Key,
		ctx.Logger)
	if err != nil {
		ctx.RespErr = e.NewWithDesc(e.ErrCodehostListProjects, util.FormatCodeHostErrorWithDefault("Failed to fetch repositories. Please verify the code host connection and namespace permissions", err))
		return
	}

	for _, project := range projects {
		if project.Namespace == "" {
			project.Namespace = repoOwner
		}
	}
	ctx.Resp = projects
}

type CodeHostGetPageNateListArgs struct {
	PerPage int    `json:"per_page"     form:"per_page,default=100"`
	Page    int    `json:"page"         form:"page,default=1"`
	Key     string `json:"key"          form:"key"`
}

const openAPIFetchAllKey = "openapiFetchAll"

// @Summary 获取代码仓库分支列表
// @Description
// @Tags 	code
// @Accept 	json
// @Produce json
// @Param 	codehostId		path		string							true	"代码仓库ID"
// @Param 	repoOwner		query		string							true	"仓库拥有者"
// @Param 	repoName		query		string							true	"仓库名称"
// @Param 	per_page		query		int								false	"每页数量"
// @Param 	page			query		int								false	"页码"
// @Param 	key				query		string							false	"关键字"
// @Success 200 			{array} 	client.Branch
// @Router /api/aslan/code/codehost/{codehostId}/branches [get]
func CodeHostGetBranchList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostID := c.Param("codehostId")
	repoOwner := c.Request.URL.Query().Get("repoOwner")
	repoName := c.Query("repoName") // pro Name, id/name -> gitlab = id
	args := new(CodeHostGetPageNateListArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if codehostID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}
	if repoOwner == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty repoOwner")
		return
	}
	if repoName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty repoName")
		return
	}

	chID, _ := strconv.Atoi(codehostID)
	branches, err := service.CodeHostListBranches(
		chID,
		repoName,
		strings.Replace(repoOwner, "%2F", "/", -1),
		args.Key,
		args.Page,
		args.PerPage,
		c.GetBool(openAPIFetchAllKey),
		ctx.Logger)
	if err != nil {
		ctx.RespErr = e.NewWithDesc(e.ErrCodehostListBranches, util.FormatCodeHostErrorWithDefault("Failed to fetch branches. Please check if the repository exists and you have access permissions", err))
		return
	}
	ctx.Resp = branches
}

// @Summary 获取代码仓库标签列表
// @Description
// @Tags 	code
// @Accept 	json
// @Produce json
// @Param 	codehostId		path		string							true	"代码仓库ID"
// @Param 	repoOwner		query		string							true	"仓库拥有者"
// @Param 	repoName		query		string							true	"仓库名称"
// @Param 	per_page		query		int								false	"每页数量"
// @Param 	page			query		int								false	"页码"
// @Param 	key				query		string							false	"关键字"
// @Success 200 			{array} 	client.Tag
// @Router /api/aslan/code/codehost/{codehostId}/tags [get]
func CodeHostGetTagList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostID := c.Param("codehostId")
	repoOwner := c.Request.URL.Query().Get("repoOwner")
	repoName := c.Query("repoName") // pro Name, id/name -> gitlab = id
	args := new(CodeHostGetPageNateListArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if codehostID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}
	if repoOwner == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty repoOwner")
		return
	}
	if repoName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty repoName")
		return
	}

	chID, _ := strconv.Atoi(codehostID)
	tags, err := service.CodeHostListTags(chID, repoName, strings.Replace(repoOwner, "%2F", "/", -1), args.Key, args.Page, args.PerPage, c.GetBool(openAPIFetchAllKey), ctx.Logger)
	if err != nil {
		ctx.RespErr = e.NewWithDesc(e.ErrCodehostListTags, util.FormatCodeHostErrorWithDefault("Failed to fetch tags. Please check if the repository exists and you have access permissions", err))
		return
	}
	ctx.Resp = tags
}

func CodeHostGetPRList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostID := c.Param("codehostId")
	repoOwner := c.Request.URL.Query().Get("repoOwner")
	repoName := c.Query("repoName") // pro Name, id/name -> gitlab = id

	args := new(CodeHostGetPageNateListArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if codehostID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}
	if repoOwner == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty repoOwner")
		return
	}
	if repoName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty repoName")
		return
	}

	targetBr := c.Request.URL.Query().Get("targetBranch")

	chID, _ := strconv.Atoi(codehostID)
	prs, err := service.CodeHostListPRs(chID, repoName, strings.Replace(repoOwner, "%2F", "/", -1), targetBr, args.Key, args.Page, args.PerPage, c.GetBool(openAPIFetchAllKey), ctx.Logger)
	if err != nil {
		ctx.RespErr = e.NewWithDesc(e.ErrCodehostListPrs, util.FormatCodeHostErrorWithDefault("Failed to fetch pull requests. Please verify repository access and permissions", err))
		return
	}
	ctx.Resp = prs
}

func authorizeOpenAPICodehost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() {
		if ctx.RespErr != nil || ctx.UnAuthorized {
			internalhandler.JSONResponse(c, ctx)
			c.Abort()
		}
	}()
	if err != nil {
		ctx.RespErr, ctx.UnAuthorized = err, true
		return
	}
	projectKey, codehostName := c.Query("projectKey"), c.Param("codehostName")
	namespace, repoName := c.Query("repoNamespace"), c.Query("repoName")
	if projectKey == "" || codehostName == "" || namespace == "" || repoName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey, codehostName, repoNamespace and repoName are required")
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
	}
	codehosts, err := codehostrepo.NewCodehostColl().AvailableCodeHost(projectKey)
	if err != nil {
		ctx.RespErr = err
		return
	}
	codehostID := 0
	for _, codehost := range codehosts {
		if codehost.Alias == codehostName {
			if codehostID != 0 {
				ctx.RespErr = e.ErrInvalidParam.AddDesc("multiple code hosts with this name are available in project")
				return
			}
			codehostID = codehost.ID
		}
	}
	if codehostID == 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("codehost is not available in project")
		return
	}
	c.Params = append(c.Params, gin.Param{Key: "codehostId", Value: strconv.Itoa(codehostID)})
	query := c.Request.URL.Query()
	query.Del("page")
	query.Del("per_page")
	query.Del("key")
	query.Del("targetBranch")
	query.Set("repoOwner", namespace)
	c.Request.URL.RawQuery = query.Encode()
	c.Set(openAPIFetchAllKey, true)
}

func CodeHostGetCommits(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostID := c.Param("codehostId")
	repoNamespace := c.Query("repoNamespace")
	repoName := c.Query("repoName") // pro Name, id/name -> gitlab = id

	args := new(CodeHostGetPageNateListArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if codehostID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}
	if repoNamespace == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty repoNamespace")
		return
	}
	if repoName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty repoName")
		return
	}

	targetBr := c.Query("branchName")

	chID, _ := strconv.Atoi(codehostID)
	commits, err := service.CodeHostListCommits(chID, repoName, strings.Replace(repoNamespace, "%2F", "/", -1), targetBr, args.Page, args.PerPage, ctx.Logger)
	if err != nil {
		ctx.RespErr = e.NewWithDesc(e.ErrCodehostListCommits, util.FormatCodeHostErrorWithDefault("Failed to fetch commits. Please check if the branch exists and you have access permissions", err))
		return
	}
	ctx.Resp = commits
}

func ListRepoInfos(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("page invalid, err: %v", err))
	}
	perPage, err := strconv.Atoi(c.Query("per_page"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("per_page invalid, err: %v", err))
	}

	args := new(service.RepoInfoList)
	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid repo args")
		return
	}

	repoInfos, err := service.ListRepoInfos(args.Infos, page, perPage, ctx.Logger)
	if err != nil {
		ctx.RespErr = e.NewWithDesc(e.ErrCodehostListProjects, util.FormatCodeHostErrorWithDefault("Failed to fetch repository information. Please verify the repository configuration and permissions", err))
		return
	}
	ctx.Resp = repoInfos
}

type MatchBranchesListRequest struct {
	RepoOwner string `json:"repo_owner"`
	RepoName  string `json:"repo_name"` // pro Name, id/name -> gitlab = id
	Regular   string `json:"regular"`
}

func MatchRegularList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostID := c.Param("codehostId")

	req := new(MatchBranchesListRequest)
	err := c.BindJSON(req)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid request")
		return
	}

	if codehostID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty codehostId")
		return
	}

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("page invalid, err: %v", err))
	}
	perPage, err := strconv.Atoi(c.Query("per_page"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("per_page invalid, err: %v", err))
	}

	chID, _ := strconv.Atoi(codehostID)
	branches, err := service.MatchRegularList(
		chID,
		req.RepoName,
		strings.Replace(req.RepoOwner, "%2F", "/", -1),
		"",
		page,
		perPage,
		req.Regular,
		ctx.Logger)
	if err != nil {
		ctx.RespErr = e.NewWithDesc(e.ErrCodehostListBranches, util.FormatCodeHostErrorWithDefault("Failed to match branches. Please check if the repository exists and you have access permissions", err))
		return
	}
	ctx.Resp = branches
}
