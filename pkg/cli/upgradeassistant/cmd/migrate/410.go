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
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func init() {
	upgradepath.RegisterHandler("4.0.0", "4.1.0", V400ToV410)
	upgradepath.RegisterHandler("4.1.0", "4.0.0", V400ToV410)
}

func V400ToV410() error {
	ctx := internalhandler.NewBackgroupContext()

	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	defer func() {
		updateMigrationError(migrationInfo.ID, err)
	}()

	err = migrateProjectReleaseMaxHistory(ctx, migrationInfo)
	if err != nil {
		return err
	}

	return nil
}

func migrateProjectReleaseMaxHistory(ctx *internalhandler.Context, migrationInfo *internalmodels.Migration) error {
	if !migrationInfo.Migration400ProjectReleaseMaxHistory {
		projects, err := templaterepo.NewProductColl().List()
		if err != nil {
			return fmt.Errorf("failed to list projects, err: %s", err)
		}
		for _, project := range projects {
			if project.IsHelmProduct() {
				project.ReleaseMaxHistory = 10
				err = templaterepo.NewProductColl().Update(project.ProductName, project)
				if err != nil {
					return fmt.Errorf("failed to update project %s, err: %s", project.ProductName, err)
				}
			}
		}
	}

	_ = internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration400ProjectManagement): true,
	})

	return nil
}

func V410ToV400() error {
	return nil
}
