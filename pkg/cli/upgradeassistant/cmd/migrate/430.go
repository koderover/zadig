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
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

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

	// 写入 globalreadonly role
	err := backfillGlobalReadOnlyRole()
	if err != nil {
		return err
	}

	_ = internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration430GlobalReadOnlyRole): true,
	})

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
