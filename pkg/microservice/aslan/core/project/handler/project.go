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
	"github.com/gin-gonic/gin"

	projectservice "github.com/koderover/zadig/pkg/microservice/aslan/core/project/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type projectListArgs struct {
	IgnoreNoEnvs bool   `json:"ignoreNoEnvs"    form:"ignoreNoEnvs"`
	Verbosity    string `json:"verbosity"    form:"verbosity"`
}

func ListProjects(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &projectListArgs{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = projectservice.ListProjects(
		&projectservice.ProjectListOptions{IgnoreNoEnvs: args.IgnoreNoEnvs, Verbosity: projectservice.QueryVerbosity(args.Verbosity)},
		ctx.Logger,
	)
}
