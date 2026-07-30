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

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	servicerepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
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

// legacyServiceForMigration500 is a local view of a pre-5.0.0 service
// template document. commonmodels.Service cannot be used for decoding here
// because its Containers field is no longer persisted to MongoDB.
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

// V430ToV500 executes 5.0.0 upgrade steps.
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

	err = migrateServiceModule500(migrationInfo)
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

	templateColl := commonrepo.NewWorkflowV4TemplateColl()
	versionColl := commonrepo.NewWorkflowV4TemplateVersionColl()
	if err := versionColl.EnsureIndex(context.Background()); err != nil {
		return fmt.Errorf("failed to ensure workflow template version indexes, err: %s", err)
	}

	cursor, err := templateColl.ListByCursor(&commonrepo.ListWorkflowV4TemplateOption{})
	if err != nil {
		return fmt.Errorf("failed to list workflow templates, err: %s", err)
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		template := new(commonmodels.WorkflowV4Template)
		if err := cursor.Decode(template); err != nil {
			return fmt.Errorf("failed to decode workflow template, err: %s", err)
		}

		if backfillWorkflowTemplateEntityIDs500(template) {
			if _, err := templateColl.UpdateOne(context.Background(), bson.M{"_id": template.ID}, bson.M{"$set": bson.M{
				"stages": template.Stages,
			}}); err != nil {
				return fmt.Errorf("failed to backfill stage/job ids for workflow template %s, err: %s", template.TemplateName, err)
			}
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

func backfillWorkflowTemplateEntityIDs500(template *commonmodels.WorkflowV4Template) bool {
	if template == nil {
		return false
	}

	changed := false
	usedStageIDs := make(map[string]struct{})
	usedJobIDs := make(map[string]struct{})
	for _, stage := range template.Stages {
		if stage == nil {
			continue
		}
		if stage.ID == "" {
			stage.ID = uuid.NewString()
			changed = true
		}
		if _, duplicated := usedStageIDs[stage.ID]; duplicated {
			stage.ID = uuid.NewString()
			changed = true
		}
		usedStageIDs[stage.ID] = struct{}{}

		for _, job := range stage.Jobs {
			if job == nil {
				continue
			}
			if job.ID == "" {
				job.ID = uuid.NewString()
				changed = true
			}
			if _, duplicated := usedJobIDs[job.ID]; duplicated {
				job.ID = uuid.NewString()
				changed = true
			}
			usedJobIDs[job.ID] = struct{}{}
		}
	}
	return changed
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

// migrateServiceModule500 mirrors legacy service containers into the
// service_module collections. The migration is idempotent and can safely be
// retried after a partial run.
func migrateServiceModule500(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration500ServiceModule {
		log.Infof("migration 5.0.0: service_module backfill already completed, skipping")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testCount, testSkipped, testErrors, err := backfillServiceModulesForCollection500(
		ctx,
		commonrepo.NewServiceColl().Collection,
		"template_service",
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to backfill service modules from template_service, err: %s", err)
	}

	prodCount, prodSkipped, prodErrors, err := backfillServiceModulesForCollection500(
		ctx,
		commonrepo.NewProductionServiceColl().Collection,
		"production_template_service",
		true,
	)
	if err != nil {
		return fmt.Errorf("failed to backfill service modules from production_template_service, err: %s", err)
	}

	allErrors := make([]string, 0, len(testErrors)+len(prodErrors))
	allErrors = append(allErrors, testErrors...)
	allErrors = append(allErrors, prodErrors...)

	log.Infof(
		"migration 5.0.0: backfilled %d test + %d production service revisions into service_module",
		testCount,
		prodCount,
	)

	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration500ServiceModule):        true,
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration500ServiceModuleSkipped): testSkipped + prodSkipped,
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration500ServiceModuleErrors):  allErrors,
	})
}

// backfillServiceModulesForCollection500 streams all revisions in one legacy
// service collection. Individual malformed records are retained in the
// migration diagnostics without blocking the remaining records.
func backfillServiceModulesForCollection500(
	ctx context.Context,
	coll *mongo.Collection,
	label string,
	production bool,
) (int, int, []string, error) {
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to open cursor over %s: %s", label, err)
	}
	defer cursor.Close(ctx)

	migrated := 0
	skipped := 0
	errors := make([]string, 0)
	for cursor.Next(ctx) {
		var legacy legacyServiceForMigration500
		if err := cursor.Decode(&legacy); err != nil {
			message := fmt.Sprintf("failed to decode %s document: %s", label, err)
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
		if err := servicerepo.SyncAutoServiceModules(ctx, svc, production); err != nil {
			message := fmt.Sprintf(
				"failed to sync %s %s/%s rev %d: %s",
				label,
				svc.ProductName,
				svc.ServiceName,
				svc.Revision,
				err,
			)
			log.Warnf("migration 5.0.0: %s", message)
			skipped++
			errors = append(errors, message)
			continue
		}

		migrated++
		if migrated%migration500ProgressEvery == 0 {
			log.Infof(
				"migration 5.0.0: %s progress - %d revisions mirrored, %d skipped",
				label,
				migrated,
				skipped,
			)
		}
	}

	if err := cursor.Err(); err != nil {
		return migrated, skipped, errors, fmt.Errorf("cursor over %s ended in error: %s", label, err)
	}
	if skipped > 0 {
		log.Warnf(
			"migration 5.0.0: %s complete - %d mirrored, %d skipped (inspect migration table or warn logs)",
			label,
			migrated,
			skipped,
		)
	}
	return migrated, skipped, errors, nil
}
