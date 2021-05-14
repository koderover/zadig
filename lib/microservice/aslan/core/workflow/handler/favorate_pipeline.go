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
	"bytes"
	"encoding/json"
	"io/ioutil"

	"github.com/gin-gonic/gin"

	"github.com/qiniu/x/log.v7"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/lib/tool/errors"
)

func DeleteFavoritePipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	productName := c.Param("productName")
	workflowName := c.Param("name")
	workflowType := c.Param("type")
	if workflowName == "" || workflowType == "" || productName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName or name or type can't be empty!")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.Username, productName, "删除", "收藏工作流", workflowName, "", "", ctx.Logger)
	ctx.Err = workflow.DeleteFavoritePipeline(&commonrepo.FavoriteArgs{UserID: ctx.User.ID, ProductName: productName, Type: workflowType, Name: workflowName})
}

func CreateFavoritePipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.Favorite)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateFavoritePipeline c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateFavoritePipeline json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductName, "新增", "收藏工作流", args.Name, "", string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid Favorite json args")
		return
	}
	args.UserID = ctx.User.ID

	ctx.Err = workflow.CreateFavoritePipeline(args, ctx.Logger)
}
