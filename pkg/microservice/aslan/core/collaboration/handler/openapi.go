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

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/config"
	cmmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/service"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

type OpenAPICreateCollaborationModeRequest struct {
	// 项目标识
	ProjectKey string `json:"project_key" binding:"required"`
	// 协作模式名称
	Name string `json:"name" binding:"required"`
	// 用户 ID 列表
	UserIDs []string `json:"user_ids"`
	// 用户组 ID 列表
	GroupIDs []string `json:"group_ids"`
	// 工作流
	Workflows []OpenAPICollaborationModeWorkflowItem `json:"workflows"`
	// 环境
	Envs []OpenAPICollaborationModeEnvItem `json:"envs"`
	// 资源回收天数，0表示不回收
	RecycleDay int64 `json:"recycle_day"`
}

func (r *OpenAPICreateCollaborationModeRequest) ToCollaborationMode() (*cmmodels.CollaborationMode, error) {
	project, err := templaterepo.NewProductColl().Find(r.ProjectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to find project %v, err: %v", r.ProjectKey, err)
	}

	memberInfo := make([]*types.Identity, 0)
	for _, userID := range r.UserIDs {
		memberInfo = append(memberInfo, &types.Identity{
			UID:          userID,
			IdentityType: "user",
		})
	}

	for _, groupID := range r.GroupIDs {
		memberInfo = append(memberInfo, &types.Identity{
			GID:          groupID,
			IdentityType: "group",
		})
	}

	memberIDs := make([]string, 0)
	for _, member := range memberInfo {
		memberIDs = append(memberIDs, member.GetID())
	}

	workflowNames := make([]string, 0)
	for _, workflow := range r.Workflows {
		workflowNames = append(workflowNames, workflow.Name)
	}

	workflows, _, err := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{
		ProjectName: r.ProjectKey,
		Names:       workflowNames,
	}, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows by names(%v), err: %v", workflowNames, err)
	}

	workflowMap := make(map[string]*commonmodels.WorkflowV4)
	for _, workflow := range workflows {
		workflowMap[workflow.Name] = workflow
	}

	workflowCMItems := make([]cmmodels.WorkflowCMItem, 0)
	for _, workflow := range r.Workflows {
		workflowV4, ok := workflowMap[workflow.Name]
		if !ok {
			return nil, fmt.Errorf("workflow %s not found", workflowV4.Name)
		}

		workflowCMItems = append(workflowCMItems, cmmodels.WorkflowCMItem{
			Name:              workflow.Name,
			DisplayName:       workflowV4.DisplayName,
			WorkflowType:      setting.CustomWorkflowType,
			CollaborationType: workflow.CollaborationType,
			Verbs:             workflow.Verbs,
		})
	}

	envs := make([]cmmodels.ProductCMItem, 0)
	for _, env := range r.Envs {
		envs = append(envs, cmmodels.ProductCMItem{
			Name:              env.Name,
			CollaborationType: env.CollaborationType,
			Verbs:             env.Verbs,
		})
	}

	return &cmmodels.CollaborationMode{
		ProjectName: r.ProjectKey,
		Name:        r.Name,
		Members:     memberIDs,
		MemberInfo:  memberInfo,
		DeployType:  project.ProductFeature.GetDeployType(),
		Workflows:   workflowCMItems,
		Products:    envs,
	}, nil
}

type OpenAPICollaborationModeWorkflowItem struct {
	// 工作流标识
	Name string `json:"name"`
	// 协作类型，share或new
	CollaborationType config.CollaborationType `json:"collaboration_type"`
	// 操作权限，get_workflow、edit_workflow、run_workflow、debug_workflow
	Verbs []string `json:"verbs"`
}

type OpenAPICollaborationModeEnvItem struct {
	// 环境名称
	Name string `json:"name"`
	// 协作类型，share或new
	CollaborationType config.CollaborationType `json:"collaboration_type"`
	// 操作权限，get_environment、config_environment、manage_environment、debug_pod
	Verbs []string `json:"verbs"`
}

// @summary 新建协作模式
// @description 新建协作模式
// @tags 	OpenAPI
// @accept 	json
// @produce json
// @Param   projectKey		query		 string	                                       true	    "项目标识"
// @Param   body 			body 		 OpenAPICreateCollaborationModeRequest         true 	"body"
// @success 200
// @router /openapi/collaborations  [post]
func OpenAPICreateCollaborationMode(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(OpenAPICreateCollaborationModeRequest)
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

	cmArgs, err := args.ToCollaborationMode()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	detail, err := generateCollaborationDetailLog(ctx.Account, cmArgs, ctx.Logger)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	internalhandler.InsertOperationLog(c, ctx.Account, args.ProjectKey, "(OpenAPI)"+"新增", "协作模式", detail, detail, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProjectKey].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.OpenAPICreateCollaborationMode(ctx.Account, cmArgs, ctx.Logger)
}

// @summary 删除协作模式
// @description 删除协作模式
// @tags 	OpenAPI
// @accept 	json
// @produce json
// @Param   projectKey		query		 string	        true	"项目标识"
// @Param   name 			path 		 string         true 	"协作模式名称"
// @success 200
// @router /openapi/collaborations/{name}  [delete]
func OpenAPIDeleteCollaborationMode(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can not be empty")
		return
	}
	name := c.Param("name")
	internalhandler.InsertOperationLog(c, ctx.Account, projectKey, "(OpenAPI)删除", "协作模式", name, name, "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.DeleteCollaborationMode(ctx.Account, projectKey, name, ctx.Logger)
}
