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

package login

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/user/core/service/login"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func LocalLogin(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &login.LoginArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = login.LocalLogin(args, ctx.Logger)
}

type LocalLogoutResp struct {
	EnableRedirect bool   `json:"enable_redirect"`
	RedirectURL    string `json:"redirect_url"`
}

func LocalLogout(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	shouldRedirect, redirectURL, _ := login.LocalLogout(ctx.UserID, ctx.Logger)
	// TODO: for now only oauth2 service need to actually logout, so we just do nothing when an error happen
	// this need to be fixed when there are more logout logic.
	ctx.Resp = &LocalLogoutResp{
		EnableRedirect: shouldRedirect,
		RedirectURL:    redirectURL,
	}
}
