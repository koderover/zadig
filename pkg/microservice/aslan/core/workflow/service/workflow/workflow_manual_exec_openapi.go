/*
Copyright 2026 The KodeRover Authors.

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

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type OpenAPIManualExecWorkflowTaskV4Request struct {
	StageName string `json:"stage_name"`
}

type OpenAPIManualExecExecutor struct {
	Type      string `json:"type"`
	UserID    string `json:"user_id,omitempty"`
	UserName  string `json:"user_name,omitempty"`
	GroupID   string `json:"group_id,omitempty"`
	GroupName string `json:"group_name,omitempty"`
}

type OpenAPIManualExecWorkflowTaskV4Context struct {
	ProjectKey  string                       `json:"project_key"`
	WorkflowKey string                       `json:"workflow_key"`
	TaskID      int64                        `json:"task_id"`
	Status      config.Status                `json:"status"`
	StageName   string                       `json:"stage_name"`
	CanExecute  bool                         `json:"can_execute"`
	Executors   []*OpenAPIManualExecExecutor `json:"executors"`
}

func GetOpenAPIManualExecWorkflowTaskV4Context(workflowName, projectKey string, taskID int64, executorID string, isSystemAdmin bool, logger *zap.SugaredLogger) (*OpenAPIManualExecWorkflowTaskV4Context, error) {
	task, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("find workflowTaskV4 error: %s", err)
		return nil, e.ErrGetTask.AddErr(err)
	}
	if task.ProjectName != projectKey {
		return nil, errors.Errorf("workflow task project %s does not match projectKey %s", task.ProjectName, projectKey)
	}

	return buildManualExecContext(task, executorID, isSystemAdmin)
}

func OpenAPIManualExecWorkflowTaskV4(workflowName, projectKey string, taskID int64, stageName, executorID, executorName string, isSystemAdmin bool, logger *zap.SugaredLogger) error {
	if stageName == "" {
		return errors.New("stage_name is required")
	}

	task, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("find workflowTaskV4 error: %s", err)
		return e.ErrGetTask.AddErr(err)
	}
	if task.ProjectName != projectKey {
		return errors.Errorf("workflow task project %s does not match projectKey %s", task.ProjectName, projectKey)
	}
	workflow, err := GetManualExecWorkflowTaskV4Info(workflowName, taskID, logger)
	if err != nil {
		return err
	}
	stage := findWorkflowStage(workflow.Stages, stageName)
	if stage == nil {
		return fmt.Errorf("stage %s not found in workflow %s", stageName, workflowName)
	}

	return ManualExecWorkflowTaskV4(workflowName, taskID, stageName, stage.Jobs, executorID, "", executorName, isSystemAdmin, logger)
}

func buildManualExecContext(task *commonmodels.WorkflowTask, executorID string, isSystemAdmin bool) (*OpenAPIManualExecWorkflowTaskV4Context, error) {
	if task == nil {
		return nil, errors.New("workflow task is required")
	}
	if task.Status != config.StatusPause {
		return nil, errors.New("工作流任务状态无法手动执行")
	}
	if task.OriginWorkflowArgs == nil || task.OriginWorkflowArgs.Stages == nil {
		return nil, errors.New("工作流任务数据异常, 无法手动执行")
	}

	stage := findPendingManualExecStage(task.Stages)
	if stage == nil {
		return nil, errors.New("workflow task has no stage waiting for manual execution")
	}

	return &OpenAPIManualExecWorkflowTaskV4Context{
		ProjectKey:  task.ProjectName,
		WorkflowKey: task.WorkflowName,
		TaskID:      task.TaskID,
		Status:      task.Status,
		StageName:   stage.Name,
		CanExecute:  canExecuteManualStage(stage.ManualExec.ManualExecUsers, task.TaskCreatorID, executorID, isSystemAdmin),
		Executors:   manualExecExecutors(stage.ManualExec.ManualExecUsers, task),
	}, nil
}

func validateManualExecRequest(task *commonmodels.WorkflowTask, stageName, executorID string, isSystemAdmin bool) error {
	context, err := buildManualExecContext(task, executorID, isSystemAdmin)
	if err != nil {
		return err
	}
	if context.StageName != stageName {
		return errors.Errorf("stage %s is not waiting for manual execution", stageName)
	}
	if !context.CanExecute {
		return errors.Errorf("user %s is not allowed to manually execute stage %s", executorID, stageName)
	}
	return nil
}

func findPendingManualExecStage(stages []*commonmodels.StageTask) *commonmodels.StageTask {
	for _, stage := range stages {
		if stage != nil && stage.Status == config.StatusPause && stage.ManualExec != nil && stage.ManualExec.Enabled && !stage.ManualExec.Excuted {
			return stage
		}
	}
	return nil
}

func findWorkflowStage(stages []*commonmodels.WorkflowStage, stageName string) *commonmodels.WorkflowStage {
	for _, stage := range stages {
		if stage != nil && stage.Name == stageName {
			return stage
		}
	}
	return nil
}

func canExecuteManualStage(users []*commonmodels.User, taskCreatorID, executorID string, isSystemAdmin bool) bool {
	if isSystemAdmin {
		return true
	}
	groups := make([]*commonmodels.User, 0)
	for _, user := range users {
		if user == nil {
			continue
		}
		switch user.Type {
		case setting.UserTypeTaskCreator:
			if executorID == taskCreatorID {
				return true
			}
		case setting.UserTypeUser, "":
			if executorID == user.UserID {
				return true
			}
		case setting.UserTypeGroup:
			groups = append(groups, user)
		}
	}
	flatUsers, _ := commonutil.GeneFlatUsers(groups)
	for _, user := range flatUsers {
		if user != nil && user.UserID == executorID {
			return true
		}
	}
	return false
}

func manualExecExecutors(users []*commonmodels.User, task *commonmodels.WorkflowTask) []*OpenAPIManualExecExecutor {
	executors := make([]*OpenAPIManualExecExecutor, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		executor := &OpenAPIManualExecExecutor{
			Type:      user.Type,
			UserID:    user.UserID,
			UserName:  user.UserName,
			GroupID:   user.GroupID,
			GroupName: user.GroupName,
		}
		if user.Type == "" {
			executor.Type = setting.UserTypeUser
		}
		if user.Type == setting.UserTypeTaskCreator {
			executor.UserID = task.TaskCreatorID
			executor.UserName = task.TaskCreator
		}
		executors = append(executors, executor)
	}
	return executors
}
