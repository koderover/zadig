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
	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/collaboration"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func GetCollaborationMode(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can not be empty")
		return
	}
	ctx.Resp, ctx.RespErr = collaboration.GetCollaborationModes([]string{projectName}, ctx.Logger)
}

func CreateCollaborationMode(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.CollaborationMode)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateCollaborationMode c.GetRawData() err: %s", err)
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateCollaborationMode json.Unmarshal err: %s", err)
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	detail, err := generateCollaborationDetailLog(ctx.Account, args, ctx.Logger)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	internalhandler.InsertOperationLog(c, ctx.Account, args.ProjectName, "新增", "协作模式", detail, detail, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = service.CreateCollaborationMode(ctx.Account, args, ctx.Logger)
}

func UpdateCollaborationMode(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.CollaborationMode)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateCollaborationMode c.GetRawData() err: %s", err)
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateCollaborationMode json.Unmarshal err: %s", err)
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	detail, err := generateCollaborationDetailLog(ctx.Account, args, ctx.Logger)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	internalhandler.InsertOperationLog(c, ctx.Account, args.ProjectName, "更新", "协作模式", detail, detail, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = service.UpdateCollaborationMode(ctx.Account, args, ctx.Logger)
}

func DeleteCollaborationMode(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can not be empty")
		return
	}
	name := c.Param("name")
	internalhandler.InsertOperationLog(c, ctx.Account, projectName, "删除", "协作模式", name, name, "", types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = service.DeleteCollaborationMode(ctx.Account, projectName, name, ctx.Logger)
}

func generateCollaborationDetailLog(username string, args *commonmodels.CollaborationMode, logger *zap.SugaredLogger) (string, error) {
	collaborationWorkflowVerbTranslateMap := map[string]string{
		"get_workflow":   "查看",
		"edit_workflow":  "编辑",
		"run_workflow":   "执行",
		"debug_workflow": "调试",
	}

	collaborationProductVerbTranslateMap := map[string]string{
		"get_environment":             "查看",
		"config_environment":          "配置",
		"manage_environment":          "管理服务实例",
		"debug_pod":                   "服务调试",
		"get_production_environment":  "查看",
		"edit_production_environment": "编辑",
		"production_debug_pod":        "服务调试",
	}

	collaborationTypeTranslateMap := map[string]string{
		"new":   "独享",
		"share": "共享",
	}

	currMode, exists, err := service.GetCollaborationMode(username, args.ProjectName, args.Name, logger)
	if err != nil {
		return "", fmt.Errorf("can't get current collaboration mode, projectName: %v, name: %v err: %v", args.ProjectName, args.Name, err)
	}
	if !exists {
		currMode = &commonmodels.CollaborationMode{
			Name:      args.Name,
			Members:   []string{},
			Workflows: []commonmodels.WorkflowCMItem{},
			Products:  []commonmodels.ProductCMItem{},
		}
	}

	currUserIDSet := sets.NewString(currMode.Members...)
	argsUserIDSet := sets.NewString(args.Members...)
	addedUserIDSet := argsUserIDSet.Difference(currUserIDSet)
	deletedUserIDSet := currUserIDSet.Difference(argsUserIDSet)

	allUserIDSet := sets.NewString(args.Members...)
	allUserIDSet.Insert(currMode.Members...)

	detail := "协作模式名称：" + args.Name + "\n\n"

	// user part
	userResp, err := user.New().SearchUsersByIDList(allUserIDSet.List())
	if err != nil {
		return "", fmt.Errorf("failed to search users by uids(%v), err: %v", args.Members, err)
	}
	userInfoMap := make(map[string]*types.UserInfo)
	for _, user := range userResp.Users {
		userInfoMap[user.Uid] = user
	}

	if deletedUserIDSet.Len() != 0 {
		detail += "删除用户："
		for _, userid := range deletedUserIDSet.List() {
			detail += getUserName(userid, userInfoMap) + "，"
		}
		detail = strings.Trim(detail, "，")
		detail += "\n\n"
	}

	if addedUserIDSet.Len() != 0 {
		detail += "新增用户："
		for _, userid := range addedUserIDSet.List() {
			detail += getUserName(userid, userInfoMap) + "，"
		}
		detail = strings.Trim(detail, "，")
		detail += "\n"

		for _, workflow := range args.Workflows {
			detail += "工作流名称：" + workflow.Name + "，类型：" + collaborationTypeTranslateMap[string(workflow.CollaborationType)] + "，"
			detail += "权限："
			for _, verb := range workflow.Verbs {
				detail += collaborationWorkflowVerbTranslateMap[verb] + "，"
			}
			detail = strings.TrimSuffix(detail, "，")
			detail += "\n"
		}

		for _, env := range args.Products {
			detail += "环境名称：" + env.Name + "，类型：" + collaborationTypeTranslateMap[string(env.CollaborationType)] + "，权限："
			for _, verb := range env.Verbs {
				detail += collaborationProductVerbTranslateMap[verb] + "，"
			}

			detail = strings.TrimSuffix(detail, "，")
			detail += "\n"
		}

		detail += "\n"
	}

	// workflow part
	currWorklowMap := make(map[string]commonmodels.WorkflowCMItem)
	argWorklowMap := make(map[string]commonmodels.WorkflowCMItem)
	currWorklowSet := sets.NewString()
	argWorklowSet := sets.NewString()

	for _, workflow := range currMode.Workflows {
		currWorklowSet.Insert(workflow.Name)
		currWorklowMap[workflow.Name] = workflow
	}
	for _, workflow := range args.Workflows {
		argWorklowSet.Insert(workflow.Name)
		argWorklowMap[workflow.Name] = workflow
	}

	addedWorkflowSet := argWorklowSet.Difference(currWorklowSet)
	deletedWorkflowSet := currWorklowSet.Difference(argWorklowSet)
	intersectionWorkflowSet := argWorklowSet.Intersection(currWorklowSet)

	detail += "权限变更：\n"
	for _, name := range addedWorkflowSet.List() {
		workflow := argWorklowMap[name]
		detail += "工作流名称：" + workflow.Name + "，类型：" + collaborationTypeTranslateMap[string(workflow.CollaborationType)] + "，"
		detail += "增加权限："
		for _, verb := range workflow.Verbs {
			detail += collaborationWorkflowVerbTranslateMap[verb] + "，"
		}
		detail = strings.TrimSuffix(detail, "，")
		detail += "\n"
	}
	for _, name := range deletedWorkflowSet.List() {
		workflow := currWorklowMap[name]
		detail += "工作流名称：" + workflow.Name + "，类型：" + collaborationTypeTranslateMap[string(workflow.CollaborationType)] + "，"
		detail += "移除权限："
		for _, verb := range workflow.Verbs {
			detail += collaborationWorkflowVerbTranslateMap[verb] + "，"
		}
		detail = strings.TrimSuffix(detail, "，")
		detail += "\n"
	}
	for _, name := range intersectionWorkflowSet.List() {
		workflow := argWorklowMap[name]
		currWorkflow := currWorklowMap[name]

		currVerbSet := sets.NewString(currWorkflow.Verbs...)
		argVerbSet := sets.NewString(workflow.Verbs...)
		addedVerbSet := argVerbSet.Difference(currVerbSet)
		deletedVerbSet := currVerbSet.Difference(argVerbSet)

		if addedVerbSet.Len() == 0 && deletedVerbSet.Len() == 0 {
			continue
		}

		detail += "工作流名称：" + workflow.Name + "，类型：" + collaborationTypeTranslateMap[string(workflow.CollaborationType)] + "，"
		if addedVerbSet.Len() != 0 {
			detail += "增加权限："
			for _, verb := range addedVerbSet.List() {
				detail += collaborationWorkflowVerbTranslateMap[verb] + "，"
			}
		}
		if deletedVerbSet.Len() != 0 {
			detail += "移除权限："
			for _, verb := range deletedVerbSet.List() {
				detail += collaborationWorkflowVerbTranslateMap[verb] + "，"
			}
		}
		detail = strings.TrimSuffix(detail, "，")
		detail += "\n"
	}

	// product part
	currProductMap := make(map[string]commonmodels.ProductCMItem)
	argProductMap := make(map[string]commonmodels.ProductCMItem)
	currProductSet := sets.NewString()
	argProductSet := sets.NewString()

	for _, product := range currMode.Products {
		currProductSet.Insert(product.Name)
		currProductMap[product.Name] = product
	}
	for _, product := range args.Products {
		argProductSet.Insert(product.Name)
		argProductMap[product.Name] = product
	}

	addedProductSet := argProductSet.Difference(currProductSet)
	deletedProductSet := currProductSet.Difference(argProductSet)
	intersectionProductSet := argProductSet.Intersection(currProductSet)

	for _, envName := range addedProductSet.List() {
		env := argProductMap[envName]
		detail += "环境名称：" + env.Name + "，类型：" + collaborationTypeTranslateMap[string(env.CollaborationType)] + "，新增权限："
		for _, verb := range env.Verbs {
			detail += collaborationProductVerbTranslateMap[verb] + "，"
		}

		detail = strings.TrimSuffix(detail, "，")
		detail += "\n"
	}
	for _, envName := range deletedProductSet.List() {
		env := currProductMap[envName]
		detail += "环境名称：" + env.Name + "，类型：" + collaborationTypeTranslateMap[string(env.CollaborationType)] + "，移除权限："
		for _, verb := range env.Verbs {
			detail += collaborationProductVerbTranslateMap[verb] + "，"
		}

		detail = strings.TrimSuffix(detail, "，")
		detail += "\n"
	}
	for _, envName := range intersectionProductSet.List() {
		argProduct := argProductMap[envName]
		currProduct := currProductMap[envName]

		currVerbSet := sets.NewString(currProduct.Verbs...)
		argVerbSet := sets.NewString(argProduct.Verbs...)
		addedVerbSet := argVerbSet.Difference(currVerbSet)
		deletedVerbSet := currVerbSet.Difference(argVerbSet)

		if addedVerbSet.Len() == 0 && deletedVerbSet.Len() == 0 {
			continue
		}

		detail += "环境名称：" + argProduct.Name + "，类型：" + collaborationTypeTranslateMap[string(argProduct.CollaborationType)] + "，"

		if addedVerbSet.Len() != 0 {
			detail += "增加权限："
			for _, verb := range addedVerbSet.List() {
				detail += collaborationProductVerbTranslateMap[verb] + "，"
			}
		}
		if deletedVerbSet.Len() != 0 {
			detail += "移除权限："
			for _, verb := range deletedVerbSet.List() {
				detail += collaborationProductVerbTranslateMap[verb] + "，"
			}
		}

		detail = strings.TrimSuffix(detail, "，")
		detail += "\n"
	}

	if strings.LastIndex(detail, "权限变更：\n") == len(detail)-len("权限变更：\n") {
		detail = strings.TrimSuffix(detail, "权限变更：\n")
		return detail, nil
	}

	detail += "影响用户："
	for _, userID := range allUserIDSet.List() {
		detail += getUserName(userID, userInfoMap) + "，"
	}
	detail = strings.TrimSuffix(detail, "，")

	return detail, nil
}

func getUserName(uid string, userInfoMap map[string]*types.UserInfo) string {
	if user, ok := userInfoMap[uid]; ok {
		return user.Name
	}
	return "deleted-user"
}
