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
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	usermodels "github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	userorm "github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	permissionservice "github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	pkgtypes "github.com/koderover/zadig/v2/pkg/types"
	"gorm.io/gorm"
)

type permissionActionSeed500 struct {
	Name     string
	Action   string
	Resource string
	Scope    int
}

// 5.0.0 新增系统操作日志查看权限。
// 历史实例升级时，除保证 action 数据存在外，还会给 global_read_only 角色补齐该只读权限。
var permissionActionSeeds500 = []permissionActionSeed500{
	{Name: "查看", Action: permissionservice.VerbGetLogOperation, Resource: "LogOperation", Scope: pkgtypes.DBSystemScope},
}

func init() {
	upgradepath.RegisterHandler("4.3.0", "5.0.0", V430ToV500)
	upgradepath.RegisterHandler("5.0.0", "4.3.0", V500ToV430)
}

// V430ToV500 executes 5.0.0 upgrade steps for permission metadata.
func V430ToV500() error {
	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	defer func() {
		updateMigrationError(migrationInfo.ID, err)
	}()

	err = migrateLogOperationPermission500(migrationInfo)
	if err != nil {
		return err
	}

	return nil
}

func migrateLogOperationPermission500(migrationInfo *internalmodels.Migration) error {
	alreadyMigrated := migrationInfo.Migration500LogOperationPermission

	tx := repository.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin migration 5.0.0 transaction, err: %s", tx.Error)
	}

	actionIDMap, err := ensurePermissionActions500(tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 回填global readonly role 的 操作日志权限
	roleCount, err := backfillGlobalReadOnlyRolePermissions500(tx, actionIDMap)
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit migration 5.0.0 permissions, err: %s", err)
	}

	log.Infof("migration 5.0.0 ensured log operation permission actions and backfilled %d global_read_only roles", roleCount)

	if alreadyMigrated {
		return nil
	}

	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration500LogOperationPermission): true,
	})
}

func ensurePermissionActions500(tx *gorm.DB) (map[string]uint, error) {
	actionIDs := make(map[string]uint, len(permissionActionSeeds500))
	for _, seed := range permissionActionSeeds500 {
		actionID, err := ensureAction500(tx, seed)
		if err != nil {
			return nil, err
		}
		actionIDs[seed.Action] = actionID
	}

	return actionIDs, nil
}

// ensureAction500 guarantees the target action exists and supports repeated execution.
func ensureAction500(tx *gorm.DB, seed permissionActionSeed500) (uint, error) {
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
		Scope:    seed.Scope,
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

func backfillGlobalReadOnlyRolePermissions500(tx *gorm.DB, actionIDMap map[string]uint) (int, error) {
	logActionID, ok := actionIDMap[permissionservice.VerbGetLogOperation]
	if !ok || logActionID == 0 {
		return 0, fmt.Errorf("action id for %s is missing", permissionservice.VerbGetLogOperation)
	}

	roles, err := userorm.ListRoleByNamespace(permissionservice.GeneralNamespace, tx)
	if err != nil {
		return 0, fmt.Errorf("failed to list system roles for log operation permission backfill, err: %s", err)
	}

	updatedCount := 0
	for _, role := range roles {
		if role == nil || role.ID == 0 {
			continue
		}
		if !role.GlobalReadOnly {
			continue
		}

		actions, err := userorm.ListActionByRole(role.ID, tx)
		if err != nil {
			return updatedCount, fmt.Errorf("failed to list actions for role %d, err: %s", role.ID, err)
		}

		hasLogOperationPermission := false
		for _, action := range actions {
			if action != nil && action.ID == logActionID {
				hasLogOperationPermission = true
				break
			}
		}
		if hasLogOperationPermission {
			continue
		}

		if err := userorm.BulkCreateRoleActionBindings(role.ID, []uint{logActionID}, tx); err != nil {
			return updatedCount, fmt.Errorf("failed to add %s permission for role %d, err: %s", permissionservice.VerbGetLogOperation, role.ID, err)
		}
		updatedCount++
	}

	return updatedCount, nil
}

func V500ToV430() error {
	return nil
}
