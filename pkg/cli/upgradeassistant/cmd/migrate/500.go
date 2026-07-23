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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	servicerepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	userrepo "github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	usermodels "github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	userorm "github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	permissionservice "github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	pkgtypes "github.com/koderover/zadig/v2/pkg/types"
	"gorm.io/gorm"
)

const migration500ProgressEvery = 200

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

// legacyServiceForMigration500 is a local-to-this-migration view of a service
// template document. We deliberately do NOT reuse commonmodels.Service here:
// that struct now has bson:"-" on Containers (5.0.0 deprecation), so a normal
// decode would silently drop the legacy "containers" field that pre-5.0.0
// documents carry - which is exactly the field this migration needs to read
// to backfill service_module.
//
// Only the fields SyncAutoServiceModules touches are declared. Everything
// else in the legacy document is ignored.
type legacyServiceForMigration500 struct {
	ServiceName string                    `bson:"service_name"`
	ProductName string                    `bson:"product_name"`
	Revision    int64                     `bson:"revision"`
	Type        string                    `bson:"type"`
	Containers  []*commonmodels.Container `bson:"containers,omitempty"`
}

func init() {
	upgradepath.RegisterHandler("4.3.0", "5.0.0", V430ToV500)
	upgradepath.RegisterHandler("5.0.0", "4.3.0", V500ToV430)
}

// V430ToV500 executes 5.0.0 upgrade steps for permission metadata and
// service_module backfill.
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

	err = migrateServiceModule500(migrationInfo)
	if err != nil {
		return err
	}

	return nil
}

func migrateLogOperationPermission500(migrationInfo *internalmodels.Migration) error {
	alreadyMigrated := migrationInfo.Migration500LogOperationPermission

	tx := userrepo.DB.Begin()
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

// migrateServiceModule500 walks both template_service and
// production_template_service collections, mirroring each Service's
// Containers slice into service_module / production_service_module as
// auto records bound to the corresponding revision.
func migrateServiceModule500(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration500ServiceModule {
		log.Infof("migration 5.0.0: service_module backfill already completed, skipping")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testCount, testSkipped, testErrors, err := backfillServiceModulesForCollection500(ctx, commonrepo.NewServiceColl().Collection, "template_service", false)
	if err != nil {
		return fmt.Errorf("failed to backfill service modules from template_service, err: %s", err)
	}

	prodCount, prodSkipped, prodErrors, err := backfillServiceModulesForCollection500(ctx, commonrepo.NewProductionServiceColl().Collection, "production_template_service", true)
	if err != nil {
		return fmt.Errorf("failed to backfill service modules from production_template_service, err: %s", err)
	}

	allErrors := make([]string, 0, len(testErrors)+len(prodErrors))
	allErrors = append(allErrors, testErrors...)
	allErrors = append(allErrors, prodErrors...)

	log.Infof("migration 5.0.0: backfilled %d test + %d production service revisions into service_module", testCount, prodCount)

	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration500ServiceModule):        true,
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration500ServiceModuleSkipped): testSkipped + prodSkipped,
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration500ServiceModuleErrors):  allErrors,
	})
}

// backfillServiceModulesForCollection500 streams every document in the given
// service-template collection and mirrors its Containers slice into the new
// service_module collection (production-side picked by `production`).
func backfillServiceModulesForCollection500(ctx context.Context, coll *mongo.Collection, label string, production bool) (int, int, []string, error) {
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to open cursor over %s: %s", label, err)
	}
	defer cursor.Close(ctx)

	migrated := 0
	skipped := 0
	errors := make([]string, 0)
	for cursor.Next(ctx) {
		// Decode into the local legacy view (see legacyServiceForMigration500
		// above) - commonmodels.Service has bson:"-" on Containers and would
		// silently drop the legacy field on decode.
		var legacy legacyServiceForMigration500
		if decodeErr := cursor.Decode(&legacy); decodeErr != nil {
			message := fmt.Sprintf("failed to decode %s document: %s", label, decodeErr)
			log.Warnf("migration 5.0.0: %s, skipping", message)
			skipped++
			errors = append(errors, message)
			continue
		}
		svc := &commonmodels.Service{
			ServiceName: legacy.ServiceName,
			ProductName: legacy.ProductName,
			Revision:    legacy.Revision,
			Type:        legacy.Type,
			Containers:  legacy.Containers,
		}
		// SyncAutoServiceModules tolerates empty Containers (no-op) and
		// validates required fields itself. Errors here are logged but not
		// fatal - one corrupt service shouldn't block the rest.
		if syncErr := servicerepo.SyncAutoServiceModules(ctx, svc, production); syncErr != nil {
			message := fmt.Sprintf("failed to sync %s %s/%s rev %d: %s",
				label, svc.ProductName, svc.ServiceName, svc.Revision, syncErr)
			log.Warnf("migration 5.0.0: %s", message)
			skipped++
			errors = append(errors, message)
			continue
		}
		migrated++
		if migrated%migration500ProgressEvery == 0 {
			log.Infof("migration 5.0.0: %s progress - %d revisions mirrored, %d skipped", label, migrated, skipped)
		}
	}
	if err := cursor.Err(); err != nil {
		return migrated, skipped, errors, fmt.Errorf("cursor over %s ended in error: %s", label, err)
	}
	if skipped > 0 {
		log.Warnf("migration 5.0.0: %s complete - %d mirrored, %d skipped (inspect migration table or warn logs)", label, migrated, skipped)
	}
	return migrated, skipped, errors, nil
}
