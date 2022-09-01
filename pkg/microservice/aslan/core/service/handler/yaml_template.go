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
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	svcservice "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func LoadServiceFromYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(svcservice.LoadServiceFromYamlTemplateReq)

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName, req.ProjectName, "新增", "项目管理-服务", fmt.Sprintf("服务名称:%s", req.ServiceName), string(bs), ctx.Logger)

	ctx.Err = svcservice.LoadServiceFromYamlTemplate(ctx.UserName, req, false, ctx.Logger)
}

func ReloadServiceFromYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(svcservice.LoadServiceFromYamlTemplateReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName, req.ProjectName, "更新", "项目管理-服务", fmt.Sprintf("服务名称:%s", req.ServiceName), string(bs), ctx.Logger)

	ctx.Err = svcservice.ReloadServiceFromYamlTemplate(ctx.UserName, req, ctx.Logger)
}

func PreviewServiceYamlFromYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(svcservice.LoadServiceFromYamlTemplateReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	req.ProjectName = c.Query("projectName")
	ctx.Resp, ctx.Err = svcservice.PreviewServiceFromYamlTemplate(req, ctx.Logger)
}
