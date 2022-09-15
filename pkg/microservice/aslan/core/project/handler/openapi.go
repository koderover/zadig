/*
Copyright 2022 The KodeRover Authors.

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
	"io/ioutil"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/project/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func OpenAPICreateProductTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.OpenAPICreateProductReq)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateProductTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateProductTemplate json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, "openAPI: "+ctx.UserName, args.ProjectName, "新增", "项目管理-项目", args.ProjectName, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}

	// input validation for OpenAPI

	// if the projectKey does not meet the requirements, we simply return an error
	match, err := regexp.MatchString(setting.ProjectKeyRegEx, args.ProjectKey)
	if err != nil || !match {
		ctx.Err = e.ErrInternalError.AddErr(err).AddDesc(`project key should match regex: ^[a-z-\\d]+$`)
		return
	}

	// finally, we create the project
	ctx.Err = service.CreateProjectOpenAPI(ctx.UserID, ctx.UserName, args, ctx.Logger)
}
