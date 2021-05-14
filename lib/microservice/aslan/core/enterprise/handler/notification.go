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

	enterpriseservice "github.com/koderover/zadig/lib/microservice/aslan/core/enterprise/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/lib/tool/errors"
)

type deleteNotificationsArgs struct {
	IDs []string `json:"ids"`
}

func DeleteNotifies(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(deleteNotificationsArgs)

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid deletenotificationsargs")
		return
	}
	ctx.Err = enterpriseservice.DeleteNotifies(ctx.Username, args.IDs, ctx.Logger)
}

func PullNotify(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = enterpriseservice.PullNotify(ctx.Username, ctx.Logger)
}

type readNotificationsArgs struct {
	IDs []string `json:"ids"`
}

func ReadNotify(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	args := new(readNotificationsArgs)

	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid readnotificationsargs")
		return
	}
	ctx.Err = enterpriseservice.ReadNotify(ctx.Username, args.IDs, ctx.Logger)
}
