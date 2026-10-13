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
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/config"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	connectormodels "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/repository/models"
	gormtool "github.com/koderover/zadig/v2/pkg/tool/gorm"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"gorm.io/gorm"
)

func init() {
	upgradepath.RegisterHandler("5.0.0", "5.1.0", V500ToV510)
	upgradepath.RegisterHandler("5.1.0", "5.0.0", V510ToV500)
}

func V500ToV510() error {
	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}
	defer func() {
		updateMigrationError(migrationInfo.ID, err)
	}()

	err = migrateLarkPreferredUsernameClaim510(migrationInfo)
	if err != nil {
		return err
	}

	err = migrateDockerBuildProxy510(migrationInfo)
	return err
}

func migrateLarkPreferredUsernameClaim510(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration510LarkPreferredUsernameClaim {
		return nil
	}
	if err := gormtool.Open(config.MysqlUser(), config.MysqlPassword(), config.MysqlHost(), config.MysqlDexDB()); err != nil {
		return fmt.Errorf("failed to open dex database, err: %s", err)
	}

	err := gormtool.DB(config.MysqlDexDB()).Transaction(func(tx *gorm.DB) error {
		var connectors []*connectormodels.Connector
		if err := tx.Where("type IN ?", []string{"lark", "lark_intl"}).Find(&connectors).Error; err != nil {
			return fmt.Errorf("failed to list lark connectors, err: %s", err)
		}
		for _, connector := range connectors {
			config, err := migrateLarkConnectorConfig510(connector.Config)
			if err != nil {
				return fmt.Errorf("failed to migrate lark connector %s config, err: %s", connector.ID, err)
			}
			connector.IncreaseResourceVersion()
			if err := tx.Model(connector).Updates(map[string]interface{}{
				"config":           config,
				"resource_version": connector.ResourceVersion,
			}).Error; err != nil {
				return fmt.Errorf("failed to update lark connector %s, err: %s", connector.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration510LarkPreferredUsernameClaim): true,
	})
}

func migrateLarkConnectorConfig510(config string) (string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(config), &fields); err != nil {
		return "", err
	}
	var emailClaim *string
	if json.Unmarshal(fields["emailClaim"], &emailClaim) != nil || emailClaim == nil {
		return "", fmt.Errorf("emailClaim must be a string")
	}
	fields["preferredUsernameClaim"] = fields["emailClaim"]
	updated, err := json.Marshal(fields)
	return string(updated), err
}

func V510ToV500() error {
	return nil
}

// migrateDockerBuildProxy510 turns on the new per-build proxy switch for existing docker build steps.
// The proxy is injected only when both the build switch and the system proxy are on, so these steps
// keep following the system proxy as they did before 5.1.0.
func migrateDockerBuildProxy510(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration510DockerBuildProxy {
		return nil
	}

	filter := bson.M{
		"post_build.docker_build":              bson.M{"$ne": nil},
		"post_build.docker_build.enable_proxy": bson.M{"$exists": false},
	}
	update := bson.M{"$set": bson.M{"post_build.docker_build.enable_proxy": true}}
	for _, coll := range []*mongo.Collection{
		commonrepo.NewBuildColl().Collection,
		commonrepo.NewBuildTemplateColl().Collection,
	} {
		res, err := coll.UpdateMany(context.Background(), filter, update)
		if err != nil {
			return fmt.Errorf("failed to set docker build proxy switch in %s, err: %s", coll.Name(), err)
		}
		log.Infof("migration 5.1.0: enabled proxy switch for %d docker build steps in %s", res.ModifiedCount, coll.Name())
	}

	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration510DockerBuildProxy): true,
	})
}
