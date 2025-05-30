/*
Copyright 2025 The KodeRover Authors.

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

package migrate

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("3.5.0", "3.6.0", V350ToV360)
	upgradepath.RegisterHandler("3.6.0", "3.5.0", V360ToV350)
}

func V350ToV360() error {
	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	ctx := internalhandler.NewBackgroupContext()

	if !migrationInfo.WorkflowV4340HookMigration {
		workflowCursor, err := commonrepo.NewWorkflowV4Coll().ListByCursor(&commonrepo.ListWorkflowV4Option{})
		if err != nil {
			return fmt.Errorf("failed to list all custom workflow to update, error: %s", err)
		}
		for workflowCursor.Next(context.Background()) {
			workflow := new(commonmodels.WorkflowV4)
			if err := workflowCursor.Decode(workflow); err != nil {
				// continue converting to have maximum converage
				log.Warnf(err.Error())
				continue
			}

			log.Infof("migrating workflow: %s in project: %s ......", workflow.Name, workflow.Project)

			for _, hook := range workflow.HookCtls {
				gitHook := convertHookToGitHook(hook)
				_, err = commonrepo.NewWorkflowV4GitHookColl().Create(ctx, gitHook)
				if err != nil {
					log.Warnf("failed to create git hook: %s in project %s, workflow: %s, error: %s", gitHook.Name, gitHook.ProjectName, workflow.Name, err)
				}
			}

			err = commonrepo.NewWorkflowV4Coll().Update(
				workflow.ID.Hex(),
				workflow,
			)
			if err != nil {
				log.Warnf("failed to update workflow: %s in project %s, error: %s", workflow.Name, workflow.Project, err)
			}
			log.Infof("workflow: %s migration done ......", workflow.Name)
		}

		_ = mongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
			"workflow_v4_340_hook_migration": true,
		})
	}

	return nil
}

func convertHookToGitHook(hook *commonmodels.WorkflowV4Hook) *commonmodels.WorkflowV4GitHook {
	var workflowName, projectName string
	if hook.WorkflowArg != nil {
		workflowName = hook.WorkflowArg.Name
		projectName = hook.WorkflowArg.Project
	}

	gitHook := &commonmodels.WorkflowV4GitHook{
		Name:                hook.Name,
		WorkflowName:        workflowName,
		ProjectName:         projectName,
		AutoCancel:          hook.AutoCancel,
		CheckPatchSetChange: hook.CheckPatchSetChange,
		Enabled:             hook.Enabled,
		MainRepo:            hook.MainRepo,
		Description:         hook.Description,
		Repos:               hook.Repos,
		IsManual:            hook.IsManual,
		WorkflowArg:         hook.WorkflowArg,
	}

	return gitHook
}

func V360ToV350() error {
	return nil
}
