package types

import (
	"github.com/koderover/zadig/pkg/setting"
)

type UserInfo struct {
	LastLoginTime      int64          `json:"last_login_time"`
	Uid                string         `json:"uid"`
	Name               string         `json:"name"`
	IdentityType       string         `gorm:"default:'unknown'" json:"identity_type"`
	Email              string         `json:"email"`
	Phone              string         `json:"phone"`
	Account            string         `json:"account"`
	APIToken           string         `json:"token"`
	SystemRoleBindings []*RoleBinding `json:"system_role_bindings"`
	Admin              bool           `json:"admin"`
}

type UsersResp struct {
	Users      []UserInfo `json:"users"`
	TotalCount int64      `json:"totalCount"`
}

type RoleBinding struct {
	Name   string               `json:"name"`
	UID    string               `json:"uid"`
	Role   string               `json:"role"`
	Preset bool                 `json:"preset"`
	Type   setting.ResourceType `json:"type"`
}
