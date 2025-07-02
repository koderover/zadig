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

	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func init() {
	upgradepath.RegisterHandler("3.4.0", "3.4.1", V340ToV341)
	upgradepath.RegisterHandler("3.4.1", "3.4.0", V341ToV340)
}

func V340ToV341() error {
	ctx := internalhandler.NewBackgroupContext()

	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	if !migrationInfo.UpdateLarkEventSetting {
		imApps, err := commonrepo.NewIMAppColl().List(context.TODO(), setting.IMLark)
		if err != nil {
			return fmt.Errorf("failed to list all custom workflow to update, error: %s", err)
		}

		for _, app := range imApps {
			app.LarkEventType = setting.LarkEventTypeCallback
			err = commonrepo.NewIMAppColl().Update(context.TODO(), app.ID.Hex(), app)
			if err != nil {
				return fmt.Errorf("failed to update lark app event type, error: %s", err)
			}
		}

		_ = mongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
			"update_lark_event_setting": true,
		})
	}

	err = migrate341WorkflowV4Hooks(ctx, migrationInfo)
	if err != nil {
		return fmt.Errorf("failed to migrate workflow v4 hooks for 3.4.1")
	}

	err = migrate341VMDeploy(ctx, migrationInfo)
	if err != nil {
		return fmt.Errorf("failed to migrate vm deploy for 3.4.1")
	}

	return nil
}

func V341ToV340() error {
	return nil
}

func migrate341WorkflowV4Hooks(ctx *internalhandler.Context, migrationInfo *internalmodels.Migration) error {
	if !migrationInfo.WorkflowV4341HookMigration {
		workflowCursor, err := commonrepo.NewWorkflowV4Coll().ListByCursor(&commonrepo.ListWorkflowV4Option{})
		if err != nil {
			return fmt.Errorf("failed to list all custom workflow to update, error: %s", err)
		}
		for workflowCursor.Next(context.Background()) {
			workflow := new(commonmodels.WorkflowV4)
			if err := workflowCursor.Decode(workflow); err != nil {
				// continue converting to have maximum converage
				log.Error(err)
				return err
			}

			updated := false

			// git hook migration
			for _, hook := range workflow.HookCtls {
				gitHook := convertHookToGitHook(hook)
				_, err = commonrepo.NewWorkflowV4GitHookColl().Create(ctx, gitHook)
				if err != nil {
					err = fmt.Errorf("failed to create git hook: %s in project %s, workflow: %s, error: %s", gitHook.Name, gitHook.ProjectName, workflow.Name, err)
					log.Error(err)
					return err
				}
				updated = true
			}
			workflow.HookCtls = nil

			// general hook migration
			for _, hook := range workflow.GeneralHookCtls {
				generalHook := convertHookToGeneralHook(hook)
				_, err = commonrepo.NewWorkflowV4GeneralHookColl().Create(ctx, generalHook)
				if err != nil {
					err = fmt.Errorf("failed to create general hook: %s in project %s, workflow: %s, error: %s", generalHook.Name, generalHook.ProjectName, workflow.Name, err)
					log.Error(err)
					return err
				}
				updated = true
			}
			workflow.GeneralHookCtls = nil

			// meego hook migration
			for _, hook := range workflow.MeegoHookCtls {
				meegoHook := convertHookToMeegoHook(hook)
				_, err = commonrepo.NewWorkflowV4MeegoHookColl().Create(ctx, meegoHook)
				if err != nil {
					err = fmt.Errorf("failed to create meego hook: %s in project %s, workflow: %s, error: %s", meegoHook.Name, meegoHook.ProjectName, workflow.Name, err)
					log.Error(err)
					return err
				}
				updated = true
			}
			workflow.MeegoHookCtls = nil

			// jira hook migration
			for _, hook := range workflow.JiraHookCtls {
				jiraHook := convertHookToJiraHook(hook)
				_, err = commonrepo.NewWorkflowV4JiraHookColl().Create(ctx, jiraHook)
				if err != nil {
					err = fmt.Errorf("failed to create jira hook: %s in project %s, workflow: %s, error: %s", jiraHook.Name, jiraHook.ProjectName, workflow.Name, err)
					log.Error(err)
					return err
				}
				updated = true
			}
			workflow.JiraHookCtls = nil

			if updated {
				err = commonrepo.NewWorkflowV4Coll().Update(
					workflow.ID.Hex(),
					workflow,
				)
				if err != nil {
					err = fmt.Errorf("failed to update workflow: %s in project %s, error: %s", workflow.Name, workflow.Project, err)
					log.Error(err)
					return err
				}
				log.Infof("workflow: %s(%s) migration done ......", workflow.DisplayName, workflow.Name)
			}
		}

		_ = mongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
			"workflow_v4_341_hook_migration": true,
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

func convertHookToGeneralHook(hook *commonmodels.GeneralHook) *commonmodels.WorkflowV4GeneralHook {
	var workflowName, projectName string
	if hook.WorkflowArg != nil {
		workflowName = hook.WorkflowArg.Name
		projectName = hook.WorkflowArg.Project
	}

	generalHook := &commonmodels.WorkflowV4GeneralHook{
		Name:         hook.Name,
		WorkflowName: workflowName,
		ProjectName:  projectName,
		Enabled:      hook.Enabled,
		Description:  hook.Description,
		WorkflowArg:  hook.WorkflowArg,
	}

	return generalHook
}

func convertHookToJiraHook(hook *commonmodels.JiraHook) *commonmodels.WorkflowV4JiraHook {
	var workflowName, projectName string
	if hook.WorkflowArg != nil {
		workflowName = hook.WorkflowArg.Name
		projectName = hook.WorkflowArg.Project
	}

	jiraHook := &commonmodels.WorkflowV4JiraHook{
		Name:                     hook.Name,
		WorkflowName:             workflowName,
		ProjectName:              projectName,
		Enabled:                  hook.Enabled,
		Description:              hook.Description,
		JiraID:                   hook.JiraID,
		JiraSystemIdentity:       hook.JiraSystemIdentity,
		JiraURL:                  hook.JiraURL,
		EnabledIssueStatusChange: hook.EnabledIssueStatusChange,
		FromStatus:               hook.FromStatus,
		ToStatus:                 hook.ToStatus,
		WorkflowArg:              hook.WorkflowArg,
	}

	return jiraHook
}

func convertHookToMeegoHook(hook *commonmodels.MeegoHook) *commonmodels.WorkflowV4MeegoHook {
	var workflowName, projectName string
	if hook.WorkflowArg != nil {
		workflowName = hook.WorkflowArg.Name
		projectName = hook.WorkflowArg.Project
	}

	meegoHook := &commonmodels.WorkflowV4MeegoHook{
		Name:                hook.Name,
		WorkflowName:        workflowName,
		ProjectName:         projectName,
		Enabled:             hook.Enabled,
		Description:         hook.Description,
		MeegoID:             hook.MeegoID,
		MeegoURL:            hook.MeegoURL,
		MeegoSystemIdentity: hook.MeegoSystemIdentity,
		WorkflowArg:         hook.WorkflowArg,
	}

	return meegoHook
}

func migrate341VMDeploy(ctx *internalhandler.Context, migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration341VMDeploy {
		return nil
	}

	vmProjects, err := templaterepo.NewProductColl().ListWithOption(&templaterepo.ProductListOpt{
		BasicFacility: setting.BasicFacilityCVM,
	})
	if err != nil {
		return fmt.Errorf("failed to list all vm projects to migrate, error: %s", err)
	}

	for _, vmProject := range vmProjects {
		// migrate vm build
		builds, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{
			ProductName: vmProject.ProductName,
		})
		if err != nil {
			return fmt.Errorf("failed to list all builds to migrate, error: %s", err)
		}

		buildMap := make(map[string]*commonmodels.Build)
		for _, build := range builds {
			for _, target := range build.Targets {
				buildMap[target.ServiceName] = build
			}
		}

		for _, build := range builds {
			build.DeployArtifactType = types.VMDeployArtifactTypeFile

			if len(build.SSHs) > 0 {
				build.DeployType = types.VMDeployTypeLocal
			} else {
				build.DeployType = types.VMDeployTypeSSHAgent
			}

			err = commonrepo.NewBuildColl().Update(build)
			if err != nil {
				return fmt.Errorf("failed to update build: %s, project: %s, error: %s", build.Name, vmProject.ProductName, err)
			}
		}

		// migrate vm deploy job
		workflowCursor, err := commonrepo.NewWorkflowV4Coll().ListByCursor(&commonrepo.ListWorkflowV4Option{ProjectName: vmProject.ProductName})
		if err != nil {
			return fmt.Errorf("failed to list all custom workflow to update, error: %s", err)
		}
		for workflowCursor.Next(context.Background()) {
			workflow := new(commonmodels.WorkflowV4)
			if err := workflowCursor.Decode(workflow); err != nil {
				// continue converting to have maximum converage
				log.Warnf(err.Error())
			}

			for _, stage := range workflow.Stages {
				for _, job := range stage.Jobs {
					if job.JobType == config.JobZadigVMDeploy {
						newSpec := new(commonmodels.ZadigVMDeployJobSpec)
						if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
							return fmt.Errorf("failed to decode zadig vm deploy job, error: %s", err)
						}

						newSpec.DefaultServiceAndVMDeploys = make([]*commonmodels.ServiceAndVMDeploy, 0)
						for _, svc := range newSpec.ServiceAndVMDeploys {
							if build, ok := buildMap[svc.ServiceName]; ok {
								svc.ServiceModule = svc.ServiceName
								svc.DeployName = build.Name
								svc.DeployArtifactType = build.DeployArtifactType
							}
							newSpec.DefaultServiceAndVMDeploys = append(newSpec.DefaultServiceAndVMDeploys, svc)
						}
						newSpec.ServiceAndVMDeploysOptions = newSpec.DefaultServiceAndVMDeploys
						newSpec.ServiceAndVMDeploys = make([]*commonmodels.ServiceAndVMDeploy, 0)
						job.Spec = newSpec
					}
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
	}

	_ = mongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		"migration_341_vm_deploy": true,
	})

	return nil
}
