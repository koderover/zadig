/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

func CreateSprintTemplate(ctx *handler.Context, args *models.SprintTemplate) error {
	if args.Name == "" {
		return e.ErrCreateSprintTemplate.AddErr(errors.New("Required parameters are missing"))
	}
	err := args.Lint()
	if err != nil {
		return e.ErrCreateSprintTemplate.AddErr(errors.Wrap(err, "Invalid sprint template"))
	}

	args.CreatedBy = ctx.GenUserBriefInfo()
	args.UpdatedBy = ctx.GenUserBriefInfo()
	args.CreateTime = time.Now().Unix()
	args.UpdateTime = time.Now().Unix()
	args.Key, args.KeyInitials = util.GetKeyAndInitials(args.Name)

	err = mongodb.NewSprintTemplateColl().Create(ctx, args)
	if err != nil {
		return e.ErrCreateSprintTemplate.AddErr(errors.Wrap(err, "Create sprint template error"))
	}
	return nil
}

func GetSprintTemplate(ctx *handler.Context, id string) (*models.SprintTemplate, error) {
	sprintTemplate, err := mongodb.NewSprintTemplateColl().Find(ctx, &mongodb.SprintTemplateQueryOption{ID: id})
	if err != nil {
		return nil, e.ErrGetSprintTemplate.AddErr(errors.Wrap(err, "GetSprintTemplate"))
	}

	err = genSprintTemplate(ctx, sprintTemplate)
	if err != nil {
		return nil, e.ErrGetSprintTemplate.AddErr(errors.Wrap(err, "genSprintTemplate"))
	}

	return sprintTemplate, nil
}

func DeleteSprintTemplate(ctx *handler.Context, username, id string) error {
	_, err := mongodb.NewSprintTemplateColl().GetByID(ctx, id)
	if err != nil {
		return e.ErrDeleteSprintTemplate.AddErr(errors.Wrapf(err, "Get sprint template %s", id))
	}
	return mongodb.NewReleasePlanColl().DeleteByID(ctx, id)
}

type ListSprintTemplateOption struct {
	ProjectName string `form:"projectName" binding:"required"`
	PageNum     int64  `form:"pageNum" binding:"required"`
	PageSize    int64  `form:"pageSize" binding:"required"`
}

type ListSprintTemplateResp struct {
	List  []*models.SprintTemplate `json:"list"`
	Total int64                    `json:"total"`
}

func ListSprintTemplate(ctx *handler.Context, opt *ListSprintTemplateOption) (*ListSprintTemplateResp, error) {
	var (
		list  []*commonmodels.SprintTemplate
		total int64
		err   error
	)
	list, err = mongodb.NewSprintTemplateColl().List(ctx, &mongodb.SprintTemplateListOption{ProjectName: opt.ProjectName})
	if err != nil {
		return nil, e.ErrListSprintTemplate.AddErr(errors.Wrap(err, "ListSprintTemplate"))
	}

	return &ListSprintTemplateResp{
		List:  list,
		Total: total,
	}, nil
}

func InitSprintTemplate(ctx *handler.Context, projectName string) {
	template := &models.SprintTemplate{
		Name: "zadig-default",
		Stages: []*models.SprintStageTemplate{
			{ID: uuid.NewString(), Name: "开发阶段"},
			{ID: uuid.NewString(), Name: "测试阶段"},
			{ID: uuid.NewString(), Name: "预发阶段"},
			{ID: uuid.NewString(), Name: "生产阶段"},
		},
	}
	template.ProjectName = projectName
	template.CreateTime = time.Now().Unix()
	template.UpdateTime = time.Now().Unix()
	template.CreatedBy = types.GeneSystemUserBriefInfo()
	template.UpdatedBy = types.GeneSystemUserBriefInfo()

	err := template.Lint()
	if err != nil {
		ctx.Logger.Errorf("Invalid sprint template: %v", err)
		return
	}

	if err := mongodb.NewSprintTemplateColl().UpsertByName(ctx, template); err != nil {
		ctx.Logger.Errorf("Update build-in sprint template error: %v", err)
	}
}

func GetDefaultSprintTemplate(ctx *handler.Context, projectName string) (*models.SprintTemplate, error) {
	sprintTemplate, err := mongodb.NewSprintTemplateColl().Find(ctx, &mongodb.SprintTemplateQueryOption{
		ProjectName: projectName,
		Name:        "zadig-default",
	})
	if err != nil {
		return nil, e.ErrGetSprintTemplate.AddErr(errors.Wrap(err, "GetSprintTemplate"))
	}

	err = genSprintTemplate(ctx, sprintTemplate)
	if err != nil {
		return nil, e.ErrGetSprintTemplate.AddErr(errors.Wrap(err, "genSprintTemplate"))
	}

	return sprintTemplate, nil
}

func genSprintTemplate(ctx *handler.Context, sprintTemplate *commonmodels.SprintTemplate) error {
	workflowNames := make([]string, 0)
	workflowMap := make(map[string]*models.SprintWorkflow)
	for _, stage := range sprintTemplate.Stages {
		for _, workflow := range stage.Workflows {
			if !workflow.IsDeleted {
				workflowMap[workflow.Name] = workflow
				workflowNames = append(workflowNames, workflow.Name)
			}
		}
	}

	if len(workflowNames) == 0 {
		return nil
	}

	workflowList, _, err := mongodb.NewWorkflowV4Coll().List(&mongodb.ListWorkflowV4Option{Names: workflowNames}, 0, 0)
	if err != nil {
		return e.ErrGetSprintTemplate.AddErr(errors.Wrapf(err, "ListWorkflow %s", workflowNames))
	}

	changed := false
	for _, workflow := range workflowList {
		if _, ok := workflowMap[workflow.Name]; !ok {
			return e.ErrGetSprintTemplate.AddErr(errors.Errorf("Workflow %s not found in sprint template", workflow.Name))
		}

		if workflowMap[workflow.Name].DisplayName != workflow.DisplayName {
			workflowMap[workflow.Name].DisplayName = workflow.DisplayName
			changed = true
		}
		delete(workflowMap, workflow.Name)
	}
	if len(workflowMap) > 0 {
		// remind workflows are deleted
		for _, workflow := range workflowMap {
			workflow.IsDeleted = true
		}
		changed = true
	}

	if changed {
		err = mongodb.NewSprintTemplateColl().Update(ctx, sprintTemplate)
		if err != nil {
			return e.ErrGetSprintTemplate.AddErr(errors.Wrapf(err, "Update sprint template %s", sprintTemplate.Name))
		}
	}
	return nil
}

func UpdateSprintTemplateStageName(ctx *handler.Context, sprintTemplateID, stageID, stageName string) error {
	session, deferFunc, err := mongotool.SessionWithTransaction(ctx)
	defer func() { deferFunc(err) }()
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "SessionWithTransaction"))
	}

	template, err := mongodb.NewSprintTemplateColl().GetByID(ctx, sprintTemplateID)
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "Get sprint template"))
	}

	for _, stage := range template.Stages {
		if stage.ID == stageID {
			stage.Name = stageName
			break
		}
	}

	err = template.Lint()
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "该流程不合法"))
	}

	sprints, _, err := mongodb.NewSprintCollWithSession(session).List(ctx, &mongodb.ListSprintOption{TemplateID: sprintTemplateID})
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrapf(err, "List sprints by template id %s", sprintTemplateID))
	}

	if len(sprints) > 0 {
		sprintIDs := []primitive.ObjectID{}
		for _, sprint := range sprints {
			sprintIDs = append(sprintIDs, sprint.ID)
		}
		err = mongodb.NewSprintCollWithSession(session).BulkUpdateStageName(ctx, sprintIDs, stageID, stageName)
		if err != nil {
			return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "Bulk Update Sprints Stage Name"))
		}
	}

	if err = mongodb.NewSprintTemplateCollWithSession(session).Update(ctx, template); err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "Update sprint template"))
	}

	return nil
}

func UpdateSprintTemplateStageWorkflows(ctx *handler.Context, sprintTemplateID, stageID string, workflowTemplates []*models.SprintWorkflow, updateTime int64) error {
	session, deferFunc, err := mongotool.SessionWithTransaction(ctx)
	defer func() { deferFunc(err) }()
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "SessionWithTransaction"))
	}

	template, err := mongodb.NewSprintTemplateColl().GetByID(ctx, sprintTemplateID)
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "Get sprint template"))
	}

	if template.UpdateTime > updateTime {
		return e.ErrUpdateSprintTemplate.AddErr(errors.New("该流程已被他人更新，请刷新后重试"))
	}

	sprints, _, err := mongodb.NewSprintCollWithSession(session).List(ctx, &mongodb.ListSprintOption{TemplateID: sprintTemplateID})
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrapf(err, "List sprints by template id %s", sprintTemplateID))
	}

	workflowMap := make(map[string]*models.SprintWorkflow)
	for _, workflow := range workflowTemplates {
		workflowMap[workflow.Name] = workflow
	}

	if len(sprints) > 0 {
		updateSprintWorkflowsMap := make(map[string][]*models.SprintWorkflow)

		for _, sprint := range sprints {
			for _, stage := range sprint.Stages {
				if stage.ID == stage.ID {
					newWorkflows := make([]*models.SprintWorkflow, 0)
					stageWorkflowMap := make(map[string]*models.SprintWorkflow)
					for _, workflow := range stage.Workflows {
						stageWorkflowMap[workflow.Name] = workflow
					}

					// set deleted workflows to IsDeleted = true
					for _, workflow := range stage.Workflows {
						if _, ok := workflowMap[workflow.Name]; !ok {
							workflow.IsDeleted = true
						}
						stageWorkflowMap[workflow.Name] = workflow
					}

					// add/update workflows in stage template
					for _, workflowTemplate := range workflowTemplates {
						if workflow, ok := stageWorkflowMap[workflowTemplate.Name]; !ok {
							// not find this workflow in stage, add new workflow
							newWorkflows = append(newWorkflows, workflowTemplate)
						} else {
							// find this workflow in stage, update it
							workflow.DisplayName = workflowTemplate.DisplayName
							workflow.IsDeleted = workflowTemplate.IsDeleted
							newWorkflows = append(newWorkflows, workflow)
							delete(stageWorkflowMap, workflow.Name)
						}
					}

					// add remind workflows in stage
					for _, workflow := range stageWorkflowMap {
						newWorkflows = append(newWorkflows, workflow)
					}

					updateSprintWorkflowsMap[sprint.ID.Hex()] = newWorkflows
					break
				}
			}

		}

		err = mongodb.NewSprintCollWithSession(session).BulkUpdateStageWorkflows(ctx, stageID, updateSprintWorkflowsMap)
		if err != nil {
			return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "Bulk Update Sprints Stage Workflows"))
		}
	}

	if err = mongodb.NewSprintTemplateCollWithSession(session).UpdateStageWorkflows(ctx, sprintTemplateID, stageID, workflowTemplates); err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "Update sprint template stage workflows"))
	}

	return nil
}

func AddSprintTemplateStage(ctx *handler.Context, sprintTemplateID, stageName string) error {
	session, deferFunc, err := mongotool.SessionWithTransaction(ctx)
	defer func() { deferFunc(err) }()
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "SessionWithTransaction"))
	}

	template, err := mongodb.NewSprintTemplateColl().GetByID(ctx, sprintTemplateID)
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "Get sprint template"))
	}

	newStage := &models.SprintStageTemplate{
		ID:        uuid.NewString(),
		Name:      stageName,
		Workflows: make([]*models.SprintWorkflow, 0),
	}
	template.Stages = append(template.Stages, newStage)

	err = template.Lint()
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "该流程不合法"))
	}

	sprints, _, err := mongodb.NewSprintCollWithSession(session).List(ctx, &mongodb.ListSprintOption{TemplateID: sprintTemplateID})
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrapf(err, "List sprints by template id %s", sprintTemplateID))
	}

	if len(sprints) > 0 {
		sprintIDs := []primitive.ObjectID{}
		for _, sprint := range sprints {
			sprintIDs = append(sprintIDs, sprint.ID)
		}

		err = mongodb.NewSprintCollWithSession(session).BulkAddStage(ctx, sprintIDs, newStage)
		if err != nil {
			return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "Bulk Add Sprint Stage"))
		}
	}

	if err = mongodb.NewSprintTemplateCollWithSession(session).Update(ctx, template); err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "Update sprint template"))
	}

	return nil
}

func DeleteSprintTemplateStage(ctx *handler.Context, sprintTemplateID, stageID string) error {
	session, deferFunc, err := mongotool.SessionWithTransaction(ctx)
	defer func() { deferFunc(err) }()
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "SessionWithTransaction"))
	}

	template, err := mongodb.NewSprintTemplateColl().GetByID(ctx, sprintTemplateID)
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "Get sprint template"))
	}

	newStages := make([]*models.SprintStageTemplate, 0)
	for _, stage := range template.Stages {
		if stage.ID != stageID {
			newStages = append(newStages, stage)
		}
	}
	template.Stages = newStages

	err = template.Lint()
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "该流程不合法"))
	}

	sprints, _, err := mongodb.NewSprintCollWithSession(session).List(ctx, &mongodb.ListSprintOption{TemplateID: sprintTemplateID})
	if err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrapf(err, "List sprints by template id %s", sprintTemplateID))
	}

	if len(sprints) > 0 {
		sprintIDs := []primitive.ObjectID{}
		for _, sprint := range sprints {
			for _, stage := range sprint.Stages {
				if stage.ID == stageID {
					if len(stage.WorkItemIDs) > 0 {
						return e.ErrUpdateSprintTemplate.AddErr(fmt.Errorf("由于「%s」的「%s」中包含工作项，所以无法删除，如需删除，请先移除相应工作项。", sprint.Name, stage.Name))
					}
				}
			}
			sprintIDs = append(sprintIDs, sprint.ID)
		}

		err = mongodb.NewSprintCollWithSession(session).BulkDeleteStage(ctx, sprintIDs, stageID)
		if err != nil {
			return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "Bulk Delete Sprint Stage"))
		}
	}

	if err = mongodb.NewSprintTemplateCollWithSession(session).Update(ctx, template); err != nil {
		return e.ErrUpdateSprintTemplate.AddErr(errors.Wrap(err, "Update sprint template"))
	}

	return nil
}
