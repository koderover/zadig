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
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/service"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types/permission"
)

func GetProductNameByWorkspacePipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	pipeline, err := commonservice.GetPipelineInfo(ctx.User.ID, c.Query("pipelineName"), ctx.Logger)
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
	internalhandler.InsertOperationLog(c, ctx.Username, c.GetString("productName"), "清理", "单服务工作流-工作目录", c.Query("pipelineName"), permission.WorkflowUpdateUUID, "", ctx.Logger)
	name := c.Query("pipelineName")
	if name == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty pipeline name")
		return
	}
	ctx.Err = service.CleanWorkspace(ctx.Username, name, ctx.Logger)
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

	fileRealPath, err := service.GetWorkspaceFilePath(ctx.Username, name, file, ctx.Logger)
	if err != nil {
		c.JSON(e.ErrorMessage(err))
		c.Abort()
		return
	}
	c.File(fileRealPath)
}

func GetGithubRepoInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostIDStr := c.Param("codehostId")
	if codehostIDStr == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty codehost ID")
		return
	}

	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("cannot convert codehost id to string")
		return
	}

	repoName := c.Param("repoName")
	if repoName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty repo name")
		return
	}
	repoName, err = url.QueryUnescape(repoName)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("repoName decode error")
		return
	}

	branchName := c.Param("branchName")
	if branchName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty branch name")
		return
	}

	path := c.Query("path")

	ctx.Resp, ctx.Err = service.GetGithubRepoInfo(codehostID, repoName, branchName, path, ctx.Logger)
}

func GetGitlabRepoInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostIDStr := c.Param("codehostId")
	if codehostIDStr == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty codehost ID")
		return
	}

	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("cannot convert codehost id to int")
		return
	}

	repoName := c.Param("repoName")
	if repoName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty repo name")
		return
	}
	repoName, err = url.QueryUnescape(repoName)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("repoName decode error")
		return
	}

	branchName := c.Param("branchName")
	if branchName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty branch name")
		return
	}

	path := c.Query("path")
	owner := c.Query("repoOwner")

	repoInfo := fmt.Sprintf("%s/%s", owner, repoName)

	ctx.Resp, ctx.Err = service.GetGitlabRepoInfo(codehostID, repoInfo, branchName, path, ctx.Logger)
}

func GetGitRepoInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostIDStr := c.Param("codehostId")
	if codehostIDStr == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty codehost ID")
		return
	}

	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("cannot convert codehost id to int")
		return
	}

	repoName := c.Param("repoName")
	if repoName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty repo name")
		return
	}
	repoName, err = url.QueryUnescape(repoName)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("repoName decode error")
		return
	}

	branchName := c.Param("branchName")
	if branchName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty branch name")
		return
	}

	remoteName := c.Param("remoteName")
	if remoteName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty remote name")
		return
	}

	dir := c.Query("dir")

	ctx.Resp, ctx.Err = service.GetGitRepoInfo(codehostID, c.Query("repoOwner"), repoName, branchName, remoteName, dir, ctx.Logger)
}

func GetPublicGitRepoInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	url := c.Query("url")
	if url == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty url")
		return
	}
	dir := c.Query("dir")
	ctx.Resp, ctx.Err = service.GetPublicGitRepoInfo(url, dir, ctx.Logger)
}

func GetCodehubRepoInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostIDStr := c.Param("codehostId")
	if codehostIDStr == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty codehost ID")
		return
	}

	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("cannot convert codehost id to int")
		return
	}

	repoUUID := c.Param("repoUUID")
	if repoUUID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty repo uuid")
		return
	}

	branchName := c.Param("branchName")
	if branchName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty branch name")
		return
	}

	path := c.Query("path")

	ctx.Resp, ctx.Err = service.GetCodehubRepoInfo(codehostID, repoUUID, branchName, path, ctx.Logger)
}
