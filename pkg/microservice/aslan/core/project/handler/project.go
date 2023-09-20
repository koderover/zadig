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
	"strconv"

	"github.com/gin-gonic/gin"

	projectservice "github.com/koderover/zadig/pkg/microservice/aslan/core/project/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
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

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := &projectListArgs{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	var authorizedProjectList []string

	if ctx.Resources.IsSystemAdmin {
		authorizedProjectList = []string{}
	} else {
		var found bool
		authorizedProjectList, found, err = internalhandler.ListAuthorizedProjects(ctx.UserID)
		if err != nil {
			ctx.Err = e.ErrInternalError.AddDesc(err.Error())
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
		ctx.Err = e.ErrInvalidParam.AddErr(fmt.Errorf("ungrouped is not bool"))
		return
	}

	ctx.Resp, ctx.Err = projectservice.ListProjects(
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
