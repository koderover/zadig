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
	"context"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func init() {
	upgradepath.RegisterHandler("3.4.0", "3.4.1", V330ToV340)
	upgradepath.RegisterHandler("3.4.1", "3.4.0", V340ToV330)
}

func V340ToV341() error {
	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	if !migrationInfo.UpdateLarkEventSetting {
		imApps, err := commonrepo.NewIMAppColl().List(context.TODO(), setting.IMLark)
		if err != nil {
			return fmt.Errorf("failed to list all custom workflow to update, error: %s", err)
		}

		for _, app := range imApps {
			app.LarkEventType = setting.LarkEventTypeCallback
			err = commonrepo.NewIMAppColl().Update(context.TODO(), app.ID.Hex(), app)
			if err != nil {
				return fmt.Errorf("failed to update lark app event type, error: %s", err)
			}
		}

		_ = mongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
			"update_lark_event_setting": true,
		})
	}

	return nil
}
