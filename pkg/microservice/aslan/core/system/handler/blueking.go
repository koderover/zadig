/*
Copyright 2024 The KodeRover Authors.

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
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/blueking"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type ListBluekingBusinessReq struct {
	ID      string `json:"id"   form:"id"`
	Page    int64  `json:"page" form:"page"`
	PerPage int64  `json:"per_page" form:"per_page"`
}

func ListBluekingBusiness(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	// TODO: PUT IT BACK
	//if !ctx.Resources.IsSystemAdmin {
	//	ctx.UnAuthorized = true
	//	return
	//}
	args := new(ListBluekingBusinessReq)
	err = c.ShouldBindQuery(&args)

	if args.ID == "" {
		ctx.Err = e.ErrInvalidParam
		return
	}

	ctx.Resp, ctx.Err = service.ListBlueKingBusiness(args.ID, args.Page, args.PerPage, ctx.Logger)
}

type ListBlueKingExecutionPlanReq struct {
	ToolID     string `json:"id"          form:"id"`
	BusinessID int64  `json:"business_id" form:"business_id"`
	Name       string `json:"name"        form:"name"`
	Page       int64  `json:"page"        form:"page"`
	PerPage    int64  `json:"per_page"    form:"per_page"`
}

func ListBlueKingExecutionPlan(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	// TODO: PUT IT BACK
	//if !ctx.Resources.IsSystemAdmin {
	//	ctx.UnAuthorized = true
	//	return
	//}

	args := new(ListBlueKingExecutionPlanReq)
	err = c.ShouldBindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = service.ListBlueKingExecutionPlan(args.ToolID, args.BusinessID, args.Name, args.Page, args.PerPage, ctx.Logger)
}

func GetBlueKingExecutionPlanDetail(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	// TODO: PUT IT BACK
	//if !ctx.Resources.IsSystemAdmin {
	//	ctx.UnAuthorized = true
	//	return
	//}

	args := new(ListBlueKingExecutionPlanReq)
	err = c.ShouldBindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	planIDStr := c.Param("id")
	planID, err := strconv.ParseInt(planIDStr, 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
	}

	ctx.Resp, ctx.Err = service.GetBlueKingExecutionPlanDetail(args.ToolID, args.BusinessID, planID, ctx.Logger)
}

type BKTopologyResp struct {
	Topology []*blueking.TopologyNode `json:"topology"`
}

func GetBlueKingBusinessTopology(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	// TODO: PUT IT BACK
	//if !ctx.Resources.IsSystemAdmin {
	//	ctx.UnAuthorized = true
	//	return
	//}

	args := new(ListBlueKingExecutionPlanReq)
	err = c.ShouldBindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	resp, err := service.GetBlueKingBusinessTopology(args.ToolID, args.BusinessID, ctx.Logger)
	if err != nil {
		ctx.Err = e.ErrInternalError.AddDesc(err.Error())
		return
	}

	ctx.Resp = &BKTopologyResp{Topology: resp}
}

type ListServerByBlueKingTopologyNodeReq struct {
	ToolID     string `json:"id"          form:"id"`
	BusinessID int64  `json:"business_id" form:"business_id"`
	InstanceID int64  `json:"instance_id" form:"instance_id"`
	ObjectID   string `json:"object_id"   form:"object_id"`
}

func ListServerByBlueKingTopologyNode(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	// TODO: PUT IT BACK
	//if !ctx.Resources.IsSystemAdmin {
	//	ctx.UnAuthorized = true
	//	return
	//}

	args := new(ListServerByBlueKingTopologyNodeReq)
	err = c.ShouldBindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = service.ListServerByBlueKingTopologyNode(args.ToolID, args.BusinessID, args.InstanceID, args.ObjectID, ctx.Logger)
}

func ListServerByBlueKingBusiness(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	// TODO: PUT IT BACK
	//if !ctx.Resources.IsSystemAdmin {
	//	ctx.UnAuthorized = true
	//	return
	//}

	args := new(ListBlueKingExecutionPlanReq)
	err = c.ShouldBindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = service.ListServerByBlueKingBusiness(args.ToolID, args.BusinessID, ctx.Logger)
}
