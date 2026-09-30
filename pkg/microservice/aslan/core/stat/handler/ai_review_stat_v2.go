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
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary 获取 AI 审查洞察概览
// @Description 支持全局、项目和代码库概览筛选
// @Tags stat
// @Accept json
// @Produce json
// @Param start_time query int true "开始时间，Unix 秒时间戳，包含"
// @Param end_time query int true "结束时间，Unix 秒时间戳，不包含，必须大于开始时间"
// @Param project_name query string false "项目标识"
// @Param codehost_id query int false "代码源 ID，大于零，查询代码库时与 repo_owner、repo_name 一起提供"
// @Param repo_owner query string false "仓库 owner 或完整 GitLab namespace"
// @Param repo_name query string false "仓库名称"
// @Success 200 {object} AIReviewStatsOverviewResponse
// @Router /api/aslan/stat/v2/ai_review/overview [get]
func GetAIReviewStatsOverview(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(AIReviewStatsOverviewRequest)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	args.ProjectName = strings.TrimSpace(args.ProjectName)
	args.RepoOwner = strings.TrimSpace(args.RepoOwner)
	args.RepoName = strings.TrimSpace(args.RepoName)
	scope := AIReviewStatsScope{Type: "global", ProjectName: args.ProjectName}
	if args.ProjectName != "" {
		scope.Type = "project"
	}
	query := c.Request.URL.Query()
	if query.Has("codehost_id") || query.Has("repo_owner") || query.Has("repo_name") {
		if args.CodehostID == nil || args.RepoOwner == "" || args.RepoName == "" {
			ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("codehost_id, repo_owner and repo_name must be provided together"))
			return
		}
		scope.Type = "repo"
		scope.CodehostID = *args.CodehostID
		scope.RepoOwner = args.RepoOwner
		scope.RepoName = args.RepoName
	}
	ctx.Resp, ctx.RespErr = queryAIReviewOverview(c.Request.Context(), args, scope)
}

// @Summary 获取 AI 审查洞察项目汇总列表
// @Description 按时间范围查询项目汇总，支持分页和排序
// @Tags stat
// @Accept json
// @Produce json
// @Param start_time query int true "开始时间，Unix 秒时间戳，包含"
// @Param end_time query int true "结束时间，Unix 秒时间戳，不包含，必须大于开始时间"
// @Param page query int false "页码" default(1) minimum(1)
// @Param page_size query int false "每页数量" default(20) minimum(1) maximum(100)
// @Param sort_by query string false "排序字段" default(pr_count) Enums(pr_count, inline_total, resolution_rate, up_down_ratio, total_tokens)
// @Param sort_order query string false "排序方向" default(desc) Enums(asc, desc)
// @Success 200 {object} AIReviewStatsProjectListResponse
// @Router /api/aslan/stat/v2/ai_review/project [get]
func GetAIReviewStatsProjects(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(AIReviewStatsListRequest)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.RespErr = queryAIReviewProjects(c.Request.Context(), args)
}

// @Summary 获取 AI 审查洞察代码库汇总列表
// @Description 按时间范围查询代码库汇总，支持项目筛选、分页和排序
// @Tags stat
// @Accept json
// @Produce json
// @Param start_time query int true "开始时间，Unix 秒时间戳，包含"
// @Param end_time query int true "结束时间，Unix 秒时间戳，不包含，必须大于开始时间"
// @Param project_name query string false "限定项目标识"
// @Param page query int false "页码" default(1) minimum(1)
// @Param page_size query int false "每页数量" default(20) minimum(1) maximum(100)
// @Param sort_by query string false "排序字段" default(pr_count) Enums(pr_count, inline_total, resolution_rate, up_down_ratio, total_tokens)
// @Param sort_order query string false "排序方向" default(desc) Enums(asc, desc)
// @Success 200 {object} AIReviewStatsRepoListResponse
// @Router /api/aslan/stat/v2/ai_review/repo [get]
func GetAIReviewStatsRepos(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(AIReviewStatsRepoListRequest)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	args.ProjectName = strings.TrimSpace(args.ProjectName)
	ctx.Resp, ctx.RespErr = queryAIReviewRepos(c.Request.Context(), args)
}

// @Summary 获取 AI 审查洞察代码库 PR 列表
// @Description 查询指定代码库的 PR 列表，支持项目筛选和分页
// @Tags stat
// @Accept json
// @Produce json
// @Param name path string true "仓库名称"
// @Param start_time query int true "开始时间，Unix 秒时间戳，包含"
// @Param end_time query int true "结束时间，Unix 秒时间戳，不包含，必须大于开始时间"
// @Param codehost_id query int true "代码源 ID，大于零"
// @Param repo_owner query string true "仓库 owner 或完整 GitLab namespace"
// @Param project_name query string false "限定项目标识"
// @Param page query int false "页码" default(1) minimum(1)
// @Param page_size query int false "每页数量" default(20) minimum(1) maximum(100)
// @Success 200 {object} AIReviewStatsPRListResponse
// @Router /api/aslan/stat/v2/ai_review/repo/{name} [get]
func GetAIReviewStatsRepoPRs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(AIReviewStatsRepoPRListRequest)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if strings.TrimSpace(args.RepoOwner) == "" || strings.TrimSpace(c.Param("name")) == "" {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("repo_owner and repository name must not be blank"))
		return
	}
	args.ProjectName = strings.TrimSpace(args.ProjectName)
	args.RepoOwner = strings.TrimSpace(args.RepoOwner)
	ctx.Resp, ctx.RespErr = queryAIReviewPRs(c.Request.Context(), args, strings.TrimSpace(c.Param("name")))
}
