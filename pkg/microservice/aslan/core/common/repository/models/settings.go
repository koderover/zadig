/*
Copyright 2022 The KodeRover Authors.

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

package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type SystemSetting struct {
	ID                  primitive.ObjectID       `bson:"_id,omitempty" json:"id,omitempty"`
	WorkflowConcurrency int64                    `bson:"workflow_concurrency" json:"workflow_concurrency"`
	BuildConcurrency    int64                    `bson:"build_concurrency" json:"build_concurrency"`
	DefaultLogin        string                   `bson:"default_login" json:"default_login"`
	Theme               *Theme                   `bson:"theme" json:"theme"`
	Security            *SecuritySettings        `bson:"security" json:"security"`
	Privacy             *PrivacySettings         `bson:"privacy"  json:"privacy"`
	Language            string                   `bson:"language" json:"language"`
	ReleasePlanHook     *ReleasePlanHookSettings `bson:"release_plan_hook" json:"release_plan_hook"`
	UpdateTime          int64                    `bson:"update_time" json:"update_time"`
}

type Theme struct {
	ThemeType   string       `bson:"theme_type" json:"theme_type"`
	CustomTheme *CustomTheme `bson:"custom_theme" json:"custom_theme"`
}

type CustomTheme struct {
	BorderGray               string `bson:"border_gray" json:"border_gray"`
	FontGray                 string `bson:"font_gray" json:"font_gray"`
	FontLightGray            string `bson:"font_light_gray" json:"font_light_gray"`
	ThemeColor               string `bson:"theme_color" json:"theme_color"`
	ThemeBorderColor         string `bson:"theme_border_color" json:"theme_border_color"`
	ThemeBackgroundColor     string `bson:"theme_background_color" json:"theme_background_color"`
	ThemeLightColor          string `bson:"theme_light_color" json:"theme_light_color"`
	BackgroundColor          string `bson:"background_color" json:"background_color"`
	GlobalBackgroundColor    string `bson:"global_background_color" json:"global_background_color"`
	Success                  string `bson:"success" json:"success"`
	Danger                   string `bson:"danger" json:"danger"`
	Warning                  string `bson:"warning" json:"warning"`
	Info                     string `bson:"info" json:"info"`
	Primary                  string `bson:"primary" json:"primary"`
	WarningLight             string `bson:"warning_light" json:"warning_light"`
	NotRunning               string `bson:"not_running" json:"not_running"`
	PrimaryColor             string `bson:"primary_color" json:"primary_color"`
	SecondaryColor           string `bson:"secondary_color" json:"secondary_color"`
	SidebarBg                string `bson:"sidebar_bg" json:"sidebar_bg"`
	SidebarActiveColor       string `bson:"sidebar_active_color" json:"sidebar_active_color"`
	ProjectItemIconColor     string `bson:"project_item_icon_color" json:"project_item_icon_color"`
	ProjectNameColor         string `bson:"project_name_color" json:"project_name_color"`
	TableCellBackgroundColor string `bson:"table_cell_background_color" json:"table_cell_background_color"`
	LinkColor                string `bson:"link_color" json:"link_color"`
}

type SecuritySettings struct {
	TokenExpirationTime int64 `json:"token_expiration_time" bson:"token_expiration_time"`
}

type PrivacySettings struct {
	ImprovementPlan bool `json:"improvement_plan" bson:"improvement_plan"`
}

type ReleasePlanHookSettings struct {
	Enable         bool                   `json:"enable" bson:"enable"`
	EnableCallBack bool                   `json:"enable_call_back" bson:"enable_call_back"`
	HookAddress    string                 `json:"hook_address" bson:"hook_address"`
	HookSecret     string                 `json:"hook_secret" bson:"hook_secret"`
	HookEvents     []ReleasePlanHookEvent `json:"hook_events" bson:"hook_events"`
}

func (r *ReleasePlanHookSettings) ToHookSettings() *HookSettings {
	return &HookSettings{
		Enable:         r.Enable,
		EnableCallBack: r.EnableCallBack,
		HookEvents:     r.HookEvents,
	}
}

type ReleasePlanHookEvent string

const (
	ReleasePlanHookEventSubmitApproval ReleasePlanHookEvent = "submit_approval"
	ReleasePlanHookEventStartExecute   ReleasePlanHookEvent = "start_execute"
	ReleasePlanHookEventAllJobDone     ReleasePlanHookEvent = "all_job_done"
)

func (SystemSetting) TableName() string {
	return "system_setting"
}
