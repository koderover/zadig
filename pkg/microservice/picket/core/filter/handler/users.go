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

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/picket/core/filter/service"
	"github.com/koderover/zadig/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func DeleteUser(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	uid := c.Param("id")
	ctx.Resp, ctx.Err = service.DeleteUser(uid, c.Request.Header, c.Request.URL.Query(), ctx.Logger)
}

func SearchUsers(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	// TODO: Authorization leak
	// comment: this API should only be used when the use's full information is required, including contact info
	// however this is currently used in multiple situation, thus having serious security leaks.
	//if !ctx.Resources.IsSystemAdmin {
	//	ctx.UnAuthorized = true
	//	return
	//}

	req := &user.SearchArgs{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Logger.Errorf("bindjson err :%s", err)
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.SearchUsers(c.Request.Header, c.Request.URL.Query(), req, ctx.Logger)
}
