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

package types

import "github.com/koderover/zadig/v2/pkg/setting"

type UserInfo struct {
	LastLoginTime      int64          `json:"last_login_time"`
	Uid                string         `json:"uid"`
	Name               string         `json:"name"`
	IdentityType       string         `gorm:"default:'unknown'" json:"identity_type"`
	Email              string         `json:"email"`
	Phone              string         `json:"phone"`
	Account            string         `json:"account"`
	APIToken           string         `json:"token"`
	UserGroups         []*UserGroup   `json:"user_groups"`
	SystemRoleBindings []*RoleBinding `json:"system_role_bindings"`
	Admin              bool           `json:"admin"`
}

type UserGroup struct {
	Name            string   `json:"name"`
	SystemRoleNames []string `json:"system_role_names"`
}

type UserBriefInfo struct {
	UID          string `json:"uid"`
	Account      string `json:"account"`
	Name         string `json:"name"`
	IdentityType string `json:"identity_type"`
}

type UsersResp struct {
	Users      []*UserInfo `json:"users"`
	TotalCount int64       `json:"total_count"`
}

type UsersBriefResp struct {
	Users      []*UserBriefInfo `json:"users"`
	TotalCount int64            `json:"total_count"`
}

type RoleBinding struct {
	Name   string               `json:"name"`
	UID    string               `json:"uid"`
	Role   string               `json:"role"`
	Preset bool                 `json:"preset"`
	Type   setting.ResourceType `json:"type"`
}

type UserCountByType struct {
	IdentityType string `gorm:"default:'unknown'" json:"identity_type" gorm:"identity_type"`
	Count        int64  `json:"count" gorm:"count"`
}

type UserStatistics struct {
	UserByType []*UserCountByType `json:"user_info"`
	ActiveUser int64              `json:"active_user"`
	TotalUser  int64              `json:"total_user"`
}

type UserSetting struct {
	Uid          string `json:"uid"`
	Theme        string `json:"theme"`
	LogBgColor   string `json:"log_bg_color"`
	LogFontColor string `json:"log_font_color"`
}

type Identity struct {
	IdentityType string `json:"identity_type"`
	UID          string `json:"uid,omitempty"`
	GID          string `json:"gid,omitempty"`
}

func GeneSystemUserBriefInfo() UserBriefInfo {
	return UserBriefInfo{
		Name:    setting.SystemUser,
		Account: setting.SystemUser,
	}
}

func (id *Identity) GetID() string {
	if id.IdentityType == "user" {
		return id.UID
	} else {
		return id.GID
	}
}
