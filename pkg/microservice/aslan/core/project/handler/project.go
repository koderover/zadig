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
	"fmt"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	projectservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/project/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type projectListArgs struct {
	IgnoreNoEnvs     bool     `json:"ignoreNoEnvs"     form:"ignoreNoEnvs"`
	IgnoreNoVersions bool     `json:"ignoreNoVersions" form:"ignoreNoVersions"`
	Verbosity        string   `json:"verbosity"        form:"verbosity"`
	Names            []string `json:"names"            form:"names"`
	PageSize         int64    `json:"page_size"        form:"page_size,default=20"`
	PageNum          int64    `json:"page_num"         form:"page_num,default=1"`
	Filter           string   `json:"filter"           form:"filter"`
	GroupName        string   `json:"group_name"       form:"group_name"`
}

type projectResp struct {
	Projects []string `json:"projects"`
	Total    int64    `json:"total"`
}

func ListProjects(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := &projectListArgs{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	groupName, err := url.QueryUnescape(args.GroupName)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	args.GroupName = groupName

	var authorizedProjectList []string

	if ctx.Resources.IsSystemAdmin {
		authorizedProjectList = []string{}
	} else {
		var found bool
		authorizedProjectList, found, err = internalhandler.ListAuthorizedProjects(ctx.UserID)
		if err != nil {
			ctx.RespErr = e.ErrInternalError.AddDesc(err.Error())
			return
		}

		if !found {
			ctx.Resp = &projectResp{
				Projects: []string{},
				Total:    0,
			}
			return
		}
	}

	ungrouped, err := strconv.ParseBool(c.Query("ungrouped"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("ungrouped is not bool"))
		return
	}

	ctx.Resp, ctx.RespErr = projectservice.ListProjects(
		&projectservice.ProjectListOptions{
			IgnoreNoEnvs:     args.IgnoreNoEnvs,
			IgnoreNoVersions: args.IgnoreNoVersions,
			Verbosity:        projectservice.QueryVerbosity(args.Verbosity),
			Names:            authorizedProjectList,
			PageSize:         args.PageSize,
			PageNum:          args.PageNum,
			Filter:           args.Filter,
			GroupName:        args.GroupName,
			Ungrouped:        ungrouped,
		},
		ctx.Logger,
	)
}

// @Summary Get Bussiness Directory
// @Description Get Bussiness Directory
// @Tags  	project
// @Accept 	json
// @Produce json
// @Success 200 		{array} 	projectservice.GroupDetail
// @Router /api/aslan/project/bizdir [get]
func GetBizDirProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.BusinessDirectory.View {
			ctx.UnAuthorized = true
			return
		}
	}

	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = projectservice.GetBizDirProject()
}

// @Summary Get Bussiness Directory Project Services
// @Description Get Bussiness Directory Project Services
// @Tags  	project
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Success 200 			{array} 	string
// @Router /api/aslan/project/bizdir/services [get]
func GetBizDirProjectServices(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.BusinessDirectory.View {
			ctx.UnAuthorized = true
			return
		}
	}

	req := new(searchBizDirByProjectReq)

	err = c.ShouldBindQuery(req)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid param, err: " + err.Error())
		return
	}

	if req.ProjectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid project name")
		return
	}

	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = projectservice.GetBizDirProjectServices(req.ProjectName, req.Labels)
}

type searchBizDirByProjectReq struct {
	ProjectName string   `json:"projectName" form:"projectName"`
	Labels      []string `json:"labels"       form:"labels"`
}

// @Summary Bussiness Directory Search By Project
// @Description Bussiness Directory Search By Project
// @Tags  	project
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Success 200 			{array} 	projectservice.GroupDetail
// @Router /api/aslan/project/bizdir/search/project [get]
func SearchBizDirByProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.BusinessDirectory.View {
			ctx.UnAuthorized = true
			return
		}
	}

	req := new(searchBizDirByProjectReq)

	err = c.ShouldBindQuery(req)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid param, err: " + err.Error())
		return
	}

	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = projectservice.SearchBizDirByProject(req.ProjectName, req.Labels)
}

type searchBizDirByServiceReq struct {
	ServiceName string   `json:"serviceName" form:"serviceName"`
	Labels      []string `json:"labels"       form:"labels"`
}

// @Summary Bussiness Directory Search By Service
// @Description Bussiness Directory Search By Service
// @Tags  	project
// @Accept 	json
// @Produce json
// @Param 	serviceName		query		string								true	"service name"
// @Success 200 			{array} 	projectservice.SearchBizDirByServiceGroup
// @Router /api/aslan/project/bizdir/search/service [get]
func SearchBizDirByService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.BusinessDirectory.View {
			ctx.UnAuthorized = true
			return
		}
	}

	req := new(searchBizDirByServiceReq)

	err = c.ShouldBindQuery(req)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid param, err: " + err.Error())
		return
	}

	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = projectservice.SearchBizDirByService(req.ServiceName, req.Labels)
}

type searchBizDirReq struct {
	ServiceName string   `json:"serviceName" form:"serviceName"`
	ProjectName string   `json:"projectName" form:"projectName"`
	Label       []string `json:"labels"       form:"labels"`
}

// @Summary Get Bussiness Directory Searvice Detail
// @Description Get Bussiness Directory Searvice Detail
// @Tags  	project
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Param 	serviceName		query		string								true	"service name"
// @Success 200 		{array} 	projectservice.GetBizDirServiceDetailResponse
// @Router /api/aslan/project/bizdir/service/detail [get]
func GetBizDirServiceDetail(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.BusinessDirectory.View {
			ctx.UnAuthorized = true
			return
		}
	}

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid project name")
		return
	}

	serviceName := c.Query("serviceName")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid service name")
		return
	}

	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = projectservice.GetBizDirServiceDetail(projectName, serviceName)
}
