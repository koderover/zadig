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
	"encoding/json"
	"fmt"

	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/config"
	connectormodels "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/repository/models"
	gormtool "github.com/koderover/zadig/v2/pkg/tool/gorm"
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
