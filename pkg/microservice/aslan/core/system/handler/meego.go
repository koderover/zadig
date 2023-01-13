/*
 * Copyright 2022 The KodeRover Authors.
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
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func GetMeegoProjects(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetMeegoProjects()
}

func GetWorkItemTypeList(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectID := c.Param("projectID")

	ctx.Resp, ctx.Err = service.GetWorkItemTypeList(projectID)
}

func ListMeegoWorkItems(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectID := c.Param("projectID")
	typeKey := c.Query("type_key")
	pageNumStr := c.Query("page_num")
	pageNum := 0
	if pageNumStr != "" {
		pageNumber, err := strconv.Atoi(pageNumStr)
		if err != nil {
			ctx.Err = errors.New("invalid page_num")
			return
		}
		pageNum = pageNumber
	}
	pageSizeStr := c.Query("page_size")
	pageSize := 0
	if pageSizeStr != "" {
		pageSizeConv, err := strconv.Atoi(pageSizeStr)
		if err != nil {
			ctx.Err = errors.New("invalid page_size")
			return
		}
		pageSize = pageSizeConv
	}
	if typeKey == "" {
		ctx.Err = errors.New("type_key cannot be empty")
		return
	}

	nameQuery := c.Query("item_name")

	ctx.Resp, ctx.Err = service.ListMeegoWorkItems(projectID, typeKey, nameQuery, pageNum, pageSize)
}

func ListAvailableWorkItemTransitions(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectID := c.Param("projectID")
	workItemIDStr := c.Param("workItemID")
	workItemID, err := strconv.Atoi(workItemIDStr)
	if err != nil {
		ctx.Err = errors.New("invalid work item id")
		return
	}
	workItemTypeKey := c.Query("type_key")

	ctx.Resp, ctx.Err = service.ListAvailableWorkItemTransitions(projectID, workItemTypeKey, workItemID)
}
