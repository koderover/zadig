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

// @Summary 列出tapd项目
// @Description 列出tapd项目
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 		path		string										true	"tapd id"
// @Success 200 	{array} 	tapd.Workspace
// @Router /api/aslan/system/tapd/{id}/projects [get]
func ListTapdProjects(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	ctx.Resp, ctx.RespErr = service.ListTapdProjects(id)
}

// @Summary 列出tapd项目迭代
// @Description 列出tapd项目迭代
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"tapd id"
// @Param 	projectID 	path		string										true	"项目ID"
// @Success 200 		{array} 	tapd.IterationInfo
// @Router /api/aslan/system/tapd/{id}/projects/{projectID}/iterations [get]
func ListTapdProjectIterations(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	projectID := c.Param("projectID")
	ctx.Resp, ctx.RespErr = service.ListTapdIterations(id, projectID)
}
