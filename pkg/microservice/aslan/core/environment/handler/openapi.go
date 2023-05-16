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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func OpenAPIScaleWorkloads(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(service.OpenAPIScaleServiceReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Logger.Errorf("CreateProductTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, req); err != nil {
		ctx.Logger.Errorf("CreateProductTemplate json.Unmarshal err : %v", err)
	}

	internalhandler.InsertDetailedOperationLog(
		c, ctx.UserName+"(openAPI)",
		req.ProjectKey, setting.OperationSceneEnv,
		"伸缩",
		"环境-服务",
		fmt.Sprintf("环境名称:%s,%s:%s", req.EnvName, req.WorkloadType, req.WorkloadName),
		string(data), ctx.Logger, req.EnvName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}

	// input validation for OpenAPI
	err = req.Validate()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.OpenAPIScale(req, ctx.Logger)
}

func OpenAPIApplyYamlService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(service.OpenAPIApplyYamlServiceReq)

	if err := c.BindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		ctx.Logger.Errorf("CreateProductTemplate c.GetRawData() err : %v", err)
	}

	// input validation for OpenAPI
	err = req.Validate()
	if err != nil {
		ctx.Err = err
		return
	}

	allowedEnvs, found := internalhandler.GetResourcesInHeader(c)
	if found {
		allowedSet := sets.NewString(allowedEnvs...)
		if !allowedSet.Has(req.EnvName) {
			c.String(http.StatusForbidden, "not all input envs are allowed, allowed envs are %v", allowedEnvs)
			return
		}
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName+"(openAPI)", req.ProjectKey, setting.OperationSceneEnv, "更新", "环境", req.EnvName, string(data), ctx.Logger, req.EnvName)

	_, err = service.OpenAPIApplyYamlService(req, ctx.RequestID, ctx.Logger)

	ctx.Err = err
}
