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

package migrate

import (
	"context"
	"fmt"
	"strings"

	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	internalmongodb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	collaborationmongodb "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/repository/mongodb"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	userrepo "github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	usermodels "github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	userorm "github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	permissionservice "github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	pkgtypes "github.com/koderover/zadig/v2/pkg/types"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/util/sets"
)

type permissionActionSeed struct {
	Name     string
	Action   string
	Resource string
}

type permissionBackfillRule struct {
	Source  string
	Targets []string
}

type listActionBindings421 func(uint, *gorm.DB) ([]*usermodels.Action, error)
type createActionBindings421 func(uint, []uint, *gorm.DB) error

// 新版本新增的 action。历史实例升级时，先保证这些 action 已经存在，再去补 role/template 绑定。
var permissionActionSeeds421 = []permissionActionSeed{
	{Name: "回滚", Action: permissionservice.VerbRollbackWorkflow, Resource: "Workflow"},
	{Name: "重启", Action: permissionservice.VerbRestartEnvironment, Resource: "Environment"},
	{Name: "回滚", Action: permissionservice.VerbRollbackEnvironment, Resource: "Environment"},
	{Name: "重启", Action: permissionservice.VerbRestartProductionEnv, Resource: "ProductionEnvironment"},
	{Name: "回滚", Action: permissionservice.VerbRollbackProductionEnv, Resource: "ProductionEnvironment"},
}

// 回填规则只认历史权限：
// 旧环境管理权限 -> 新环境重启/回滚
// 旧工作流执行权限 -> 新工作流回滚
var permissionBackfillRules421 = []permissionBackfillRule{
	{
		Source: permissionservice.VerbManageEnvironment,
		Targets: []string{
			permissionservice.VerbRestartEnvironment,
			permissionservice.VerbRollbackEnvironment,
		},
	},
	{
		Source: permissionservice.VerbEditProductionEnv,
		Targets: []string{
			permissionservice.VerbRestartProductionEnv,
			permissionservice.VerbRollbackProductionEnv,
		},
	},
	{
		Source: permissionservice.VerbRunWorkflow,
		Targets: []string{
			permissionservice.VerbRollbackWorkflow,
		},
	},
}

func init() {
	upgradepath.RegisterHandler("4.2.0", "4.2.1", V420ToV421)
	upgradepath.RegisterHandler("4.2.1", "4.2.0", V421ToV420)
}

func V420ToV421() error {
	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	defer func() {
		updateMigrationError(migrationInfo.ID, err)
	}()

	// 这次迁移分两段：
	// 1. MySQL: action + role/template 绑定
	// 2. Mongo: collaboration mode / instance verbs
	if err = migrateRollbackPermissions(migrationInfo); err != nil {
		return err
	}

	if err = migrateCollaborationRollbackPermissions(migrationInfo); err != nil {
		return err
	}

	if !migrationInfo.Migration421WorkflowDeploySpec {
		workflowCursor, err := commonrepo.NewWorkflowV4Coll().ListByCursor(&commonrepo.ListWorkflowV4Option{})
		if err != nil {
			return fmt.Errorf("failed to list all custom workflow to update, error: %s", err)
		}

		for workflowCursor.Next(context.Background()) {
			workflow := new(commonmodels.WorkflowV4)
			if err := workflowCursor.Decode(workflow); err != nil {
				// continue converting to have maximum coverage
				log.Warnf(err.Error())
			}

			log.Infof("migrating workflow: %s in project: %s ......", workflow.Name, workflow.Project)

			err = update421WorkflowDeployJobTaskSpec(workflow.Stages)
			if err != nil {
				// continue converting to have maximum coverage
				log.Warnf(err.Error())
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
			"migration_421_workflow_deploy_spec": true,
		})
	}

	return nil
}

func V421ToV420() error {
	return nil
}

func migrateRollbackPermissions(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration421RollbackPermission {
		return nil
	}

	tx := userrepo.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin migration 4.2.1 transaction, err: %s", tx.Error)
	}

	// 先补 action，再根据历史权限补 role 和 role template 绑定。
	actionIDs, err := ensurePermissionActions421(tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	templateCount, err := backfillRoleTemplatePermissions421(tx, actionIDs)
	if err != nil {
		tx.Rollback()
		return err
	}

	roleCount, err := backfillRolePermissions421(tx, actionIDs)
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit migration 4.2.1 permissions, err: %s", err)
	}

	log.Infof("migration 4.2.1 backfilled split permissions for %d role templates and %d roles", templateCount, roleCount)

	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration421RollbackPermission): true,
	})
}

func ensurePermissionActions421(tx *gorm.DB) (map[string]uint, error) {
	actionIDs := make(map[string]uint, len(permissionActionSeeds421))
	for _, seed := range permissionActionSeeds421 {
		actionID, err := ensureAction421(tx, seed)
		if err != nil {
			return nil, err
		}
		actionIDs[seed.Action] = actionID
	}

	return actionIDs, nil
}

// ensureAction421 保证某个 action 在表里存在。
// 如果是重复执行迁移，会直接复用已有数据。
func ensureAction421(tx *gorm.DB, seed permissionActionSeed) (uint, error) {
	action, err := userorm.GetActionByVerb(seed.Action, tx)
	if err != nil {
		return 0, fmt.Errorf("failed to query action %s, err: %s", seed.Action, err)
	}
	if action != nil && action.ID != 0 {
		return action.ID, nil
	}

	action = &usermodels.Action{
		Name:     seed.Name,
		Action:   seed.Action,
		Resource: seed.Resource,
		Scope:    pkgtypes.DBProjectScope,
	}
	if err := userorm.CreateAction(action, tx); err != nil {
		action, err = userorm.GetActionByVerb(seed.Action, tx)
		if err != nil {
			return 0, fmt.Errorf("failed to create action %s, err: %s", seed.Action, err)
		}
	}

	if action == nil || action.ID == 0 {
		return 0, fmt.Errorf("action %s still missing after migration", seed.Action)
	}

	return action.ID, nil
}

func update421WorkflowDeployJobTaskSpec(stages []*commonmodels.WorkflowStage) error {
	for _, stage := range stages {
		for _, job := range stage.Jobs {
			switch job.JobType {
			case config.JobZadigDeploy:
				newSpec := new(commonmodels.ZadigDeployJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				if strings.HasPrefix(newSpec.Env, "<+fixed>") {
					newSpec.Env = strings.TrimPrefix(newSpec.Env, "<+fixed>")
					newSpec.EnvSource = config.ParamSourceFixed
				} else {
					newSpec.EnvSource = config.ParamSourceRuntime
				}
				newSpec.YAMLMergeStrategy = config.YAMLMergeStrategyMerge
				newSpec.MergeStrategySource = config.ParamSourceFixed
				job.Spec = newSpec
			default:
			}
		}
	}
	return nil
}

func backfillRoleTemplatePermissions421(tx *gorm.DB, actionIDs map[string]uint) (int, error) {
	roleTemplates, err := userorm.ListRoleTemplates(tx)
	if err != nil {
		return 0, fmt.Errorf("failed to list role templates, err: %s", err)
	}

	ids := make([]uint, 0, len(roleTemplates))
	for _, roleTemplate := range roleTemplates {
		ids = append(ids, roleTemplate.ID)
	}

	return backfillActionBindings421(tx, ids, actionIDs, userorm.ListActionByRoleTemplate, userorm.BulkCreateRoleTemplateActionBindings)
}

func backfillRolePermissions421(tx *gorm.DB, actionIDs map[string]uint) (int, error) {
	roles := make([]*usermodels.NewRole, 0)
	if err := tx.Where("namespace <> ?", permissionservice.GeneralNamespace).Find(&roles).Error; err != nil {
		return 0, fmt.Errorf("failed to list project roles, err: %s", err)
	}

	ids := make([]uint, 0, len(roles))
	for _, role := range roles {
		// 绑定了 role template 的项目角色，权限来源于 template，本身不重复补绑定。
		binding, err := userorm.GetRoleTemplateBindingByRoleID(role.ID, tx)
		if err != nil {
			return 0, fmt.Errorf("failed to query role template binding for role %d, err: %s", role.ID, err)
		}
		if binding != nil {
			continue
		}
		ids = append(ids, role.ID)
	}

	return backfillActionBindings421(tx, ids, actionIDs, userorm.ListActionByRole, userorm.BulkCreateRoleActionBindings)
}

// backfillActionBindings421 复用同一套“旧权限 -> 新权限”规则，
// 分别处理 role_template_action_binding 和 role_action_binding。
func backfillActionBindings421(tx *gorm.DB, ids []uint, actionIDs map[string]uint, listActions listActionBindings421, createBindings createActionBindings421) (int, error) {
	updatedCount := 0
	for _, id := range ids {
		actions, err := listActions(id, tx)
		if err != nil {
			return updatedCount, fmt.Errorf("failed to list actions by id %d, err: %s", id, err)
		}

		missingActionIDs := collectMissingActionIDs421(actions, actionIDs)
		if len(missingActionIDs) == 0 {
			continue
		}

		if err := createBindings(id, missingActionIDs, tx); err != nil {
			return updatedCount, fmt.Errorf("failed to backfill action bindings by id %d, err: %s", id, err)
		}
		updatedCount++
	}

	return updatedCount, nil
}

// 先把当前实体已有的 action 转成 verb，再套统一的回填规则，最后映射回 action id。
func collectMissingActionIDs421(actions []*usermodels.Action, actionIDs map[string]uint) []uint {
	missingVerbs := collectMissingBackfillTargets421(actionVerbs421(actions))
	missingActionIDs := make([]uint, 0, len(missingVerbs))
	for _, verb := range missingVerbs {
		actionID, ok := actionIDs[verb]
		if ok {
			missingActionIDs = append(missingActionIDs, actionID)
		}
	}
	return missingActionIDs
}

func actionVerbs421(actions []*usermodels.Action) []string {
	verbs := make([]string, 0, len(actions))
	for _, action := range actions {
		verbs = append(verbs, action.Action)
	}
	return verbs
}

func migrateCollaborationRollbackPermissions(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration421CollaborationRollbackPermission {
		return nil
	}

	modeColl := collaborationmongodb.NewCollaborationModeColl()
	instanceColl := collaborationmongodb.NewCollaborationInstanceColl()

	modes, err := modeColl.List(&collaborationmongodb.CollaborationModeListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list collaboration modes, err: %s", err)
	}

	modeRevisionMap := make(map[string]int64, len(modes))
	modeUpdatedCount := 0
	for _, mode := range modes {
		changed := false
		for i := range mode.Workflows {
			itemChanged, verbs := appendBackfillTargets421(mode.Workflows[i].Verbs)
			if itemChanged {
				mode.Workflows[i].Verbs = verbs
				changed = true
			}
		}
		for i := range mode.Products {
			itemChanged, verbs := appendBackfillTargets421(mode.Products[i].Verbs)
			if itemChanged {
				mode.Products[i].Verbs = verbs
				changed = true
			}
		}

		revision := mode.Revision
		if changed {
			// mode 更新时 revision 会递增，后面 instance 需要对齐到这个 revision，
			// 否则协作实例同步逻辑会认为版本倒挂。
			if err := modeColl.Update("system", mode); err != nil {
				return fmt.Errorf("failed to update collaboration mode %s/%s, err: %s", mode.ProjectName, mode.Name, err)
			}
			revision++
			modeUpdatedCount++
		}
		modeRevisionMap[collaborationModeKey(mode.ProjectName, mode.Name)] = revision
	}

	instances, err := instanceColl.List(&collaborationmongodb.CollaborationInstanceFindOptions{})
	if err != nil {
		return fmt.Errorf("failed to list collaboration instances, err: %s", err)
	}

	instanceUpdatedCount := 0
	for _, instance := range instances {
		changed := false
		for i := range instance.Workflows {
			itemChanged, verbs := appendBackfillTargets421(instance.Workflows[i].Verbs)
			if itemChanged {
				instance.Workflows[i].Verbs = verbs
				changed = true
			}
		}
		for i := range instance.Products {
			itemChanged, verbs := appendBackfillTargets421(instance.Products[i].Verbs)
			if itemChanged {
				instance.Products[i].Verbs = verbs
				changed = true
			}
		}

		// instance 即使 verbs 没变化，也要把 revision 对齐到最新 mode。
		if revision, ok := modeRevisionMap[collaborationModeKey(instance.ProjectName, instance.CollaborationName)]; ok && instance.Revision != revision {
			instance.Revision = revision
			changed = true
		}

		if !changed {
			continue
		}

		if err := instanceColl.Update(instance.UserUID, instance); err != nil {
			return fmt.Errorf("failed to update collaboration instance %s/%s/%s, err: %s", instance.ProjectName, instance.CollaborationName, instance.UserUID, err)
		}
		instanceUpdatedCount++
	}

	log.Infof("migration 4.2.1 backfilled collaboration permissions for %d modes and %d instances", modeUpdatedCount, instanceUpdatedCount)

	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration421CollaborationRollbackPermission): true,
	})
}

func collaborationModeKey(projectName, modeName string) string {
	return fmt.Sprintf("%s/%s", projectName, modeName)
}

// appendBackfillTargets421 返回补齐后的 verbs。没有缺失项时直接复用原切片。
func appendBackfillTargets421(verbs []string) (bool, []string) {
	missingVerbs := collectMissingBackfillTargets421(verbs)
	if len(missingVerbs) == 0 {
		return false, verbs
	}

	updatedVerbs := append([]string{}, verbs...)
	updatedVerbs = append(updatedVerbs, missingVerbs...)
	return true, updatedVerbs
}

// collectMissingBackfillTargets421 是整份迁移的核心规则计算：
// 只要发现旧权限存在，就把当前缺失的新权限补出来。
func collectMissingBackfillTargets421(verbs []string) []string {
	verbSet := sets.NewString(verbs...)
	missingVerbs := make([]string, 0)
	for _, rule := range permissionBackfillRules421 {
		if !verbSet.Has(rule.Source) {
			continue
		}
		for _, target := range rule.Targets {
			if verbSet.Has(target) {
				continue
			}
			missingVerbs = append(missingVerbs, target)
			verbSet.Insert(target)
		}
	}
	return missingVerbs
}
