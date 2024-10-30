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
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

type CreateSprintResponse struct {
	ID string `json:"id"`
}

func CreateSprint(ctx *handler.Context, projectName, templateID, name string) (*CreateSprintResponse, error) {
	if templateID == "" {
		return nil, e.ErrCreateSprint.AddErr(errors.New("Required templateID is missing"))
	}

	template, err := mongodb.NewSprintTemplateColl().Find(ctx, &mongodb.SprintTemplateQueryOption{ID: templateID})
	if err != nil {
		return nil, e.ErrCreateSprint.AddErr(errors.Wrapf(err, "Get sprint template %s", templateID))
	}

	stages := make([]*models.SprintStage, 0)
	for _, stageTemplate := range template.Stages {
		stage := &models.SprintStage{
			ID:          stageTemplate.ID,
			Name:        stageTemplate.Name,
			Workflows:   stageTemplate.Workflows,
			WorkItemIDs: make([]string, 0),
		}
		stages = append(stages, stage)
	}

	sprint := &models.Sprint{
		Name:        name,
		TemplateID:  template.ID.Hex(),
		ProjectName: projectName,
		Stages:      stages,
		CreatedBy:   ctx.GenUserBriefInfo(),
		UpdatedBy:   ctx.GenUserBriefInfo(),
		CreateTime:  time.Now().Unix(),
		UpdateTime:  time.Now().Unix(),
	}
	sprint.Key, sprint.KeyInitials = util.GetKeyAndInitials(sprint.Name)

	err = sprint.Lint()
	if err != nil {
		return nil, e.ErrCreateSprint.AddErr(errors.Wrap(err, "Invalid sprint"))
	}

	id, err := mongodb.NewSprintColl().Create(ctx, sprint)
	if err != nil {
		return nil, e.ErrCreateSprint.AddErr(errors.Wrap(err, "Create sprint"))
	}

	resp := &CreateSprintResponse{
		ID: id.Hex(),
	}
	return resp, nil
}

type Sprint struct {
	ID         primitive.ObjectID  `json:"id"`
	Name       string              `json:"name"`
	CreatedBy  types.UserBriefInfo `json:"created_by"`
	CreateTime int64               `json:"create_time"`
	UpdatedBy  types.UserBriefInfo `json:"updated_by"`
	UpdateTime int64               `json:"update_time"`
	IsArchived bool                `json:"is_archived"`
	IsDeleted  bool                `json:"is_deleted"`
	Stages     []*SprintStage      `json:"stages"`
}

type SprintStage struct {
	ID        string                         `json:"id"`
	Name      string                         `json:"name"`
	Workflows []*commonmodels.SprintWorkflow `json:"workflows"`
	WorkItems []*SprintWorkitem              `json:"workitems"`
}

type SprintWorkitem struct {
	ID         primitive.ObjectID    `json:"id"`
	Title      string                `json:"title"`
	Owners     []types.UserBriefInfo `json:"owners"`
	SprintID   string                `json:"sprint_id"`
	SprintName string                `json:"sprint_name"`
	StageIndex int                   `json:"stage_index"`
	StageID    string                `json:"stage_id"`
	CreatedBy  types.UserBriefInfo   `json:"created_by"`
	CreateTime int64                 `json:"create_time"`
	UpdatedBy  types.UserBriefInfo   `json:"updated_by"`
	UpdateTime int64                 `json:"update_time"`
}

func GetSprint(ctx *handler.Context, id string) (*Sprint, error) {
	sprint, err := mongodb.NewSprintColl().GetByID(ctx, id)
	if err != nil {
		return nil, e.ErrGetSprint.AddErr(errors.Wrap(err, "Get sprint"))
	}

	workitems, err := mongodb.NewSprintWorkItemColl().List(ctx, mongodb.ListSprintWorkItemOption{SprintID: id})
	if err != nil {
		return nil, e.ErrGetSprint.AddErr(errors.Wrapf(err, "Get workitems by sprint id %s", id))
	}
	workitemMap := make(map[string]*models.SprintWorkItem)
	for _, workitem := range workitems {
		workitemMap[workitem.ID.Hex()] = workitem
	}

	err = updateSprintWorklow(ctx, sprint)
	if err != nil {
		return nil, e.ErrGetSprint.AddErr(errors.Wrap(err, "Update sprint workflow"))
	}

	resp := genSprint(ctx, sprint, workitemMap)

	return resp, nil
}

func updateSprintWorklow(ctx *handler.Context, sprint *commonmodels.Sprint) error {
	workflowNames := make([]string, 0)
	workflowMap := make(map[string]*models.SprintWorkflow)
	for _, stage := range sprint.Stages {
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
		// err = mongodb.NewSprintColl().UpdateStages(ctx, sprint.ID.Hex(), sprint.Stages)
		// if err != nil {
		// 	return e.ErrGetSprintTemplate.AddErr(errors.Wrapf(err, "Update sprint stage %s", sprint.Name))
		// }
	}
	return nil
}

func genSprint(ctx *handler.Context, sprint *commonmodels.Sprint, workitemMap map[string]*commonmodels.SprintWorkItem) *Sprint {
	resp := &Sprint{
		ID:         sprint.ID,
		Name:       sprint.Name,
		CreatedBy:  sprint.CreatedBy,
		CreateTime: sprint.CreateTime,
		UpdatedBy:  sprint.UpdatedBy,
		UpdateTime: sprint.UpdateTime,
		IsArchived: sprint.IsArchived,
		Stages:     make([]*SprintStage, 0),
	}
	for _, stage := range sprint.Stages {
		sprintStage := &SprintStage{
			ID:        stage.ID,
			Name:      stage.Name,
			Workflows: stage.Workflows,
			WorkItems: make([]*SprintWorkitem, 0),
		}
		for _, workitemID := range stage.WorkItemIDs {
			workitem, ok := workitemMap[workitemID]
			if !ok {
				ctx.Logger.Errorf("workitem %s not found in sprint %s's stage %s", workitemID, sprint.Name, stage.Name)
				continue
			}
			sprintWorkitem := &SprintWorkitem{
				ID:         workitem.ID,
				Title:      workitem.Title,
				Owners:     workitem.Owners,
				SprintID:   workitem.SprintID,
				StageID:    workitem.StageID,
				CreateTime: workitem.CreateTime,
				UpdateTime: workitem.UpdateTime,
			}
			sprintStage.WorkItems = append(sprintStage.WorkItems, sprintWorkitem)
		}
		resp.Stages = append(resp.Stages, sprintStage)
	}
	return resp
}

func DeleteSprint(ctx *handler.Context, id string) error {
	sprint, err := mongodb.NewSprintColl().GetByID(ctx, id)
	if err != nil {
		return e.ErrDeleteSprint.AddErr(errors.Wrap(err, "Get sprint"))
	}

	if sprint.IsArchived {
		return e.ErrUpdateSprintName.AddDesc("Sprint is archived")
	}

	workflowItemIDs := make([]string, 0)
	for _, stage := range sprint.Stages {
		workflowItemIDs = append(workflowItemIDs, stage.WorkItemIDs...)
	}
	err = mongodb.NewSprintWorkItemColl().DeleteByIDs(ctx, workflowItemIDs)
	if err != nil {
		return e.ErrDeleteSprint.AddErr(errors.Wrap(err, "Delete sprint workitems"))
	}
	err = mongodb.NewSprintColl().DeleteByID(ctx, id)
	if err != nil {
		return e.ErrDeleteSprint.AddErr(errors.Wrap(err, "Delete sprint"))
	}

	return nil
}

func ArchiveSprint(ctx *handler.Context, id string) error {
	sprint, err := mongodb.NewSprintColl().GetByID(ctx, id)
	if err != nil {
		return e.ErrUpdateSprintName.AddErr(errors.Wrap(err, "Get sprint"))
	}

	if sprint.IsArchived {
		return e.ErrUpdateSprintName.AddDesc("Sprint is archived")
	}

	err = mongodb.NewSprintColl().ArchiveByID(ctx, id)
	if err != nil {
		return e.ErrArchiveSprint.AddErr(errors.Wrapf(err, "Archive sprint %s", id))
	}

	return nil
}

func UpdateSprintName(ctx *handler.Context, id, name string) error {
	sprint, err := mongodb.NewSprintColl().GetByID(ctx, id)
	if err != nil {
		return e.ErrUpdateSprintName.AddErr(errors.Wrap(err, "Get sprint"))
	}

	if sprint.IsArchived {
		return e.ErrUpdateSprintName.AddDesc("Sprint is archived")
	}

	sprint.Name = name
	sprint.Key, sprint.KeyInitials = util.GetKeyAndInitials(name)

	if err = mongodb.NewSprintColl().UpdateName(ctx, id, sprint); err != nil {
		return e.ErrUpdateSprintName.AddErr(errors.Wrap(err, "Update sprint"))
	}

	return nil
}

type ListSprintOption struct {
	ProjectName string `form:"projectName" binding:"required"`
	PageNum     int64  `form:"pageNum" binding:"required"`
	PageSize    int64  `form:"pageSize" binding:"required"`
	IsArchived  bool   `form:"isArchived"`
	Filter      string `form:"filter"`
}

type ListSprintResp struct {
	List  []*models.Sprint `json:"list"`
	Total int64            `json:"total"`
}

func ListSprint(ctx *handler.Context, opt *ListSprintOption) (*ListSprintResp, error) {
	var (
		list  []*commonmodels.Sprint
		total int64
		err   error
	)
	list, total, err = mongodb.NewSprintColl().List(ctx, &mongodb.ListSprintOption{
		ProjectName: opt.ProjectName,
		IsArchived:  opt.IsArchived,
		PageNum:     opt.PageNum,
		PageSize:    opt.PageSize,
		Filter:      opt.Filter,
	})
	if err != nil {
		return nil, e.ErrListSprint.AddErr(errors.Wrap(err, "List sprint"))
	}

	return &ListSprintResp{
		List:  list,
		Total: total,
	}, nil
}
