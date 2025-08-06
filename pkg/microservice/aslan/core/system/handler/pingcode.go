/*
 * Copyright 2025 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

// @Summary 列出pingcode项目
// @Description 列出pingcode项目
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 		path		string										true	"pingcode id"
// @Success 200 	{array} 	pingcode.ProjectInfo
// @Router /api/aslan/system/pingcode/{id}/projects [get]
func ListPingCodeProjects(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	ctx.Resp, ctx.RespErr = service.ListPingCodeProjects(id)
}

// @Summary 列出pingcode工作项可用状态
// @Description 列出pingcode工作项可用状态
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"pingcode id"
// @Param 	projectID 	path		string										true	"项目ID"
// @Param 	fromStateID query		string										true	"当前状态ID"
// @Success 200 		{array} 	pingcode.WorkItemState
// @Router /api/aslan/system/pingcode/{id}/projects/{projectID}/workitem/status [get]
func ListPingCodeProjectStatus(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	projectID := c.Param("projectID")
	fromStateID := c.Query("fromStateID")
	ctx.Resp, ctx.RespErr = service.ListPingCodeWorkItemStates(id, projectID, fromStateID)
}

// @Summary 列出pingcode项目看板
// @Description 列出pingcode项目看板
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"pingcode id"
// @Param 	projectID 	path		string										true	"项目ID"
// @Success 200 		{array} 	pingcode.BoardInfo
// @Router /api/aslan/system/pingcode/{id}/projects/{projectID}/boards [get]
func ListPingCodeProjectBoards(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	projectID := c.Param("projectID")
	ctx.Resp, ctx.RespErr = service.ListPingCodeBoards(id, projectID)
}

// @Summary 列出pingcode项目迭代
// @Description 列出pingcode项目迭代
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"pingcode id"
// @Param 	projectID 	path		string										true	"项目ID"
// @Success 200 		{array} 	pingcode.SprintInfo
// @Router /api/aslan/system/pingcode/{id}/projects/{projectID}/sprints [get]
func ListPingCodeProjectSprints(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	projectID := c.Param("projectID")
	ctx.Resp, ctx.RespErr = service.ListPingCodeSprints(id, projectID)
}

// @Summary 列出pingcode项目工作项类型
// @Description 列出pingcode项目工作项类型
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"pingcode id"
// @Param 	projectID 	path		string										true	"项目ID"
// @Success 200 		{array} 	pingcode.WorkItemType
// @Router /api/aslan/system/pingcode/{id}/projects/{projectID}/workitem_types [get]
func ListPingCodeProjectWorkItemTypes(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	projectID := c.Param("projectID")
	ctx.Resp, ctx.RespErr = service.ListPingCodeWorkItemTypes(id, projectID)
}

// @Summary 列出pingcode项目工作项
// @Description 列出pingcode项目工作项
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"pingcode id"
// @Param 	projectID 	path		string										true	"项目ID"
// @Param 	stateIDs    query		string										false	"状态ID数组"
// @Param 	sprintIDs   query		string										false	"迭代ID数组"
// @Param 	boardIDs    query		string										false	"看板ID数组"
// @Param 	keywords    query		string										false	"工作项编号或工作项标题关键字"
// @Success 200 		{array} 	pingcode.WorkItem
// @Router /api/aslan/system/pingcode/{id}/projects/{projectID}/workitems [get]
func ListPingCodeProjectWorkItems(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	projectID := c.Param("projectID")
	keywords := c.Query("keywords")
	stateIDs := c.QueryArray("stateIDs")
	sprintIDs := c.QueryArray("sprintIDs")
	boardIDs := c.QueryArray("boardIDs")
	typeIDs := c.QueryArray("typeIDs")
	ctx.Resp, ctx.RespErr = service.ListPingCodeWorkItems(id, projectID, stateIDs, sprintIDs, boardIDs, typeIDs, keywords)
}
