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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type GetDeployStatArgs struct {
	StartDate int64 `json:"startDate"      form:"startDate,default:0"`
	EndDate   int64 `json:"endDate"        form:"endDate,default:0"`
}

func GetDeployStat(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(GetDeployStatArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = service.GetDeployDailyTotalAndSuccess(&models.DeployStatOption{
		StartDate: args.StartDate,
		EndDate:   args.EndDate,
	}, ctx.Logger)
}

func GetDeployStatsOpenAPI(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(GetDeployStatArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = service.GetDeployStats(&models.DeployStatOption{
		StartDate: args.StartDate,
		EndDate:   args.EndDate,
	}, ctx.Logger)
}
