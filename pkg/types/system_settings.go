package types

type Theme struct {
	ThemeType   string       `json:"theme_type"`
	CustomTheme *CustomTheme `json:"custom_theme"`
}

type CustomTheme struct {
	BorderGray               string `json:"border_gray"`
	FontGray                 string `json:"font_gray"`
	FontLightGray            string `json:"font_light_gray"`
	ThemeColor               string `json:"theme_color"`
	ThemeBorderColor         string `json:"theme_border_color"`
	ThemeBackgroundColor     string `json:"theme_background_color"`
	ThemeLightColor          string `json:"theme_light_color"`
	BackgroundColor          string `json:"background_color"`
	GlobalBackgroundColor    string `json:"global_background_color"`
	Success                  string `json:"success"`
	Danger                   string `json:"danger"`
	Warning                  string `json:"warning"`
	Info                     string `json:"info"`
	Primary                  string `json:"primary"`
	WarningLight             string `json:"warning_light"`
	NotRunning               string `json:"not_running"`
	PrimaryColor             string `json:"primary_color"`
	SecondaryColor           string `json:"secondary_color"`
	SidebarBg                string `json:"sidebar_bg"`
	SidebarActiveColor       string `json:"sidebar_active_color"`
	ProjectItemIconColor     string `json:"project_item_icon_color"`
	ProjectNameColor         string `json:"project_name_color"`
	TableCellBackgroundColor string `json:"table_cell_background_color"`
}
