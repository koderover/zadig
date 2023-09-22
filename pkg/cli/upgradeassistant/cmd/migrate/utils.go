package migrate

import (
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"

	internalmodels "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	internaldb "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/mongodb"
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
			return internaldb.NewMigrationColl().InitializeMigrationInfo()
		}
	}
	return migrationInfo, nil
}
