package migrate

import (
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	internaldb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

// getMigrationInfo get the current migration status from the mongodb, if none exists, initialize one and return the
// initialized data.
// NOTE THAT THE INITIALIZATION FUNCTION NEEDS TO BE UPDATED TO AVOID DATA CORRUPTION
func getMigrationInfo() (*internalmodels.Migration, error) {
	migrationInfo, err := internaldb.NewMigrationColl().GetMigrationInfo()
	if err != nil {
		if err != mongo.ErrNoDocuments {
			return nil, fmt.Errorf("failed to get migration info from db, err: %s", err)
		} else {
			err := internaldb.NewMigrationColl().InitializeMigrationInfo()
			if err != nil {
				return nil, fmt.Errorf("failed to create migration info in db, err: %s", err)
			}
			createdInfo, err := internaldb.NewMigrationColl().GetMigrationInfo()
			if err != nil {
				return nil, fmt.Errorf("failed to get migration info from db, err: %s", err)
			}
			return createdInfo, nil
		}
	}
	return migrationInfo, nil
}

func getMigrationFieldBsonTag(migrationInst *internalmodels.Migration, fieldPtr interface{}) string {
	val := reflect.ValueOf(migrationInst).Elem()
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if field.Addr().Interface() == fieldPtr {
			return typ.Field(i).Tag.Get("bson")
		}
	}
	return ""
}

func updateMigrationError(migrationID primitive.ObjectID, err error) {
	if err != nil {
		err = mongodb.NewMigrationColl().UpdateMigrationError(migrationID, err.Error())
		if err != nil {
			log.Errorf("failed to update migration error: %s", err)
		}
	} else {
		err = mongodb.NewMigrationColl().UpdateMigrationError(migrationID, "")
		if err != nil {
			log.Errorf("failed to update migration error: %s", err)
		}
	}
}
