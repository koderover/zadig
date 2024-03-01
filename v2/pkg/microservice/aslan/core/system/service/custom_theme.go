/*
Copyright 2023 The KodeRover Authors.

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
package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/types"
)

func GetThemeInfos(log *zap.SugaredLogger) (*types.Theme, error) {
	setting, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		log.Errorf("get theme infos error: %v", err)
		return nil, err
	}

	theme := setting.Theme
	if theme == nil {
		return &types.Theme{}, nil
	}
	if theme.ThemeType != config.CUSTOME_THEME {
		return &types.Theme{ThemeType: theme.ThemeType}, nil
	}

	return convertThemeToRsp(theme), nil
}

func UpdateThemeInfo(theme *models.Theme, log *zap.SugaredLogger) error {
	return commonrepo.NewSystemSettingColl().UpdateTheme(theme)
}

func convertThemeToRsp(m *models.Theme) *types.Theme {
	return &types.Theme{
		ThemeType: m.ThemeType,
		CustomTheme: &types.CustomTheme{
			BorderGray:               m.CustomTheme.BorderGray,
			FontGray:                 m.CustomTheme.FontGray,
			FontLightGray:            m.CustomTheme.FontLightGray,
			ThemeColor:               m.CustomTheme.ThemeColor,
			ThemeBorderColor:         m.CustomTheme.ThemeBorderColor,
			ThemeBackgroundColor:     m.CustomTheme.ThemeBackgroundColor,
			ThemeLightColor:          m.CustomTheme.ThemeLightColor,
			BackgroundColor:          m.CustomTheme.BackgroundColor,
			GlobalBackgroundColor:    m.CustomTheme.GlobalBackgroundColor,
			Success:                  m.CustomTheme.Success,
			Danger:                   m.CustomTheme.Danger,
			Warning:                  m.CustomTheme.Warning,
			Info:                     m.CustomTheme.Info,
			Primary:                  m.CustomTheme.Primary,
			WarningLight:             m.CustomTheme.WarningLight,
			NotRunning:               m.CustomTheme.NotRunning,
			PrimaryColor:             m.CustomTheme.PrimaryColor,
			SecondaryColor:           m.CustomTheme.SecondaryColor,
			SidebarBg:                m.CustomTheme.SidebarBg,
			SidebarActiveColor:       m.CustomTheme.SidebarActiveColor,
			ProjectItemIconColor:     m.CustomTheme.ProjectItemIconColor,
			ProjectNameColor:         m.CustomTheme.ProjectNameColor,
			TableCellBackgroundColor: m.CustomTheme.TableCellBackgroundColor,
			LinkColor:                m.CustomTheme.LinkColor,
		},
	}
}
