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
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	permissionservice "github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	pkgtypes "github.com/koderover/zadig/v2/pkg/types"
)

type permissionActionSeed430 struct {
	Name     string
	Action   string
	Resource string
	Scope    int
}

var businessDirectoryActionSeeds430 = []permissionActionSeed430{
	{Name: "查看", Action: permissionservice.VerbGetBusinessDirectory, Resource: "BusinessDirectory", Scope: pkgtypes.DBSystemScope},
	{Name: "新建", Action: permissionservice.VerbCreateBusinessDirectory, Resource: "BusinessDirectory", Scope: pkgtypes.DBSystemScope},
	{Name: "编辑", Action: permissionservice.VerbEditBusinessDirectory, Resource: "BusinessDirectory", Scope: pkgtypes.DBSystemScope},
	{Name: "删除", Action: permissionservice.VerbDeleteBusinessDirectory, Resource: "BusinessDirectory", Scope: pkgtypes.DBSystemScope},
}

func init() {
	upgradepath.RegisterHandler("4.2.1", "4.3.0", V421ToV430)
	upgradepath.RegisterHandler("4.3.0", "4.2.1", V430ToV421)
}

func V421ToV430() error {
	ctx := internalhandler.NewBackgroupContext()

	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	defer func() {
		updateMigrationError(migrationInfo.ID, err)
	}()

	err = migrateUserAPITokenEnabledColumn(ctx, migrationInfo)
	if err != nil {
		return err
	}

	err = migrateGlobalReadOnlyRole(ctx, migrationInfo)
	if err != nil {
		return err
	}

	return nil
}

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

// check global read only role column
func migrateGlobalReadOnlyRole(_ *internalhandler.Context, migrationInfo *internalmodels.Migration) error {
	if !migrationInfo.Migration430GlobalReadOnlyRole {
		if !repository.DB.Migrator().HasColumn(&usermodels.NewRole{}, "GlobalReadOnly") {
			if err := repository.DB.Migrator().AddColumn(&usermodels.NewRole{}, "GlobalReadOnly"); err != nil {
				return fmt.Errorf("failed to add global_read_only column for role table, err: %s", err)
			}
		}
	}

	// write globalreadonly role into system roles
	err := backfillGlobalReadOnlyRole()
	if err != nil {
		return err
	}
	// Ensure business-directory actions exist for upgraded instances.
	if err := ensureBusinessDirectoryActions430(); err != nil {
		return err
	}
	// Fallback backfill:
	// - if a role already has get_business_directory, append create/edit/delete
	if err := backfillBusinessDirectoryRolePermissions430(); err != nil {
		return err
	}

	_ = internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration430GlobalReadOnlyRole): true,
	})

	return nil
}

func ensureBusinessDirectoryActions430() error {
	tx := repository.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin tx for business directory action migration, err: %s", tx.Error)
	}

	for _, seed := range businessDirectoryActionSeeds430 {
		action, err := orm.GetActionByVerb(seed.Action, tx)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to query action %s, err: %s", seed.Action, err)
		}
		if action != nil && action.ID != 0 {
			continue
		}

		action = &usermodels.Action{
			Name:     seed.Name,
			Action:   seed.Action,
			Resource: seed.Resource,
			Scope:    seed.Scope,
		}
		if err := orm.CreateAction(action, tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create action %s, err: %s", seed.Action, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit business directory action migration tx, err: %s", err)
	}
	return nil
}

// backfillBusinessDirectoryRolePermissions430 provides a migration fallback for
// historical system roles:
// 1) If a role already has get_business_directory, only append write verbs.
func backfillBusinessDirectoryRolePermissions430() error {
	tx := repository.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin tx for business directory permission backfill, err: %s", tx.Error)
	}

	actionIDMap := make(map[string]uint, len(businessDirectoryActionSeeds430))
	for _, seed := range businessDirectoryActionSeeds430 {
		action, err := orm.GetActionByVerb(seed.Action, tx)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to query action %s for backfill, err: %s", seed.Action, err)
		}
		if action == nil || action.ID == 0 {
			tx.Rollback()
			return fmt.Errorf("action %s is missing while backfilling business directory permissions", seed.Action)
		}
		actionIDMap[seed.Action] = action.ID
	}

	roles, err := orm.ListRoleByNamespace(permissionservice.GeneralNamespace, tx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to list system roles for business directory backfill, err: %s", err)
	}

	for _, role := range roles {
		if role == nil || role.ID == 0 {
			continue
		}
		// Keep global-read-only role as readonly.
		if role.GlobalReadOnly {
			continue
		}

		actions, err := orm.ListActionByRole(role.ID, tx)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to list actions for role %d during business directory backfill, err: %s", role.ID, err)
		}

		existingVerbs := map[string]struct{}{}
		for _, action := range actions {
			if action == nil {
				continue
			}
			existingVerbs[action.Action] = struct{}{}
		}

		// Only backfill write verbs for roles that already have get_business_directory.
		if _, hasGet := existingVerbs[permissionservice.VerbGetBusinessDirectory]; !hasGet {
			continue
		}
		targetVerbs := []string{
			permissionservice.VerbCreateBusinessDirectory,
			permissionservice.VerbEditBusinessDirectory,
			permissionservice.VerbDeleteBusinessDirectory,
		}

		missingActionIDs := make([]uint, 0)
		for _, verb := range targetVerbs {
			if _, ok := existingVerbs[verb]; ok {
				continue
			}
			if actionID, ok := actionIDMap[verb]; ok {
				missingActionIDs = append(missingActionIDs, actionID)
			}
		}

		if len(missingActionIDs) == 0 {
			continue
		}
		if err := orm.BulkCreateRoleActionBindings(role.ID, missingActionIDs, tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to backfill business directory permissions for role %d, err: %s", role.ID, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit business directory permission backfill tx, err: %s", err)
	}
	return nil
}

// backfill global read only role
func backfillGlobalReadOnlyRole() error {
	tx := repository.DB.Begin()
	role := &usermodels.NewRole{
		Name:           "global-read-only",
		Description:    "拥有系统全局只读的权限",
		Type:           int64(setting.RoleTypeSystem),
		Namespace:      "*",
		GlobalReadOnly: true,
	}

	// Check if role already exists
	existingRole, err := orm.GetRole("global-read-only", "*", tx)
	if err == nil && existingRole != nil && existingRole.ID != 0 {
		tx.Commit()
		return nil
	}

	if err := orm.CreateRole(role, tx); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create global-read-only role in backfill, error: %s", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit tx, err: %s", err)
	}

	return nil
}

func V430ToV421() error {
	return nil
}
