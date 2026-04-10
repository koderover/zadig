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
	"fmt"

	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	collaborationmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/repository/models"
	collaborationmongodb "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	usermodels "github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	userorm "github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	permissionservice "github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	pkgtypes "github.com/koderover/zadig/v2/pkg/types"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/util/sets"
)

type permissionActionSeed430 struct {
	Name     string
	Action   string
	Resource string
}

type permissionBackfillRule430 struct {
	Source  string
	Targets []string
}

type listActionBindings430 func(uint, *gorm.DB) ([]*usermodels.Action, error)
type createActionBindings430 func(uint, []uint, *gorm.DB) error

// 新版本新增的 action。历史实例升级时，先保证这些 action 已经存在，再去补 role/template 绑定。
var permissionActionSeeds430 = []permissionActionSeed430{
	{Name: "调整副本", Action: permissionservice.VerbScaleEnvironment, Resource: "Environment"},
	{Name: "调整副本", Action: permissionservice.VerbScaleProductionEnv, Resource: "ProductionEnvironment"},
}

// 回填规则只认历史权限：
// 旧环境管理权限 -> 新环境调整副本
// 旧生产环境管理权限 -> 新生产环境调整副本
var permissionBackfillRules430 = []permissionBackfillRule430{
	{
		Source: permissionservice.VerbManageEnvironment,
		Targets: []string{
			permissionservice.VerbScaleEnvironment,
		},
	},
	{
		Source: permissionservice.VerbEditProductionEnv,
		Targets: []string{
			permissionservice.VerbScaleProductionEnv,
		},
	},
}

// collaborationBackfillRules430 backfills collaboration scale permissions.
var collaborationBackfillRules430 = []permissionBackfillRule430{
	{
		Source: pkgtypes.EnvActionManagePod,
		Targets: []string{
			pkgtypes.EnvActionScale,
		},
	},
	{
		Source: pkgtypes.ProductionEnvActionManagePod,
		Targets: []string{
			pkgtypes.ProductionEnvActionScale,
		},
	},
}

func init() {
	upgradepath.RegisterHandler("4.2.1", "4.3.0", V421ToV430)
	upgradepath.RegisterHandler("4.3.0", "4.2.1", V430ToV421)
}

// V421ToV430 executes all 4.3.0 upgrade steps for user/collaboration permissions.
func V421ToV430() error {
	ctx := internalhandler.NewBackgroupContext()

	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	defer func() {
		updateMigrationError(migrationInfo.ID, err)
	}()

	// 这次迁移分三段：
	// 1. MySQL: user 表新增 api_token_enabled
	// 2. MySQL: permission action + role/template 绑定
	// 3. Mongo: collaboration mode / instance verbs
	err = migrateUserAPITokenEnabledColumn(ctx, migrationInfo)
	if err != nil {
		return err
	}

	err = migrateScalePermissions(migrationInfo)
	if err != nil {
		return err
	}

	err = migrateCollaborationScalePermissions(migrationInfo)
	if err != nil {
		return err
	}

	return nil
}

// migrateUserAPITokenEnabledColumn adds api_token_enabled column for user table.
func migrateUserAPITokenEnabledColumn(_ *internalhandler.Context, migrationInfo *internalmodels.Migration) error {
	if !migrationInfo.Migration430UserAPITokenEnabled {
		if !repository.DB.Migrator().HasColumn(&usermodels.User{}, "APITokenEnabled") {
			if err := repository.DB.Migrator().AddColumn(&usermodels.User{}, "APITokenEnabled"); err != nil {
				return fmt.Errorf("failed to add api_token_enabled column for user table, err: %s", err)
			}
		}
	}

	_ = internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration430UserAPITokenEnabled): true,
	})

	return nil
}

func migrateScalePermissions(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration430ScalePermission {
		return nil
	}

	tx := repository.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin migration 4.3.0 transaction, err: %s", tx.Error)
	}

	// 先补 action，再根据历史权限补 role 和 role template 绑定。
	actionIDs, err := ensurePermissionActions430(tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	templateCount, err := backfillRoleTemplatePermissions430(tx, actionIDs)
	if err != nil {
		tx.Rollback()
		return err
	}

	roleCount, err := backfillRolePermissions430(tx, actionIDs)
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit migration 4.3.0 permissions, err: %s", err)
	}

	log.Infof("migration 4.3.0 backfilled scale permissions for %d role templates and %d roles", templateCount, roleCount)

	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration430ScalePermission): true,
	})
}

// ensurePermissionActions430 ensures permission actions exist.
func ensurePermissionActions430(tx *gorm.DB) (map[string]uint, error) {
	actionIDs := make(map[string]uint, len(permissionActionSeeds430))
	for _, seed := range permissionActionSeeds430 {
		actionID, err := ensureAction430(tx, seed)
		if err != nil {
			return nil, err
		}
		actionIDs[seed.Action] = actionID
	}

	return actionIDs, nil
}

// ensureAction430 保证某个 action 在表里存在。
// 如果是重复执行迁移，会直接复用已有数据。
func ensureAction430(tx *gorm.DB, seed permissionActionSeed430) (uint, error) {
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

// backfillRoleTemplatePermissions430 backfills role template permissions
func backfillRoleTemplatePermissions430(tx *gorm.DB, actionIDs map[string]uint) (int, error) {
	roleTemplates, err := userorm.ListRoleTemplates(tx)
	if err != nil {
		return 0, fmt.Errorf("failed to list role templates, err: %s", err)
	}
	ids := make([]uint, 0, len(roleTemplates))
	for _, roleTemplate := range roleTemplates {
		ids = append(ids, roleTemplate.ID)
	}

	return backfillActionBindings430(tx, ids, actionIDs, userorm.ListActionByRoleTemplate, userorm.BulkCreateRoleTemplateActionBindings)
}

// backfillRolePermissions430 backfills role permissions
func backfillRolePermissions430(tx *gorm.DB, actionIDs map[string]uint) (int, error) {
	roles := make([]*usermodels.NewRole, 0)
	if err := tx.Where("namespace <> ?", permissionservice.GeneralNamespace).Find(&roles).Error; err != nil {
		return 0, fmt.Errorf("failed to list project roles, err: %s", err)
	}

	ids := make([]uint, 0, len(roles))
	for _, role := range roles {
		binding, err := userorm.GetRoleTemplateBindingByRoleID(role.ID, tx)
		if err != nil {
			return 0, fmt.Errorf("failed to query role template binding for role %d, err: %s", role.ID, err)
		}
		if binding != nil {
			continue
		}
		ids = append(ids, role.ID)
	}

	return backfillActionBindings430(tx, ids, actionIDs, userorm.ListActionByRole, userorm.BulkCreateRoleActionBindings)
}

func backfillActionBindings430(tx *gorm.DB, ids []uint, actionIDs map[string]uint, listActions listActionBindings430, createBindings createActionBindings430) (int, error) {
	updatedCount := 0
	for _, id := range ids {
		actions, err := listActions(id, tx)
		if err != nil {
			return updatedCount, fmt.Errorf("failed to list actions by id %d, err: %s", id, err)
		}

		missingActionIDs := collectMissingActionIDs430(actions, actionIDs)
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

func collectMissingActionIDs430(actions []*usermodels.Action, actionIDs map[string]uint) []uint {
	missingVerbs := collectMissingBackfillTargets430(actionVerbs430(actions))
	missingActionIDs := make([]uint, 0, len(missingVerbs))
	for _, verb := range missingVerbs {
		actionID, ok := actionIDs[verb]
		if ok {
			missingActionIDs = append(missingActionIDs, actionID)
		}
	}
	return missingActionIDs
}

func actionVerbs430(actions []*usermodels.Action) []string {
	verbs := make([]string, 0, len(actions))
	for _, action := range actions {
		verbs = append(verbs, action.Action)
	}
	return verbs
}

func collectMissingBackfillTargets430(verbs []string) []string {
	return collectMissingBackfillTargetsByRules430(verbs, permissionBackfillRules430)
}

func collectMissingBackfillTargetsByRules430(verbs []string, rules []permissionBackfillRule430) []string {
	verbSet := sets.NewString(verbs...)
	missingVerbs := make([]string, 0)
	for _, rule := range rules {
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

func migrateCollaborationScalePermissions(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration430CollaborationScalePermission {
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
		changed := appendBackfillTargetsToMode430(mode)

		revision := mode.Revision
		if changed {
			if err := modeColl.Update("system", mode); err != nil {
				return fmt.Errorf("failed to update collaboration mode %s/%s, err: %s", mode.ProjectName, mode.Name, err)
			}
			revision++
			modeUpdatedCount++
		}
		modeRevisionMap[collaborationModeKey430(mode.ProjectName, mode.Name)] = revision
	}

	instances, err := instanceColl.List(&collaborationmongodb.CollaborationInstanceFindOptions{})
	if err != nil {
		return fmt.Errorf("failed to list collaboration instances, err: %s", err)
	}

	instanceUpdatedCount := 0
	for _, instance := range instances {
		changed := appendBackfillTargetsToInstance430(instance)

		if revision, ok := modeRevisionMap[collaborationModeKey430(instance.ProjectName, instance.CollaborationName)]; ok && instance.Revision != revision {
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

	log.Infof("migration 4.3.0 backfilled collaboration scale permissions for %d modes and %d instances", modeUpdatedCount, instanceUpdatedCount)

	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration430CollaborationScalePermission): true,
	})
}

func collaborationModeKey430(projectName, modeName string) string {
	return fmt.Sprintf("%s/%s", projectName, modeName)
}

func appendBackfillTargets430(verbs []string) (bool, []string) {
	missingVerbs := collectMissingBackfillTargetsByRules430(verbs, collaborationBackfillRules430)
	if len(missingVerbs) == 0 {
		return false, verbs
	}

	updatedVerbs := append([]string{}, verbs...)
	updatedVerbs = append(updatedVerbs, missingVerbs...)
	return true, updatedVerbs
}

func appendBackfillTargetsToMode430(mode *collaborationmodels.CollaborationMode) bool {
	changed := false
	for i := range mode.Workflows {
		itemChanged, verbs := appendBackfillTargets430(mode.Workflows[i].Verbs)
		if itemChanged {
			mode.Workflows[i].Verbs = verbs
			changed = true
		}
	}
	for i := range mode.Products {
		itemChanged, verbs := appendBackfillTargets430(mode.Products[i].Verbs)
		if itemChanged {
			mode.Products[i].Verbs = verbs
			changed = true
		}
	}
	return changed
}

// appendBackfillTargetsToInstance430 backfills collaboration scale permissions for instance
func appendBackfillTargetsToInstance430(instance *collaborationmodels.CollaborationInstance) bool {
	changed := false
	for i := range instance.Workflows {
		itemChanged, verbs := appendBackfillTargets430(instance.Workflows[i].Verbs)
		if itemChanged {
			instance.Workflows[i].Verbs = verbs
			changed = true
		}
	}
	for i := range instance.Products {
		itemChanged, verbs := appendBackfillTargets430(instance.Products[i].Verbs)
		if itemChanged {
			instance.Products[i].Verbs = verbs
			changed = true
		}
	}
	return changed
}

func V430ToV421() error {
	return nil
}
