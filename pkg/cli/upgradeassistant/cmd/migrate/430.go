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
)

func init() {
	upgradepath.RegisterHandler("4.2.0", "4.3.0", V420ToV430)
	upgradepath.RegisterHandler("4.2.1", "4.3.0", V420ToV430)
	upgradepath.RegisterHandler("4.3.0", "4.2.0", V430ToV420)
	upgradepath.RegisterHandler("4.3.0", "4.2.1", V430ToV420)
}

func V420ToV430() error {
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

func migrateGlobalReadOnlyRole(_ *internalhandler.Context, migrationInfo *internalmodels.Migration) error {
	if !migrationInfo.Migration430GlobalReadOnlyRole {
		if !repository.DB.Migrator().HasColumn(&usermodels.NewRole{}, "GlobalReadOnly") {
			if err := repository.DB.Migrator().AddColumn(&usermodels.NewRole{}, "GlobalReadOnly"); err != nil {
				return fmt.Errorf("failed to add global_read_only column for role table, err: %s", err)
			}
		}

		tx := repository.DB.Begin()

		role, err := orm.GetRole("global-read-only", "*", tx)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to query global-read-only role, err: %s", err)
		}

		if role == nil || role.ID == 0 {
			role = &usermodels.NewRole{
				Name:           "global-read-only",
				Description:    "拥有所有项目中只读资源的权限",
				Type:           int64(setting.RoleTypeSystem),
				Namespace:      "*",
				GlobalReadOnly: true,
			}
			if err := orm.CreateRole(role, tx); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create global-read-only role, err: %s", err)
			}
		} else if !role.GlobalReadOnly {
			if err := orm.UpdateRoleInfo(role.ID, &usermodels.NewRole{GlobalReadOnly: true}, tx); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to update global-read-only role flag, err: %s", err)
			}
		}

		readOnlyAction := []string{
			permissionservice.VerbGetDelivery,
			permissionservice.VerbGetTest,
			permissionservice.VerbGetService,
			permissionservice.VerbGetProductionService,
			permissionservice.VerbGetBuild,
			permissionservice.VerbGetWorkflow,
			permissionservice.VerbGetEnvironment,
			permissionservice.VerbGetProductionEnv,
			permissionservice.VerbGetScan,
			permissionservice.VerbGetSprint,
		}
		actionIDList := make([]uint, 0, len(readOnlyAction))
		for _, verb := range readOnlyAction {
			action, err := orm.GetActionByVerb(verb, tx)
			if err != nil || action.ID == 0 {
				tx.Rollback()
				return fmt.Errorf("failed to find action %s for global-read-only role, err: %s", verb, err)
			}
			actionIDList = append(actionIDList, action.ID)
		}

		if err := orm.DeleteRoleActionBindingByRole(role.ID, tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to clear role-action bindings for global-read-only role, err: %s", err)
		}
		if err := orm.BulkCreateRoleActionBindings(role.ID, actionIDList, tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create role-action bindings for global-read-only role, err: %s", err)
		}

		tx.Commit()
	}

	_ = internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration430GlobalReadOnlyRole): true,
	})

	return nil
}

func V430ToV420() error {
	return nil
}
