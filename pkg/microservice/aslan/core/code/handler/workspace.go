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

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/service"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func GetProductNameByWorkspacePipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	pipeline, err := commonservice.GetPipelineInfo(c.Query("pipelineName"), ctx.Logger)
	if err != nil {
		log.Errorf("GetProductNameByWorkspacePipeline err : %v", err)
		return
	}
	c.Set("productName", pipeline.ProductName)
	c.Next()
}

func CleanWorkspace(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.UserName, c.GetString("productName"), "清理", "单服务工作流-工作目录", c.Query("pipelineName"), "", types.RequestBodyTypeJSON, ctx.Logger)
	name := c.Query("pipelineName")
	if name == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty pipeline name")
		return
	}
	ctx.RespErr = service.CleanWorkspace(ctx.UserName, name, ctx.Logger)
}

func GetWorkspaceFile(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	name := c.Query("pipelineName")
	if name == "" {
		c.JSON(e.ErrorMessage(e.ErrInvalidParam.AddDesc("empty pipeline name")))
		c.Abort()
		return
	}

	file := c.Query("file")
	if file == "" {
		c.JSON(e.ErrorMessage(e.ErrInvalidParam.AddDesc("empty file")))
		c.Abort()
		return
	}

	fileRealPath, err := service.GetWorkspaceFilePath(ctx.UserName, name, file, ctx.Logger)
	if err != nil {
		c.JSON(e.ErrorMessage(err))
		c.Abort()
		return
	}
	c.File(fileRealPath)
}

func GetGitRepoInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostIDStr := c.Param("codehostId")
	if codehostIDStr == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty codehost ID")
		return
	}

	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cannot convert codehost id to int")
		return
	}

	repoName := c.Query("repoName")
	if repoName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty repo name")
		return
	}

	branchName := c.Query("branchName")
	if branchName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty branch name")
		return
	}

	remoteName := c.Query("remoteName")
	if remoteName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty remote name")
		return
	}

	dir := c.Query("dir")

	namespace := c.Query("repoNamespace")
	if namespace == "" {
		namespace = c.Query("repoOwner")
	}
	ctx.Resp, ctx.RespErr = service.GetGitRepoInfo(codehostID, c.Query("repoOwner"), namespace, repoName, branchName, remoteName, dir, ctx.Logger)
}

type repoInfo struct {
	CodeHostID int    `json:"codehost_id" form:"codehost_id"`
	Owner      string `json:"owner"       form:"owner"`
	Namespace  string `json:"namespace"   form:"namespace"`
	Repo       string `json:"repo"        form:"repo"`
	Path       string `json:"path"        form:"path"`
	Branch     string `json:"branch"      form:"branch"`
	RepoLink   string `json:"repoLink"    form:"repoLink"`
}

func GetRepoTree(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	info := &repoInfo{}
	if err := c.ShouldBindQuery(info); err != nil {
		ctx.RespErr = err
		return
	}

	if info.RepoLink != "" {
		ctx.Resp, ctx.RespErr = service.GetPublicRepoTree(info.RepoLink, info.Path, ctx.Logger)
		return
	}

	owner := info.Owner
	if len(info.Namespace) > 0 {
		owner = info.Namespace
	}

	ctx.Resp, ctx.RespErr = service.GetRepoTree(info.CodeHostID, owner, info.Repo, info.Path, info.Branch, ctx.Logger)
}

func GetContents(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostIDStr := c.Param("codehostId")
	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cannot convert codehost id to int")
		return
	}

	repoName := c.Query("repoName")
	if repoName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("repoName cannot be empty")
		return
	}
	branchName := c.Query("branchName")
	path := c.Query("path")
	isDir, err := strconv.ParseBool(c.Query("isDir"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalidParam isDir")
		return
	}
	repoOwner := c.Query("repoOwner")

	ctx.Resp, ctx.RespErr = service.GetContents(codehostID, repoOwner, repoName, path, branchName, isDir, ctx.Logger)
}
