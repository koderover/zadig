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

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func DeleteJira(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Err = service.DeleteJira(ctx.Logger)
}

func GetJira(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.Err = e.ErrInvalidParam
		return
	}
	ctx.Resp, ctx.Err = service.GeJira(encryptedKey, ctx.Logger)
}

func GetJiraInternal(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service.GeJiraInternal(ctx.Logger)
}

func CreateJira(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.Jira)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.CreateJira(req, ctx.Logger)
}

func UpdateJira(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.Jira)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.UpdateJira(req, ctx.Logger)
}
