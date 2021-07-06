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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

const (
	reqParamTarget = "target"
)

// DryRunFlag indicates whether a run is a dry run or not.
// If it is a dry run, the relevant API is supposed to be no-op except logging.
type DryRunFlag struct {
	DryRun bool `json:"dryrun"`
}

func UpdateStrategy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(models.CapacityStrategy)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	if err := service.UpdateSysCapStrategy(args); err != nil {
		ctx.Err = err
	}
}

func GarbageCollection(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	flag := new(DryRunFlag)
	if err := c.BindJSON(flag); err != nil {
		ctx.Err = err
		return
	}
	if err := service.HandleSystemGC(flag.DryRun); err != nil {
		ctx.Err = err
	}
}

func CleanCache(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err := service.CleanCache(); err != nil {
		ctx.Err = err
	}
}

func GetStrategy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	target := c.Param(reqParamTarget)
	resp, err := service.GetCapacityStrategy(models.CapacityTarget(target))
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp = resp
}
