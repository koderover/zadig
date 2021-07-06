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
	"strconv"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func PullNotify(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.PullNotify(ctx.Username, ctx.Logger)
}

type readNotificationsArgs struct {
	IDs []string `json:"ids"`
}

func ReadNotify(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(readNotificationsArgs)

	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid readnotificationsargs")
		return
	}
	ctx.Err = service.ReadNotify(ctx.Username, args.IDs, ctx.Logger)
}

type deleteNotificationsArgs struct {
	IDs []string `json:"ids"`
}

func DeleteNotifies(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(deleteNotificationsArgs)

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid args")
		return
	}
	ctx.Err = service.DeleteNotifies(ctx.Username, args.IDs, ctx.Logger)
}

func UpsertSubscription(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(commonmodels.Subscription)

	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid subscription args")
		return
	}
	ctx.Err = service.UpsertSubscription(ctx.Username, args, ctx.Logger)
}

func UpdateSubscribe(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(commonmodels.Subscription)

	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid subscription args")
		return
	}
	notifytype, err := strconv.Atoi(c.Param("type"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid notification type")
		return
	}
	ctx.Err = service.UpdateSubscribe(ctx.Username, notifytype, args, ctx.Logger)
}

func Unsubscribe(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	notifytype, err := strconv.Atoi(c.Param("type"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid notification type")
		return
	}
	ctx.Err = service.Unsubscribe(ctx.Username, notifytype, ctx.Logger)
}

func ListSubscriptions(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListSubscriptions(ctx.Username, ctx.Logger)
}
