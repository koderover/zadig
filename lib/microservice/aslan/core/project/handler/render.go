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

	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/lib/tool/errors"
)

//func ListTmplRenderKeys(c *gin.Context) {
//	ctx := internalhandler.NewContext(c)
//	defer func() { internalhandler.JsonResponse(c, ctx) }()
//
//	ctx.Resp, ctx.Err = projectservice.ListTmplRenderKeys(c.Query("productTmpl"), ctx.Logger)
//}
//
//func ListRenderSets(c *gin.Context) {
//	ctx := internalhandler.NewContext(c)
//	defer func() { internalhandler.JsonResponse(c, ctx) }()
//
//	ctx.Resp, ctx.Err = projectservice.ListRenderSets(c.Query("productTmpl"), ctx.Logger)
//}
//
//func GetRenderSet(c *gin.Context) {
//	ctx := internalhandler.NewContext(c)
//	defer func() { internalhandler.JsonResponse(c, ctx) }()
//	//默认取revision最大的渲染集
//	ctx.Resp, ctx.Err = projectservice.GetRenderSet(c.Param("name"), 0, ctx.Logger)
//}

func GetRenderSetInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	//默认取revision最大的渲染集
	revision, err := strconv.ParseInt(c.Param("revision"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.Err = service.GetRenderSetInfo(c.Param("name"), revision)
}

//func ValidateRenderSet(c *gin.Context) {
//	ctx := internalhandler.NewContext(c)
//	defer func() { internalhandler.JsonResponse(c, ctx) }()
//	_, ctx.Err = projectservice.ValidateRenderSet(c.Param("productName"), c.Param("renderName"), "", ctx.Logger)
//}
//
//func RelateRender(c *gin.Context) {
//	ctx := internalhandler.NewContext(c)
//	defer func() { internalhandler.JsonResponse(c, ctx) }()
//	ctx.Err = projectservice.RelateRender(c.Param("productName"), c.Param("renderName"), ctx.Logger)
//}
//
//func CreateRenderSet(c *gin.Context) {
//	ctx := internalhandler.NewContext(c)
//	defer func() { internalhandler.JsonResponse(c, ctx) }()
//
//	args := new(models.RenderSet)
//	ctx.Err = c.BindJSON(args)
//	if ctx.Err != nil {
//		return
//	}
//	args.UpdateBy = ctx.Username
//	ctx.Err = projectservice.CreateRenderSet(args, ctx.Logger)
//}

func UpdateRenderSet(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(models.RenderSet)
	ctx.Err = c.BindJSON(args)
	if ctx.Err != nil {
		return
	}
	args.UpdateBy = ctx.Username
	ctx.Err = service.UpdateRenderSet(args, ctx.Logger)
}
