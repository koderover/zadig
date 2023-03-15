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
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type getStatReqV2 struct {
	StartTime   int64  `form:"start_time"`
	EndTime     int64  `form:"end_time"`
	ProductName string `form:"product_name"`
}

func (req *getStatReqV2) Validate() error {
	if req.StartTime == 0 || req.EndTime == 0 {
		return e.ErrInvalidParam.AddDesc("start_time and end_time is empty")
	}

	if req.EndTime < req.StartTime {
		return e.ErrInvalidParam.AddDesc("invalid time range")
	}

	// currently, for efficiency consideration, 31 days is the max time range for the stats.
	// if we need longer time range, we should consider saving the stats in a table instead of
	// calculating it on the fly.
	if req.EndTime-req.StartTime > 60*60*24*31 {
		return e.ErrInvalidParam.AddDesc("time range should be less than 31 days")
	}

	return nil
}

func GetReleaseStatOpenAPI(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	//params validate
	args := new(getStatReqV2)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	if err := args.Validate(); err != nil {
		ctx.Err = err
		return
	}

	ctx.Resp, ctx.Err = service.GetReleaseStatOpenAPI(args.StartTime, args.EndTime, args.ProductName, ctx.Logger)
}
