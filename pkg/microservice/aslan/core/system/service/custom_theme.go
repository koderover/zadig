package service

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/types"
	"go.uber.org/zap"
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
		},
	}
}
