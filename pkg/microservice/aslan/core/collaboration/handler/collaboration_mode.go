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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/collaboration"
	"github.com/koderover/zadig/pkg/microservice/user/core/service/user"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetCollaborationMode(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can not be empty")
		return
	}
	ctx.Resp, ctx.Err = collaboration.GetCollaborationModes([]string{projectName}, ctx.Logger)
}

func CreateCollaborationMode(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.CollaborationMode)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateCollaborationMode c.GetRawData() err: %s", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateCollaborationMode json.Unmarshal err: %s", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	detail, err := generateCollaborationDetailLog(args, ctx.Logger)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	internalhandler.InsertOperationLog(c, ctx.Account, args.ProjectName, "新增", "协作模式", detail, string(data), ctx.Logger)

	ctx.Err = service.CreateCollaborationMode(ctx.Account, args, ctx.Logger)
}

func UpdateCollaborationMode(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.CollaborationMode)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateCollaborationMode c.GetRawData() err: %s", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateCollaborationMode json.Unmarshal err: %s", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	detail, err := generateCollaborationDetailLog(args, ctx.Logger)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	internalhandler.InsertOperationLog(c, ctx.Account, args.ProjectName, "更新", "协作模式", detail, string(data), ctx.Logger)

	ctx.Err = service.UpdateCollaborationMode(ctx.Account, args, ctx.Logger)
}

func DeleteCollaborationMode(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can not be empty")
		return
	}
	name := c.Param("name")
	internalhandler.InsertOperationLog(c, ctx.Account, projectName, "删除", "协作模式", name, "", ctx.Logger)

	ctx.Err = service.DeleteCollaborationMode(ctx.Account, projectName, name, ctx.Logger)
}

func generateCollaborationDetailLog(args *commonmodels.CollaborationMode, logger *zap.SugaredLogger) (string, error) {
	collaborationWorkflowVerbTranslateMap := map[string]string{
		"get_workflow":   "查看",
		"edit_workflow":  "编辑",
		"run_workflow":   "执行",
		"debug_workflow": "调试",
	}

	collaborationProductVerbTranslateMap := map[string]string{
		"get_environment":    "查看",
		"config_environment": "配置",
		"manage_environment": "管理服务实例",
		"debug_pod":          "服务调试",
	}

	collaborationTypeTranslateMap := map[string]string{
		"new":   "独享",
		"share": "共享",
	}

	detail := "用户名称："
	userResp, err := user.SearchUsersByUIDs(args.Members, logger)
	if err != nil {
		return "", fmt.Errorf("failed to search users by uids(%v), err: %v", args.Members, err)
	}
	for _, user := range userResp.Users {
		detail += user.Name + "，"
	}
	detail = strings.Trim(detail, "，")
	detail += "\n"

	for _, workflow := range args.Workflows {
		detail += "工作流名称：" + workflow.Name + "，类型：" + collaborationTypeTranslateMap[string(workflow.CollaborationType)] + "，权限："
		for _, verb := range workflow.Verbs {
			detail += collaborationWorkflowVerbTranslateMap[verb] + "，"
		}
		detail = strings.TrimSuffix(detail, "，")
		detail += "；"
	}
	detail += "\n"

	for _, env := range args.Products {
		detail += "环境名称：" + env.Name + "，类型：" + collaborationTypeTranslateMap[string(env.CollaborationType)] + "，权限："
		for _, verb := range env.Verbs {
			detail += collaborationProductVerbTranslateMap[verb] + "，"
		}
		detail = strings.TrimSuffix(detail, "，")
		detail += "；"
	}

	return detail, nil
}
