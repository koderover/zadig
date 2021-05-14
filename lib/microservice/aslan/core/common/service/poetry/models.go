/*
Copyright 2021 The KodeRover Authors.

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

package poetry

import (
	"github.com/koderover/zadig/lib/types/permission"
)

type Team interface {
	GetName() string
	GetDesc() string
}

type User interface {
	GetName() string
	GetEmail() string
	GetPhone() string
}

type UserInfo struct {
	ID             int        `json:"id"`
	Name           string     `json:"name"`
	Email          string     `json:"email"`
	Password       string     `json:"password"`
	Phone          string     `json:"phone"`
	IsAdmin        bool       `json:"isAdmin"`
	IsSuperUser    bool       `json:"isSuperUser"`
	IsTeamLeader   bool       `json:"isTeamLeader"`
	OrganizationID int        `json:"organization_id"`
	Directory      string     `json:"directory"`
	LastLoginAt    int64      `json:"lastLogin"`
	CreatedAt      int64      `json:"created_at"`
	UpdatedAt      int64      `json:"updated_at"`
	Teams          []TeamInfo `json:"teams"`
}

func ConvertUserInfo(userInfo *UserInfo) *permission.User {
	if userInfo == nil {
		return nil
	}
	user := new(permission.User)
	user.ID = userInfo.ID
	user.Name = userInfo.Name
	user.Email = userInfo.Email
	user.Password = userInfo.Password
	user.Phone = userInfo.Phone
	user.IsAdmin = userInfo.IsAdmin
	user.IsSuperUser = userInfo.IsSuperUser
	user.IsTeamLeader = userInfo.IsTeamLeader
	user.OrganizationID = userInfo.OrganizationID
	user.Directory = userInfo.Directory
	user.LastLoginAt = userInfo.LastLoginAt
	user.CreatedAt = userInfo.CreatedAt
	user.UpdatedAt = userInfo.UpdatedAt
	return user
}

type TeamInfo struct {
	ID           int     `json:"id"`
	OrgID        int     `json:"orgId"`
	Name         string  `json:"name"`
	Desc         string  `json:"desc"`
	IsTeamLeader bool    `json:"isTeamLeader"`
	Users        []*User `json:"leaders"`
	CreatedAt    int64   `json:"created_at"`
	UpdatedAt    int64   `json:"updated_at"`
}

type Organization struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	Website   string `json:"website"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type UserViewReponseModel struct {
	User         *UserInfo     `json:"info"`
	Teams        []*TeamInfo   `json:"teams"`
	Organization *Organization `json:"organization"`
}

type ResponseMessage struct {
	ResultCode int    `json:"resultCode"`
	ErrorMsg   string `json:"errorMsg"`
}

type EnvRolePermission struct {
	ID             int64  `json:"id"`
	ProductName    string `json:"productName"`
	EnvName        string `json:"envName"`
	RoleID         int64  `json:"roleId"`
	PermissionUUID string `json:"permissionUUID"`
	RoleName       string `json:"roleName"`
}

const (
	ProjectOwner = 3
)
