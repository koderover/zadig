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
	"github.com/gin-gonic/gin"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	codehost := router.Group("codehost")
	{
		codehost.GET("", GetCodeHostList)
		codehost.GET("/:codehostId/namespaces", CodeHostGetNamespaceList)
		codehost.GET("/:codehostId/projects", CodeHostGetProjectsList)
		codehost.GET("/:codehostId/branches", CodeHostGetBranchList)
		codehost.GET("/:codehostId/tags", CodeHostGetTagList)
		codehost.GET("/:codehostId/prs", CodeHostGetPRList)
		codehost.GET("/:codehostId/commits", CodeHostGetCommits)
		codehost.PUT("/infos", ListRepoInfos)
		codehost.POST("/:codehostId/regular/check", MatchRegularList)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline workspace 管理接口
	// ---------------------------------------------------------------------------------------
	workspace := router.Group("workspace")
	{
		workspace.DELETE("", GetProductNameByWorkspacePipeline, CleanWorkspace)
		workspace.GET("/file", GetWorkspaceFile)
		workspace.GET("/git/:codehostId", GetGitRepoInfo)

		workspace.GET("/tree", GetRepoTree)
		workspace.GET("/getcontents/:codehostId", GetContents)
	}

}
