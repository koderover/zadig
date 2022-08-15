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
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func InitTestStat(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = service.InitTestStat(ctx.Logger)
}

func GetTestAverageMeasure(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	//params validate
	args := new(getStatReq)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.Err = service.GetTestAverageMeasure(args.StartDate, args.EndDate, args.ProductNames, ctx.Logger)
}

func GetTestCaseMeasure(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	//params validate
	args := new(getStatReq)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.Err = service.GetTestCaseMeasure(args.StartDate, args.EndDate, args.ProductNames, ctx.Logger)
}

func GetTestDeliveryDeployMeasure(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	//params validate
	args := new(getStatReq)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.Err = service.GetTestDeliveryDeployMeasure(args.StartDate, args.EndDate, args.ProductNames, ctx.Logger)
}

func GetTestHealthMeasure(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	//params validate
	args := new(getStatReq)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.Err = service.GetTestHealthMeasure(args.StartDate, args.EndDate, args.ProductNames, ctx.Logger)
}

func GetTestTrendMeasure(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	//params validate
	args := new(getStatReq)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.Err = service.GetTestTrendMeasure(args.StartDate, args.EndDate, args.ProductNames, ctx.Logger)
}

//func GetTestTrendOpenAPI(c *gin.Context) {
//	ctx := internalhandler.NewContext(c)
//	defer func() { internalhandler.JSONResponse(c, ctx) }()
//
//	//params validate
//	args := new(getStatReq)
//	if err := c.BindJSON(args); err != nil {
//		ctx.Err = e.ErrInvalidParam.AddErr(err)
//		return
//	}
//
//	resp, err := service.GetTestTrendMeasure(args.StartDate, args.EndDate, args.ProductNames, ctx.Logger)
//	if err != nil {
//		ctx.Err = err
//		return
//	}
//
//}
