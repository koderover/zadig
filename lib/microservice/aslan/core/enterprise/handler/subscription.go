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

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	enterpriseservice "github.com/koderover/zadig/lib/microservice/aslan/core/enterprise/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/lib/tool/errors"
)

func UpsertSubscription(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	args := new(commonmodels.Subscription)

	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid subscription args")
		return
	}
	ctx.Err = enterpriseservice.UpsertSubscription(ctx.Username, args, ctx.Logger)
}

func UpdateSubscribe(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
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
	ctx.Err = enterpriseservice.UpdateSubscribe(ctx.Username, notifytype, args, ctx.Logger)
}

func Unsubscribe(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	notifytype, err := strconv.Atoi(c.Param("type"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid notification type")
		return
	}
	ctx.Err = enterpriseservice.Unsubscribe(ctx.Username, notifytype, ctx.Logger)
}

func ListSubscriptions(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = enterpriseservice.ListSubscriptions(ctx.Username, ctx.Logger)
}
