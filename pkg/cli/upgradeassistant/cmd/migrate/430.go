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
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func init() {
	upgradepath.RegisterHandler("4.2.0", "4.3.0", V420ToV430)
	upgradepath.RegisterHandler("4.3.0", "4.2.0", V430ToV420)
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

func V430ToV420() error {
	return nil
}
