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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type getStatReqV2 struct {
	StartTime   int64  `form:"startTime"`
	EndTime     int64  `form:"endTime"`
	ProjectName string `form:"projectKey"`
}

func (req *getStatReqV2) Validate() error {
	if req.StartTime == 0 || req.EndTime == 0 {
		return e.ErrInvalidParam.AddDesc("starTime and endTime is empty")
	}

	if req.EndTime < req.StartTime {
		return e.ErrInvalidParam.AddDesc("invalid time range")
	}

	// currently, for efficiency consideration, 31 days is the max time range for the stats.
	// if we need longer time range, we should consider saving the stats in a table instead of
	// calculating it on the fly.
	if req.EndTime-req.StartTime > 60*60*24*365 {
		return e.ErrInvalidParam.AddDesc("time range should be less than 365 days")
	}

	return nil
}

func GetReleaseStatOpenAPI(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	//params validate
	args := new(getStatReqV2)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	if err := args.Validate(); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.GetReleaseStatOpenAPI(args.StartTime, args.EndTime, args.ProjectName, ctx.Logger)
}

type getRollbackStatDetail struct {
	StartTime   int64  `form:"startTime"`
	EndTime     int64  `form:"endTime"`
	ProjectKey  string `form:"projectKey"`
	EnvName     string `form:"envName"`
	ServiceName string `form:"serviceName"`
	PageNum     int    `form:"pageNum"`
	PageSize    int    `form:"pageSize"`
}

func (req *getRollbackStatDetail) Validate() error {
	if req.StartTime == 0 || req.EndTime == 0 {
		return e.ErrInvalidParam.AddDesc("starTime and endTime is empty")
	}

	if req.EndTime < req.StartTime {
		return e.ErrInvalidParam.AddDesc("invalid time range")
	}

	// currently, for efficiency consideration, 31 days is the max time range for the stats.
	// if we need longer time range, we should consider saving the stats in a table instead of
	// calculating it on the fly.
	if req.EndTime-req.StartTime > 60*60*24*365 {
		return e.ErrInvalidParam.AddDesc("time range should be less than 365 days")
	}

	if req.PageSize == 0 {
		return e.ErrInvalidParam.AddDesc("pageSize is empty")
	}

	return nil
}

// @Summary 获取回滚统计详情(OpenAPI)
// @Description 返回值中_sae_app和_service根据环境类型不同，可能为null
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	projectKey		query		string							false	"项目标识"
// @Param 	envName			query		string							false	"环境名称"
// @Param 	serviceName		query		string							false	"服务名称"
// @Param 	startTime 		query		int								true	"开始时间，格式为时间戳"
// @Param 	endTime 		query		int								true	"结束时间，格式为时间戳"
// @Param 	pageNum 		query		int								true	"当前页码"
// @Param 	pageSize 		query		int								true	"分页大小"
// @Success 200 			{object} 	service.GetRollbackStatDetailResponse
// @Router /openapi/statistics/v2/rollback/detail [get]
func GetRollbackStatDetailOpenAPI(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	//params validate
	args := new(getRollbackStatDetail)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	if err := args.Validate(); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.GetRollbackStatDetail(ctx, args.ProjectKey, args.EnvName, args.ServiceName, args.StartTime, args.EndTime, args.PageNum, args.PageSize)
}
