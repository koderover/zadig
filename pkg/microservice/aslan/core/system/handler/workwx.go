/*
 * Copyright 2024 The KodeRover Authors.
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
	"encoding/json"
	"encoding/xml"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type getWorkWXDepartmentReq struct {
	DepartmentID int `json:"department_id" form:"department_id"`
}

func GetWorkWxDepartment(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(getWorkWXDepartmentReq)
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	appID := c.Param("id")

	ctx.Resp, ctx.Err = service.GetWorkWxDepartment(appID, req.DepartmentID, ctx.Logger)
}

type getWorkWXUsersReq struct {
	DepartmentID int `json:"department_id" form:"department_id"`
}

func GetWorkWxUsers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(getWorkWXUsersReq)
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	appID := c.Param("id")

	ctx.Resp, ctx.Err = service.GetWorkWxUsers(appID, req.DepartmentID, ctx.Logger)
}

func ValidateWorkWXCallback(c *gin.Context) {
	var header interface{}
	var body interface{}
	var query interface{}

	c.ShouldBindHeader(&header)
	c.ShouldBindQuery(&query)
	c.ShouldBindXML(&body)

	headerStr, _ := json.Marshal(header)
	fmt.Println("header is:", string(headerStr))

	queryStr, _ := json.Marshal(query)
	fmt.Println("query is:", string(queryStr))

	bodyStr, _ := xml.Marshal(body)
	fmt.Println("body is:", string(bodyStr))
}
