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

package workflow

import (
	"fmt"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func CreateWorkflowView(name, projectName string, workflowList []*commonmodels.WorkflowViewDetail, username string, logger *zap.SugaredLogger) error {
	workflowView := &commonmodels.WorkflowView{
		Name:        name,
		ProjectName: projectName,
		Workflows:   workflowList,
		UpdateBy:    username,
	}

	err := commonrepo.NewWorkflowViewColl().Create(workflowView)
	if err != nil {
		msg := fmt.Sprintf("create workflow view error: %v", err)
		logger.Errorf(msg)
		return e.ErrCreateView.AddDesc(msg)
	}
	return nil
}

func UpdateWorkflowView(input *commonmodels.WorkflowView, userName string, logger *zap.SugaredLogger) error {
	if input.Name == "" || input.ProjectName == "" {
		msg := ("update workflow view error: invalid params")
		log.Error(msg)
		return e.ErrUpdateView.AddDesc(msg)
	}
	input.UpdateBy = userName
	if err := commonrepo.NewWorkflowViewColl().Update(input); err != nil {
		msg := fmt.Sprintf("update workflow view error: %v", err)
		log.Error(msg)
		return e.ErrUpdateView.AddDesc(msg)
	}
	return nil
}

func ListWorkflowViewNames(projectName string, logger *zap.SugaredLogger) ([]string, error) {
	if projectName == "" {
		msg := ("list workflow view error: invalid params")
		log.Error(msg)
		return []string{}, e.ErrListView.AddDesc(msg)
	}

	names, err := commonrepo.NewWorkflowViewColl().ListNamesByProject(projectName)
	if err != nil {
		msg := fmt.Sprintf("list workflow view error: %v", err)
		log.Error(msg)
		return []string{}, e.ErrListView.AddDesc(msg)
	}
	return names, nil
}

func DeleteWorkflowView(projectName, viewName string, logger *zap.SugaredLogger) error {
	view, err := commonrepo.NewWorkflowViewColl().Find(projectName, viewName)
	if err != nil {
		msg := fmt.Sprintf("find workflow view error: %v", err)
		log.Error(msg)
		return e.ErrDeleteView.AddDesc(msg)
	}
	if err := commonrepo.NewWorkflowViewColl().DeleteByID(view.ID.Hex()); err != nil {
		msg := fmt.Sprintf("delete workflow view error: %v", err)
		log.Error(msg)
		return e.ErrDeleteView.AddDesc(msg)
	}
	return nil
}

func GetWorkflowViewPreset(projectName, viewName string, logger *zap.SugaredLogger) (*commonmodels.WorkflowView, error) {
	view := &commonmodels.WorkflowView{ProjectName: projectName}
	if viewName != "" {
		existView, err := commonrepo.NewWorkflowViewColl().Find(projectName, viewName)
		if err != nil {
			msg := fmt.Sprintf("get workflow view error: %v", err)
			log.Error(msg)
			return nil, e.ErrGetView.AddDesc(msg)
		}
		view = existView
	}
	workflowV4Map := map[string]bool{}
	workflowMap := map[string]bool{}
	for _, workflow := range view.Workflows {
		if workflow.WorkflowType == setting.CustomWorkflowType {
			workflowV4Map[workflow.WorkflowName] = workflow.Enabled
		} else {
			workflowMap[workflow.WorkflowName] = workflow.Enabled
		}
	}
	newWorkflowDetails := []*commonmodels.WorkflowViewDetail{}
	workkflowV4s, _, err := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{ProjectName: projectName}, 0, 0)
	if err != nil {
		msg := fmt.Sprintf("list workflow v4 error: %v", err)
		log.Error(msg)
		return nil, e.ErrGetView.AddDesc(msg)
	}
	for _, workflowV4 := range workkflowV4s {
		workflowDetail := &commonmodels.WorkflowViewDetail{
			WorkflowName:        workflowV4.Name,
			WorkflowType:        setting.CustomWorkflowType,
			WorkflowDisplayName: workflowV4.DisplayName,
		}
		if enabled := workflowV4Map[workflowV4.Name]; enabled {
			workflowDetail.Enabled = true
		}
		newWorkflowDetails = append(newWorkflowDetails, workflowDetail)
	}
	workflows, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{Projects: []string{projectName}})
	if err != nil {
		msg := fmt.Sprintf("list workflow error: %v", err)
		log.Error(msg)
		return nil, e.ErrGetView.AddDesc(msg)
	}
	for _, workflow := range workflows {
		workflowDetail := &commonmodels.WorkflowViewDetail{
			WorkflowName:        workflow.Name,
			WorkflowType:        setting.ProductWorkflowType,
			WorkflowDisplayName: workflow.DisplayName,
		}
		if enabled := workflowMap[workflow.Name]; enabled {
			workflowDetail.Enabled = true
		}
		newWorkflowDetails = append(newWorkflowDetails, workflowDetail)
	}
	view.Workflows = newWorkflowDetails
	return view, nil
}
