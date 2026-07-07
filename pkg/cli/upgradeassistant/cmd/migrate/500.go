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

	"go.mongodb.org/mongo-driver/mongo"

	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
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
// 历史实例升级时，只保证 action 数据存在，不自动给现有自定义角色放权。
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

	err = migrateUserContactIndexes500(migrationInfo)
	if err != nil {
		return err
	}

	err = migrateWorkflowTemplateVersion500(migrationInfo)
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

	for _, seed := range permissionActionSeeds500 {
		if _, err := ensureAction500(tx, seed); err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit migration 5.0.0 permissions, err: %s", err)
	}

	log.Infof("migration 5.0.0 ensured log operation permission actions")

	if alreadyMigrated {
		return nil
	}

	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration500LogOperationPermission): true,
	})
}

// migrateUserContactIndexes500 adds indexes for dynamic notification recipient lookups.
// The index names are bound to User.Email/User.Phone through their GORM tags.
func migrateUserContactIndexes500(migrationInfo *internalmodels.Migration) error {
	if !migrationInfo.Migration500UserContactIndexes {
		if !repository.DB.Migrator().HasIndex(&usermodels.User{}, "idx_email") {
			if err := repository.DB.Migrator().CreateIndex(&usermodels.User{}, "idx_email"); err != nil {
				return fmt.Errorf("failed to add idx_email index for user table, err: %s", err)
			}
		}

		if !repository.DB.Migrator().HasIndex(&usermodels.User{}, "idx_phone") {
			if err := repository.DB.Migrator().CreateIndex(&usermodels.User{}, "idx_phone"); err != nil {
				return fmt.Errorf("failed to add idx_phone index for user table, err: %s", err)
			}
		}
	}

	if err := internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration500UserContactIndexes): true,
	}); err != nil {
		return fmt.Errorf("failed to update migration 5.0.0 user contact indexes status, err: %s", err)
	}

	return nil
}

func migrateWorkflowTemplateVersion500(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration500WorkflowTemplateVersion {
		return nil
	}

	versionColl := commonrepo.NewWorkflowV4TemplateVersionColl()
	if err := versionColl.EnsureIndex(context.Background()); err != nil {
		return fmt.Errorf("failed to ensure workflow template version indexes, err: %s", err)
	}

	cursor, err := commonrepo.NewWorkflowV4TemplateColl().ListByCursor(&commonrepo.ListWorkflowV4TemplateOption{})
	if err != nil {
		return fmt.Errorf("failed to list workflow templates, err: %s", err)
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		template := new(commonmodels.WorkflowV4Template)
		if err := cursor.Decode(template); err != nil {
			return fmt.Errorf("failed to decode workflow template, err: %s", err)
		}

		latest, err := versionColl.GetLatest(template.ID.Hex())
		if err == mongo.ErrNoDocuments {
			user := template.UpdatedBy
			if user == "" {
				user = template.CreatedBy
			}
			latest, err = versionColl.CreateNext(template, user)
			if err != nil {
				return fmt.Errorf("failed to create workflow template version for template %s, err: %s", template.TemplateName, err)
			}
			log.Infof("created initial workflow template version for template: %s", template.TemplateName)
		} else if err != nil {
			return fmt.Errorf("failed to get latest workflow template version for template %s, err: %s", template.TemplateName, err)
		}

		if template.LatestVersion != latest.Version || template.LatestVersionID != latest.ID.Hex() {
			if err := commonrepo.NewWorkflowV4TemplateColl().UpdateVersionInfo(template.ID, latest.Version, latest.ID.Hex()); err != nil {
				return fmt.Errorf("failed to update workflow template version info for template %s, err: %s", template.TemplateName, err)
			}
		}
	}

	if err := cursor.Err(); err != nil {
		return fmt.Errorf("workflow template cursor error, err: %s", err)
	}

	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration500WorkflowTemplateVersion): true,
	})
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

func V500ToV430() error {
	return nil
}
