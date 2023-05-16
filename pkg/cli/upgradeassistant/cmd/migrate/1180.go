/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package migrate

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	aslanConfig "github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("1.17.0", "1.18.0", V1170ToV1180)
	upgradepath.RegisterHandler("1.18.0", "1.17.0", V1180ToV1170)
}

func V1170ToV1180() error {
	if err := migrateJiraAuthType(); err != nil {
		log.Errorf("migrateJiraAuthType err: %v", err)
	}
	if err := migrateSystemTheme(); err != nil {
		log.Errorf("migrateSystemTheme err: %v", err)
	}
	return nil
}

func V1180ToV1170() error {
	return nil
}

func migrateJiraAuthType() error {
	if jira, err := mongodb.NewProjectManagementColl().GetJira(); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		return errors.Wrap(err, "get jira")
	} else {
		if jira.JiraAuthType != "" {
			log.Warnf("migrateJiraAuthType: find jira auth type %s, skip", jira.JiraAuthType)
			return nil
		}
		jira.JiraAuthType = config.JiraBasicAuth
		if err := mongodb.NewProjectManagementColl().UpdateByID(jira.ID.Hex(), jira); err != nil {
			return errors.Wrap(err, "update")
		}
	}
	return nil
}

func migrateSystemTheme() error {
	mdb := mongodb.NewSystemSettingColl()
	if systemSetting, err := mdb.Get(); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		return errors.Wrap(err, "get system setting")
	} else {
		if systemSetting.Theme != nil {
			return nil
		}
		theme := &models.Theme{
			ThemeType: aslanConfig.CUSTOME_THEME,
			CustomTheme: &models.CustomTheme{
				BorderGray:               "#d2d7dc",
				FontGray:                 "#888888",
				FontLightGray:            "#a0a0a0",
				ThemeColor:               "#0066ff",
				ThemeBorderColor:         "#66bbff",
				ThemeBackgroundColor:     "#eeeeff",
				ThemeLightColor:          "#66bbff",
				BackgroundColor:          "#e5e5e5",
				GlobalBackgroundColor:    "#f6f6f6",
				Success:                  "#67c23a",
				Danger:                   "#f56c6c",
				Warning:                  "#e6a23c",
				Info:                     "#909399",
				Primary:                  "#0066ff",
				WarningLight:             "#cdb62c",
				NotRunning:               "#303133",
				PrimaryColor:             "#000",
				SecondaryColor:           "#888888",
				SidebarBg:                "#f5f7fa",
				SidebarActiveColor:       "#0066ff12",
				ProjectItemIconColor:     "#0066ff",
				ProjectNameColor:         "#121212",
				TableCellBackgroundColor: "#eaeaea",
			},
		}
		err := mdb.UpdateTheme(theme)
		if err != nil {
			return err
		}
	}
	return nil
}
